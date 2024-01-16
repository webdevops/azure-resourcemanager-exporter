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
		reservationInfo                  *prometheus.GaugeVec
		reservationUsage                 *prometheus.GaugeVec
		reservationMinUsage              *prometheus.GaugeVec
		reservationMaxUsage              *prometheus.GaugeVec
		reservationUsedHours             *prometheus.GaugeVec
		reservationReservedHours         *prometheus.GaugeVec
		reservationTotalReservedQuantity *prometheus.GaugeVec
	}
}

// Setup method to initialize Prometheus metrics
func (m *MetricsCollectorAzureRmReservation) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	m.prometheus.reservationInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_reservation_information",
			Help: "Azure ResourceManager Reservation Information",
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

	m.prometheus.reservationMinUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_reservation_minUtilization",
			Help: "Azure ResourceManager Reservation Min Utilization",
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

	m.prometheus.reservationMaxUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_reservation_maxUtilization",
			Help: "Azure ResourceManager Reservation Max Utilization",
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

	m.prometheus.reservationUsedHours = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_reservation_usedHours",
			Help: "Azure ResourceManager Reservation Used Hours",
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

	m.prometheus.reservationReservedHours = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_reservation_reservedHours",
			Help: "Azure ResourceManager Reservation Reserved Hours",
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

	m.prometheus.reservationTotalReservedQuantity = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_reservation_totalReservedQuantity",
			Help: "Azure ResourceManager Reservation Total Reserved Quantity",
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

	m.Collector.RegisterMetricList("reservationInfo", m.prometheus.reservationInfo, true)
	m.Collector.RegisterMetricList("reservationUsage", m.prometheus.reservationUsage, true)
	m.Collector.RegisterMetricList("reservationMinUsage", m.prometheus.reservationMinUsage, true)
	m.Collector.RegisterMetricList("reservationMaxUsage", m.prometheus.reservationMaxUsage, true)
	m.Collector.RegisterMetricList("reservationUsedHours", m.prometheus.reservationUsedHours, true)
	m.Collector.RegisterMetricList("reservationReservedHours", m.prometheus.reservationReservedHours, true)
	m.Collector.RegisterMetricList("reservationTotalReservedQuantity", m.prometheus.reservationTotalReservedQuantity, true)
}

func (m *MetricsCollectorAzureRmReservation) Reset() {}

func (m *MetricsCollectorAzureRmReservation) Collect(callback chan<- func()) {
	m.collectReservationUsage(logger, callback)
}

func (m *MetricsCollectorAzureRmReservation) collectReservationUsage(logger *zap.SugaredLogger, callback chan<- func()) {
	reservationInfo := m.Collector.GetMetricList("reservationInfo")
	reservationUsage := m.Collector.GetMetricList("reservationUsage")
	reservationMinUsage := m.Collector.GetMetricList("reservationMinUsage")
	reservationMaxUsage := m.Collector.GetMetricList("reservationMaxUsage")
	reservationUsedHours := m.Collector.GetMetricList("reservationUsedHours")
	reservationReservedHours := m.Collector.GetMetricList("reservationReservedHours")
	reservationTotalReservedQuantity := m.Collector.GetMetricList("reservationTotalReservedQuantity")

	ctx := context.Background()
	days := Config.Collectors.Reservation.FromDays
	resourceScope := Config.Collectors.Reservation.ResourceScope
	granularity := Config.Collectors.Reservation.Granularity

	now := time.Now()
	formattedDate := now.AddDate(0, 0, -days).Format("2006-01-02")
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
		for _, reservationProperties := range page.Value {
			ReservationOrderID := reservationProperties.Properties.ReservationOrderID
			ReservationID := reservationProperties.Properties.ReservationID
			SkuName := reservationProperties.Properties.SKUName
			Kind := reservationProperties.Properties.Kind
			ReservedHours := reservationProperties.Properties.ReservedHours
			UsageDate := reservationProperties.Properties.UsageDate.String()
			UsedHours := reservationProperties.Properties.UsedHours
			MinUtilizationPercentage := reservationProperties.Properties.MinUtilizationPercentage
			AvgUtilizationPercentage := reservationProperties.Properties.AvgUtilizationPercentage
			MaxUtilizationPercentage := reservationProperties.Properties.MaxUtilizationPercentage
			TotalReservedQuantity := reservationProperties.Properties.TotalReservedQuantity

			Labels := prometheus.Labels{
				"ReservationOrderID":       *ReservationOrderID,
				"ReservationID":            *ReservationID,
				"SkuName":                  *SkuName,
				"Kind":                     *Kind,
				"ReservedHours":            fmt.Sprintf("%.2f", *ReservedHours),
				"UsedHours":                fmt.Sprintf("%.2f", *UsedHours),
				"UsageDate":                UsageDate,
				"MinUtilizationPercentage": fmt.Sprintf("%.2f", *MinUtilizationPercentage),
				"AvgUtilizationPercentage": fmt.Sprintf("%.2f", *AvgUtilizationPercentage),
				"MaxUtilizationPercentage": fmt.Sprintf("%.2f", *MaxUtilizationPercentage),
				"TotalReservedQuantity":    fmt.Sprintf("%.2f", *TotalReservedQuantity),
			}

			reservationInfo.Add(Labels, 1)
			reservationUsage.Add(Labels, *AvgUtilizationPercentage)
			reservationMinUsage.Add(Labels, *MinUtilizationPercentage)
			reservationMaxUsage.Add(Labels, *MaxUtilizationPercentage)
			reservationUsedHours.Add(Labels, *UsedHours)
			reservationReservedHours.Add(Labels, *ReservedHours)
			reservationTotalReservedQuantity.Add(Labels, *TotalReservedQuantity)
		}
	}
}
