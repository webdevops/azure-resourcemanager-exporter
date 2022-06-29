package main

import (
	"github.com/Azure/azure-sdk-for-go/profiles/latest/graphrbac/graphrbac"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/azure-sdk-for-go/services/preview/authorization/mgmt/2020-04-01-preview/authorization" // nolint waiting for migration until sdk is fully GA
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	azureCommon "github.com/webdevops/go-common/azure"
	prometheusCommon "github.com/webdevops/go-common/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
)

type MetricsCollectorAzureRmIam struct {
	collector.Processor

	graphclient *graphrbac.ObjectsClient

	prometheus struct {
		roleAssignmentCount *prometheus.GaugeVec
		roleAssignment      *prometheus.GaugeVec
		roleDefinition      *prometheus.GaugeVec
		principal           *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorAzureRmIam) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	// init azure client
	authorizer, err := auth.NewAuthorizerFromEnvironmentWithResource(AzureClient.Environment.GraphEndpoint)
	if err != nil {
		m.Logger().Panic(err)
	}
	graphclient := graphrbac.NewObjectsClientWithBaseURI(AzureClient.Environment.GraphEndpoint, *opts.Azure.Tenant)
	AzureClient.DecorateAzureAutorestWithAuthorizer(&graphclient.Client, authorizer)

	m.graphclient = &graphclient

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
	err := AzureSubscriptionsIterator.ForEachAsync(m.Logger(), func(subscription subscriptions.Subscription, logger *log.Entry) {
		m.collectRoleDefinitions(subscription, logger, callback)
		m.collectRoleAssignments(subscription, logger, callback)
	})
	if err != nil {
		m.Logger().Panic(err)
	}
}

func (m *MetricsCollectorAzureRmIam) collectRoleDefinitions(subscription subscriptions.Subscription, logger *log.Entry, callback chan<- func()) {
	client := authorization.NewRoleDefinitionsClientWithBaseURI(AzureClient.Environment.ResourceManagerEndpoint, *subscription.SubscriptionID)
	AzureClient.DecorateAzureAutorest(&client.Client)

	list, err := client.ListComplete(m.Context(), *subscription.ID, "")

	if err != nil {
		logger.Panic(err)
	}

	infoMetric := prometheusCommon.NewMetricsList()

	for list.NotDone() {
		val := list.Value()

		resourceId := to.String(val.ID)
		azureResource, _ := azureCommon.ParseResourceId(resourceId)

		infoLabels := prometheus.Labels{
			"subscriptionID":   azureResource.Subscription,
			"roleDefinitionID": stringToStringLower(resourceId),
			"name":             to.String(val.Name),
			"roleName":         to.String(val.RoleName),
			"roleType":         stringPtrToStringLower(val.RoleType),
		}
		infoMetric.AddInfo(infoLabels)

		if list.NextWithContext(m.Context()) != nil {
			break
		}
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.roleDefinition)
	}
}

func (m *MetricsCollectorAzureRmIam) collectRoleAssignments(subscription subscriptions.Subscription, logger *log.Entry, callback chan<- func()) {
	client := authorization.NewRoleAssignmentsClientWithBaseURI(AzureClient.Environment.ResourceManagerEndpoint, *subscription.SubscriptionID)
	AzureClient.DecorateAzureAutorest(&client.Client)

	list, err := client.ListComplete(m.Context(), "", "")

	if err != nil {
		logger.Panic(err)
	}

	infoMetric := prometheusCommon.NewMetricsList()

	principalIdMap := map[string]string{}

	count := float64(0)
	for list.NotDone() {
		val := list.Value()
		principalId := *val.PrincipalID
		count++

		resourceId := to.String(val.Scope)
		azureResource, _ := azureCommon.ParseResourceId(resourceId)

		infoLabels := prometheus.Labels{
			"subscriptionID":   azureResource.Subscription,
			"roleAssignmentID": stringPtrToStringLower(val.ID),
			"roleDefinitionID": extractRoleDefinitionIdFromAzureId(to.String(val.RoleDefinitionID)),
			"resourceID":       stringToStringLower(resourceId),
			"resourceGroup":    azureResource.ResourceGroup,
			"principalID":      stringToStringLower(principalId),
		}
		infoMetric.AddInfo(infoLabels)

		principalIdMap[principalId] = principalId

		if list.NextWithContext(m.Context()) != nil {
			break
		}
	}

	principalIdList := []string{}
	for _, val := range principalIdMap {
		principalIdList = append(principalIdList, val)
	}
	m.collectPrincipals(subscription, logger, callback, principalIdList)

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.roleAssignment)
		m.prometheus.roleAssignmentCount.With(prometheus.Labels{
			"subscriptionID": stringPtrToStringLower(subscription.SubscriptionID),
		}).Set(count)
	}
}

func (m *MetricsCollectorAzureRmIam) collectPrincipals(subscription subscriptions.Subscription, logger *log.Entry, callback chan<- func(), principalIdList []string) {
	var infoLabels *prometheus.Labels
	infoMetric := prometheusCommon.NewMetricsList()

	// azure limits objects ids
	chunkSize := 999
	for i := 0; i < len(principalIdList); i += chunkSize {
		end := i + chunkSize
		if end > len(principalIdList) {
			end = len(principalIdList)
		}

		principalIdChunkList := principalIdList[i:end]
		opts := graphrbac.GetObjectsParameters{
			ObjectIds: &principalIdChunkList,
		}

		list, err := m.graphclient.GetObjectsByObjectIdsComplete(m.Context(), opts)
		if err != nil {
			logger.Panic(err)
		}

		for list.NotDone() {
			val := list.Value()
			infoLabels = nil

			if object, valid := val.AsADGroup(); valid {
				infoLabels = &prometheus.Labels{
					"subscriptionID": stringPtrToStringLower(subscription.SubscriptionID),
					"principalID":    stringPtrToStringLower(object.ObjectID),
					"principalName":  to.String(object.DisplayName),
					"principalType":  stringToStringLower(string(object.ObjectType)),
				}
			} else if object, valid := val.AsApplication(); valid {
				infoLabels = &prometheus.Labels{
					"subscriptionID": stringPtrToStringLower(subscription.SubscriptionID),
					"principalID":    stringPtrToStringLower(object.ObjectID),
					"principalName":  to.String(object.DisplayName),
					"principalType":  stringToStringLower(string(object.ObjectType)),
				}
			} else if object, valid := val.AsServicePrincipal(); valid {
				infoLabels = &prometheus.Labels{
					"subscriptionID": stringPtrToStringLower(subscription.SubscriptionID),
					"principalID":    stringPtrToStringLower(object.ObjectID),
					"principalName":  to.String(object.DisplayName),
					"principalType":  stringToStringLower(string(object.ObjectType)),
				}
			} else if object, valid := val.AsUser(); valid {
				infoLabels = &prometheus.Labels{
					"subscriptionID": stringPtrToStringLower(subscription.SubscriptionID),
					"principalID":    stringPtrToStringLower(object.ObjectID),
					"principalName":  to.String(object.DisplayName),
					"principalType":  stringToStringLower(string(object.ObjectType)),
				}
			}

			if infoLabels != nil {
				infoMetric.AddInfo(*infoLabels)
			}

			if list.NextWithContext(m.Context()) != nil {
				break
			}
		}
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.principal)
	}
}
