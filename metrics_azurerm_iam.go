package main

import (
	"log/slog"

	armauthorization "github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization/v2"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/azuresdk/armclient"
	"github.com/webdevops/go-common/prometheus/collector"
	"github.com/webdevops/go-common/utils/to"
)

type MetricsCollectorAzureRmIam struct {
	collector.Processor

	prometheus struct {
		roleAssignmentCount *prometheus.GaugeVec
		roleAssignment      *prometheus.GaugeVec
		roleDefinition      *prometheus.GaugeVec
		principal           *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorAzureRmIam) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	m.prometheus.roleAssignmentCount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_iam_roleassignment_count",
			Help: "Azure IAM RoleAssignment count",
		},
		[]string{
			"subscriptionID",
		},
	)
	m.Collector.RegisterMetricList("roleAssignmentCount", m.prometheus.roleAssignmentCount, true)

	m.prometheus.roleAssignment = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_iam_roleassignment_info",
			Help: "Azure IAM RoleAssignment information",
		},
		[]string{
			"subscriptionID",
			"roleAssignmentID",
			"resourceID",
			"resourceGroup",
			"principalID",
			"roleDefinitionID",
		},
	)
	m.Collector.RegisterMetricList("roleAssignment", m.prometheus.roleAssignment, true)

	m.prometheus.roleDefinition = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_iam_roledefinition_info",
			Help: "Azure IAM RoleDefinition information",
		},
		[]string{
			"subscriptionID",
			"roleDefinitionID",
			"name",
			"roleName",
			"roleType",
		},
	)
	m.Collector.RegisterMetricList("roleDefinition", m.prometheus.roleDefinition, true)

	m.prometheus.principal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_iam_principal_info",
			Help: "Azure IAM Principal information",
		},
		[]string{
			"subscriptionID",
			"principalID",
			"principalName",
			"principalType",
		},
	)
	m.Collector.RegisterMetricList("principal", m.prometheus.principal, true)
}

func (m *MetricsCollectorAzureRmIam) Reset() {}

func (m *MetricsCollectorAzureRmIam) Collect(callback chan<- func()) {
	err := AzureSubscriptionsIterator.ForEachAsync(m.Logger(), func(subscription *armsubscriptions.Subscription, logger *slog.Logger) {
		m.collectRoleDefinitions(subscription, logger, callback)
		m.collectRoleAssignments(subscription, logger, callback)
	})
	if err != nil {
		panic(err)
	}
}

func (m *MetricsCollectorAzureRmIam) collectRoleDefinitions(subscription *armsubscriptions.Subscription, logger *slog.Logger, callback chan<- func()) {
	client, err := armauthorization.NewRoleDefinitionsClient(AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		panic(err)
	}

	infoMetric := m.Collector.GetMetricList("roleDefinition")

	pager := client.NewListPager(*subscription.ID, nil)

	for pager.More() {
		result, err := pager.NextPage(m.Context())
		if err != nil {
			panic(err)
		}

		if result.Value == nil {
			continue
		}

		for _, roleDefinition := range result.Value {
			resourceId := to.StringLower(roleDefinition.ID)
			azureResource, _ := armclient.ParseResourceId(resourceId)

			infoLabels := prometheus.Labels{
				"subscriptionID":   azureResource.Subscription,
				"roleDefinitionID": resourceId,
				"name":             to.String(roleDefinition.Name),
				"roleName":         to.String(roleDefinition.Properties.RoleName),
				"roleType":         to.StringLower(roleDefinition.Properties.RoleType),
			}
			infoMetric.AddInfo(infoLabels)
		}
	}
}

func (m *MetricsCollectorAzureRmIam) collectRoleAssignments(subscription *armsubscriptions.Subscription, logger *slog.Logger, callback chan<- func()) {
	principalIdMap := map[string]string{}

	client, err := armauthorization.NewRoleAssignmentsClient(*subscription.SubscriptionID, AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		panic(err)
	}

	infoMetric := m.Collector.GetMetricList("roleAssignment")
	principalMetric := m.Collector.GetMetricList("principal")
	roleAssignmentCountMetric := m.Collector.GetMetricList("roleAssignmentCount")

	pager := client.NewListForSubscriptionPager(nil)

	count := float64(0)
	for pager.More() {
		result, err := pager.NextPage(m.Context())
		if err != nil {
			panic(err)
		}

		if result.Value == nil {
			continue
		}

		for _, roleAssignment := range result.Value {
			count++
			principalId := to.String(roleAssignment.Properties.PrincipalID)

			resourceId := to.StringLower(roleAssignment.Properties.Scope)
			azureResource, _ := armclient.ParseResourceId(resourceId)

			infoLabels := prometheus.Labels{
				"subscriptionID":   azureResource.Subscription,
				"roleAssignmentID": to.StringLower(roleAssignment.ID),
				"roleDefinitionID": extractRoleDefinitionIdFromAzureId(to.StringLower(roleAssignment.Properties.RoleDefinitionID)),
				"resourceID":       resourceId,
				"resourceGroup":    azureResource.ResourceGroup,
				"principalID":      principalId,
			}
			infoMetric.AddInfo(infoLabels)

			principalIdMap[principalId] = principalId
		}
	}

	principalIdList := []string{}
	for _, val := range principalIdMap {
		principalIdList = append(principalIdList, val)
	}

	principalList, err := MsGraphClient.LookupPrincipalID(m.Context(), principalIdList...)
	if err != nil {
		panic(err)
	}

	for _, principal := range principalList {
		principalMetric.AddInfo(prometheus.Labels{
			"subscriptionID": to.StringLower(subscription.SubscriptionID),
			"principalID":    principal.ObjectID,
			"principalName":  principal.DisplayName,
			"principalType":  principal.Type,
		})
	}

	roleAssignmentCountMetric.Add(prometheus.Labels{
		"subscriptionID": to.StringLower(subscription.SubscriptionID),
	}, count)
}
