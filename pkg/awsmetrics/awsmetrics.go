package awsmetrics

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go/aws/session"
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

// MustInstrument takes an aws session and instruments the underlying HTTPClient to emit prometheus metrics on SDK calls
// and panics if an error occurs
func MustInstrument(session *session.Session, registry prometheus.Registerer) *session.Session {
	sess, err := Instrument(session, registry)
	if err != nil {
		panic(err)
	}
	return sess
}

// Instrument takes an aws session and instruments the underlying HTTPClient to emit prometheus metrics on SDK calls
func Instrument(session *session.Session, registry prometheus.Registerer) (*session.Session, error) {
	if session.Config == nil {
		return session, fmt.Errorf("aws session must have valid config to instrument")
	}
	httpClient, err := InstrumentedHTTPClient(session.Config.HTTPClient, registry)
	if err != nil {
		return session, fmt.Errorf("unable to construct instrumented http client, %v", err)
	}
	session.Config.HTTPClient = httpClient
	return session, nil
}

// WithInstrumentedClient returns a LoadOptionsFunc for use with aws-sdk-go-v2 config
func WithInstrumentedClients(registry prometheus.Registerer) config.LoadOptionsFunc {
	client, err := InstrumentedAWSHTTPClient(awshttp.NewBuildableClient(), registry)
	if err != nil {
		return nil
	}
	return config.WithHTTPClient(client)
}

// type startTime struct{}

// // TODO: This doesn't work
// func instrumentV2MW(cfg aws.Config, registry prometheus.Registerer) (aws.Config, error) {
// 	if err := registerMetrics(registry); err != nil {
// 		return cfg, err
// 	}
// 	cfg.APIOptions = append(cfg.APIOptions, func(stack *middleware.Stack) error {
// 		return stack.Initialize.Add(middleware.InitializeMiddlewareFunc("instrument-before", func(ctx context.Context, input middleware.InitializeInput, next middleware.InitializeHandler) (middleware.InitializeOutput, middleware.Metadata, error) {
// 			return next.HandleInitialize(middleware.WithStackValue(ctx, startTime{}, time.Now().UTC()), input)
// 		}), middleware.Before)
// 	}, func(stack *middleware.Stack) error {
// 		return stack.Deserialize.Add(middleware.DeserializeMiddlewareFunc("instrument-after", func(ctx context.Context, input middleware.DeserializeInput, next middleware.DeserializeHandler) (middleware.DeserializeOutput, middleware.Metadata, error) {
// 			start, ok := middleware.GetStackValue(ctx, startTime{}).(time.Time)
// 			if !ok {
// 				return next.HandleDeserialize(ctx, input)
// 			}
// 			latency := time.Since(start)
// 			// TODO: how to get response code
// 			statusCode := 200
// 			requestLabels := requestLabels(mw.GetServiceID(ctx), mw.GetOperationName(ctx), statusCode)
// 			totalRequests.With(requestLabels).Inc()
// 			// TODO: I think this is before the response :(
// 			requestLatency.With(requestLabels).Observe(float64(latency.Milliseconds()))
// 			return next.HandleDeserialize(ctx, input)
// 		}), middleware.After)
// 	})
// 	return cfg, nil
// }

func InstrumentedAWSHTTPClient(httpClient *awshttp.BuildableClient, registry prometheus.Registerer) (aws.HTTPClient, error) {
	if err := registerMetrics(registry); err != nil {
		return httpClient, err
	}

	var transport *http.Transport
	if httpClient.GetTransport() == nil {
		transport = http.DefaultTransport.(*http.Transport)
	} else {
		transport = httpClient.GetTransport()
	}
	err := http2.ConfigureTransport(transport)
	if err != nil {
		panic(err)
	}
	return MetricsRoundTripper{BaseRT: httpClient.GetTransport()}, nil
}

// InstrumentedHTTPClient turns an arbitrary http client into an aws sdk instrumented client
func InstrumentedHTTPClient(httpClient *http.Client, registry prometheus.Registerer) (*http.Client, error) {
	if err := registerMetrics(registry); err != nil {
		return httpClient, err
	}

	var transport *http.Transport
	if httpClient.Transport == nil {
		transport = http.DefaultTransport.(*http.Transport)
	} else {
		transport = httpClient.Transport.(*http.Transport)
	}
	err := http2.ConfigureTransport(transport)
	if err != nil {
		panic(err)
	}
	httpClient.Transport = MetricsRoundTripper{BaseRT: transport}
	return httpClient, nil
}

func registerMetrics(registry prometheus.Registerer) error {
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

type MetricsRoundTripper struct {
	BaseRT http.RoundTripper
}

// Do implements the aws.HTTPClient interface
func (mrt MetricsRoundTripper) Do(req *http.Request) (*http.Response, error) {
	return mrt.RoundTrip(req)
}

// RoundTrip implements an instrumented RoundTrip for AWS API calls
func (mrt MetricsRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	log.Println(req.Proto)
	// dump, err := httputil.DumpRequestOut(req, true)
	// if err != nil {
	// 	log.Fatalf("unable to dump request: %v", err)
	// }
	service, err := getService(req)
	if err != nil {
		log.Fatalf("Unable to parse request for service: %v", err)
	}
	action, err := getAction(req)
	if err != nil {
		log.Fatalf("Unable to parse request for action: %v", err)
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
	// log.Printf("REQUEST: %v\n", string(dump))
	requestLatency.With(requestLabels).Observe(float64(latency.Milliseconds()))

	return res, err
}

func getService(req *http.Request) (string, error) {
	authz := req.Header.Get("Authorization")
	authzTokens := strings.Split(authz, "=")
	credentialHeader := ""
	for i, token := range authzTokens {
		fmt.Println(token)
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
