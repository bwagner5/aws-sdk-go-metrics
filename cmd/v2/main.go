package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/eks"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bwagner5/aws-sdk-go-metrics/pkg/awsmetrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	registry := prometheus.NewRegistry()
	// Load the Shared AWS Configuration (~/.aws/config)
	cfg, err := config.LoadDefaultConfig(context.TODO(), awsmetrics.WithInstrumentedClients(registry))
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for {
			log.Println("demoing")
			demo(cfg)
			time.Sleep(time.Second * 30)
		}
	}()
	http.Handle("/metrics", promhttp.HandlerFor(
		registry,
		promhttp.HandlerOpts{EnableOpenMetrics: false},
	))
	http.ListenAndServe(":2112", nil)
}

func demo(cfg aws.Config) {
	s3svc := s3.NewFromConfig(cfg)
	ec2svc := ec2.NewFromConfig(cfg)
	ekssvc := eks.NewFromConfig(cfg)
	ctx := context.Background()
	s3svc.ListBuckets(ctx, &s3.ListBucketsInput{})
	ec2svc.DescribeInstances(ctx, &ec2.DescribeInstancesInput{})
	ekssvc.DescribeAddonVersions(ctx, &eks.DescribeAddonVersionsInput{})
}
