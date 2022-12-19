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

// Package awsmetricsv2 enables instrumenting the aws-sdk-go-v2 to emit prometheus metrics on AWS API calls
package awsmetricsv2

import (
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/http2"

	"github.com/bwagner5/aws-sdk-go-metrics/pkg/commons"
)

// WithInstrumentedClient returns a LoadOptionsFunc for use with aws-sdk-go-v2 config
func WithInstrumentedClients(registry prometheus.Registerer) config.LoadOptionsFunc {
	client, err := InstrumentAWSHTTPClient(awshttp.NewBuildableClient(), registry)
	if err != nil {
		return nil
	}
	return config.WithHTTPClient(client)
}

// InstrumentAWSHTTPClient turns an arbitrary AWS http client into an aws-sdk-go-v2 instrumented client
func InstrumentAWSHTTPClient(httpClient *awshttp.BuildableClient, registry prometheus.Registerer) (aws.HTTPClient, error) {
	if err := commons.RegisterMetrics(registry); err != nil {
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
	return commons.MetricsRoundTripper{BaseRT: httpClient.GetTransport()}, nil
}
