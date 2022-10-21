package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/bwagner5/aws-sdk-go-metrics/pkg/metricsclient"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	sess := session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	}))
	registry := prometheus.NewRegistry()
	metricsClient, err := metricsclient.New(sess.Config.HTTPClient, registry)
	if err != nil {
		log.Fatalf("Unable to create metrics client for the aws-sdk-go v1: %v", err)
	}
	sess.Config.HTTPClient = metricsClient

	go func() {
		for {
			demo(sess)
			time.Sleep(time.Second * 30)
		}
	}()
	http.Handle("/metrics", promhttp.HandlerFor(
		registry,
		promhttp.HandlerOpts{EnableOpenMetrics: false},
	))
	http.ListenAndServe(":2112", nil)
}

func demo(sess *session.Session) {
	s3svc := s3.New(sess)
	ec2svc := ec2.New(sess)
	ekssvc := eks.New(sess)

	s3svc.ListBucketsWithContext(context.Background(), &s3.ListBucketsInput{})
	ec2svc.DescribeInstances(&ec2.DescribeInstancesInput{})
	ekssvc.DescribeAddonVersions(&eks.DescribeAddonVersionsInput{})
}
