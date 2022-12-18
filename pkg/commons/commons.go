package commons

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
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

type MetricsRoundTripper struct {
	BaseRT http.RoundTripper
}

func RegisterMetrics(registry prometheus.Registerer) error {
	for _, c := range []prometheus.Collector{totalRequests, requestLatency} {
		if err := registry.Register(c); err != nil {
			return err
		}
	}
	return nil
}

func requestLabels(service string, action string, statusCode int) prometheus.Labels {
	return prometheus.Labels{
		"service":     service,
		"action":      action,
		"status_code": fmt.Sprint(statusCode),
	}
}

// Do implements the aws.HTTPClient interface
func (mrt MetricsRoundTripper) Do(req *http.Request) (*http.Response, error) {
	return mrt.RoundTrip(req)
}

// RoundTrip implements an instrumented RoundTrip for AWS API calls
func (mrt MetricsRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	service, err := getService(req)
	if err != nil {
		log.Printf("Unable to parse request for service: %v", err)
	}
	action, err := getAction(req)
	if err != nil {
		log.Printf("Unable to parse request for action: %v", err)
	}
	start := time.Now().UTC()
	res, err := mrt.BaseRT.RoundTrip(req)
	latency := time.Since(start)
	statusCode := 0
	if res != nil {
		statusCode = res.StatusCode
	}
	requestLabels := requestLabels(service, action, statusCode)
	totalRequests.With(requestLabels).Inc()
	requestLatency.With(requestLabels).Observe(float64(latency.Milliseconds()))
	return res, err
}

func getService(req *http.Request) (string, error) {
	authz := req.Header.Get("Authorization")
	authzTokens := strings.Split(authz, "=")
	credentialHeader := ""
	for i, token := range authzTokens {
		if strings.Contains(token, "Credential") && len(authzTokens) > i {
			credentialHeader = authzTokens[i+1]
		}
	}
	if credentialHeader == "" {
		return "", fmt.Errorf("unable to find credential header: %v", authzTokens)
	}
	credentialHeaderTokens := strings.Split(credentialHeader, "/")
	if len(credentialHeaderTokens) >= 5 {
		return credentialHeaderTokens[3], nil
	} else {
		return "", fmt.Errorf("unable to find service in credential header, only found %d credential tokens", len(credentialHeaderTokens))
	}
}

func getAction(req *http.Request) (string, error) {
	switch req.Method {
	case http.MethodPost, http.MethodPut:
		body, err := io.ReadAll(req.Body)
		if err != nil {
			return "", err
		}
		defer req.Body.Close()
		req.Body = io.NopCloser(bytes.NewBuffer(body))
		tokens := strings.Split(string(body), "=")
		for i, token := range tokens {
			if token == "Action" && len(tokens) > i {
				return strings.Split(tokens[i+1], "&")[0], nil
			}
		}
		return "", fmt.Errorf("unable to find Action token in the request body")
	default:
		return req.URL.Path, nil
	}
}
