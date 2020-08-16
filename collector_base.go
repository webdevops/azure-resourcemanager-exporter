package main

import (
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	log "github.com/sirupsen/logrus"
	"time"
)

type CollectorBase struct {
	Name       string
	scrapeTime *time.Duration

	LastScrapeDuration  *time.Duration
	collectionStartTime time.Time

	AzureSubscriptions []subscriptions.Subscription
	AzureLocations     []string

	logger *log.Entry

	isHidden bool
}

func (c *CollectorBase) Init() {
	c.isHidden = false
	c.logger = log.WithField("collector", c.Name)
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
		c.logger.Info("starting metrics collection")
	}
}

func (c *CollectorBase) collectionFinish() {
	duration := time.Now().Sub(c.collectionStartTime)
	c.LastScrapeDuration = &duration

	if !c.isHidden {
		c.logger.WithField("duration", c.LastScrapeDuration.Seconds()).Infof("finished metrics collection (duration: %v)", c.LastScrapeDuration)
	}
}

func (c *CollectorBase) sleepUntilNextCollection() {
	if !c.isHidden {
		c.logger.Debugf("sleeping %v", c.GetScrapeTime().String())
	}
	time.Sleep(*c.GetScrapeTime())
}
