/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package awsmetrics enables instrumenting the aws-sdk-go to emit prometheus metrics on AWS API calls
package awsmetrics

import (
	"fmt"
	"net/http"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/http2"

	"github.com/bwagner5/aws-sdk-go-metrics/pkg/commons"
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
		return session, fmt.Errorf("unable to construct instrumented http client, %w", err)
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
	_ = http2.ConfigureTransport(transport)
	httpClient.Transport = commons.MetricsRoundTripper{BaseRT: transport}
	return httpClient, nil
}
