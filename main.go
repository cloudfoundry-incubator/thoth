package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	cf_lager "github.com/cloudfoundry-incubator/cf-lager"
	"github.com/cloudfoundry-incubator/thoth/assistant"
	"github.com/cloudfoundry-incubator/thoth/benchmark"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/pivotal-golang/lager"
	"github.com/tedsuo/ifrit"
	"github.com/tedsuo/ifrit/grouper"
	"github.com/tedsuo/ifrit/sigmon"
)

var (
	systemDomain      = os.Getenv("CF_SYSTEM_DOMAIN")
	username          = os.Getenv("CF_USERNAME")
	password          = os.Getenv("CF_PASSWORD")
	org               = os.Getenv("CF_ORG")
	space             = os.Getenv("CF_SPACE")
	skipSSLValidation = os.Getenv("CF_SKIP_SSL_VALIDATION") == "true"
	appName           = os.Getenv("CF_APP_NAME")
	deploymentName    = os.Getenv("CF_DEPLOYMENT_NAME")
	threadsString     = os.Getenv("THOTH_THREADS")

	dogURL = "https://app.datadoghq.com/api/v1/series?api_key=" + os.Getenv("DATADOG_API_KEY")

	logger     lager.Logger
	threads    int
	tokenMutex = sync.Mutex{}
	token      string

	appGuid, appUrl, dopplerAddress string
	cfAssistant                     *assistant.Assistant
)

type Clock struct {
	currentTime time.Time
}

func NewClock() *Clock {
	return &Clock{}
}

func (fc *Clock) Now() time.Time {
	return time.Now()
}

func (fc *Clock) Since(t time.Time) time.Duration {
	return time.Since(t)
}

func main() {
	cf_lager.AddFlags(flag.CommandLine)
	flag.Parse()
	logger, _ = cf_lager.New("thoth")

	threads, err := strconv.Atoi(threadsString)
	if err != nil {
		threads = 1
	}
	logger.Info("starting", lager.Data{"threads": threads})

	apiUrl := "api." + systemDomain
	cfAssistant = assistant.NewAssistant(apiUrl, username, password, org, space, skipSSLValidation)
	cfAssistant.GetOauthToken()

	appGuid, err = cfAssistant.AppGuid(appName)
	if err != nil {
		logger.Fatal("app-guid", err)
	}

	hostname := cfAssistant.AppUrl(appName)
	if hostname == "" {
		logger.Fatal("app-url", errors.New("Could not find app hostname."))
	}

	appUrl = "http://" + hostname
	dopplerAddress = "wss://doppler." + systemDomain + ":4443"
	refreshToken(cfAssistant)

	members := grouper.Members{}
	for i := 0; i < threads; i++ {
		member := grouper.Member{"measure-" + strconv.Itoa(i), &measurer{i}}
		members = append(members, member)
	}
	group := grouper.NewParallel(os.Interrupt, members)

	monitor := ifrit.Invoke(sigmon.New(group))

	logger.Info("started")

	err = <-monitor.Wait()
	if err != nil {
		logger.Error("exited-with-failure", err)
		os.Exit(1)
	}

	logger.Info("exited")
}

type measurer struct {
	index int
}

func (m *measurer) Run(signals <-chan os.Signal, ready chan<- struct{}) error {
	log := logger.Session("measurer-" + strconv.Itoa(m.index))

	log.Info("streaming-logs")
	channel, errorChan := connectToFirehose(cfAssistant, dopplerAddress, appGuid)
	close(ready)
	log.Info("ready")

	clock := NewClock()
	ticker := time.Tick(5 * time.Second)
	for {
		select {
		case <-ticker:
			log.Info("tick")
			br, err := benchmark.NewBenchmarkRequest(appUrl, channel, clock, 2*time.Second)
			if err != nil {
				log.Error("benchmark-request-creation-failed", err)
				continue
			}
			response, err := br.Do()
			if err != nil {
				log.Error("benchmark-request-failed", err)
				continue
			}

			log.Info("benchmark", lager.Data{
				"response-code:":   response.ResponseCode,
				"total-roundtrip":  response.TotalRoundrip,
				"time-in-app":      response.TimeInApp,
				"time-in-gorouter": response.TimeInRouter,
				"rest-of-time":     response.RestOfTime,
			})

			m.emitMetric(response.ToDatadog(deploymentName, m.index))
		case err := <-errorChan:
			if err != nil {
				log.Error("firehose-disconnect", err)
			} else {
				refreshToken(cfAssistant)
				channel, errorChan = connectToFirehose(cfAssistant, dopplerAddress, appGuid)
			}
			continue
		case s := <-signals:
			log.Error("closing", nil, lager.Data{"signal": s})
			return nil
		}
	}
	return nil
}

func (m *measurer) emitMetric(req interface{}) {
	log := logger.Session("datadog-" + strconv.Itoa(m.index))
	buf, err := json.Marshal(req)
	if err != nil {
		log.Error("cannot-marshal-metric", err)
		return
	}
	resp, err := http.Post(dogURL, "application/json", bytes.NewReader(buf))
	if err != nil {
		log.Error("cannot-emit-metric", err)
		return
	}
	respBody, _ := ioutil.ReadAll(resp.Body)
	log.Info("metric-emitted", lager.Data{
		"response-code": resp.StatusCode,
		"body":          respBody,
	})
}

func connectToFirehose(cfAssistant *assistant.Assistant, dopplerAddress, appGuid string) (<-chan *events.Envelope, chan error) {
	errorChan := make(chan error)
	channel := assistant.StreamRouterLogs(dopplerAddress, token, appGuid, errorChan)
	return channel, errorChan
}

func refreshToken(cfAssistant *assistant.Assistant) {
	tokenMutex.Lock()
	defer tokenMutex.Unlock()
	token = cfAssistant.GetOauthToken()
}
