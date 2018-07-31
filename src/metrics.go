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
	"github.com/Azure/go-autorest/autorest"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/network/mgmt/network"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/storage/mgmt/storage"
)

var (
	prometheusSubscriptionInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_subscription_info",
			Help: "Azure ResourceManager subscription info",
		},
		[]string{"subscriptionID", "subscriptionName", "spendingLimit", "quotaID", "locationPlacementID"},
	)

	prometheusApiQuota = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_ratelimit",
			Help: "Azure ResourceManager ratelimit",
		},
		[]string{"subscriptionID", "scope", "type"},
	)


	prometheusQuotaInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_quota_info",
			Help: "Azure ResourceManager quota info",
		},
		[]string{"subscriptionID", "location", "scope", "quota", "quotaName"},
	)

	prometheusQuotaCurrent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_quota_current",
			Help: "Azure ResourceManager quota current value",
		},
		[]string{"subscriptionID", "location", "scope", "quota"},
	)

	prometheusQuotaLimit = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_quota_limit",
			Help: "Azure ResourceManager quota limit",
		},
		[]string{"subscriptionID", "location", "scope", "quota"},
	)
)

func initMetrics() {
	prometheus.MustRegister(prometheusSubscriptionInfo)
	prometheus.MustRegister(prometheusApiQuota)
	prometheus.MustRegister(prometheusQuotaInfo)
	prometheus.MustRegister(prometheusQuotaCurrent)
	prometheus.MustRegister(prometheusQuotaLimit)

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

	for _, subscription := range AzureSubscriptions {
		subscriptionClient := subscriptions.NewClient()
		subscriptionClient.Authorizer = AzureAuthorizer

		sub, err := subscriptionClient.Get(context, *subscription.SubscriptionID)
		if err != nil {
			panic(err)
		}

		prometheusSubscriptionInfo.With(
			prometheus.Labels{
				"subscriptionID": *sub.SubscriptionID,
				"subscriptionName": *sub.DisplayName,
				"spendingLimit": string(sub.SubscriptionPolicies.SpendingLimit),
				"quotaID": *sub.SubscriptionPolicies.QuotaID,
				"locationPlacementID": *sub.SubscriptionPolicies.LocationPlacementID,
			},
		).Set(1)

		// subscription rate limits
		probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-subscription-reads", prometheus.Labels{"subscriptionID": *subscription.SubscriptionID, "scope": "subscription", "type": "read"})
		probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-subscription-resource-requests", prometheus.Labels{"subscriptionID": *subscription.SubscriptionID, "scope": "subscription", "type": "resource-requests"})
		probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-subscription-resource-entities-read", prometheus.Labels{"subscriptionID": *subscription.SubscriptionID, "scope": "subscription", "type": "resource-entities-read"})

		// tenant rate limits
		probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-tenant-reads", prometheus.Labels{"subscriptionID": *subscription.SubscriptionID, "scope": "tenant", "type": "read"})
		probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-tenant-resource-requests", prometheus.Labels{"subscriptionID": *subscription.SubscriptionID, "scope": "tenant", "type": "resource-requests"})
		probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-tenant-resource-entities-read", prometheus.Labels{"subscriptionID": *subscription.SubscriptionID, "scope": "tenant", "type": "resource-entities-read"})

		// compute usage
		computeClient := compute.NewUsageClient(*sub.SubscriptionID)
		computeClient.Authorizer = AzureAuthorizer
		for _, location := range opts.AzureLocation {
			list, err := computeClient.List(context, location)

			if err != nil {
				panic(err)
			}

			for _, val := range list.Values() {
				labels := prometheus.Labels{"subscriptionID": *sub.SubscriptionID, "location": location, "scope": "compute", "quota": *val.Name.Value}
				infoLabels := prometheus.Labels{"subscriptionID": *sub.SubscriptionID, "location": location, "scope": "compute", "quota": *val.Name.Value, "quotaName": *val.Name.LocalizedValue}
				prometheusQuotaInfo.With(infoLabels).Set(1)
				prometheusQuotaCurrent.With(labels).Set(float64(*val.CurrentValue))
				prometheusQuotaLimit.With(labels).Set(float64(*val.Limit))
			}
		}

		// network usage
		// disabled due to
		// https://github.com/Azure/azure-sdk-for-go/issues/2340
		// https://github.com/Azure/azure-rest-api-specs/issues/1624
		networkClient := network.NewUsagesClient(*sub.SubscriptionID)
		networkClient.Authorizer = AzureAuthorizer
		//for _, location := range opts.AzureLocation {
		//	list, err := networkClient.List(context, location)
		//
		//	if err != nil {
		//		panic(err)
		//	}
		//
		//	for _, val := range list.Values() {
		//      labels := prometheus.Labels{"subscriptionID": *sub.SubscriptionID, "location": location, "scope": "storage", "quota": *val.Name.Value}
		//      infoLabels := prometheus.Labels{"subscriptionID": *sub.SubscriptionID, "location": location, "scope": "storage", "quota": *val.Name.Value, "quotaName": *val.Name.LocalizedValue}
		//		prometheusQuotaInfo.With(infoLabels).Set(1)
		//		prometheusQuotaCurrent.With(labels).Set(float64(*val.CurrentValue))
		//		prometheusQuotaLimit.With(labels).Set(float64(*val.Limit))
		//	}
		//}

		// storage usage
		storageClient := storage.NewUsageClient(*sub.SubscriptionID)
		storageClient.Authorizer = AzureAuthorizer
		for _, location := range opts.AzureLocation {
			list, err := storageClient.List(context)

			if err != nil {
				panic(err)
			}

			for _, val := range *list.Value {
				labels := prometheus.Labels{"subscriptionID": *sub.SubscriptionID, "location": location, "scope": "storage", "quota": *val.Name.Value}
				infoLabels := prometheus.Labels{"subscriptionID": *sub.SubscriptionID, "location": location, "scope": "storage", "quota": *val.Name.Value, "quotaName": *val.Name.LocalizedValue}
				prometheusQuotaInfo.With(infoLabels).Set(1)
				prometheusQuotaCurrent.With(labels).Set(float64(*val.CurrentValue))
				prometheusQuotaLimit.With(labels).Set(float64(*val.Limit))
			}
		}

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
