package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"sync"
	"time"
)

type MetricCollectorGeneralInterface interface {
	Setup()
	Reset()
	Collect(ctx context.Context, callback chan<- func(), subscription subscriptions.Subscription)
}

type MetricCollectorGeneral struct {
	Collector MetricCollectorGeneralInterface
	Name string
	ScrapeTime  *time.Duration
	AzureSubscriptions []subscriptions.Subscription
}

func (m *MetricCollectorGeneral) Run(scrapeTime time.Duration) {
	m.ScrapeTime = &scrapeTime

	m.Collector.Setup()
	go func() {
		for {
			go func() {
				m.Collect()
			}()
			Logger.Messsage("collector[%s]: sleeping %v", m.Name, m.ScrapeTime.String())
			time.Sleep(*m.ScrapeTime)
		}
	}()
}

func (m *MetricCollectorGeneral) Collect() {
	var wg sync.WaitGroup
	var wgCallback sync.WaitGroup

	ctx := context.Background()

	callbackChannel := make(chan func())

	Logger.Messsage(
		"collector[%s]: starting metrics collection",
		m.Name,
	)

	for _, subscription := range m.AzureSubscriptions {
		wg.Add(1)
		go func(ctx context.Context, callback chan<- func(), subscription subscriptions.Subscription) {
			defer wg.Done()
			m.Collector.Collect(ctx, callbackChannel, subscription)
		}(ctx, callbackChannel, subscription)
	}

	// collect metrics (callbacks) and proceses them
	wgCallback.Add(1)
	go func() {
		defer wgCallback.Done()
		var callbackList []func()
		for callback := range callbackChannel {
			callbackList = append(callbackList, callback)
		}

		// reset metric values
		m.Collector.Reset()

		// process callbacks (set metrics)
		for _, callback := range callbackList {
			callback()
		}
	}()

	// wait for all funcs
	wg.Wait()
	close(callbackChannel)
	wgCallback.Wait()

	Logger.Verbose(
		"collector[%s]: finished metrics collection",
		m.Name,
	)
}
