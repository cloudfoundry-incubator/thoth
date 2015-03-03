package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/cloudfoundry/noaa/events"
	"github.com/pivotal-cf-experimental/thoth/assistant"
	"github.com/pivotal-cf-experimental/thoth/benchmark"
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

	dogURL = "https://app.datadoghq.com/api/v1/series?api_key=" + os.Getenv("DATADOG_API_KEY")
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
	fmt.Println("Starting...")
	apiUrl := "api." + systemDomain
	cfAssistant := assistant.NewAssistant(apiUrl, username, password, org, space, skipSSLValidation)
	cfAssistant.GetOauthToken()
	appGuid := cfAssistant.AppGuid(appName)
	appUrl := "http://" + cfAssistant.AppUrl(appName)
	dopplerAddress := "wss://doppler." + systemDomain + ":4443"
	fmt.Println("Streaming Logs...")
	channel, errorChan := connectToFirehose(cfAssistant, dopplerAddress, appGuid)

	clock := NewClock()
	for {
		select {
		case err := <-errorChan:
			fmt.Fprintln(os.Stderr, err)
			close(errorChan)
			channel, errorChan = connectToFirehose(cfAssistant, dopplerAddress, appGuid)
			continue
		default:
			br := benchmark.NewBenchmarkRequest(appUrl, channel, clock, 2*time.Second)
			response, err := br.Do()
			if err != nil {
				fmt.Fprintln(os.Stderr, err)
				continue
			}

			emitMetric(response.ToDatadog(deploymentName))

			fmt.Println("Response code:", response.ResponseCode)
			fmt.Println("Total roundtrip time:", response.TotalRoundrip)
			fmt.Println("Time spent sending to App:", response.TimeInApp)
			fmt.Println("Time spent in the GoRouter:", response.TimeInRouter)
			fmt.Println("Rest of the time:", response.RestOfTime)
			time.Sleep(5 * time.Second)
		}
	}
}

func connectToFirehose(cfAssistant *assistant.Assistant, dopplerAddress, appGuid string) (<-chan *events.Envelope, chan error) {
	errorChan := make(chan error)
	token := cfAssistant.GetOauthToken()
	channel := assistant.StreamRouterLogs(dopplerAddress, token, appGuid, errorChan)
	return channel, errorChan
}

func emitMetric(req interface{}) {
	buf, err := json.Marshal(req)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	resp, err := http.Post(dogURL, "application/json", bytes.NewReader(buf))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}
	fmt.Println(resp)
}
