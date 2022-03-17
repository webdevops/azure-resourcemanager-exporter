package main

import (
	"context"

	log "github.com/sirupsen/logrus"
)

type CollectorProcessorCustomInterface interface {
	Setup(collector *CollectorCustom)
	Collect(ctx context.Context, contextLogger *log.Entry)
}

type CollectorProcessorCustom struct {
	CollectorProcessorCustomInterface
	CollectorReference *CollectorCustom
}

func NewCollectorCustom(name string, processor CollectorProcessorCustomInterface) *CollectorCustom {
	collector := CollectorCustom{
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

func (c *CollectorProcessorCustom) logger() *log.Entry {
	return c.CollectorReference.logger
}
