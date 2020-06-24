package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/authorization/mgmt/authorization"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/graphrbac/graphrbac"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/go-autorest/autorest/azure/auth"
	"github.com/prometheus/client_golang/prometheus"
	prometheusCommon "github.com/webdevops/go-prometheus-common"
	"os"
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
	auth, err := auth.NewAuthorizerFromEnvironmentWithResource(opts.azureEnvironment.GraphEndpoint)
	if err != nil {
		panic(err)
	}
	graphclient := graphrbac.NewObjectsClient(os.Getenv("AZURE_TENANT_ID"))
	graphclient.Authorizer = auth
	m.graphclient = &graphclient

	m.prometheus.roleAssignment = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_iam_roleassignment_info",
			Help: "Azure IAM RoleAssignment info",
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

	m.prometheus.roleDefinition = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_iam_roledefinition_info",
			Help: "Azure IAM RoleDefinition info",
		},
		[]string{
			"subscriptionID",
			"roleDefinitionID",
			"name",
			"roleName",
			"roleType",
		},
	)

	m.prometheus.principal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_iam_principal_info",
			Help: "Azure IAM Principal info",
		},
		[]string{
			"subscriptionID",
			"principalID",
			"principalName",
			"principalType",
		},
	)

	prometheus.MustRegister(m.prometheus.roleDefinition)
	prometheus.MustRegister(m.prometheus.roleAssignment)
	prometheus.MustRegister(m.prometheus.principal)
}

func (m *MetricsCollectorAzureRmIam) Reset() {
	m.prometheus.roleDefinition.Reset()
	m.prometheus.roleAssignment.Reset()
	m.prometheus.principal.Reset()
}

func (m *MetricsCollectorAzureRmIam) Collect(ctx context.Context, callback chan<- func(), subscription subscriptions.Subscription) {
	m.collectRoleDefinitions(ctx, callback, subscription)
	m.collectRoleAssignments(ctx, callback, subscription)
}

func (m *MetricsCollectorAzureRmIam) collectRoleDefinitions(ctx context.Context, callback chan<- func(), subscription subscriptions.Subscription) {
	client := authorization.NewRoleDefinitionsClient(*subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer

	list, err := client.ListComplete(ctx, *subscription.ID, "")

	if err != nil {
		panic(err)
	}

	infoMetric := prometheusCommon.NewMetricsList()

	for list.NotDone() {
		val := list.Value()

		infoLabels := prometheus.Labels{
			"subscriptionID":   *subscription.SubscriptionID,
			"roleDefinitionID": extractRoleDefinitionIdFromAzureId(*val.ID),
			"name":             *val.Name,
			"roleName":         *val.RoleName,
			"roleType":         *val.RoleType,
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

func (m *MetricsCollectorAzureRmIam) collectRoleAssignments(ctx context.Context, callback chan<- func(), subscription subscriptions.Subscription) {
	client := authorization.NewRoleAssignmentsClient(*subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer

	list, err := client.ListComplete(ctx, "")

	if err != nil {
		panic(err)
	}

	infoMetric := prometheusCommon.NewMetricsList()

	principalIdList := []string{}

	for list.NotDone() {
		val := list.Value()

		infoLabels := prometheus.Labels{
			"subscriptionID":   *subscription.SubscriptionID,
			"roleAssignmentID": *val.ID,
			"roleDefinitionID": extractRoleDefinitionIdFromAzureId(*val.Properties.RoleDefinitionID),
			"resourceID":       *val.Properties.Scope,
			"resourceGroup":    extractResourceGroupFromAzureId(*val.Properties.Scope),
			"principalID":      *val.Properties.PrincipalID,
		}
		infoMetric.AddInfo(infoLabels)

		principalIdList = append(principalIdList, *val.Properties.PrincipalID)

		if list.NextWithContext(ctx) != nil {
			break
		}
	}

	m.collectPrincipals(ctx, callback, subscription, principalIdList)

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.roleAssignment)
	}
}

func (m *MetricsCollectorAzureRmIam) collectPrincipals(ctx context.Context, callback chan<- func(), subscription subscriptions.Subscription, principalIdList []string) {
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
			panic(err)
		}

		for list.NotDone() {
			val := list.Value()

			infoLabels = nil

			if object, valid := val.AsADGroup(); valid {
				infoLabels = &prometheus.Labels{
					"subscriptionID": *subscription.SubscriptionID,
					"principalID":    stringPtrToString(object.ObjectID),
					"principalName":  stringPtrToString(object.DisplayName),
					"principalType":  string(object.ObjectType),
				}
			} else if object, valid := val.AsApplication(); valid {
				infoLabels = &prometheus.Labels{
					"subscriptionID": *subscription.SubscriptionID,
					"principalID":    stringPtrToString(object.ObjectID),
					"principalName":  stringPtrToString(object.DisplayName),
					"principalType":  string(object.ObjectType),
				}
			} else if object, valid := val.AsServicePrincipal(); valid {
				infoLabels = &prometheus.Labels{
					"subscriptionID": *subscription.SubscriptionID,
					"principalID":    stringPtrToString(object.ObjectID),
					"principalName":  stringPtrToString(object.DisplayName),
					"principalType":  string(object.ObjectType),
				}
			} else if object, valid := val.AsUser(); valid {
				infoLabels = &prometheus.Labels{
					"subscriptionID": *subscription.SubscriptionID,
					"principalID":    stringPtrToString(object.ObjectID),
					"principalName":  stringPtrToString(object.DisplayName),
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
