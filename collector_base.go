package main

import (
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"time"
)

type CollectorBase struct {
	Name       string
	scrapeTime *time.Duration

	LastScrapeDuration  *time.Duration
	collectionStartTime time.Time

	AzureSubscriptions []subscriptions.Subscription
	AzureLocations     []string

	isHidden bool
}

func (c *CollectorBase) Init() {
	c.isHidden = false
}

func (c *CollectorBase) SetScrapeTime(scrapeTime time.Duration) {
	c.scrapeTime = &scrapeTime
}

func (c *CollectorBase) GetScrapeTime() *time.Duration {
	return c.scrapeTime
}

func (c *CollectorBase) SetIsHidden(v bool) {
	c.isHidden = v
}

func (c *CollectorBase) collectionStart() {
	c.collectionStartTime = time.Now()

	if !c.isHidden {
		Logger.Infof("collector[%s]: starting metrics collection", c.Name)
	}
}

func (c *CollectorBase) collectionFinish() {
	duration := time.Now().Sub(c.collectionStartTime)
	c.LastScrapeDuration = &duration

	if !c.isHidden {
		Logger.Infof("collector[%s]: finished metrics collection (duration: %v)", c.Name, c.LastScrapeDuration)
	}
}

func (c *CollectorBase) sleepUntilNextCollection() {
	if !c.isHidden {
		Logger.Verbosef("collector[%s]: sleeping %v", c.Name, c.GetScrapeTime().String())
	}
	time.Sleep(*c.GetScrapeTime())
}
