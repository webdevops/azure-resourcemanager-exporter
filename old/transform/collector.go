package old

import (
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"sync"
	"time"
	"context"
)

type MetricCollectorInterface interface {
	Collect()
}

type MetricCollector struct {
	collector MetricCollectorInterface
	Name string
	scrapeTime  *time.Duration
	ctx *context.Context
}

func (m *MetricCollector) Init(scrapeTime time.Duration) {
	m.scrapeTime = &scrapeTime
	ctx := context.Background()
	m.ctx = &ctx
}

func (m *MetricCollector) Run() {
	go func() {
		for {
			go func() {
				m.collector.Collect()
			}()
			time.Sleep(*m.scrapeTime)
		}
	}()
}

func (m *MetricCollector) Collect() {
	var wg sync.WaitGroup

	callbackChannel := make(chan func())

	for _, subscription := range AzureSubscriptions {
		Logger.Messsage(
			"subscription[%v]: starting metrics collection",
			*subscription.SubscriptionID,
		)

		m.collectMetrics(callbackChannel, subscription)
	}

	// collect metrics (callbacks) and proceses them
	go func() {
		var callbackList []func()
		for callback := range callbackChannel {
			callbackList = append(callbackList, callback)
		}

		// reset metric values
		m.reset()

		// process callbacks (set metrics)
		for _, callback := range callbackList {
			callback()
		}

		Logger.Messsage("run[%s]: finished", m.Name)
	}()

	// wait for all funcs
	wg.Wait()
	close(callbackChannel)
}
