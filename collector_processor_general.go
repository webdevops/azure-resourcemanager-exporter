package main

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	log "github.com/sirupsen/logrus"
)

type CollectorProcessorGeneralInterface interface {
	Setup(collector *CollectorGeneral)
	Reset()
	Collect(ctx context.Context, contextLogger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription)
}

type CollectorProcessorGeneral struct {
	CollectorProcessorGeneralInterface
	CollectorReference *CollectorGeneral
}

func NewCollectorGeneral(name string, processor CollectorProcessorGeneralInterface) *CollectorGeneral {
	collector := CollectorGeneral{
		CollectorBase: CollectorBase{
			Name:               name,
			AzureSubscriptions: AzureSubscriptions,
			AzureLocations:     opts.Azure.Location,
		},

		Processor: processor,
	}
	collector.CollectorBase.Init()

	return &collector
}

func (c *CollectorProcessorGeneral) logger() *log.Entry {
	return c.CollectorReference.logger
}
