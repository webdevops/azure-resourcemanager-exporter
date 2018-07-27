package main

import (
	"fmt"
	"time"
	"log"
	"strconv"
	"context"
	"net/http"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/go-autorest/autorest"
)

var (
	prometheusApiQuota = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_ratelimit",
			Help: "Azure ResourceManager ratelimit",
		},
		[]string{"subscription", "scope", "type"},
	)
)

func initMetrics() {
	prometheus.MustRegister(prometheusApiQuota)

	go func() {
		for {
			go func() {
				probeCollect()
			}()
			time.Sleep(time.Duration(opts.ScrapeTime) * time.Second)
		}
	}()
}

func startHttpServer() {
	http.Handle("/metrics", promhttp.Handler())
	log.Fatal(http.ListenAndServe(opts.ServerBind, nil))
}

func probeCollect() {
	context := context.Background()

	prometheusApiQuota.Reset()

	for _, subscription := range AzureSubscriptions {
		subscriptionClient := subscriptions.NewClient()
		subscriptionClient.Authorizer = AzureAuthorizer

		sub, err := subscriptionClient.Get(context, *subscription.SubscriptionID)
		if err != nil {
			panic(err)
		}

		// subscription rate limits
		probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-subscription-reads", prometheus.Labels{"subscription": *subscription.SubscriptionID, "scope": "subscription", "type": "read"})
		probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-subscription-resource-requests", prometheus.Labels{"subscription": *subscription.SubscriptionID, "scope": "subscription", "type": "resource-requests"})
		probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-subscription-resource-entities-read", prometheus.Labels{"subscription": *subscription.SubscriptionID, "scope": "subscription", "type": "resource-entities-read"})

		// tenant rate limits
		probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-tenant-reads", prometheus.Labels{"subscription": *subscription.SubscriptionID, "scope": "tenant", "type": "read"})
		probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-tenant-resource-requests", prometheus.Labels{"subscription": *subscription.SubscriptionID, "scope": "tenant", "type": "resource-requests"})
		probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-tenant-resource-entities-read", prometheus.Labels{"subscription": *subscription.SubscriptionID, "scope": "tenant", "type": "resource-entities-read"})
	}
}

func probeProcessHeader(response autorest.Response, header string, labels prometheus.Labels) {
	if val := response.Header.Get(header); val != "" {
		valFloat, err := strconv.ParseFloat(val, 64)

		if err == nil {
			prometheusApiQuota.With(labels).Set(valFloat)
		} else {
			ErrorLogger.Error(fmt.Sprintf("Failed to parse value '%v':", val), err)
		}
	}
}
