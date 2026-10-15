package main

import (
	"log/slog"
	"strings"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/consumption/armconsumption"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/reservations/armreservations"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/prometheus/collector"
	"github.com/webdevops/go-common/utils/to"
)

var provisioningStates = []string{
	"Creating",
	"PendingResourceHold",
	"ConfirmedResourceHold",
	"PendingBilling",
	"ConfirmedBilling",
	"Created",
	"Succeeded",
	"Cancelled",
	"Expired",
	"BillingFailed",
	"Failed",
	"Split",
	"Merged",
}

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
		reservationProvisioningState     *prometheus.GaugeVec
		reservationExpiryTimestamp       *prometheus.GaugeVec
	}
}

// Setup method to initialize Prometheus metrics
func (m *MetricsCollectorAzureRmReservation) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	commonLabels := []string{
		"scope",
		"reservationOrderID",
		"reservationID",
		"skuName",
		"kind",
		"usageDate",
	}

	m.prometheus.reservationInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_reservation_info",
			Help: "Azure ResourceManager Reservation Information",
		},
		commonLabels,
	)
	m.Collector.RegisterMetricList("reservationInfo", m.prometheus.reservationInfo, true)

	m.prometheus.reservationUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_reservation_utilization",
			Help: "Azure ResourceManager Reservation Utilization",
		},
		commonLabels,
	)
	m.Collector.RegisterMetricList("reservationUsage", m.prometheus.reservationUsage, true)

	m.prometheus.reservationMinUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_reservation_utilization_min",
			Help: "Azure ResourceManager Reservation Min Utilization",
		},
		commonLabels,
	)
	m.Collector.RegisterMetricList("reservationMinUsage", m.prometheus.reservationMinUsage, true)

	m.prometheus.reservationMaxUsage = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_reservation_utilization_max",
			Help: "Azure ResourceManager Reservation Max Utilization",
		},
		commonLabels,
	)
	m.Collector.RegisterMetricList("reservationMaxUsage", m.prometheus.reservationMaxUsage, true)

	m.prometheus.reservationUsedHours = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_reservation_used_hours",
			Help: "Azure ResourceManager Reservation Used Hours",
		},
		commonLabels,
	)
	m.Collector.RegisterMetricList("reservationUsedHours", m.prometheus.reservationUsedHours, true)

	m.prometheus.reservationReservedHours = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_reservation_reserved_hours",
			Help: "Azure ResourceManager Reservation Reserved Hours",
		},
		commonLabels,
	)
	m.Collector.RegisterMetricList("reservationReservedHours", m.prometheus.reservationReservedHours, true)

	m.prometheus.reservationTotalReservedQuantity = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_reservation_total_reserved_quantity",
			Help: "Azure ResourceManager Reservation Total Reserved Quantity",
		},
		commonLabels,
	)
	m.Collector.RegisterMetricList("reservationTotalReservedQuantity", m.prometheus.reservationTotalReservedQuantity, true)

	// Metrics from management.azure.com/providers/Microsoft.Capacity/reservations (List All, no scopes required)
	reservationListAllLabels := []string{
		"reservationOrderID",
		"reservationID",
		"skuName",
		"kind",
		"displayName",
		"purchaseDate",
		"reservedResourceType",
		"skuDescription",
	}
	reservationListAllLabelsWithState := append(append([]string{}, reservationListAllLabels...), "provisioningState")
	m.prometheus.reservationProvisioningState = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_reservation_provisioning_state",
			Help: "Azure ResourceManager Reservation provisioning state as a label (value 1 for current state, 0 otherwise)",
		},
		reservationListAllLabelsWithState,
	)
	m.Collector.RegisterMetricList("reservationProvisioningState", m.prometheus.reservationProvisioningState, true)

	m.prometheus.reservationExpiryTimestamp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_reservation_expiry_timestamp",
			Help: "Azure ResourceManager Reservation expiry date as Unix timestamp",
		},
		reservationListAllLabels,
	)
	m.Collector.RegisterMetricList("reservationExpiryTimestamp", m.prometheus.reservationExpiryTimestamp, true)
}

func (m *MetricsCollectorAzureRmReservation) Reset() {}

func (m *MetricsCollectorAzureRmReservation) Collect(callback chan<- func()) {
	scopes := Config.Collectors.Reservation.Scopes
	if len(scopes) == 0 {
		// No scopes: only produce metrics from List All (management.azure.com/providers/Microsoft.Capacity/reservations)
		m.collectReservationListAll(m.Logger(), callback)
		return
	}
	// Scopes set: produce scope-based usage metrics and List All metrics
	for _, scope := range scopes {
		m.collectReservationUsage(m.Logger(), scope, callback)
	}
	m.collectReservationListAll(m.Logger(), callback)
}

