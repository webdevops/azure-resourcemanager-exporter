package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/network/mgmt/network"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/storage/mgmt/storage"
	"github.com/prometheus/client_golang/prometheus"
)


// Collect Azure ComputeUsage metrics
func (m *MetricCollectorAzureRm) collectAzureComputeUsage(ctx context.Context, subscriptionId string, callback chan<- func()) {
	client := compute.NewUsageClient(subscriptionId)
	client.Authorizer = AzureAuthorizer

	quotaMetric := prometheusMetricsList{}
	quotaCurrentMetric := prometheusMetricsList{}
	quotaLimitMetric := prometheusMetricsList{}

	for _, location := range opts.AzureLocation {
		list, err := client.List(ctx, location)

		if err != nil {
			panic(err)
		}

		for _, val := range list.Values() {
			quotaName := *val.Name.Value
			quotaNameLocalized := *val.Name.LocalizedValue
			currentValue := float64(*val.CurrentValue)
			limitValue := float64(*val.Limit)

			labels := prometheus.Labels{
				"subscriptionID": subscriptionId,
				"location": location,
				"scope": "compute",
				"quota": quotaName,
			}

			infoLabels := prometheus.Labels{
				"subscriptionID": subscriptionId,
				"location": location,
				"scope": "compute",
				"quota": quotaName,
				"quotaName": quotaNameLocalized,
			}

			quotaMetric.Add(infoLabels, 1)
			quotaCurrentMetric.Add(labels, currentValue)
			quotaLimitMetric.Add(labels, limitValue)
		}
	}

	callback <- func() {
		quotaMetric.GaugeSet(m.prometheus.quota)
		quotaCurrentMetric.GaugeSet(m.prometheus.quotaCurrent)
		quotaLimitMetric.GaugeSet(m.prometheus.quotaLimit)
	}
}

// Collect Azure NetworkUsage metrics
func (m *MetricCollectorAzureRm) collectAzureNetworkUsage(ctx context.Context, subscriptionId string, callback chan<- func()) {
	client := network.NewUsagesClient(subscriptionId)
	client.Authorizer = AzureAuthorizer

	quotaMetric := prometheusMetricsList{}
	quotaCurrentMetric := prometheusMetricsList{}
	quotaLimitMetric := prometheusMetricsList{}

	for _, location := range opts.AzureLocation {
		list, err := client.List(ctx, location)

		if err != nil {
			panic(err)
		}

		for _, val := range list.Values() {
			quotaName := *val.Name.Value
			quotaNameLocalized := *val.Name.LocalizedValue
			currentValue := float64(*val.CurrentValue)
			limitValue := float64(*val.Limit)

			labels := prometheus.Labels{
				"subscriptionID": subscriptionId,
				"location": location,
				"scope": "network",
				"quota": quotaName,
			}

			infoLabels := prometheus.Labels{
				"subscriptionID": subscriptionId,
				"location": location,
				"scope": "network",
				"quota": quotaName,
				"quotaName": quotaNameLocalized,
			}

			quotaMetric.Add(infoLabels, 1)
			quotaCurrentMetric.Add(labels, currentValue)
			quotaLimitMetric.Add(labels, limitValue)
		}
	}

	callback <- func() {
		quotaMetric.GaugeSet(m.prometheus.quota)
		quotaCurrentMetric.GaugeSet(m.prometheus.quotaCurrent)
		quotaLimitMetric.GaugeSet(m.prometheus.quotaLimit)
	}
}

// Collect Azure StorageUsage metrics
func (m *MetricCollectorAzureRm) collectAzureStorageUsage(ctx context.Context, subscriptionId string, callback chan<- func()) {
	client := storage.NewUsageClient(subscriptionId)
	client.Authorizer = AzureAuthorizer

	quotaMetric := prometheusMetricsList{}
	quotaCurrentMetric := prometheusMetricsList{}
	quotaLimitMetric := prometheusMetricsList{}

	for _, location := range opts.AzureLocation {
		list, err := client.List(ctx)

		if err != nil {
			panic(err)
		}

		for _, val := range *list.Value {
			quotaName := *val.Name.Value
			quotaNameLocalized := *val.Name.LocalizedValue
			currentValue := float64(*val.CurrentValue)
			limitValue := float64(*val.Limit)

			labels := prometheus.Labels{
				"subscriptionID": subscriptionId,
				"location": location,
				"scope": "storage",
				"quota": quotaName,
			}

			infoLabels := prometheus.Labels{
				"subscriptionID": subscriptionId,
				"location": location,
				"scope": "storage",
				"quota": quotaName,
				"quotaName": quotaNameLocalized,
			}

			quotaMetric.Add(infoLabels, 1)
			quotaCurrentMetric.Add(labels, currentValue)
			quotaLimitMetric.Add(labels, limitValue)
		}
	}

	callback <- func() {
		quotaMetric.GaugeSet(m.prometheus.quota)
		quotaCurrentMetric.GaugeSet(m.prometheus.quotaCurrent)
		quotaLimitMetric.GaugeSet(m.prometheus.quotaLimit)
	}
}
