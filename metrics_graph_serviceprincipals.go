package main

import (
	"strings"

	abstractions "github.com/microsoft/kiota-abstractions-go"
	msgraphcore "github.com/microsoftgraph/msgraph-sdk-go-core"
	"github.com/microsoftgraph/msgraph-sdk-go/serviceprincipals"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
	"github.com/webdevops/go-common/utils/to"
)

type MetricsCollectorGraphServicePrincipals struct {
	collector.Processor

	prometheus struct {
		serviceprincipals            *prometheus.GaugeVec
		serviceprincipalsCredentials *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorGraphServicePrincipals) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	m.prometheus.serviceprincipals = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_graph_serviceprincipal_info",
			Help: "Azure GraphQL serviceprincipals information",
		},
		[]string{
			"appAppID",
			"appObjectID",
			"appDisplayName",
		},
	)
	m.Collector.RegisterMetricList("serviceprincipals", m.prometheus.serviceprincipals, true)

	m.prometheus.serviceprincipalsCredentials = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_graph_serviceprincipal_credential",
			Help: "Azure GraphQL application credentials status",
		},
		[]string{
			"appAppID",
			"credentialName",
			"credentialID",
			"credentialType",
			"type",
		},
	)
	m.Collector.RegisterMetricList("serviceprincipalsCredentials", m.prometheus.serviceprincipalsCredentials, true)
}

func (m *MetricsCollectorGraphServicePrincipals) Reset() {}

func (m *MetricsCollectorGraphServicePrincipals) Collect(callback chan<- func()) {
	headers := abstractions.NewRequestHeaders()
	headers.Add("ConsistencyLevel", "eventual")
	opts := serviceprincipals.ItemMemberOfMicrosoftGraphServicePrincipalRequestBuilderGetRequestConfiguration{
		Headers: headers,
		Options: nil,
		QueryParameters: &serviceprincipals.ItemMemberOfMicrosoftGraphServicePrincipalRequestBuilderGetQueryParameters{
			Filter: &opts.Graph.ServicePrincipalFilter,
			Count: &opts.Graph.ServicePrincipalCount,
		},
	}
	result, err := MsGraphClient.ServiceClient().serviceprincipals().Get(m.Context(), &opts)
	if err != nil {
		m.Logger().Panic(err)
	}

	serviceprincipalsMetrics := m.Collector.GetMetricList("serviceprincipals")
	serviceprincipalsCredentialMetrics := m.Collector.GetMetricList("serviceprincipalsCredentials")

	pageIterator, err := msgraphcore.NewPageIterator(result, MsGraphClient.RequestAdapter(), models.CreateServicePrincipalCollectionResponseFromDiscriminatorValue)
	if err != nil {
		m.Logger().Panic(err)
	}

	err = pageIterator.Iterate(m.Context(), func(pageItem interface{}) bool {
		application := pageItem.(*models.Application)

		appId := to.StringLower(serviceprinicpal.GetAppId())
		objId := to.StringLower(serviceprinicpal.GetId())

		serviceprincipalsMetrics.AddInfo(prometheus.Labels{
			"appAppID":       appId,
			"appObjectID":    objId,
			"appDisplayName": to.String(serviceprinicpal.GetDisplayName()),
		})

		for _, credential := range serviceprinicpal.GetPasswordCredentials() {
			credential.GetDisplayName()
			if credential.GetStartDateTime() != nil {
				serviceprincipalsCredentialMetrics.AddTime(prometheus.Labels{
					"appAppID":       appId,
					"credentialName": to.String(credential.GetDisplayName()),
					"credentialID":   strings.ToLower(credential.GetKeyId().String()),
					"credentialType": "password",
					"type":           "startDate",
				}, credential.GetStartDateTime().UTC())
			}

			if credential.GetEndDateTime() != nil {
				serviceprincipalsCredentialMetrics.AddTime(prometheus.Labels{
					"appAppID":       appId,
					"credentialName": to.String(credential.GetDisplayName()),
					"credentialID":   strings.ToLower(credential.GetKeyId().String()),
					"credentialType": "password",
					"type":           "endDate",
				}, credential.GetEndDateTime().UTC())
			}
		}

		for _, credential := range serviceprinicpal.GetKeyCredentials() {
			credential.GetDisplayName()
			if credential.GetStartDateTime() != nil {
				serviceprincipalsCredentialMetrics.AddTime(prometheus.Labels{
					"appAppID":       appId,
					"credentialName": to.String(credential.GetDisplayName()),
					"credentialID":   strings.ToLower(credential.GetKeyId().String()),
					"credentialType": "key",
					"type":           "startDate",
				}, credential.GetStartDateTime().UTC())
			}

			if credential.GetEndDateTime() != nil {
				serviceprincipalsCredentialMetrics.AddTime(prometheus.Labels{
					"appAppID":       appId,
					"credentialName": to.String(credential.GetDisplayName()),
					"credentialID":   strings.ToLower(credential.GetKeyId().String()),
					"credentialType": "key",
					"type":           "endDate",
				}, credential.GetEndDateTime().UTC())
			}
		}

		return true
	})
	if err != nil {
		m.Logger().Panic(err)
	}
}
