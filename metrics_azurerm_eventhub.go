package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/eventhub/mgmt/eventhub"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/prometheus/client_golang/prometheus"
)

type MetricsCollectorAzureRmEventhub struct {
	CollectorProcessorGeneral

	prometheus struct {
		namespace               *prometheus.GaugeVec
		namespaceStatus         *prometheus.GaugeVec
		namespaceEventhub       *prometheus.GaugeVec
		namespaceEventhubStatus *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorAzureRmEventhub) Setup(collector *CollectorGeneral) {
	m.CollectorReference = collector

	m.prometheus.namespace = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_eventhub_namespace_info",
			Help: "Azure ResourceManager EventHub namespaces",
		},
		append(
			[]string{
				"resourceID",
				"subscriptionID",
				"resourceGroup",
				"location",
				"namespace",
				"skuName",
				"skuTier",
				"skuCapacity",
				"isAutoInflateEnabled",
				"kafkaEnabled",
			},
			opts.azureResourceTags.prometheusLabels...,
		),
	)

	m.prometheus.namespaceStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_eventhub_namespace_status",
			Help: "Azure ResourceManager EventHub namespaces",
		},
		[]string{
			"resourceID",
			"type",
		},
	)

	m.prometheus.namespaceEventhub = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_eventhub_namespace_eventhub_info",
			Help: "Azure ResourceManager EventHub namespace eventhub",
		},
		append(
			[]string{
				"resourceID",
				"namespace",
				"name",
			},
			opts.azureResourceTags.prometheusLabels...,
		),
	)

	m.prometheus.namespaceEventhubStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_eventhub_namespace_eventhub_status",
			Help: "Azure ResourceManager EventHub namespace eventhub",
		},
		[]string{
			"resourceID",
			"namespace",
			"name",
			"type",
		},
	)

	prometheus.MustRegister(m.prometheus.namespace)
	prometheus.MustRegister(m.prometheus.namespaceStatus)
	prometheus.MustRegister(m.prometheus.namespaceEventhub)
	prometheus.MustRegister(m.prometheus.namespaceEventhubStatus)
}

func (m *MetricsCollectorAzureRmEventhub) Reset() {
	m.prometheus.namespace.Reset()
	m.prometheus.namespaceEventhub.Reset()
}

func (m *MetricsCollectorAzureRmEventhub) Collect(ctx context.Context, callback chan<- func(), subscription subscriptions.Subscription) {
	namespaceClient := eventhub.NewNamespacesClient(*subscription.SubscriptionID)
	namespaceClient.Authorizer = AzureAuthorizer

	eventhubClient := eventhub.NewEventHubsClient(*subscription.SubscriptionID)
	eventhubClient.Authorizer = AzureAuthorizer

	namespaceResult, err := namespaceClient.ListComplete(ctx)
	if err != nil {
		panic(err)
	}

	namespaceMetric := MetricCollectorList{}
	namespaceStatusMetric := MetricCollectorList{}
	namespaceEventhubMetric := MetricCollectorList{}
	namespaceEventhubStatusMetric := MetricCollectorList{}

	for namespaceResult.NotDone() {
		namespace := namespaceResult.Value()

		resourceGroup := extractResourceGroupFromAzureId(*namespace.ID)

		infoLabels := prometheus.Labels{
			"resourceID":           *namespace.ID,
			"subscriptionID":       *subscription.SubscriptionID,
			"resourceGroup":        resourceGroup,
			"location":             stringPtrToString(namespace.Location),
			"namespace":            stringPtrToString(namespace.Name),
			"skuName":              string(namespace.Sku.Name),
			"skuTier":              string(namespace.Sku.Tier),
			"skuCapacity":          int32ToString(*namespace.Sku.Capacity),
			"isAutoInflateEnabled": boolToString(*namespace.IsAutoInflateEnabled),
			"kafkaEnabled":         boolToString(*namespace.KafkaEnabled),
		}
		infoLabels = opts.azureResourceTags.appendPrometheusLabel(infoLabels, namespace.Tags)
		namespaceMetric.AddInfo(infoLabels)

		if namespace.MaximumThroughputUnits != nil {
			statusLabels := prometheus.Labels{
				"resourceID": *namespace.ID,
				"type":       "maximumThroughputUnits",
			}
			namespaceStatusMetric.Add(statusLabels, float64(*namespace.MaximumThroughputUnits))
		}

		eventhubResult, err := eventhubClient.ListByNamespaceComplete(ctx, resourceGroup, *namespace.Name, nil, nil)
		if err != nil {
			panic(err)
		}

		for eventhubResult.NotDone() {
			eventhub := eventhubResult.Value()

			infoLabels := prometheus.Labels{
				"resourceID": *eventhub.ID,
				"namespace":  stringPtrToString(namespace.Name),
				"name":       stringPtrToString(eventhub.Name),
			}
			infoLabels = opts.azureResourceTags.appendPrometheusLabel(infoLabels, namespace.Tags)
			namespaceEventhubMetric.AddInfo(infoLabels)

			if eventhub.PartitionCount != nil {
				statusLabels := prometheus.Labels{
					"resourceID": *eventhub.ID,
					"namespace":  stringPtrToString(namespace.Name),
					"name":       stringPtrToString(eventhub.Name),
					"type":       "partitionCount",
				}
				namespaceEventhubStatusMetric.Add(statusLabels, float64(*eventhub.PartitionCount))
			}

			if eventhub.MessageRetentionInDays != nil {
				statusLabels := prometheus.Labels{
					"resourceID": *eventhub.ID,
					"namespace":  stringPtrToString(namespace.Name),
					"name":       stringPtrToString(eventhub.Name),
					"type":       "messageRetentionInDays",
				}
				namespaceEventhubStatusMetric.Add(statusLabels, float64(*eventhub.MessageRetentionInDays))
			}

			if eventhubResult.NextWithContext(ctx) != nil {
				break
			}
		}

		if namespaceResult.NextWithContext(ctx) != nil {
			break
		}
	}

	callback <- func() {
		namespaceMetric.GaugeSet(m.prometheus.namespace)
		namespaceStatusMetric.GaugeSet(m.prometheus.namespaceStatus)
		namespaceEventhubMetric.GaugeSet(m.prometheus.namespaceEventhub)
		namespaceEventhubStatusMetric.GaugeSet(m.prometheus.namespaceEventhubStatus)
	}
}
