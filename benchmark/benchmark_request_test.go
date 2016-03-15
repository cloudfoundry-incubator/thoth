package benchmark_test

import (
	"net/http"
	"time"

	. "github.com/cloudfoundry-incubator/thoth/benchmark"
	"github.com/cloudfoundry/sonde-go/events"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/ghttp"
)

type FakeClock struct {
	currentTime time.Time
}

func NewFakeClock() *FakeClock {
	return &FakeClock{
		currentTime: time.Unix(123456789, 0),
	}
}

func (fc *FakeClock) Now() time.Time {
	return fc.currentTime
}

func (fc *FakeClock) Since(t time.Time) time.Duration {
	return fc.Now().Sub(t)
}

func (fc *FakeClock) Elapse(d time.Duration) {
	fc.currentTime = fc.currentTime.Add(d)
}

var _ = Describe("BenchmarkRequest", func() {
	Describe("Do()", func() {
		var (
			server *ghttp.Server
			br     *BenchmarkRequest
			clock  *FakeClock
			ch     chan *events.Envelope
		)

		BeforeEach(func() {
			ch = make(chan *events.Envelope, 2)
			clock = NewFakeClock()
			server = ghttp.NewServer()
			var err error
			br, err = NewBenchmarkRequest(server.URL(), ch, clock, 100*time.Millisecond)
			Expect(err).NotTo(HaveOccurred())
		})

		Context("everything works", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					func(w http.ResponseWriter, r *http.Request) {
						clock.Elapse(50 * time.Millisecond)
						w.WriteHeader(http.StatusOK)
						eventType := events.Envelope_HttpStartStop
						startTime := time.Time{}
						startTimeUnix := startTime.UnixNano()
						stopTimeUnix := startTime.Add(20 * time.Millisecond).UnixNano()
						uri := server.URL() + "/" + br.Guid.String() + ".html"

						ch <- &events.Envelope{
							EventType: &eventType,
							HttpStartStop: &events.HttpStartStop{
								Uri:            &uri,
								StartTimestamp: &startTimeUnix,
								StopTimestamp:  &stopTimeUnix,
							},
						}

						eventTypeLog := events.Envelope_LogMessage
						logMessage := "response_time:0.03 /" + br.Guid.String() + ".html"
						ch <- &events.Envelope{
							EventType: &eventTypeLog,
							LogMessage: &events.LogMessage{
								Message: []byte(logMessage),
							},
						}
					},
				)
			})

			It("returns a response including response code", func() {
				response, err := br.Do()
				Expect(err).NotTo(HaveOccurred())
				Expect(response.ResponseCode).To(Equal(http.StatusOK))
			})

			It("returns a response including response time", func() {
				response, err := br.Do()
				Expect(err).NotTo(HaveOccurred())
				Expect(response.TotalRoundrip).To(Equal(50 * time.Millisecond))
			})

			It("returns time spent in app", func() {
				response, err := br.Do()
				Expect(err).NotTo(HaveOccurred())
				Expect(response.TimeInApp).To(Equal(20 * time.Millisecond))
			})

			It("returns time spent in router", func() {
				response, err := br.Do()
				Expect(err).NotTo(HaveOccurred())
				Expect(response.TimeInRouter).To(Equal(10 * time.Millisecond))
			})

			It("returns time spent in other things", func() {
				response, err := br.Do()
				Expect(err).NotTo(HaveOccurred())
				Expect(response.RestOfTime).To(Equal(20 * time.Millisecond))
			})

			It("returns timestamp when it started", func() {
				response, err := br.Do()
				Expect(err).NotTo(HaveOccurred())
				Expect(response.Timestamp).To(Equal(time.Unix(123456789, 0)))
			})
		})

		Context("messages are not delivered", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusOK)
					},
				)
			})

			It("times out", func() {
				_, err := br.Do()
				Expect(err).To(HaveOccurred())
			})
		})

		Context("HttpStartStop is not received", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusOK)

						eventTypeLog := events.Envelope_LogMessage
						logMessage := "response_time:0.03 /potato"
						ch <- &events.Envelope{
							EventType: &eventTypeLog,
							LogMessage: &events.LogMessage{
								Message: []byte(logMessage),
							},
						}
					},
				)
			})

			It("times out", func() {
				_, err := br.Do()
				Expect(err).To(HaveOccurred())
			})
		})

		Context("LogMessage is not received", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusOK)
						eventType := events.Envelope_HttpStartStop
						startTime := time.Time{}
						startTimeUnix := startTime.UnixNano()
						stopTimeUnix := startTime.Add(20 * time.Millisecond).UnixNano()
						uri := "notyourapp"

						ch <- &events.Envelope{
							EventType: &eventType,
							HttpStartStop: &events.HttpStartStop{
								Uri:            &uri,
								StartTimestamp: &startTimeUnix,
								StopTimestamp:  &stopTimeUnix,
							},
						}
					},
				)
			})

			It("times out", func() {
				_, err := br.Do()
				Expect(err).To(HaveOccurred())
			})
		})

		Context("receiving non-matching HttpStartStop", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusOK)
						eventType := events.Envelope_HttpStartStop
						startTime := time.Time{}
						startTimeUnix := startTime.UnixNano()
						stopTimeUnix := startTime.Add(20 * time.Millisecond).UnixNano()
						uri := server.URL()

						ch <- &events.Envelope{
							EventType: &eventType,
							HttpStartStop: &events.HttpStartStop{
								Uri:            &uri,
								StartTimestamp: &startTimeUnix,
								StopTimestamp:  &stopTimeUnix,
							},
						}

						eventTypeLog := events.Envelope_LogMessage
						logMessage := "response_time:0.03 /" + br.Guid.String() + ".html"
						ch <- &events.Envelope{
							EventType: &eventTypeLog,
							LogMessage: &events.LogMessage{
								Message: []byte(logMessage),
							},
						}
					},
				)
			})

			It("times out", func() {
				_, err := br.Do()
				Expect(err).To(HaveOccurred())
			})
		})

		Context("receiving non-matching log message", func() {
			BeforeEach(func() {
				server.AppendHandlers(
					func(w http.ResponseWriter, r *http.Request) {
						w.WriteHeader(http.StatusOK)
						eventType := events.Envelope_HttpStartStop
						startTime := time.Time{}
						startTimeUnix := startTime.UnixNano()
						stopTimeUnix := startTime.Add(20 * time.Millisecond).UnixNano()
						uri := server.URL() + "/" + br.Guid.String() + ".html"

						ch <- &events.Envelope{
							EventType: &eventType,
							HttpStartStop: &events.HttpStartStop{
								Uri:            &uri,
								StartTimestamp: &startTimeUnix,
								StopTimestamp:  &stopTimeUnix,
							},
						}

						eventTypeLog := events.Envelope_LogMessage
						logMessage := "response_time:0.03 /potato.html"
						ch <- &events.Envelope{
							EventType: &eventTypeLog,
							LogMessage: &events.LogMessage{
								Message: []byte(logMessage),
							},
						}
					},
				)
			})

			It("times out", func() {
				_, err := br.Do()
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
