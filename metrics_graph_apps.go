package main

import (
	"strings"

	msgraphcore "github.com/microsoftgraph/msgraph-sdk-go-core"
	"github.com/microsoftgraph/msgraph-sdk-go/applications"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
	"github.com/webdevops/go-common/utils/to"
)

type MetricsCollectorGraphApps struct {
	collector.Processor

	prometheus struct {
		apps            *prometheus.GaugeVec
		appsCredentials *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorGraphApps) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	m.prometheus.apps = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_graph_app_info",
			Help: "Azure GraphQL applications information",
		},
		[]string{
			"appAppID",
			"appObjectID",
			"appDisplayName",
		},
	)
	m.Collector.RegisterMetricList("apps", m.prometheus.apps, true)

	m.prometheus.appsCredentials = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_graph_app_credential",
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
	m.Collector.RegisterMetricList("appsCredentials", m.prometheus.appsCredentials, true)
}

func (m *MetricsCollectorGraphApps) Reset() {}

func (m *MetricsCollectorGraphApps) Collect(callback chan<- func()) {
	opts := applications.ApplicationsRequestBuilderGetRequestConfiguration{
		Headers: nil,
		Options: nil,
		QueryParameters: &applications.ApplicationsRequestBuilderGetQueryParameters{
			Filter: &opts.Graph.ApplicationFilter,
		},
	}
	result, err := MsGraphClient.ServiceClient().Applications().Get(m.Context(), &opts)
	if err != nil {
		m.Logger().Panic(err)
	}

	appsMetrics := m.Collector.GetMetricList("apps")
	appsCredentialMetrics := m.Collector.GetMetricList("appsCredentials")

	pageIterator, err := msgraphcore.NewPageIterator(result, MsGraphClient.RequestAdapter(), models.CreateApplicationCollectionResponseFromDiscriminatorValue)
	if err != nil {
		m.Logger().Panic(err)
	}

	err = pageIterator.Iterate(m.Context(), func(pageItem interface{}) bool {
		application := pageItem.(*models.Application)

		appId := to.StringLower(application.GetAppId())
		objId := to.StringLower(application.GetId())

		appsMetrics.AddInfo(prometheus.Labels{
			"appAppID":       appId,
			"appObjectID":    objId,
			"appDisplayName": to.String(application.GetDisplayName()),
		})

		for _, credential := range application.GetPasswordCredentials() {
			credential.GetDisplayName()
			if credential.GetStartDateTime() != nil {
				appsCredentialMetrics.AddTime(prometheus.Labels{
					"appAppID":       appId,
					"credentialName": to.String(credential.GetDisplayName()),
					"credentialID":   strings.ToLower(credential.GetKeyId().String()),
					"credentialType": "password",
					"type":           "startDate",
				}, credential.GetStartDateTime().UTC())
			}

			if credential.GetEndDateTime() != nil {
				appsCredentialMetrics.AddTime(prometheus.Labels{
					"appAppID":       appId,
					"credentialName": to.String(credential.GetDisplayName()),
					"credentialID":   strings.ToLower(credential.GetKeyId().String()),
					"credentialType": "password",
					"type":           "endDate",
				}, credential.GetEndDateTime().UTC())
			}
		}

		for _, credential := range application.GetKeyCredentials() {
			credential.GetDisplayName()
			if credential.GetStartDateTime() != nil {
				appsCredentialMetrics.AddTime(prometheus.Labels{
					"appAppID":       appId,
					"credentialName": to.String(credential.GetDisplayName()),
					"credentialID":   strings.ToLower(credential.GetKeyId().String()),
					"credentialType": "key",
					"type":           "startDate",
				}, credential.GetStartDateTime().UTC())
			}

			if credential.GetEndDateTime() != nil {
				appsCredentialMetrics.AddTime(prometheus.Labels{
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
