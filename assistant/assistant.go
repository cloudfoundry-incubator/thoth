package assistant

import (
	"crypto/tls"
	"errors"
	"fmt"
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
	CF_TIMEOUT = 30 * time.Second
	ORIGIN     = "gorouter"
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

func (a *Assistant) AppGuid(appName string) (string, error) {
	var appGuid string
	var err error
	cf.AsUser(a.userContext, func() {
		session := cf.Cf("app", appName, "--guid").Wait(CF_TIMEOUT)
		if session.ExitCode() != 0 {
			err = errors.New(fmt.Sprintf("cf app --guid command failed: %s", string(session.Out.Contents())))
			return
		}

		appGuid = strings.TrimSpace(string(session.Out.Contents()))
	})
	return appGuid, err
}

func (a *Assistant) AppUrl(appName string) string {
	var appUrl string
	cf.AsUser(a.userContext, func() {
		session := runner.Run("bash", "-c", `cf app `+appName+` | grep urls | cut -d" " -f2`)
		bytes := session.Wait(CF_TIMEOUT).Out.Contents()
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

func StreamRouterLogs(dopplerAddress, authToken, appGuid string, errorChan chan error, stopChan chan struct{}) <-chan *events.Envelope {
	connection := noaa.NewConsumer(dopplerAddress, &tls.Config{InsecureSkipVerify: true}, nil)

	msgChan := make(chan *events.Envelope)
	go func() {
		defer close(msgChan)
		connection.Stream(appGuid, authToken, msgChan, errorChan, stopChan)
	}()

	routerChan := make(chan *events.Envelope, 2)
	go func(c chan<- *events.Envelope) {
		defer close(c)
		for {
			select {
			case msg := <-msgChan:
				if strings.HasPrefix(*msg.Origin, ORIGIN) && (*msg.EventType == events.Envelope_HttpStartStop || *msg.EventType == events.Envelope_LogMessage) {
					c <- msg
				}
			case <-stopChan:
				return
			}
		}
	}(routerChan)
	return routerChan
}
