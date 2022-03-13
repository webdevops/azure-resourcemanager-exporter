package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	prometheusCommon "github.com/webdevops/go-prometheus-common"
	"strings"
)

type MetricsCollectorAzureRmResources struct {
	CollectorProcessorGeneral

	prometheus struct {
		resource      *prometheus.GaugeVec
		resourceGroup *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorAzureRmResources) Setup(collector *CollectorGeneral) {
	m.CollectorReference = collector

	m.prometheus.resource = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_resource_info",
			Help: "Azure Resource information",
		},
		append(
			[]string{
				"resourceID",
				"resourceName",
				"subscriptionID",
				"resourceGroup",
				"provider",
				"location",
				"provisioningState",
			},
			azureResourceTags.prometheusLabels...,
		),
	)
	prometheus.MustRegister(m.prometheus.resource)

	m.prometheus.resourceGroup = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_resourcegroup_info",
			Help: "Azure ResourceManager resourcegroup information",
		},
		append(
			[]string{
				"resourceID",
				"subscriptionID",
				"resourceGroup",
				"location",
				"provisioningState",
			},
			azureResourceGroupTags.prometheusLabels...,
		),
	)
	prometheus.MustRegister(m.prometheus.resourceGroup)
}

func (m *MetricsCollectorAzureRmResources) Reset() {
	m.prometheus.resource.Reset()
	m.prometheus.resourceGroup.Reset()
}

func (m *MetricsCollectorAzureRmResources) Collect(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	m.collectAzureResourceGroup(ctx, logger, callback, subscription)
	m.collectAzureResources(ctx, logger, callback, subscription)
}

// Collect Azure ResourceGroup metrics
func (m *MetricsCollectorAzureRmResources) collectAzureResourceGroup(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	client := resources.NewGroupsClientWithBaseURI(azureEnvironment.ResourceManagerEndpoint, *subscription.SubscriptionID)
	decorateAzureAutorest(&client.Client)

	resourceGroupResult, err := client.ListComplete(ctx, "", nil)
	if err != nil {
		logger.Panic(err)
	}

	infoMetric := prometheusCommon.NewMetricsList()

	for _, item := range *resourceGroupResult.Response().Value {
		infoLabels := azureResourceGroupTags.appendPrometheusLabel(prometheus.Labels{
			"resourceID":        stringPtrToAzureResourceInfo(item.ID),
			"subscriptionID":    stringPtrToAzureResourceInfo(subscription.SubscriptionID),
			"resourceGroup":     stringPtrToAzureResourceInfo(item.Name),
			"location":          stringPtrToAzureResourceInfo(item.Location),
			"provisioningState": strings.ToLower(to.String(item.Properties.ProvisioningState)),
		}, item.Tags)
		infoMetric.AddInfo(infoLabels)
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.resourceGroup)
	}
}

func (m *MetricsCollectorAzureRmResources) collectAzureResources(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	client := resources.NewClientWithBaseURI(azureEnvironment.ResourceManagerEndpoint, *subscription.SubscriptionID)
	decorateAzureAutorest(&client.Client)

	list, err := client.ListComplete(ctx, "", "createdTime,changedTime,provisioningState", nil)

	if err != nil {
		logger.Panic(err)
	}

	resourceMetric := prometheusCommon.NewMetricsList()

	for list.NotDone() {
		val := list.Value()

		infoLabels := prometheus.Labels{
			"subscriptionID":    stringPtrToAzureResourceInfo(subscription.SubscriptionID),
			"resourceID":        stringPtrToAzureResourceInfo(val.ID),
			"resourceName":      stringPtrToAzureResourceInfo(val.Name),
			"resourceGroup":     extractResourceGroupFromAzureId(to.String(val.ID)),
			"provider":          extractProviderFromAzureId(to.String(val.ID)),
			"location":          stringPtrToAzureResourceInfo(val.Location),
			"provisioningState": stringPtrToAzureResourceInfo(val.ProvisioningState),
		}
		infoLabels = azureResourceTags.appendPrometheusLabel(infoLabels, val.Tags)
		resourceMetric.AddInfo(infoLabels)

		if list.NextWithContext(ctx) != nil {
			break
		}
	}

	callback <- func() {
		resourceMetric.GaugeSet(m.prometheus.resource)
	}
}
