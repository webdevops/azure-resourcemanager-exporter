package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/azure-sdk-for-go/profiles/preview/containerinstance/mgmt/containerinstance"
	"github.com/prometheus/client_golang/prometheus"
)

type MetricsCollectorAzureRmContainerInstances struct {
	CollectorProcessorGeneral

	prometheus struct {
		containerInstance *prometheus.GaugeVec
		containerInstanceContainer *prometheus.GaugeVec
		containerInstanceContainerResource *prometheus.GaugeVec
		containerInstanceContainerPort *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorAzureRmContainerInstances) Setup(collector *CollectorGeneral) {
	m.CollectorReference = collector

	m.prometheus.containerInstance = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_containerinstance_info",
			Help: "Azure ContainerInstance limit",
		},
		append(
			[]string{"resourceID", "subscriptionID", "location", "instanceName", "resourceGroup", "osType", "ipAdress"},
			prefixSlice(AZURE_RESOURCE_TAG_PREFIX, opts.AzureResourceTags)...
		),
	)

	m.prometheus.containerInstanceContainer = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_containerinstance_container",
			Help: "Azure ContainerInstance container",
		},
		[]string{"resourceID", "containerName", "containerImage", "livenessProbe", "readinessProbe"},
	)

	m.prometheus.containerInstanceContainerResource = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_containerinstance_container_resource",
			Help: "Azure ContainerInstance container resource",
		},
		[]string{"resourceID", "containerName", "type", "resource"},
	)

	m.prometheus.containerInstanceContainerPort = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_containerinstance_container_port",
			Help: "Azure ContainerInstance container port",
		},
		[]string{"resourceID", "containerName", "protocol"},
	)

	prometheus.MustRegister(m.prometheus.containerInstance)
	prometheus.MustRegister(m.prometheus.containerInstanceContainer)
	prometheus.MustRegister(m.prometheus.containerInstanceContainerResource)
	prometheus.MustRegister(m.prometheus.containerInstanceContainerPort)
}

func (m *MetricsCollectorAzureRmContainerInstances) Reset() {
	m.prometheus.containerInstance.Reset()
	m.prometheus.containerInstanceContainer.Reset()
	m.prometheus.containerInstanceContainerResource.Reset()
	m.prometheus.containerInstanceContainerPort.Reset()
}

func (m *MetricsCollectorAzureRmContainerInstances) Collect(ctx context.Context, callback chan<- func(), subscription subscriptions.Subscription) {
	client := containerinstance.NewContainerGroupsClient(*subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer

	list, err := client.ListComplete(ctx)

	if err != nil {
		panic(err)
	}

	infoMetric := MetricCollectorList{}
	containerMetric := MetricCollectorList{}
	containerResourceMetric := MetricCollectorList{}
	containerPortMetric := MetricCollectorList{}

	for list.NotDone() {
		val := list.Value()

		infoLabels := prometheus.Labels{
			"resourceID": *val.ID,
			"subscriptionID": *subscription.SubscriptionID,
			"location": *val.Location,
			"instanceName": *val.Name,
			"resourceGroup": extractResourceGroupFromAzureId(*val.ID),
			"osType": string(val.OsType),
			"ipAdress": *val.IPAddress.IP,
		}
		infoLabels = addAzureResourceTags(infoLabels, val.Tags)
		infoMetric.Add(infoLabels, 1)

		if val.Containers != nil {
			for _, container := range *val.Containers {
				containerMetric.Add(prometheus.Labels{
					"resourceID": *val.ID,
					"containerName": *container.Name,
					"containerImage": *container.Image,
					"livenessProbe": boolToString(container.LivenessProbe != nil),
					"readinessProbe": boolToString(container.ReadinessProbe != nil),
				}, 1)

				// ports
				if container.Ports != nil {
					for _, port := range *container.Ports {
						containerPortMetric.Add(prometheus.Labels{
							"resourceID": *val.ID,
							"containerName": *container.Name,
							"protocol": string(port.Protocol),
						}, float64(*port.Port))
					}
				}

				// resource limit&request
				if container.Resources != nil {
					if container.Resources.Requests != nil {
						if container.Resources.Requests.CPU != nil {
							containerResourceMetric.Add(prometheus.Labels{
								"resourceID": *val.ID,
								"containerName": *container.Name,
								"type": "request",
								"resource": "cpu",
							}, *container.Resources.Requests.CPU)
						}

						if container.Resources.Requests.MemoryInGB != nil {
							containerResourceMetric.Add(prometheus.Labels{
								"resourceID": *val.ID,
								"containerName": *container.Name,
								"type": "request",
								"resource": "memory",
							}, *container.Resources.Requests.MemoryInGB * 1073741824)
						}
					}

					if container.Resources.Limits != nil {
						if container.Resources.Limits.CPU != nil {
							containerResourceMetric.Add(prometheus.Labels{
								"resourceID": *val.ID,
								"containerName": *container.Name,
								"type": "limit",
								"resource": "cpu",
							}, *container.Resources.Limits.CPU)
						}

						if container.Resources.Limits.MemoryInGB != nil {
							containerResourceMetric.Add(prometheus.Labels{
								"resourceID": *val.ID,
								"containerName": *container.Name,
								"type": "limit",
								"resource": "memory",
							}, *container.Resources.Limits.MemoryInGB * 1073741824)
						}
					}
				}

			}
		}

		if list.NextWithContext(ctx) != nil {
			break
		}
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.containerInstance)
		containerMetric.GaugeSet(m.prometheus.containerInstanceContainer)
		containerResourceMetric.GaugeSet(m.prometheus.containerInstanceContainerResource)
		containerPortMetric.GaugeSet(m.prometheus.containerInstanceContainerPort)
	}
}
