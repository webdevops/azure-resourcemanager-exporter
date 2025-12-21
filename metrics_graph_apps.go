package main

import (
	"strings"

	abstractions "github.com/microsoft/kiota-abstractions-go"
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
		app           *prometheus.GaugeVec
		appTags       *prometheus.GaugeVec
		appCredential *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorGraphApps) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	m.prometheus.app = prometheus.NewGaugeVec(
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
	m.Collector.RegisterMetricList("app", m.prometheus.app, true)

	m.prometheus.appTags = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_graph_app_tag",
			Help: "Azure GraphQL applications tag",
		},
		[]string{
			"appAppID",
			"appObjectID",
			"appTag",
		},
	)
	m.Collector.RegisterMetricList("appTag", m.prometheus.appTags, true)

	m.prometheus.appCredential = prometheus.NewGaugeVec(
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
	m.Collector.RegisterMetricList("appCredential", m.prometheus.appCredential, true)
}

func (m *MetricsCollectorGraphApps) Reset() {}

func (m *MetricsCollectorGraphApps) Collect(callback chan<- func()) {
	headers := abstractions.NewRequestHeaders()
	headers.Add("ConsistencyLevel", "eventual")
	const requestCount = true
	rcount := requestCount
	opts := applications.ApplicationsRequestBuilderGetRequestConfiguration{
		Headers: headers,
		Options: nil,
		QueryParameters: &applications.ApplicationsRequestBuilderGetQueryParameters{
			Filter: Config.Collectors.Graph.Filter.Application,
			Count:  &rcount,
		},
	}
	result, err := MsGraphClient.ServiceClient().Applications().Get(m.Context(), &opts)
	if err != nil {
		panic(err)
	}

	appMetrics := m.Collector.GetMetricList("app")
	appTagMetrics := m.Collector.GetMetricList("appTag")
	appCredentialMetrics := m.Collector.GetMetricList("appCredential")

	i, err := msgraphcore.NewPageIterator[models.Applicationable](result, MsGraphClient.RequestAdapter(), models.CreateApplicationCollectionResponseFromDiscriminatorValue)
	if err != nil {
		panic(err)
	}

	err = i.Iterate(m.Context(), func(application models.Applicationable) bool {
		appId := to.StringLower(application.GetAppId())
		objId := to.StringLower(application.GetId())

		appMetrics.AddInfo(prometheus.Labels{
			"appAppID":       appId,
			"appObjectID":    objId,
			"appDisplayName": to.String(application.GetDisplayName()),
		})

		for _, tagValue := range application.GetTags() {
			appTagMetrics.AddInfo(prometheus.Labels{
				"appAppID":    appId,
				"appObjectID": objId,
				"appTag":      tagValue,
			})
		}

		for _, credential := range application.GetPasswordCredentials() {
			credential.GetDisplayName()
			if credential.GetStartDateTime() != nil {
				appCredentialMetrics.AddTime(prometheus.Labels{
					"appAppID":       appId,
					"credentialName": to.String(credential.GetDisplayName()),
					"credentialID":   strings.ToLower(credential.GetKeyId().String()),
					"credentialType": "password",
					"type":           "startDate",
				}, credential.GetStartDateTime().UTC())
			}

			if credential.GetEndDateTime() != nil {
				appCredentialMetrics.AddTime(prometheus.Labels{
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
				appCredentialMetrics.AddTime(prometheus.Labels{
					"appAppID":       appId,
					"credentialName": to.String(credential.GetDisplayName()),
					"credentialID":   strings.ToLower(credential.GetKeyId().String()),
					"credentialType": "key",
					"type":           "startDate",
				}, credential.GetStartDateTime().UTC())
			}

			if credential.GetEndDateTime() != nil {
				appCredentialMetrics.AddTime(prometheus.Labels{
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
		panic(err)
	}
}