func (m *MetricsCollectorAzureRmReservation) collectReservationUsage(logger *slog.Logger, scope string, callback chan<- func()) {
	reservationInfo := m.Collector.GetMetricList("reservationInfo")
	reservationUsage := m.Collector.GetMetricList("reservationUsage")
	reservationMinUsage := m.Collector.GetMetricList("reservationMinUsage")
	reservationMaxUsage := m.Collector.GetMetricList("reservationMaxUsage")
	reservationUsedHours := m.Collector.GetMetricList("reservationUsedHours")
	reservationReservedHours := m.Collector.GetMetricList("reservationReservedHours")
	reservationTotalReservedQuantity := m.Collector.GetMetricList("reservationTotalReservedQuantity")

	days := Config.Collectors.Reservation.FromDays
	granularity := Config.Collectors.Reservation.Granularity

	now := time.Now()
	startDate := now.AddDate(0, 0, -days).Format("2006-01-02")
	endDate := now.Format("2006-01-02")

	clientFactory, err := armconsumption.NewClientFactory("<subscription-id>", AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		panic(err)
	}

	// Create a pager to retrieve daily booking summaries
	pager := clientFactory.NewReservationsSummariesClient().NewListPager(scope, armconsumption.Datagrain(granularity), &armconsumption.ReservationsSummariesClientListOptions{
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
			panic(err)
		}

		for _, reservationProperties := range page.Value {
			labels := prometheus.Labels{
				"scope":              scope,
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

// parseReservationID extracts reservationOrderID and reservationID from the reservation resource ID.
func parseReservationID(id string) (reservationOrderID, reservationID string) {
	if id == "" {
		return "", ""
	}
	parts := strings.Split(strings.TrimPrefix(id, "/"), "/")
	for i := 0; i < len(parts)-1; i++ {
		switch strings.ToLower(parts[i]) {
		case "reservationorders":
			if i+1 < len(parts) {
				reservationOrderID = parts[i+1]
			}
		case "reservations":
			if i+1 < len(parts) {
				reservationID = parts[i+1]
			}
		}
	}
	return reservationOrderID, reservationID
}

func (m *MetricsCollectorAzureRmReservation) collectReservationListAll(logger *slog.Logger, callback chan<- func()) {
	provisioningStateMetric := m.Collector.GetMetricList("reservationProvisioningState")
	expiryTimestampMetric := m.Collector.GetMetricList("reservationExpiryTimestamp")

	client, err := armreservations.NewReservationClient(AzureClient.GetCred(), AzureClient.NewArmClientOptions())
	if err != nil {
		logger.Error("failed to create reservations client", slog.Any("error", err))
		return
	}

	// Filter out archived reservations; API expects URL-encoded filter: (properties/archived eq false)
	pager := client.NewListAllPager(&armreservations.ReservationClientListAllOptions{
		Filter: to.Ptr("(properties/archived eq false)"),
	})

	for pager.More() {
		page, err := pager.NextPage(m.Context())
		if err != nil {
			logger.Error("failed to get next reservations page", slog.Any("error", err))
			return
		}

		for _, item := range page.Value {
			if item == nil || item.Properties == nil {
				continue
			}
			reservationOrderID, reservationID := parseReservationID(to.String(item.ID))
			if reservationOrderID == "" || reservationID == "" {
				continue
			}

			skuName := ""
			if item.SKU != nil && item.SKU.Name != nil {
				skuName = *item.SKU.Name
			}
			kind := to.String(item.Kind)

			displayName := to.String(item.Properties.DisplayName)
			purchaseDate := ""
			if item.Properties.PurchaseDate != nil {
				purchaseDate = item.Properties.PurchaseDate.Format("2006-01-02")
			}
			reservedResourceType := ""
			if item.Properties.ReservedResourceType != nil {
				reservedResourceType = string(*item.Properties.ReservedResourceType)
			}
			skuDescription := to.String(item.Properties.SKUDescription)

			baseLabels := prometheus.Labels{
				"reservationOrderID":   reservationOrderID,
				"reservationID":        reservationID,
				"skuName":              skuName,
				"kind":                 kind,
				"displayName":          displayName,
				"purchaseDate":         purchaseDate,
				"reservedResourceType": reservedResourceType,
				"skuDescription":       skuDescription,
			}

			currentState := ""
			if item.Properties.ProvisioningState != nil {
				currentState = string(*item.Properties.ProvisioningState)
			}

			for _, state := range provisioningStates {
				labels := prometheus.Labels{}
				for k, v := range baseLabels {
					labels[k] = v
				}
				labels["provisioningState"] = state

				value := 0.0
				if state == currentState {
					value = 1.0
				}

				provisioningStateMetric.Add(labels, value)
			}

			if item.Properties.ExpiryDate != nil {
				expiryTimestampMetric.Add(baseLabels, float64(item.Properties.ExpiryDate.Unix()))
			}
		}
	}
}
