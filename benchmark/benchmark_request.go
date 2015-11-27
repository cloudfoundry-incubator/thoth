package benchmark

import (
	"errors"
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
	if respTimeSecs == nil {
		err = errors.New("Error could not parse 'response_time' in log messages")
		return BenchmarkResponse{}, err
	}
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

func complete(messages []*events.Envelope) bool {
	var http, log bool
	for _, message := range messages {
		if *message.EventType == events.Envelope_HttpStartStop {
			http = true
		} else if *message.EventType == events.Envelope_LogMessage {
			log = true
		}
	}
	return http && log
}

func (br *BenchmarkRequest) grabMessages() error {
	messages := []*events.Envelope{}
	timeout := time.After(br.timeout)

	for !complete(messages) {
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
		if !br.hasHttpStartStop() && *message.EventType == events.Envelope_HttpStartStop {
			br.httpStartStop = *message.GetHttpStartStop()
		} else if !br.hasLogMessage() && *message.EventType == events.Envelope_LogMessage {
			br.logMessage = *message.GetLogMessage()
		}
	}

	return nil
}

func (br *BenchmarkRequest) hasHttpStartStop() bool {
	return br.httpStartStop.StartTimestamp != nil && br.httpStartStop.StopTimestamp != nil
}

func (br *BenchmarkRequest) hasLogMessage() bool {
	return len(br.logMessage.Message) > 0
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
