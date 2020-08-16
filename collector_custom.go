package main

import (
	"context"
	"time"
)

type CollectorCustom struct {
	CollectorBase
	Processor CollectorProcessorCustomInterface
}

func (m *CollectorCustom) Run(scrapeTime time.Duration) {
	m.SetScrapeTime(scrapeTime)

	m.Processor.Setup(m)
	go func() {
		for {
			go func() {
				m.Collect()
			}()
			m.sleepUntilNextCollection()
		}
	}()
}

func (m *CollectorCustom) Collect() {
	ctx := context.Background()
	m.collectionStart()
	m.Processor.Collect(ctx, m.logger)
	m.collectionFinish()
}
