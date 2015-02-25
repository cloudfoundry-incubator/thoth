package assistant

import (
	"crypto/tls"
	"strings"
	"time"

	"github.com/cloudfoundry-incubator/cf-test-helpers/cf"
	"github.com/cloudfoundry-incubator/cf-test-helpers/runner"
	"github.com/cloudfoundry/noaa"
	"github.com/cloudfoundry/noaa/events"
	"github.com/onsi/ginkgo"
	"github.com/onsi/gomega"
)

const (
	CF_TIMEOUT = 5 * time.Second
)

type Assistant struct {
	apiUrl, username, password, org, space string
	skipSSLValidation                      bool
	userContext                            cf.UserContext
}

func NewAssistant(apiUrl, username, password, org, space string, skipSSLValidation bool) *Assistant {
	return &Assistant{
		apiUrl:            apiUrl,
		username:          username,
		password:          password,
		org:               org,
		space:             space,
		skipSSLValidation: skipSSLValidation,
		userContext:       cf.NewUserContext(apiUrl, username, password, org, space, skipSSLValidation),
	}
}

func (a *Assistant) AppGuid(appName string) string {
	var appGuid string
	cf.AsUser(a.userContext, func() {
		appGuid = strings.TrimSpace(string(cf.Cf("app", appName, "--guid").Wait(CF_TIMEOUT).Out.Contents()))
	})
	return appGuid
}

func (a *Assistant) AppUrl(appName string) string {
	var appUrl string
	cf.AsUser(a.userContext, func() {
		bytes := runner.Run("bash", "-c", `cf app `+appName+` | grep urls | cut -d" " -f2`).Wait(CF_TIMEOUT).Out.Contents()
		appUrl = strings.TrimSpace(string(bytes))
	})
	return appUrl
}

func (a *Assistant) GetOauthToken() string {
	var token string

	gomega.RegisterFailHandler(ginkgo.Fail)

	cf.AsUser(a.userContext, func() {
		bytes := runner.Run("bash", "-c", "cf oauth-token | tail -n +4").Wait(CF_TIMEOUT).Out.Contents()
		token = strings.TrimSpace(string(bytes))
	})

	return token
}

func StreamRouterLogs(dopplerAddress, authToken, appGuid string) <-chan *events.Envelope {
	connection := noaa.NewConsumer(dopplerAddress, &tls.Config{InsecureSkipVerify: true}, nil)

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