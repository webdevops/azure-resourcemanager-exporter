package main

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	azureCommon "github.com/webdevops/go-common/azure"
	prometheusCommon "github.com/webdevops/go-common/prometheus"
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
		azureCommon.AddResourceTagsToPrometheusLabelsDefinition(
			[]string{
				"resourceID",
				"resourceName",
				"subscriptionID",
				"resourceGroup",
				"resourceType",
				"provider",
				"location",
				"provisioningState",
			},
			opts.Azure.ResourceTags,
		),
	)
	prometheus.MustRegister(m.prometheus.resource)

	m.prometheus.resourceGroup = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_resourcegroup_info",
			Help: "Azure ResourceManager resourcegroup information",
		},
		azureCommon.AddResourceTagsToPrometheusLabelsDefinition(
			[]string{
				"resourceID",
				"subscriptionID",
				"resourceGroup",
				"location",
				"provisioningState",
			},
			opts.Azure.ResourceGroupTags,
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
		resourceId := to.String(item.ID)
		azureResource, _ := azureCommon.ParseResourceId(resourceId)

		infoLabels := prometheus.Labels{
			"resourceID":        stringPtrToStringLower(item.ID),
			"subscriptionID":    azureResource.Subscription,
			"resourceGroup":     azureResource.ResourceGroup,
			"location":          stringPtrToStringLower(item.Location),
			"provisioningState": stringPtrToStringLower(item.Properties.ProvisioningState),
		}
		infoLabels = azureCommon.AddResourceTagsToPrometheusLabels(infoLabels, item.Tags, opts.Azure.ResourceGroupTags)
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

		resourceId := to.String(val.ID)
		azureResource, _ := azureCommon.ParseResourceId(resourceId)

		infoLabels := prometheus.Labels{
			"subscriptionID":    azureResource.Subscription,
			"resourceID":        stringToStringLower(resourceId),
			"resourceName":      azureResource.ResourceName,
			"resourceGroup":     azureResource.ResourceGroup,
			"provider":          azureResource.ResourceProviderName,
			"resourceType":      azureResource.ResourceType,
			"location":          stringPtrToStringLower(val.Location),
			"provisioningState": stringPtrToStringLower(val.ProvisioningState),
		}
		infoLabels = azureCommon.AddResourceTagsToPrometheusLabels(infoLabels, val.Tags, opts.Azure.ResourceTags)
		resourceMetric.AddInfo(infoLabels)

		if list.NextWithContext(ctx) != nil {
			break
		}
	}

	callback <- func() {
		resourceMetric.GaugeSet(m.prometheus.resource)
	}
}
