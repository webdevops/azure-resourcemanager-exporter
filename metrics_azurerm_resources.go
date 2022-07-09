package main

import (
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armresources"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	azureCommon "github.com/webdevops/go-common/azure"
	prometheusCommon "github.com/webdevops/go-common/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
	"github.com/webdevops/go-common/utils/to"
)

type MetricsCollectorAzureRmResources struct {
	collector.Processor

	prometheus struct {
		resource      *prometheus.GaugeVec
		resourceGroup *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorAzureRmResources) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

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

func (m *MetricsCollectorAzureRmResources) Collect(callback chan<- func()) {
	err := AzureSubscriptionsIterator.ForEachAsync(m.Logger(), func(subscription *armsubscriptions.Subscription, logger *log.Entry) {
		m.collectAzureResourceGroup(subscription, logger, callback)
		m.collectAzureResources(subscription, logger, callback)

	})
	if err != nil {
		m.Logger().Panic(err)
	}
}

// Collect Azure ResourceGroup metrics
func (m *MetricsCollectorAzureRmResources) collectAzureResourceGroup(subscription *armsubscriptions.Subscription, logger *log.Entry, callback chan<- func()) {
	client, err := armresources.NewResourceGroupsClient(*subscription.SubscriptionID, AzureClient.GetCred(), nil)
	if err != nil {
		logger.Panic(err)
	}

	infoMetric := prometheusCommon.NewMetricsList()

	pager := client.NewListPager(nil)

	for pager.More() {
		nextResult, err := pager.NextPage(m.Context())
		if err != nil {
			logger.Panic(err)
		}

		if nextResult.ResourceGroupListResult.Value != nil {
			for _, resourceGroup := range nextResult.ResourceGroupListResult.Value {
				resourceId := to.String(resourceGroup.ID)
				azureResource, _ := azureCommon.ParseResourceId(resourceId)

				infoLabels := prometheus.Labels{
					"resourceID":        stringPtrToStringLower(resourceGroup.ID),
					"subscriptionID":    azureResource.Subscription,
					"resourceGroup":     azureResource.ResourceGroup,
					"location":          stringPtrToStringLower(resourceGroup.Location),
					"provisioningState": stringPtrToStringLower(resourceGroup.Properties.ProvisioningState),
				}
				infoLabels = azureCommon.AddResourceTagsToPrometheusLabels(infoLabels, resourceGroup.Tags, opts.Azure.ResourceGroupTags)
				infoMetric.AddInfo(infoLabels)
			}
		}
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.resourceGroup)
	}
}

func (m *MetricsCollectorAzureRmResources) collectAzureResources(subscription *armsubscriptions.Subscription, logger *log.Entry, callback chan<- func()) {
	client, err := armresources.NewClient(*subscription.SubscriptionID, AzureClient.GetCred(), nil)
	if err != nil {
		logger.Panic(err)
	}

	resourceMetric := prometheusCommon.NewMetricsList()

	pager := client.NewListPager(nil)

	for pager.More() {
		nextResult, err := pager.NextPage(m.Context())
		if err != nil {
			logger.Panic(err)
		}

		if nextResult.ResourceListResult.Value != nil {
			for _, resource := range nextResult.ResourceListResult.Value {
				resourceId := to.String(resource.ID)
				azureResource, _ := azureCommon.ParseResourceId(resourceId)

				infoLabels := prometheus.Labels{
					"subscriptionID":    azureResource.Subscription,
					"resourceID":        stringToStringLower(resourceId),
					"resourceName":      azureResource.ResourceName,
					"resourceGroup":     azureResource.ResourceGroup,
					"provider":          azureResource.ResourceProviderName,
					"resourceType":      azureResource.ResourceType,
					"location":          stringPtrToStringLower(resource.Location),
					"provisioningState": stringPtrToStringLower(resource.ProvisioningState),
				}
				infoLabels = azureCommon.AddResourceTagsToPrometheusLabels(infoLabels, resource.Tags, opts.Azure.ResourceTags)
				resourceMetric.AddInfo(infoLabels)
			}
		}
	}

	callback <- func() {
		resourceMetric.GaugeSet(m.prometheus.resource)
	}
}
