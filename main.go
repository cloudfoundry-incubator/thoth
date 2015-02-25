package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"time"

	"github.com/cloudfoundry/noaa/events"
	"github.com/pivotal-cf-experimental/thoth/assistant"
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

func main() {
	fmt.Println("Starting...")
	apiUrl := "api." + systemDomain
	cfAssistant := assistant.NewAssistant(apiUrl, username, password, org, space, skipSSLValidation)
	token := cfAssistant.GetOauthToken()

	appGuid := cfAssistant.AppGuid(appName)
	appUrl := "http://" + cfAssistant.AppUrl(appName)
	dopplerAddress := "wss://doppler." + systemDomain + ":4443"
	fmt.Println("Streaming Logs...")
	channel := assistant.StreamRouterLogs(dopplerAddress, token, appGuid)
	time.Sleep(3 * time.Second)
	fmt.Println("Ready To Bench...")
	for {
		timeForRequest, respCode := makeRequest(appUrl)
		message1 := <-channel
		message2 := <-channel
		var startStopMessage, logMessage events.Envelope
		if *message1.EventType == events.Envelope_HttpStartStop {
			startStopMessage = *message1
			logMessage = *message2
		} else {
			startStopMessage = *message2
			logMessage = *message1
		}
		startStop := startStopMessage.GetHttpStartStop()
		timeInApp := time.Unix(0, *startStop.StopTimestamp).Sub(time.Unix(0, *startStop.StartTimestamp))
		re, _ := regexp.Compile("response_time:([^ ]+)")
		respTimeSecs := re.FindSubmatch(logMessage.GetLogMessage().Message)
		respTime, _ := time.ParseDuration(string(respTimeSecs[1]) + "s")
		timeInRouter := respTime - timeInApp
		restOfTime := timeForRequest - respTime
		now := time.Now()
		tags := []string{
			"status:" + strconv.Itoa(respCode),
			"deployment:" + deploymentName,
		}
		request := map[string]interface{}{
			"series": []map[string]interface{}{
				{
					"metric": "app_benchmarking.total_roundtrip",
					"points": [][]int64{
						{now.Unix(), timeForRequest.Nanoseconds()},
					},
					"tags": tags,
				},
				{
					"metric": "app_benchmarking.time_in_gorouter",
					"points": [][]int64{
						{now.Unix(), timeInRouter.Nanoseconds()},
					},
					"tags": tags,
				},
				{
					"metric": "app_benchmarking.time_in_app",
					"points": [][]int64{
						{now.Unix(), timeInApp.Nanoseconds()},
					},
					"tags": tags,
				},
				{
					"metric": "app_benchmarking.rest_of_time",
					"points": [][]int64{
						{now.Unix(), restOfTime.Nanoseconds()},
					},
					"tags": tags,
				},
			},
		}
		fmt.Println("Benching...")
		emitMetric(request)
		fmt.Println("Response code:", respCode)
		fmt.Println("Total roundtrip time:", timeForRequest)
		fmt.Println("Time spent sending to App:", timeInApp)
		fmt.Println("Time from GoRouter receiving request to sending response:", respTime)
		fmt.Println("Time spent in the GoRouter:", timeInRouter)
		fmt.Println("Rest of the time:", restOfTime)
		time.Sleep(5 * time.Second)
	}
}

func makeRequest(u string) (time.Duration, int) {
	start := time.Now()
	resp, _ := http.Get(u)
	return time.Since(start), resp.StatusCode
}

func emitMetric(req interface{}) {
	buf, err := json.Marshal(req)
	if err != nil {
		log.Fatal(err)
	}
	resp, err := http.Post(dogURL, "application/json", bytes.NewReader(buf))
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp)
}
