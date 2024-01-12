package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/consumption/armconsumption"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
	"go.uber.org/zap"
)

// Define MetricsCollectorAzureRmReservation struct
type MetricsCollectorAzureRmReservation struct {
	collector.Processor

	prometheus struct {
		reservationUsage *prometheus.GaugeVec
	}
}

// Setup method to initialize Prometheus metrics
func (m *MetricsCollectorAzureRmReservation) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	m.prometheus.reservationUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_reservation_avg_utilisation",
			Help: "Azure ResourceManager Reservation Average Utilization Percentage",
		},
		[]string{
			"subscriptionID",
			"scope",
			"skuName",
			"reservationAvgUtilizationPercentage",
			"usageDate",
		},
	)

	m.Collector.RegisterMetricList("reservationUsage", m.prometheus.reservationUsage, true)
	// prometheus.MustRegister(m.prometheus.reservationUsage) // ANCIEN SCRIPT
}

func (m *MetricsCollectorAzureRmReservation) Reset() {}

func (m *MetricsCollectorAzureRmReservation) Collect(callback chan<- func()) {
	err := AzureSubscriptionsIterator.ForEachAsync(m.Logger(), func(subscription *armsubscriptions.Subscription, logger *zap.SugaredLogger) {
		m.collectReservationUsage(subscription, logger, callback)
	})
	if err != nil {
		m.Logger().Panic(err)
	}
}

func (m *MetricsCollectorAzureRmReservation) collectReservationUsage(subscription *armsubscriptions.Subscription, logger *zap.SugaredLogger, callback chan<- func()) {
	// options := AzureClient.NewArmClientOptions()
	// ep := cloud.AzurePublic.Services[cloud.ResourceManager].Endpoint
	// if c, ok := options.Cloud.Services[cloud.ResourceManager]; ok {
	// 	ep = c.Endpoint
	// }

	// pl, err := armruntime.NewPipeline("azurerm-quota", gitTag, AzureClient.GetCred(), runtime.PipelineOptions{}, options)
	// if err != nil {
	// 	logger.Panic(err)
	// }

	reservationUsage := m.Collector.GetMetricList("reservationUsage")

	ctx := context.Background()
	now := time.Now()
	formattedDate := now.Format("2006-01-02")

	// credential, err := initAzureConnection() // ANCIEN SCRIPT
	// if err != nil {
	// 	fmt.Printf("Échec de l'initialisation de la connexion Azure: %v\n", err)
	// 	return
	// }

	billingAccountID := "providers/Microsoft.Billing/billingAccounts/4c612ae7-0d01-512a-391a-e16024131950:59a12fd2-744c-45b6-b82f-fc0963569b8e_2019-05-31/billingProfiles/7QDV-V6E3-BG7-PGB"
	startDate := formattedDate
	endDate := formattedDate

	clientFactory, err := armconsumption.NewClientFactory(*subscription.SubscriptionID, AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		logger.Panic(err)
	}

	// Créez un pager pour récupérer les résumés de réservations quotidiens
	pager := clientFactory.NewReservationsSummariesClient().NewListPager(billingAccountID, armconsumption.DatagrainDailyGrain, &armconsumption.ReservationsSummariesClientListOptions{
		StartDate:          to.Ptr(startDate),
		EndDate:            to.Ptr(endDate),
		Filter:             nil,
		ReservationID:      nil,
		ReservationOrderID: nil,
	})

	// Enregistrez les métriques Prometheus // ANCIEN SCRIPT
	// prometheus.MustRegister(reservationUsage)

	// Ajoutez un point de terminaison HTTP pour Prometheus // ANCIEN SCRIPT
	// http.Handle("/metrics", promhttp.Handler())
	// go func() {
	// 	log.Fatal(http.ListenAndServe(":8080", nil))
	// }()

	// Collectez et exportez les métriques
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			logger.Panic(err)
		}
		for _, reservationInfo := range page.Value {
			fmt.Printf("SKUName: %s\n", *reservationInfo.Properties.SKUName)
			skuName := reservationInfo.Properties.SKUName
			fmt.Printf("reservationUsage: %f\n", *reservationInfo.Properties.AvgUtilizationPercentage)
			reservationAvgUtilizationPercentage := reservationInfo.Properties.AvgUtilizationPercentage
			fmt.Printf("UsageDate: %s\n", reservationInfo.Properties.UsageDate.String())
			usageDate := reservationInfo.Properties.UsageDate.String()

			// reservationUsage.WithLabelValues(*reservationInfo.Properties.SKUName, *v.Properties.ReservationID, v.Properties.UsageDate.String()).Set(*v.Properties.AvgUtilizationPercentage)
			infoLabels := prometheus.Labels{
				"subscriptionID":                      strings.ToLower(*subscription.SubscriptionID),
				"scope":                               "reservation",
				"skuName":                             *skuName,
				"reservationAvgUtilizationPercentage": fmt.Sprintf("%f", *reservationAvgUtilizationPercentage),
				"usageDate":                           usageDate,
			}

			// labels := prometheus.Labels{
			// 	"subscriptionID": to.StringLower(subscription.SubscriptionID),
			// 	"location":       strings.ToLower(location),
			// 	"scope":          "reservation",
			// 	"skuName":        skuName,
			// }

			reservationUsage.Add(infoLabels, 1)
		}
	}
}
