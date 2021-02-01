package main

import (
	"context"

	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/azure-sdk-for-go/profiles/preview/containerservice/mgmt/containerservice"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	prometheusCommon "github.com/webdevops/go-prometheus-common"
)

type MetricsCollectorAzureRmKubernetesService struct {
	CollectorProcessorGeneral

	prometheus struct {
		managedCluster                 *prometheus.GaugeVec
		managedClusterAgentPoolCurrent *prometheus.GaugeVec
		managedClusterAgentPoolLimit   *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorAzureRmKubernetesService) Setup(collector *CollectorGeneral) {
	m.CollectorReference = collector

	m.prometheus.managedCluster = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_kubernetesservice_info",
			Help: "Azure Kuberenetes Service info",
		},
		append(
			[]string{
				"resourceID",
				"subscriptionID",
				"location",
				"name",
				"nodeResourceGroup",
				"kubernetesVersion",
			},
			azureResourceTags.prometheusLabels...,
		),
	)

	m.prometheus.managedClusterAgentPoolCurrent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_kubernetesservice_agentpool_current",
			Help: "Azure Kuberenetes Service Agent Pool current value",
		},
		[]string{
			"resourceID",
			"name",
			"nodeSize",
		},
	)

	m.prometheus.managedClusterAgentPoolLimit = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_kubernetesservice_agentpool_limit",
			Help: "Azure Kuberenetes Service Agent Pool limit",
		},
		[]string{
			"resourceID",
			"name",
			"nodeSize",
		},
	)

	prometheus.MustRegister(m.prometheus.managedCluster)
	prometheus.MustRegister(m.prometheus.managedClusterAgentPoolCurrent)
	prometheus.MustRegister(m.prometheus.managedClusterAgentPoolLimit)
}

func (m *MetricsCollectorAzureRmKubernetesService) Reset() {
	m.prometheus.managedCluster.Reset()
	m.prometheus.managedClusterAgentPoolCurrent.Reset()
	m.prometheus.managedClusterAgentPoolLimit.Reset()
}

func (m *MetricsCollectorAzureRmKubernetesService) Collect(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	client := containerservice.NewManagedClustersClient(*subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer

	list, err := client.ListComplete(ctx)

	if err != nil {
		logger.Panic(err)
	}

	infoMetric := prometheusCommon.NewMetricsList()
	agentPoolMetricCurrent := prometheusCommon.NewMetricsList()
	agentPoolMetricLimit := prometheusCommon.NewMetricsList()

	for list.NotDone() {
		val := list.Value()

		infoLabels := prometheus.Labels{
			"resourceID":        *val.ID,
			"subscriptionID":    *subscription.SubscriptionID,
			"location":          *val.Location,
			"name":              *val.Name,
			"nodeResourceGroup": *val.ManagedClusterProperties.NodeResourceGroup,
			"kubernetesVersion": *val.ManagedClusterProperties.KubernetesVersion,
		}
		infoLabels = azureResourceTags.appendPrometheusLabel(infoLabels, val.Tags)
		infoMetric.AddInfo(infoLabels)

		if val.ManagedClusterProperties.AgentPoolProfiles != nil {
			for _, agentPool := range *val.ManagedClusterProperties.AgentPoolProfiles {
				agentPoolName := *agentPool.Name
				agentPoolNodeSize := stringPtrToString((*string)(&agentPool.VMSize))
				currentValue := float64(*agentPool.Count)
				limitValue := float64(0)
				if agentPool.MaxCount != nil {
					limitValue = float64(*agentPool.MaxCount)
				}
				int32PtrToString(agentPool.MaxCount)

				labels := prometheus.Labels{
					"resourceID": *val.ID,
					"name":       agentPoolName,
					"nodeSize":   agentPoolNodeSize,
				}

				agentPoolMetricCurrent.Add(labels, currentValue)
				agentPoolMetricLimit.Add(labels, limitValue)
			}
		}

		if list.NextWithContext(ctx) != nil {
			break
		}
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.managedCluster)
		agentPoolMetricCurrent.GaugeSet(m.prometheus.managedClusterAgentPoolCurrent)
		agentPoolMetricLimit.GaugeSet(m.prometheus.managedClusterAgentPoolLimit)
	}
}
