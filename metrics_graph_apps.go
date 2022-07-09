package main

//
// type MetricsCollectorGraphApps struct {
// 	collector.Processor
//
// 	client *graphrbac.ApplicationsClient
//
// 	prometheus struct {
// 		apps            *prometheus.GaugeVec
// 		appsCredentials *prometheus.GaugeVec
// 	}
// }
//
// func (m *MetricsCollectorGraphApps) Setup(collector *collector.Collector) {
// 	m.Processor.Setup(collector)
//
// 	// init azure client
// 	auth, _ := auth.NewAuthorizerFromEnvironmentWithResource(AzureClient.Environment.GraphEndpoint)
// 	client := graphrbac.NewApplicationsClientWithBaseURI(AzureClient.Environment.GraphEndpoint, *opts.Azure.Tenant)
// 	AzureClient.DecorateAzureAutorestWithAuthorizer(&client.Client, auth)
//
// 	m.client = &client
//
// 	m.prometheus.apps = prometheus.NewGaugeVec(
// 		prometheus.GaugeOpts{
// 			Name: "azurerm_graph_app_info",
// 			Help: "Azure GraphQL applications information",
// 		},
// 		[]string{
// 			"appAppID",
// 			"appObjectID",
// 			"appDisplayName",
// 			"appObjectType",
// 		},
// 	)
// 	prometheus.MustRegister(m.prometheus.apps)
//
// 	m.prometheus.appsCredentials = prometheus.NewGaugeVec(
// 		prometheus.GaugeOpts{
// 			Name: "azurerm_graph_app_credential",
// 			Help: "Azure GraphQL application credentials status",
// 		},
// 		[]string{
// 			"appAppID",
// 			"credentialID",
// 			"credentialType",
// 			"type",
// 		},
// 	)
// 	prometheus.MustRegister(m.prometheus.appsCredentials)
// }
//
// func (m *MetricsCollectorGraphApps) Reset() {
// 	m.prometheus.apps.Reset()
// 	m.prometheus.appsCredentials.Reset()
// }
//
// func (m *MetricsCollectorGraphApps) Collect(callback chan<- func()) {
// 	logger := m.Logger()
//
// 	appsMetrics := prometheusCommon.NewMetricsList()
// 	appsCredentialMetrics := prometheusCommon.NewMetricsList()
//
// 	list, err := m.client.List(context.Background(), opts.Graph.ApplicationFilter)
// 	if err != nil {
// 		logger.Panic(err)
// 	}
//
// 	for _, row := range list.Values() {
// 		appsMetrics.AddInfo(prometheus.Labels{
// 			"appAppID":       stringPtrToStringLower(row.AppID),
// 			"appObjectID":    stringPtrToStringLower(row.ObjectID),
// 			"appDisplayName": to.String(row.DisplayName),
// 			"appObjectType":  stringToStringLower(string(row.ObjectType)),
// 		})
//
// 		// password credentials
// 		if row.PasswordCredentials != nil {
// 			for _, credential := range *row.PasswordCredentials {
// 				if credential.StartDate != nil {
// 					appsCredentialMetrics.AddTime(prometheus.Labels{
// 						"appAppID":       stringPtrToStringLower(row.AppID),
// 						"credentialID":   stringPtrToStringLower(credential.KeyID),
// 						"credentialType": "password",
// 						"type":           "startDate",
// 					}, (*credential.StartDate).ToTime())
// 				}
//
// 				if credential.EndDate != nil {
// 					appsCredentialMetrics.AddTime(prometheus.Labels{
// 						"appAppID":       stringPtrToStringLower(row.AppID),
// 						"credentialID":   stringPtrToStringLower(credential.KeyID),
// 						"credentialType": "password",
// 						"type":           "endDate",
// 					}, (*credential.EndDate).ToTime())
// 				}
// 			}
// 		}
//
// 		// key credentials
// 		if row.KeyCredentials != nil {
// 			for _, credential := range *row.KeyCredentials {
// 				if credential.StartDate != nil {
// 					appsCredentialMetrics.AddTime(prometheus.Labels{
// 						"appAppID":       stringPtrToStringLower(row.AppID),
// 						"credentialID":   stringPtrToStringLower(credential.KeyID),
// 						"credentialType": "key",
// 						"type":           "startDate",
// 					}, (*credential.StartDate).ToTime())
// 				}
//
// 				if credential.EndDate != nil {
// 					appsCredentialMetrics.AddTime(prometheus.Labels{
// 						"appAppID":       stringPtrToStringLower(row.AppID),
// 						"credentialID":   stringPtrToStringLower(credential.KeyID),
// 						"credentialType": "key",
// 						"type":           "endDate",
// 					}, (*credential.EndDate).ToTime())
// 				}
// 			}
// 		}
// 	}
//
// 	callback <- func() {
// 		appsMetrics.GaugeSet(m.prometheus.apps)
// 		appsCredentialMetrics.GaugeSet(m.prometheus.appsCredentials)
// 	}
// }
