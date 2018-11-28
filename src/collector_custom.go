package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"time"
)

type MetricCollectorCustomInterface interface {
	Setup()
	Collect(ctx context.Context, subscriptions []subscriptions.Subscription)
}

type MetricCollectorCustom struct {
	Collector MetricCollectorCustomInterface
	Name string
	ScrapeTime  *time.Duration
	AzureSubscriptions []subscriptions.Subscription
}

func (m *MetricCollectorCustom) Run(scrapeTime time.Duration) {
	m.ScrapeTime = &scrapeTime

	m.Collector.Setup()
	go func() {
		for {
			m.Collect()
			Logger.Messsage("collector[%s]: sleeping %v", m.Name, m.ScrapeTime.String())
			time.Sleep(*m.ScrapeTime)
		}
	}()
}

func (m *MetricCollectorCustom) Collect() {
	ctx := context.Background()
	m.Collector.Collect(ctx, m.AzureSubscriptions)
}
