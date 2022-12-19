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

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/samber/lo"

	"github.com/bwagner5/aws-sdk-go-metrics/pkg/awsmetrics"
)

type Options struct {
	Port int
}

func main() {
	opts := Options{}
	flag.IntVar(&opts.Port, "port", 2112, "port to serve prometheus metrics on")
	flag.Parse()

	registry := prometheus.NewRegistry()
	sess, err := awsmetrics.Instrument(session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})), registry)
	if err != nil {
		log.Fatalf("Unable to create instrumented aws sdk session for the aws-sdk-go v1: %v", err)
	}

	go func() {
		for {
			log.Println("demoing api calls")
			demo(sess)
			time.Sleep(30 * time.Second)
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

func demo(sess *session.Session) {
	s3svc := s3.New(sess)
	ec2svc := ec2.New(sess)
	ekssvc := eks.New(sess)

	bucketNames := lo.Map(lo.Must(s3svc.ListBucketsWithContext(context.Background(), &s3.ListBucketsInput{})).Buckets, func(b *s3.Bucket, _ int) string {
		return *b.Name
	})
	fmt.Printf("S3 Buckets: %v\n", bucketNames)

	addonNames := lo.Map(lo.Must(ekssvc.DescribeAddonVersions(&eks.DescribeAddonVersionsInput{})).Addons, func(a *eks.AddonInfo, _ int) string {
		return *a.AddonName
	})
	fmt.Printf("EKS Addons: %v\n", addonNames)

	instanceIDs := lo.Flatten(lo.Map(lo.Must(ec2svc.DescribeInstances(&ec2.DescribeInstancesInput{})).Reservations, func(r *ec2.Reservation, _ int) []string {
		return lo.Map(r.Instances, func(i *ec2.Instance, _ int) string { return *i.InstanceId })
	}))
	fmt.Printf("EC2 Instances: %v\n", instanceIDs)
}
