package main

import (
	"fmt"
	"context"
	"strconv"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/go-autorest/autorest"
	"github.com/prometheus/client_golang/prometheus"
)

// Collect Azure Subscription metrics
func (m *MetricCollectorAzureRm) collectAzureSubscription(ctx context.Context, subscriptionId string, callback chan<- func()) {
	client := subscriptions.NewClient()
	client.Authorizer = AzureAuthorizer

	sub, err := client.Get(ctx, subscriptionId)
	if err != nil {
		panic(err)
	}

	subscriptionMetric := prometheusMetricRow{
		labels: prometheus.Labels{
			"resourceID": *sub.ID,
			"subscriptionID":      *sub.SubscriptionID,
			"subscriptionName":    *sub.DisplayName,
			"spendingLimit":       string(sub.SubscriptionPolicies.SpendingLimit),
			"quotaID":             *sub.SubscriptionPolicies.QuotaID,
			"locationPlacementID": *sub.SubscriptionPolicies.LocationPlacementID,
		},
		value: 1,
	}

	// subscription rate limits
	m.probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-subscription-reads", prometheus.Labels{"subscriptionID": subscriptionId, "scope": "subscription", "type": "read"}, callback)
	m.probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-subscription-resource-requests", prometheus.Labels{"subscriptionID": subscriptionId, "scope": "subscription", "type": "resource-requests"}, callback)
	m.probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-subscription-resource-entities-read", prometheus.Labels{"subscriptionID": subscriptionId, "scope": "subscription", "type": "resource-entities-read"}, callback)

	// tenant rate limits
	m.probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-tenant-reads", prometheus.Labels{"subscriptionID": subscriptionId, "scope": "tenant", "type": "read"}, callback)
	m.probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-tenant-resource-requests", prometheus.Labels{"subscriptionID": subscriptionId, "scope": "tenant", "type": "resource-requests"}, callback)
	m.probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-tenant-resource-entities-read", prometheus.Labels{"subscriptionID": subscriptionId, "scope": "tenant", "type": "resource-entities-read"}, callback)

	callback <- func() {
		m.prometheus.subscription.With(subscriptionMetric.labels).Set(subscriptionMetric.value)
	}
}


// Collect Azure ResourceGroup metrics
func (m *MetricCollectorAzureRm) collectAzureResourceGroup(ctx context.Context, subscriptionId string, callback chan<- func()) {
	client := resources.NewGroupsClient(subscriptionId)
	client.Authorizer = AzureAuthorizer

	resourceGroupResult, err := client.ListComplete(ctx, "", nil)
	if err != nil {
		panic(err)
	}

	infoMetric := prometheusMetricsList{}

	for _, item := range *resourceGroupResult.Response().Value {
		infoLabels := m.addAzureResourceTags(prometheus.Labels{
			"resourceID": *item.ID,
			"subscriptionID": subscriptionId,
			"resourceGroup": *item.Name,
			"location": *item.Location,
		}, item.Tags)

		infoMetric.Add(infoLabels, 1)
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.resourceGroup)
	}
}


// read header and set prometheus api quota (if found)
func (m *MetricCollectorAzureRm) probeProcessHeader(response autorest.Response, header string, labels prometheus.Labels, callback chan<- func()) {
	if val := response.Header.Get(header); val != "" {
		valFloat, err := strconv.ParseFloat(val, 64)

		if err == nil {
			callback <- func() {
				m.prometheus.apiQuota.With(labels).Set(valFloat)
			}
		} else {
			ErrorLogger.Error(fmt.Sprintf("Failed to parse value '%v':", val), err)
		}
	}
}
