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

package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	ec2types "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	ekstypes "github.com/aws/aws-sdk-go-v2/service/eks/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/samber/lo"

	"github.com/bwagner5/aws-sdk-go-metrics/pkg/awsmetricsv2"
)

type Options struct {
	Port int
}

func main() {
	opts := Options{}
	flag.IntVar(&opts.Port, "port", 2112, "port to serve prometheus metrics on")
	flag.Parse()

	registry := prometheus.NewRegistry()
	// Load the Shared AWS Configuration (~/.aws/config)
	cfg, err := config.LoadDefaultConfig(context.TODO(), awsmetricsv2.WithInstrumentedClients(registry))
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			log.Println("demoing api calls")
			demo(cfg)
			time.Sleep(time.Second * 30)
		}
	}()
	http.Handle("/metrics", promhttp.HandlerFor(
		registry,
		promhttp.HandlerOpts{EnableOpenMetrics: false},
	))
	srv := &http.Server{
		ReadTimeout:       1 * time.Second,
		WriteTimeout:      1 * time.Second,
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
		Addr:              fmt.Sprintf("127.0.0.1:%d", opts.Port),
	}
	log.Printf("Serving prometheus metrics at http://127.0.0.1:%d/metrics", opts.Port)
	lo.Must0(srv.ListenAndServe())
}

func demo(cfg aws.Config) {
	s3svc := s3.NewFromConfig(cfg)
	ec2svc := ec2.NewFromConfig(cfg)
	ekssvc := eks.NewFromConfig(cfg)
	ctx := context.Background()

	bucketNames := lo.Map(lo.Must(s3svc.ListBuckets(ctx, &s3.ListBucketsInput{})).Buckets, func(b s3types.Bucket, _ int) string { return *b.Name })
	fmt.Printf("S3 Buckets: %v\n", bucketNames)

	instanceIDs := lo.Map(lo.Flatten(lo.Map(lo.Must(ec2svc.DescribeInstances(ctx, &ec2.DescribeInstancesInput{})).Reservations, func(r ec2types.Reservation, _ int) []ec2types.Instance {
		return r.Instances
	})), func(i ec2types.Instance, _ int) string { return *i.InstanceId })
	fmt.Printf("EC2 Instances: %v\n", instanceIDs)

	addonNames := lo.Map(lo.Must(ekssvc.DescribeAddonVersions(ctx, &eks.DescribeAddonVersionsInput{})).Addons, func(a ekstypes.AddonInfo, _ int) string { return *a.AddonName })
	fmt.Printf("EKS Addons: %v\n", addonNames)
}
