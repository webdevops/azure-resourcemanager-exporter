package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/azure-sdk-for-go/profiles/preview/containerinstance/mgmt/containerinstance"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	prometheusCommon "github.com/webdevops/go-prometheus-common"
)

type MetricsCollectorAzureRmContainerInstances struct {
	CollectorProcessorGeneral

	prometheus struct {
		containerInstance                  *prometheus.GaugeVec
		containerInstanceContainer         *prometheus.GaugeVec
		containerInstanceContainerResource *prometheus.GaugeVec
		containerInstanceContainerPort     *prometheus.GaugeVec
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
			[]string{
				"resourceID",
				"subscriptionID",
				"location",
				"instanceName",
				"resourceGroup",
				"osType",
				"ipAdress",
			},
			azureResourceTags.prometheusLabels...,
		),
	)
	prometheus.MustRegister(m.prometheus.containerInstance)

	m.prometheus.containerInstanceContainer = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_containerinstance_container",
			Help: "Azure ContainerInstance container",
		},
		[]string{
			"resourceID",
			"containerName",
			"containerImage",
			"livenessProbe",
			"readinessProbe",
		},
	)
	prometheus.MustRegister(m.prometheus.containerInstanceContainer)

	m.prometheus.containerInstanceContainerResource = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_containerinstance_container_resource",
			Help: "Azure ContainerInstance container resource",
		},
		[]string{
			"resourceID",
			"containerName",
			"type",
			"resource",
		},
	)
	prometheus.MustRegister(m.prometheus.containerInstanceContainerResource)

	m.prometheus.containerInstanceContainerPort = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_containerinstance_container_port",
			Help: "Azure ContainerInstance container port",
		},
		[]string{
			"resourceID",
			"containerName",
			"protocol",
		},
	)
	prometheus.MustRegister(m.prometheus.containerInstanceContainerPort)
}

func (m *MetricsCollectorAzureRmContainerInstances) Reset() {
	m.prometheus.containerInstance.Reset()
	m.prometheus.containerInstanceContainer.Reset()
	m.prometheus.containerInstanceContainerResource.Reset()
	m.prometheus.containerInstanceContainerPort.Reset()
}

func (m *MetricsCollectorAzureRmContainerInstances) Collect(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	client := containerinstance.NewContainerGroupsClientWithBaseURI(azureEnvironment.ResourceManagerEndpoint, *subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer
	client.ResponseInspector = azureResponseInspector(&subscription)

	list, err := client.ListComplete(ctx)

	if err != nil {
		logger.Panic(err)
	}

	infoMetric := prometheusCommon.NewMetricsList()
	containerMetric := prometheusCommon.NewMetricsList()
	containerResourceMetric := prometheusCommon.NewMetricsList()
	containerPortMetric := prometheusCommon.NewMetricsList()

	for list.NotDone() {
		val := list.Value()

		infoLabels := prometheus.Labels{
			"resourceID":     to.String(val.ID),
			"subscriptionID": to.String(subscription.SubscriptionID),
			"location":       to.String(val.Location),
			"instanceName":   to.String(val.Name),
			"resourceGroup":  extractResourceGroupFromAzureId(to.String(val.ID)),
			"osType":         string(val.OsType),
			"ipAdress":       to.String(val.IPAddress.IP),
		}
		infoLabels = azureResourceTags.appendPrometheusLabel(infoLabels, val.Tags)
		infoMetric.AddInfo(infoLabels)

		if val.Containers != nil {
			for _, container := range *val.Containers {
				containerMetric.AddInfo(prometheus.Labels{
					"resourceID":     to.String(val.ID),
					"containerName":  to.String(container.Name),
					"containerImage": to.String(container.Image),
					"livenessProbe":  boolToString(container.LivenessProbe != nil),
					"readinessProbe": boolToString(container.ReadinessProbe != nil),
				})

				// ports
				if container.Ports != nil {
					for _, port := range *container.Ports {
						containerPortMetric.Add(prometheus.Labels{
							"resourceID":    to.String(val.ID),
							"containerName": to.String(container.Name),
							"protocol":      string(port.Protocol),
						}, float64(*port.Port))
					}
				}

				// resource limit&request
				if container.Resources != nil {
					if container.Resources.Requests != nil {
						if container.Resources.Requests.CPU != nil {
							containerResourceMetric.Add(prometheus.Labels{
								"resourceID":    to.String(val.ID),
								"containerName": to.String(container.Name),
								"type":          "request",
								"resource":      "cpu",
							}, *container.Resources.Requests.CPU)
						}

						if container.Resources.Requests.MemoryInGB != nil {
							containerResourceMetric.Add(prometheus.Labels{
								"resourceID":    to.String(val.ID),
								"containerName": to.String(container.Name),
								"type":          "request",
								"resource":      "memory",
							}, *container.Resources.Requests.MemoryInGB*1073741824)
						}
					}

					if container.Resources.Limits != nil {
						if container.Resources.Limits.CPU != nil {
							containerResourceMetric.Add(prometheus.Labels{
								"resourceID":    to.String(val.ID),
								"containerName": to.String(container.Name),
								"type":          "limit",
								"resource":      "cpu",
							}, *container.Resources.Limits.CPU)
						}

						if container.Resources.Limits.MemoryInGB != nil {
							containerResourceMetric.Add(prometheus.Labels{
								"resourceID":    to.String(val.ID),
								"containerName": to.String(container.Name),
								"type":          "limit",
								"resource":      "memory",
							}, *container.Resources.Limits.MemoryInGB*1073741824)
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
