package main

import (
	"context"
	"log"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/azuresdk/armclient"
	"github.com/webdevops/go-common/prometheus/collector"
	"github.com/webdevops/go-common/utils/to"
	"go.uber.org/zap"
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
		AzureResourceTagManager.AddToPrometheusLabels(
			[]string{
				"resourceID",
				"resourceName",
				"subscriptionID",
				"resourceGroup",
				"resourceType",
				"skuName",
				"tier",
				"provider",
				"location",
				"provisioningState",
			},
		),
	)
	m.Collector.RegisterMetricList("resource", m.prometheus.resource, true)

	m.prometheus.resourceGroup = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_resourcegroup_info",
			Help: "Azure ResourceManager resourcegroup information",
		},
		AzureResourceGroupTagManager.AddToPrometheusLabels(
			[]string{
				"resourceID",
				"subscriptionID",
				"resourceGroup",
				"location",
				"provisioningState",
			},
		),
	)
	m.Collector.RegisterMetricList("resourceGroup", m.prometheus.resourceGroup, true)
}

func (m *MetricsCollectorAzureRmResources) Reset() {}

func (m *MetricsCollectorAzureRmResources) Collect(callback chan<- func()) {
	err := AzureSubscriptionsIterator.ForEachAsync(m.Logger(), func(subscription *armsubscriptions.Subscription, logger *zap.SugaredLogger) {
		m.collectAzureResourceGroup(subscription, logger, callback)
		m.collectAzureResources(subscription, logger, callback)
	})
	if err != nil {
		m.Logger().Panic(err)
	}
}

// Collect Azure ResourceGroup metrics
func (m *MetricsCollectorAzureRmResources) collectAzureResourceGroup(subscription *armsubscriptions.Subscription, logger *zap.SugaredLogger, callback chan<- func()) {
	list, err := AzureClient.ListResourceGroups(m.Context(), *subscription.SubscriptionID)
	if err != nil {
		logger.Panic(err)
	}

	infoMetric := m.Collector.GetMetricList("resourceGroup")

	for _, resourceGroup := range list {
		resourceId := to.String(resourceGroup.ID)
		azureResource, _ := armclient.ParseResourceId(resourceId)

		infoLabels := prometheus.Labels{
			"resourceID":        to.StringLower(resourceGroup.ID),
			"subscriptionID":    azureResource.Subscription,
			"resourceGroup":     azureResource.ResourceGroup,
			"location":          to.StringLower(resourceGroup.Location),
			"provisioningState": to.StringLower(resourceGroup.Properties.ProvisioningState),
		}

		infoLabels = AzureResourceGroupTagManager.AddResourceTagsToPrometheusLabels(m.Context(), infoLabels, resourceId)
		infoMetric.AddInfo(infoLabels)
	}
}

func (m *MetricsCollectorAzureRmResources) collectAzureResources(subscription *armsubscriptions.Subscription, logger *zap.SugaredLogger, callback chan<- func()) {
	list, err := AzureClient.ListResources(m.Context(), *subscription.SubscriptionID)
	if err != nil {
		logger.Panic(err)
	}
	authorizer, err := auth.NewAuthorizerFromEnvironment()
	if err != nil {
		log.Fatalf("Failed to create Azure authorizer: %v", err)
		return
	}
	resourceMetric := m.Collector.GetMetricList("resource")

	// Obtenez le disque Azure
	diskClient := compute.NewDisksClient(*subscription.SubscriptionID)
	diskClient.Authorizer = authorizer

	for _, resource := range list {
		resourceId := to.String(resource.ID)
		azureResource, _ := armclient.ParseResourceId(resourceId)
		if azureResource.ResourceType == "microsoft.compute/disks" {
			resourceGroupName := azureResource.ResourceGroup
			diskName := azureResource.ResourceName

			disk, err := diskClient.Get(context.Background(), resourceGroupName, diskName)
			if err != nil {
				log.Fatalf("Failed to get disk: %v", err)
				return
			}

			if disk.DiskProperties.Tier != nil {
				Labels := prometheus.Labels{
					"subscriptionID":    azureResource.Subscription,
					"resourceID":        stringToStringLower(resourceId),
					"resourceName":      azureResource.ResourceName,
					"resourceGroup":     azureResource.ResourceGroup,
					"provider":          azureResource.ResourceProviderName,
					"resourceType":      azureResource.ResourceType,
					"skuName":           string(disk.Sku.Name),
					"tier":              *disk.DiskProperties.Tier,
					"location":          to.StringLower(resource.Location),
					"provisioningState": to.StringLower(resource.ProvisioningState),
				}
				Labels = AzureResourceTagManager.AddResourceTagsToPrometheusLabels(m.Context(), Labels, resourceId)
				resourceMetric.AddInfo(Labels)
			} else {
				Labels := prometheus.Labels{
					"subscriptionID":    azureResource.Subscription,
					"resourceID":        stringToStringLower(resourceId),
					"resourceName":      azureResource.ResourceName,
					"resourceGroup":     azureResource.ResourceGroup,
					"provider":          azureResource.ResourceProviderName,
					"resourceType":      azureResource.ResourceType,
					"skuName":           string(disk.Sku.Name),
					"tier":              "null",
					"location":          to.StringLower(resource.Location),
					"provisioningState": to.StringLower(resource.ProvisioningState),
				}
				Labels = AzureResourceTagManager.AddResourceTagsToPrometheusLabels(m.Context(), Labels, resourceId)
				resourceMetric.AddInfo(Labels)
			}
		} else {
			Labels := prometheus.Labels{
				"subscriptionID":    azureResource.Subscription,
				"resourceID":        stringToStringLower(resourceId),
				"resourceName":      azureResource.ResourceName,
				"resourceGroup":     azureResource.ResourceGroup,
				"provider":          azureResource.ResourceProviderName,
				"resourceType":      azureResource.ResourceType,
				"skuName":           "null",
				"tier":              "null",
				"location":          to.StringLower(resource.Location),
				"provisioningState": to.StringLower(resource.ProvisioningState),
			}
			Labels = AzureResourceTagManager.AddResourceTagsToPrometheusLabels(m.Context(), Labels, resourceId)
			resourceMetric.AddInfo(Labels)
		}
	}
}
