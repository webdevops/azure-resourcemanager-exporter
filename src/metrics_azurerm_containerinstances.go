package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/preview/containerinstance/mgmt/containerinstance"
	"github.com/prometheus/client_golang/prometheus"
)

func (m *MetricCollectorAzureRm) collectAzureContainerInstances(ctx context.Context, subscriptionId string, callback chan<- func()) {
	client := containerinstance.NewContainerGroupsClient(subscriptionId)
	client.Authorizer = AzureAuthorizer

	list, err := client.ListComplete(ctx)

	if err != nil {
		panic(err)
	}

	infoMetric := prometheusMetricsList{}
	containerMetric := prometheusMetricsList{}
	containerResourceMetric := prometheusMetricsList{}
	containerPortMetric := prometheusMetricsList{}

	for list.NotDone() {
		val := list.Value()

		infoLabels := prometheus.Labels{
			"resourceID": *val.ID,
			"subscriptionID": subscriptionId,
			"location": *val.Location,
			"instanceName": *val.Name,
			"resourceGroup": extractResourceGroupFromAzureId(*val.ID),
			"osType": string(val.OsType),
			"ipAdress": *val.IPAddress.IP,
		}
		infoLabels = m.addAzureResourceTags(infoLabels, val.Tags)

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
