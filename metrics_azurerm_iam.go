package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/graphrbac/graphrbac"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/azure-sdk-for-go/services/preview/authorization/mgmt/2020-04-01-preview/authorization"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	prometheusCommon "github.com/webdevops/go-prometheus-common"
)

type MetricsCollectorAzureRmIam struct {
	CollectorProcessorGeneral

	graphclient *graphrbac.ObjectsClient

	prometheus struct {
		roleAssignment *prometheus.GaugeVec
		roleDefinition *prometheus.GaugeVec
		principal      *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorAzureRmIam) Setup(collector *CollectorGeneral) {
	m.CollectorReference = collector

	// init azure client
	auth, err := auth.NewAuthorizerFromEnvironmentWithResource(azureEnvironment.GraphEndpoint)
	if err != nil {
		m.logger().Panic(err)
	}
	graphclient := graphrbac.NewObjectsClientWithBaseURI(azureEnvironment.GraphEndpoint, *opts.Azure.Tenant)
	graphclient.Authorizer = auth
	graphclient.ResponseInspector = azureResponseInspector(nil)

	m.graphclient = &graphclient

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
	m.prometheus.roleDefinition.Reset()
	m.prometheus.roleAssignment.Reset()
	m.prometheus.principal.Reset()
}

func (m *MetricsCollectorAzureRmIam) Collect(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	m.collectRoleDefinitions(ctx, logger, callback, subscription)
	m.collectRoleAssignments(ctx, logger, callback, subscription)
}

func (m *MetricsCollectorAzureRmIam) collectRoleDefinitions(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	client := authorization.NewRoleDefinitionsClientWithBaseURI(azureEnvironment.ResourceManagerEndpoint, *subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer
	client.ResponseInspector = azureResponseInspector(&subscription)

	list, err := client.ListComplete(ctx, *subscription.ID, "")

	if err != nil {
		logger.Panic(err)
	}

	infoMetric := prometheusCommon.NewMetricsList()

	for list.NotDone() {
		val := list.Value()

		infoLabels := prometheus.Labels{
			"subscriptionID":   to.String(subscription.SubscriptionID),
			"roleDefinitionID": extractRoleDefinitionIdFromAzureId(to.String(val.ID)),
			"name":             to.String(val.Name),
			"roleName":         to.String(val.RoleName),
			"roleType":         to.String(val.RoleType),
		}
		infoMetric.AddInfo(infoLabels)

		if list.NextWithContext(ctx) != nil {
			break
		}
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.roleDefinition)
	}
}

func (m *MetricsCollectorAzureRmIam) collectRoleAssignments(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	client := authorization.NewRoleAssignmentsClientWithBaseURI(azureEnvironment.ResourceManagerEndpoint, *subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer
	client.ResponseInspector = azureResponseInspector(&subscription)

	list, err := client.ListComplete(ctx, "", "")

	if err != nil {
		logger.Panic(err)
	}

	infoMetric := prometheusCommon.NewMetricsList()

	principalIdMap := map[string]string{}

	for list.NotDone() {
		val := list.Value()
		principalId := *val.PrincipalID

		infoLabels := prometheus.Labels{
			"subscriptionID":   to.String(subscription.SubscriptionID),
			"roleAssignmentID": toResourceId(val.ID),
			"roleDefinitionID": extractRoleDefinitionIdFromAzureId(to.String(val.RoleDefinitionID)),
			"resourceID":       toResourceId(val.Scope),
			"resourceGroup":    extractResourceGroupFromAzureId(to.String(val.Scope)),
			"principalID":      principalId,
		}
		infoMetric.AddInfo(infoLabels)

		principalIdMap[principalId] = principalId

		if list.NextWithContext(ctx) != nil {
			break
		}
	}

	principalIdList := []string{}
	for _, val := range principalIdMap {
		principalIdList = append(principalIdList, val)
	}
	m.collectPrincipals(ctx, logger, callback, subscription, principalIdList)

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.roleAssignment)
	}
}

func (m *MetricsCollectorAzureRmIam) collectPrincipals(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription, principalIdList []string) {
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

		list, err := m.graphclient.GetObjectsByObjectIdsComplete(ctx, opts)
		if err != nil {
			logger.Panic(err)
		}

		for list.NotDone() {
			val := list.Value()

			infoLabels = nil

			if object, valid := val.AsADGroup(); valid {
				infoLabels = &prometheus.Labels{
					"subscriptionID": to.String(subscription.SubscriptionID),
					"principalID":    to.String(object.ObjectID),
					"principalName":  to.String(object.DisplayName),
					"principalType":  string(object.ObjectType),
				}
			} else if object, valid := val.AsApplication(); valid {
				infoLabels = &prometheus.Labels{
					"subscriptionID": to.String(subscription.SubscriptionID),
					"principalID":    to.String(object.ObjectID),
					"principalName":  to.String(object.DisplayName),
					"principalType":  string(object.ObjectType),
				}
			} else if object, valid := val.AsServicePrincipal(); valid {
				infoLabels = &prometheus.Labels{
					"subscriptionID": to.String(subscription.SubscriptionID),
					"principalID":    to.String(object.ObjectID),
					"principalName":  to.String(object.DisplayName),
					"principalType":  string(object.ObjectType),
				}
			} else if object, valid := val.AsUser(); valid {
				infoLabels = &prometheus.Labels{
					"subscriptionID": to.String(subscription.SubscriptionID),
					"principalID":    to.String(object.ObjectID),
					"principalName":  to.String(object.DisplayName),
					"principalType":  string(object.ObjectType),
				}
			}

			if infoLabels != nil {
				infoMetric.AddInfo(*infoLabels)
			}

			if list.NextWithContext(ctx) != nil {
				break
			}
		}
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.principal)
	}
}
