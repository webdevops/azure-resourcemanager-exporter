package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"time"
)

type CollectorCustom struct {
	Processor CollectorProcessorCustomInterface
	Name string
	ScrapeTime  *time.Duration
	AzureSubscriptions []subscriptions.Subscription
	AzureLocations []string
}

func (m *CollectorCustom) Run(scrapeTime time.Duration) {
	m.ScrapeTime = &scrapeTime

	m.Processor.Setup(m)
	go func() {
		for {
			go func() {
				m.Collect()
			}()
			Logger.Verbose("collector[%s]: sleeping %v", m.Name, m.ScrapeTime.String())
			time.Sleep(*m.ScrapeTime)
		}
	}()
}

func (m *CollectorCustom) Collect() {
	ctx := context.Background()
	m.Processor.Collect(ctx)
}
