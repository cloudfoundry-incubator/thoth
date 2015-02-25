package main

import (
	"crypto/tls"
	"fmt"
	"strings"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	"github.com/cloudfoundry-incubator/cf-test-helpers/runner"
	"github.com/cloudfoundry/noaa"
	"github.com/cloudfoundry/noaa/events"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
)

func main() {
	token := getOauthToken()
	appGuid := ""
	for msg := range streamRouterLogs(token, appGuid) {
		fmt.Printf("%v \n", msg)
	}
}

func getOauthToken() string {
	var token string

	gomega.RegisterFailHandler(ginkgo.Fail)
	apiUrl := ""
	username := "name"
	password := "password"
	org := "app-benchmarking"
	space := "app-benchmarking"
	skipSSLValidation := true

	userContext := cf.NewUserContext(apiUrl, username, password, org, space, skipSSLValidation)
	cf.AsUser(userContext, func() {
		bytes := runner.Run("bash", "-c", "cf oauth-token | tail -n +4").Wait(5).Out.Contents()
		token = strings.TrimSpace(string(bytes))
	})

	return token
}

const DopplerAddress = "wss://doppler:4443"

func streamRouterLogs(authToken, appGuid string) <-chan *events.Envelope {
	connection := noaa.NewConsumer(DopplerAddress, &tls.Config{InsecureSkipVerify: true}, nil)

	msgChan := make(chan *events.Envelope)
	go func() {
		defer close(msgChan)
		errorChan := make(chan error)
		connection.Stream(appGuid, authToken, msgChan, errorChan, nil)
	}()

	routerChan := make(chan *events.Envelope)
	go func(c chan<- *events.Envelope) {
		for msg := range msgChan {
			if strings.HasPrefix(*msg.Origin, "router_") && (*msg.EventType == events.Envelope_HttpStartStop || *msg.EventType == events.Envelope_LogMessage) {
				c <- msg
			}
		}
	}(routerChan)
	return routerChan
}
