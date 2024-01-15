package main

import (
	"context"
	"fmt"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore/to"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/consumption/armconsumption"
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
			Name: "azurerm_reservation_utilization",
			Help: "Azure ResourceManager Reservation Utilization",
		},
		[]string{
			"ReservationOrderID",
			"ReservationID",
			"SkuName",
			"Kind",
			"ReservedHours",
			"UsedHours",
			"UsageDate",
			"MinUtilizationPercentage",
			"AvgUtilizationPercentage",
			"MaxUtilizationPercentage",
			"TotalReservedQuantity",
		},
	)

	m.Collector.RegisterMetricList("reservationUsage", m.prometheus.reservationUsage, true)
}

func (m *MetricsCollectorAzureRmReservation) Reset() {}

func (m *MetricsCollectorAzureRmReservation) Collect(callback chan<- func()) {
	m.collectReservationUsage(logger, callback)
}

func (m *MetricsCollectorAzureRmReservation) collectReservationUsage(logger *zap.SugaredLogger, callback chan<- func()) {
	reservationUsage := m.Collector.GetMetricList("reservationUsage")

	ctx := context.Background()
	now := time.Now()
	formattedDate := now.AddDate(-3, 0, 0).Format("2006-01-02")

	resourceScope := Config.Collectors.Reservation.ResourceScope
	granularity := Config.Collectors.Reservation.Granularity

	startDate := formattedDate
	endDate := formattedDate

	clientFactory, err := armconsumption.NewClientFactory("<subscription-id>", AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		logger.Panic(err)
	}

	// Créez un pager pour récupérer les résumés de réservations quotidiens
	pager := clientFactory.NewReservationsSummariesClient().NewListPager(resourceScope, armconsumption.Datagrain(granularity), &armconsumption.ReservationsSummariesClientListOptions{
		StartDate:          to.Ptr(startDate),
		EndDate:            to.Ptr(endDate),
		Filter:             nil,
		ReservationID:      nil,
		ReservationOrderID: nil,
	})

	// Collectez et exportez les métriques
	for pager.More() {
		page, err := pager.NextPage(ctx)
		if err != nil {
			logger.Panic(err)
		}
		for _, reservationInfo := range page.Value {
			ReservationOrderID := reservationInfo.Properties.ReservationOrderID
			ReservationID := reservationInfo.Properties.ReservationID
			SkuName := reservationInfo.Properties.SKUName
			Kind := reservationInfo.Properties.Kind
			ReservedHours := reservationInfo.Properties.ReservedHours
			UsageDate := reservationInfo.Properties.UsageDate.String()
			UsedHours := reservationInfo.Properties.UsedHours
			MinUtilizationPercentage := reservationInfo.Properties.MinUtilizationPercentage
			AvgUtilizationPercentage := reservationInfo.Properties.AvgUtilizationPercentage
			MaxUtilizationPercentage := reservationInfo.Properties.MaxUtilizationPercentage
			TotalReservedQuantity := reservationInfo.Properties.TotalReservedQuantity

			infoLabels := prometheus.Labels{
				"ReservationOrderID":       *ReservationOrderID,
				"ReservationID":            *ReservationID,
				"SkuName":                  *SkuName,
				"Kind":                     *Kind,
				"ReservedHours":            fmt.Sprintf("%f", *ReservedHours),
				"UsedHours":                fmt.Sprintf("%f", *UsedHours),
				"UsageDate":                UsageDate,
				"MinUtilizationPercentage": fmt.Sprintf("%f", *MinUtilizationPercentage),
				"AvgUtilizationPercentage": fmt.Sprintf("%f", *AvgUtilizationPercentage),
				"MaxUtilizationPercentage": fmt.Sprintf("%f", *MaxUtilizationPercentage),
				"TotalReservedQuantity":    fmt.Sprintf("%f", *TotalReservedQuantity),
			}

			reservationUsage.Add(infoLabels, *AvgUtilizationPercentage)
		}
	}
}
