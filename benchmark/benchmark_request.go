package benchmark

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"code.google.com/p/go-uuid/uuid"
	"github.com/cloudfoundry/noaa/events"
)

type Clock interface {
	Now() time.Time
	Since(t time.Time) time.Duration
}

type BenchmarkRequest struct {
	Guid          uuid.UUID
	httpStartStop events.HttpStartStop
	logMessage    events.LogMessage

	appUrl  string
	ch      <-chan *events.Envelope
	clock   Clock
	timeout time.Duration
}

func NewBenchmarkRequest(appUrl string, ch <-chan *events.Envelope, clock Clock, timeout time.Duration) *BenchmarkRequest {
	return &BenchmarkRequest{
		Guid:    uuid.NewRandom(),
		appUrl:  appUrl,
		ch:      ch,
		clock:   clock,
		timeout: timeout,
	}
}

func (br *BenchmarkRequest) Do() (BenchmarkResponse, error) {
	timestamp := br.clock.Now()
	timeForRequest, respCode := br.makeRequest()
	err := br.grabMessages()
	if err != nil {
		return BenchmarkResponse{}, err
	}

	timeInApp := time.Unix(0, *br.httpStartStop.StopTimestamp).Sub(time.Unix(0, *br.httpStartStop.StartTimestamp))
	re, _ := regexp.Compile("response_time:([^ ]+)")
	respTimeSecs := re.FindSubmatch(br.logMessage.Message)
	respTime, _ := time.ParseDuration(string(respTimeSecs[1]) + "s")
	timeInRouter := respTime - timeInApp
	restOfTime := timeForRequest - respTime

	response := BenchmarkResponse{
		TotalRoundrip: timeForRequest,
		TimeInApp:     timeInApp,
		TimeInRouter:  timeInRouter,
		RestOfTime:    restOfTime,
		ResponseCode:  respCode,
		Timestamp:     timestamp,
	}

	return response, nil
}

func (br *BenchmarkRequest) grabMessages() error {
	messages := []*events.Envelope{}
	timeout := time.After(br.timeout)

	for len(messages) < 2 {
		select {
		case message := <-br.ch:
			if br.checkMessage(message) {
				messages = append(messages, message)
			}
		case <-timeout:
			return errors.New("timed out getting messages for request: " + br.Guid.String())
		}
	}

	for _, message := range messages {
		if *message.EventType == events.Envelope_HttpStartStop {
			br.httpStartStop = *message.GetHttpStartStop()
		} else if *message.EventType == events.Envelope_LogMessage {
			br.logMessage = *message.GetLogMessage()
		} else {
			fmt.Printf("invalid msg type: %v", *message.EventType)
			return errors.New("got invalid msg type: " + br.Guid.String())
		}
	}

	return nil
}

func (br *BenchmarkRequest) checkMessage(message *events.Envelope) bool {
	var toCheck string

	if *message.EventType == events.Envelope_HttpStartStop {
		toCheck = *message.HttpStartStop.Uri
	} else if *message.EventType == events.Envelope_LogMessage {
		toCheck = string(message.LogMessage.Message)
	}

	return strings.Contains(toCheck, br.Guid.String())
}

func (br *BenchmarkRequest) makeRequest() (time.Duration, int) {
	start := br.clock.Now()
	resp, _ := http.Get(br.appUrl + "/" + br.Guid.String() + ".html")
	return br.clock.Since(start), resp.StatusCode
}
