package main

import (
	"strings"

	abstractions "github.com/microsoft/kiota-abstractions-go"
	msgraphcore "github.com/microsoftgraph/msgraph-sdk-go-core"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/microsoftgraph/msgraph-sdk-go/serviceprincipals"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
	"github.com/webdevops/go-common/utils/to"
)

type MetricsCollectorGraphServicePrincipals struct {
	collector.Processor

	prometheus struct {
		serviceprincipal           *prometheus.GaugeVec
		serviceprincipalTag        *prometheus.GaugeVec
		serviceprincipalCredential *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorGraphServicePrincipals) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	m.prometheus.serviceprincipal = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_graph_serviceprincipal_info",
			Help: "Azure GraphQL serviceprincipal information",
		},
		[]string{
			"appAppID",
			"appObjectID",
			"appDisplayName",
		},
	)
	m.Collector.RegisterMetricList("serviceprincipal", m.prometheus.serviceprincipal, true)

	m.prometheus.serviceprincipalTag = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_graph_serviceprincipal_tag",
			Help: "Azure GraphQL serviceprincipal tag",
		},
		[]string{
			"appAppID",
			"appObjectID",
			"appTag",
		},
	)
	m.Collector.RegisterMetricList("serviceprincipalTag", m.prometheus.serviceprincipalTag, true)

	m.prometheus.serviceprincipalCredential = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_graph_serviceprincipal_credential",
			Help: "Azure GraphQL serviceprincipal credentials status",
		},
		[]string{
			"appAppID",
			"credentialName",
			"credentialID",
			"credentialType",
			"type",
		},
	)
	m.Collector.RegisterMetricList("serviceprincipalCredential", m.prometheus.serviceprincipalCredential, true)
}

func (m *MetricsCollectorGraphServicePrincipals) Reset() {}

func (m *MetricsCollectorGraphServicePrincipals) Collect(callback chan<- func()) {
	headers := abstractions.NewRequestHeaders()
	const requestCount = true
	rcount := requestCount
	headers.Add("ConsistencyLevel", "eventual")
	opts := serviceprincipals.ServicePrincipalsRequestBuilderGetRequestConfiguration{
		Headers: headers,
		Options: nil,
		QueryParameters: &serviceprincipals.ServicePrincipalsRequestBuilderGetQueryParameters{
			Filter: Config.Collectors.Graph.Filter.ServicePrincipal,
			Count:  &rcount,
		},
	}
	result, err := MsGraphClient.ServiceClient().ServicePrincipals().Get(m.Context(), &opts)
	if err != nil {
		panic(err)
	}

	serviceprincipalMetrics := m.Collector.GetMetricList("serviceprincipal")
	serviceprincipalTagMetrics := m.Collector.GetMetricList("serviceprincipalTag")
	serviceprincipalCredentialMetrics := m.Collector.GetMetricList("serviceprincipalCredential")

	i, err := msgraphcore.NewPageIterator[models.ServicePrincipalable](result, MsGraphClient.RequestAdapter(), models.CreateServicePrincipalCollectionResponseFromDiscriminatorValue)
	if err != nil {
		panic(err)
	}

	err = i.Iterate(m.Context(), func(serviceprincipal models.ServicePrincipalable) bool {
		appId := to.StringLower(serviceprincipal.GetAppId())
		objId := to.StringLower(serviceprincipal.GetId())

		serviceprincipalMetrics.AddInfo(prometheus.Labels{
			"appAppID":       appId,
			"appObjectID":    objId,
			"appDisplayName": to.String(serviceprincipal.GetDisplayName()),
		})

		for _, tagValue := range serviceprincipal.GetTags() {
			serviceprincipalTagMetrics.AddInfo(prometheus.Labels{
				"appAppID":    appId,
				"appObjectID": objId,
				"appTag":      tagValue,
			})
		}

		for _, credential := range serviceprincipal.GetPasswordCredentials() {
			credential.GetDisplayName()
			if credential.GetStartDateTime() != nil {
				serviceprincipalCredentialMetrics.AddTime(prometheus.Labels{
					"appAppID":       appId,
					"credentialName": to.String(credential.GetDisplayName()),
					"credentialID":   strings.ToLower(credential.GetKeyId().String()),
					"credentialType": "password",
					"type":           "startDate",
				}, credential.GetStartDateTime().UTC())
			}

			if credential.GetEndDateTime() != nil {
				serviceprincipalCredentialMetrics.AddTime(prometheus.Labels{
					"appAppID":       appId,
					"credentialName": to.String(credential.GetDisplayName()),
					"credentialID":   strings.ToLower(credential.GetKeyId().String()),
					"credentialType": "password",
					"type":           "endDate",
				}, credential.GetEndDateTime().UTC())
			}
		}

		for _, credential := range serviceprincipal.GetKeyCredentials() {
			credential.GetDisplayName()
			if credential.GetStartDateTime() != nil {
				serviceprincipalCredentialMetrics.AddTime(prometheus.Labels{
					"appAppID":       appId,
					"credentialName": to.String(credential.GetDisplayName()),
					"credentialID":   strings.ToLower(credential.GetKeyId().String()),
					"credentialType": strings.ToLower(*credential.GetTypeEscaped()),
					"type":           "startDate",
				}, credential.GetStartDateTime().UTC())
			}

			if credential.GetEndDateTime() != nil {
				serviceprincipalCredentialMetrics.AddTime(prometheus.Labels{
					"appAppID":       appId,
					"credentialName": to.String(credential.GetDisplayName()),
					"credentialID":   strings.ToLower(credential.GetKeyId().String()),
					"credentialType": strings.ToLower(*credential.GetTypeEscaped()),
					"type":           "endDate",
				}, credential.GetEndDateTime().UTC())
			}
		}

		return true
	})
	if err != nil {
		panic(err)
	}
}
