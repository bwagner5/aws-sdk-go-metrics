// Package awsmetrics enables instrumenting the aws-sdk-go and aws-sdk-go-v2 to emit prometheus metrics on AWS API calls
package awsmetrics

import (
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/bwagner5/aws-sdk-go-metrics/pkg/commons"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/http2"
)

var (
	labels        = []string{"service", "action", "status_code"}
	totalRequests = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "aws_sdk_go_requests",
		Help: "The total number of AWS SDK Go requests",
	}, labels)

	requestLatency = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name: "aws_sdk_go_request_latency",
		Help: "Latency of AWS SDK Go requests",
		Buckets: []float64{
			10, 20, 30, 40, 50, 60, 70, 80, 90, 100,
			125, 150, 175, 200, 225, 250, 275, 300,
			400, 500, 600, 700, 800, 900,
			1_000, 1_500, 2_000, 2_500, 3_000, 3_500, 4_000, 4_500, 5_000,
			6_000, 7_000, 8_000, 9_000, 10_000,
		},
	}, labels)
)

// MustInstrument takes an aws-sdk-go (v1) session and instruments the underlying HTTPClient to emit prometheus metrics on SDK calls
// and panic if an error occurs
func MustInstrument(session *session.Session, registry prometheus.Registerer) *session.Session {
	sess, err := Instrument(session, registry)
	if err != nil {
		panic(err)
	}
	return sess
}

// Instrument takes an aws-sdk-go (v1) session and instruments the underlying HTTPClient to emit prometheus metrics on SDK calls
func Instrument(session *session.Session, registry prometheus.Registerer) (*session.Session, error) {
	if session.Config == nil {
		return session, fmt.Errorf("aws session must have valid config to instrument")
	}
	httpClient, err := InstrumentHTTPClient(session.Config.HTTPClient, registry)
	if err != nil {
		return session, fmt.Errorf("unable to construct instrumented http client, %v", err)
	}
	session.Config.HTTPClient = httpClient
	return session, nil
}

// InstrumentHTTPClient turns an arbitrary http client into an aws sdk (v1) instrumented client
func InstrumentHTTPClient(httpClient *http.Client, registry prometheus.Registerer) (*http.Client, error) {
	if err := commons.RegisterMetrics(registry); err != nil {
		return httpClient, err
	}

	var transport *http.Transport
	if httpClient.Transport == nil {
		transport = http.DefaultTransport.(*http.Transport)
	} else {
		transport = httpClient.Transport.(*http.Transport)
	}
	// no need to handle error since its idempotent
	http2.ConfigureTransport(transport)
	httpClient.Transport = commons.MetricsRoundTripper{BaseRT: transport}
	return httpClient, nil
}
