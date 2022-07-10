package main

import (
	"github.com/microsoftgraph/msgraph-sdk-go/applications"
	"github.com/prometheus/client_golang/prometheus"
	prometheusCommon "github.com/webdevops/go-common/prometheus"
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
	prometheus.MustRegister(m.prometheus.apps)

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
	prometheus.MustRegister(m.prometheus.appsCredentials)
}

func (m *MetricsCollectorGraphApps) Reset() {
	m.prometheus.apps.Reset()
	m.prometheus.appsCredentials.Reset()
}

func (m *MetricsCollectorGraphApps) Collect(callback chan<- func()) {
	opts := applications.ApplicationsRequestBuilderGetRequestConfiguration{
		Headers: nil,
		Options: nil,
		QueryParameters: &applications.ApplicationsRequestBuilderGetQueryParameters{
			Filter: &opts.Graph.ApplicationFilter,
		},
	}
	result, err := MsGraphClient.ServiceClient().Applications().GetWithRequestConfigurationAndResponseHandler(&opts, nil)
	if err != nil {
		m.Logger().Panic(err)
	}

	appsMetrics := prometheusCommon.NewMetricsList()
	appsCredentialMetrics := prometheusCommon.NewMetricsList()

	for _, application := range result.GetValue() {
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
					"credentialID":   to.StringLower(credential.GetKeyId()),
					"credentialType": "password",
					"type":           "startDate",
				}, credential.GetStartDateTime().UTC())
			}

			if credential.GetEndDateTime() != nil {
				appsCredentialMetrics.AddTime(prometheus.Labels{
					"appAppID":       appId,
					"credentialName": to.String(credential.GetDisplayName()),
					"credentialID":   to.StringLower(credential.GetKeyId()),
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
					"credentialID":   to.StringLower(credential.GetKeyId()),
					"credentialType": "key",
					"type":           "startDate",
				}, credential.GetStartDateTime().UTC())
			}

			if credential.GetEndDateTime() != nil {
				appsCredentialMetrics.AddTime(prometheus.Labels{
					"appAppID":       appId,
					"credentialName": to.String(credential.GetDisplayName()),
					"credentialID":   to.StringLower(credential.GetKeyId()),
					"credentialType": "key",
					"type":           "endDate",
				}, credential.GetEndDateTime().UTC())
			}
		}
	}

	callback <- func() {
		appsMetrics.GaugeSet(m.prometheus.apps)
		appsCredentialMetrics.GaugeSet(m.prometheus.appsCredentials)
	}
}
