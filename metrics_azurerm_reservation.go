package main

import (
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/consumption/armconsumption"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
	"github.com/webdevops/go-common/utils/to"
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
			Name: "azurerm_reservation_info",
			Help: "Azure ResourceManager Reservation Information",
		},
		[]string{
			"reservationOrderID",
			"reservationID",
			"skuName",
			"kind",
			"usageDate",
		},
	)
	m.Collector.RegisterMetricList("reservationInfo", m.prometheus.reservationInfo, true)

	m.prometheus.reservationUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_reservation_utilization",
			Help: "Azure ResourceManager Reservation Utilization",
		},
		[]string{
			"reservationOrderID",
			"reservationID",
			"skuName",
			"kind",
			"usageDate",
		},
	)
	m.Collector.RegisterMetricList("reservationUsage", m.prometheus.reservationUsage, true)

	m.prometheus.reservationMinUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_reservation_utilization_min",
			Help: "Azure ResourceManager Reservation Min Utilization",
		},
		[]string{
			"reservationOrderID",
			"reservationID",
			"skuName",
			"kind",
			"usageDate",
		},
	)
	m.Collector.RegisterMetricList("reservationMinUsage", m.prometheus.reservationMinUsage, true)

	m.prometheus.reservationMaxUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_reservation_utilization_max",
			Help: "Azure ResourceManager Reservation Max Utilization",
		},
		[]string{
			"reservationOrderID",
			"reservationID",
			"skuName",
			"kind",
			"usageDate",
		},
	)
	m.Collector.RegisterMetricList("reservationMaxUsage", m.prometheus.reservationMaxUsage, true)

	m.prometheus.reservationUsedHours = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_reservation_used_hours",
			Help: "Azure ResourceManager Reservation Used Hours",
		},
		[]string{
			"reservationOrderID",
			"reservationID",
			"skuName",
			"kind",
			"usageDate",
		},
	)
	m.Collector.RegisterMetricList("reservationUsedHours", m.prometheus.reservationUsedHours, true)

	m.prometheus.reservationReservedHours = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_reservation_reserved_hours",
			Help: "Azure ResourceManager Reservation Reserved Hours",
		},
		[]string{
			"reservationOrderID",
			"reservationID",
			"skuName",
			"kind",
			"usageDate",
		},
	)
	m.Collector.RegisterMetricList("reservationReservedHours", m.prometheus.reservationReservedHours, true)

	m.prometheus.reservationTotalReservedQuantity = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_reservation_total_reserved_quantity",
			Help: "Azure ResourceManager Reservation Total Reserved Quantity",
		},
		[]string{
			"reservationOrderID",
			"reservationID",
			"skuName",
			"kind",
			"usageDate",
		},
	)
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

	days := Config.Collectors.Reservation.FromDays
	resourceScope := Config.Collectors.Reservation.ResourceScope
	granularity := Config.Collectors.Reservation.Granularity

	now := time.Now()
	formattedDate := now.AddDate(0, 0, -days).Format("2006-01-02")
	startDate := formattedDate
	endDate := time.Now().Format("2006-01-02")

	clientFactory, err := armconsumption.NewClientFactory("<subscription-id>", AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		logger.Panic(err)
	}

	// "Create a pager to retrieve daily booking summaries
	pager := clientFactory.NewReservationsSummariesClient().NewListPager(resourceScope, armconsumption.Datagrain(granularity), &armconsumption.ReservationsSummariesClientListOptions{
		StartDate:          to.Ptr(startDate),
		EndDate:            to.Ptr(endDate),
		Filter:             nil,
		ReservationID:      nil,
		ReservationOrderID: nil,
	})

	// Collect and export metrics
	for pager.More() {
		page, err := pager.NextPage(m.Context())
		if err != nil {
			logger.Panic(err)
		}

		for _, reservationProperties := range page.Value {
			labels := prometheus.Labels{
				"reservationOrderID": to.String(reservationProperties.Properties.ReservationOrderID),
				"reservationID":      to.String(reservationProperties.Properties.ReservationID),
				"skuName":            to.String(reservationProperties.Properties.SKUName),
				"kind":               to.String(reservationProperties.Properties.Kind),
				"usageDate":          reservationProperties.Properties.UsageDate.String(),
			}

			reservationInfo.AddInfo(labels)
			reservationUsage.AddIfNotNil(labels, reservationProperties.Properties.AvgUtilizationPercentage)
			reservationMinUsage.AddIfNotNil(labels, reservationProperties.Properties.MinUtilizationPercentage)
			reservationMaxUsage.AddIfNotNil(labels, reservationProperties.Properties.MaxUtilizationPercentage)
			reservationUsedHours.AddIfNotNil(labels, reservationProperties.Properties.UsedHours)
			reservationReservedHours.AddIfNotNil(labels, reservationProperties.Properties.ReservedHours)
			reservationTotalReservedQuantity.AddIfNotNil(labels, reservationProperties.Properties.TotalReservedQuantity)
		}
	}
}
