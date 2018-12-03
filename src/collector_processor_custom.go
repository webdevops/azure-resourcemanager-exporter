package main

import (
	"context"
)

type CollectorProcessorCustomInterface interface {
	Setup(collector *CollectorCustom)
	Collect(ctx context.Context)
}

type CollectorProcessorCustom struct {
	CollectorProcessorCustomInterface
	CollectorReference *CollectorCustom
}

func NewCollectorCustom(name string, processor CollectorProcessorCustomInterface) (*CollectorCustom) {
	collector := CollectorCustom{
		Name: name,
		AzureSubscriptions: AzureSubscriptions,
		AzureLocations: opts.AzureLocation,
		Processor: processor,
	}

	return &collector
}
