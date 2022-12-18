package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
	"github.com/aws/aws-sdk-go/service/eks"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/bwagner5/aws-sdk-go-metrics/pkg/awsmetrics"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/samber/lo"
)

func main() {
	registry := prometheus.NewRegistry()
	sess, err := awsmetrics.Instrument(session.Must(session.NewSessionWithOptions(session.Options{
		SharedConfigState: session.SharedConfigEnable,
	})), registry)
	if err != nil {
		log.Fatalf("Unable to create instrumented aws sdk session for the aws-sdk-go v1: %v", err)
	}

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
