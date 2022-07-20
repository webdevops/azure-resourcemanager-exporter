package main

import (
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/authorization/armauthorization"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"github.com/webdevops/go-common/azuresdk/armclient"
	prometheusCommon "github.com/webdevops/go-common/prometheus"
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
	prometheus.MustRegister(m.prometheus.roleAssignmentCount)

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
	prometheus.MustRegister(m.prometheus.roleAssignment)

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
	prometheus.MustRegister(m.prometheus.roleDefinition)

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
	prometheus.MustRegister(m.prometheus.principal)
}

func (m *MetricsCollectorAzureRmIam) Reset() {
	m.prometheus.roleAssignmentCount.Reset()
	m.prometheus.roleDefinition.Reset()
	m.prometheus.roleAssignment.Reset()
	m.prometheus.principal.Reset()
}

func (m *MetricsCollectorAzureRmIam) Collect(callback chan<- func()) {
	err := AzureSubscriptionsIterator.ForEachAsync(m.Logger(), func(subscription *armsubscriptions.Subscription, logger *log.Entry) {
		m.collectRoleDefinitions(subscription, logger, callback)
		m.collectRoleAssignments(subscription, logger, callback)
	})
	if err != nil {
		m.Logger().Panic(err)
	}
}

func (m *MetricsCollectorAzureRmIam) collectRoleDefinitions(subscription *armsubscriptions.Subscription, logger *log.Entry, callback chan<- func()) {
	client, err := armauthorization.NewRoleDefinitionsClient(AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		logger.Panic(err)
	}

	infoMetric := prometheusCommon.NewMetricsList()

	pager := client.NewListPager(*subscription.ID, nil)

	for pager.More() {
		result, err := pager.NextPage(m.Context())
		if err != nil {
			logger.Panic(err)
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

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.roleDefinition)
	}
}

func (m *MetricsCollectorAzureRmIam) collectRoleAssignments(subscription *armsubscriptions.Subscription, logger *log.Entry, callback chan<- func()) {
	principalIdMap := map[string]string{}

	client, err := armauthorization.NewRoleAssignmentsClient(*subscription.SubscriptionID, AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		logger.Panic(err)
	}

	infoMetric := prometheusCommon.NewMetricsList()
	principalMetric := prometheusCommon.NewMetricsList()

	pager := client.NewListPager(nil)

	count := float64(0)
	for pager.More() {
		result, err := pager.NextPage(m.Context())
		if err != nil {
			logger.Panic(err)
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

	principalList, err := MsGraphClient.LookupPrincipalID(principalIdList...)
	if err != nil {
		logger.Panic(err)
	}

	for _, principal := range principalList {
		principalMetric.AddInfo(prometheus.Labels{
			"subscriptionID": to.StringLower(subscription.SubscriptionID),
			"principalID":    principal.ObjectID,
			"principalName":  principal.DisplayName,
			"principalType":  principal.Type,
		})
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.roleAssignment)
		principalMetric.GaugeSet(m.prometheus.principal)
		m.prometheus.roleAssignmentCount.With(prometheus.Labels{
			"subscriptionID": to.StringLower(subscription.SubscriptionID),
		}).Set(count)
	}
}
