package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/storage/mgmt/storage"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	prometheusCommon "github.com/webdevops/go-prometheus-common"
)

type MetricsCollectorAzureRmStorage struct {
	CollectorProcessorGeneral

	prometheus struct {
		storageAccount   *prometheus.GaugeVec
		managedDisk      *prometheus.GaugeVec
		managedDiskSize  *prometheus.GaugeVec
		managedDiskStats *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorAzureRmStorage) Setup(collector *CollectorGeneral) {
	m.CollectorReference = collector

	m.prometheus.storageAccount = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_storageaccount_info",
			Help: "Azure ResourceManager StorageAccount",
		},
		append(
			[]string{
				"resourceID",
				"subscriptionID",
				"resourceGroup",
				"storageAccountName",
				"location",
				"httpsOnly",
				"sku",
				"accessTier",
				"encrypted",
				"provisioningState",
			},
			azureResourceTags.prometheusLabels...,
		),
	)
	prometheus.MustRegister(m.prometheus.storageAccount)

	m.prometheus.managedDisk = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_manageddisk_info",
			Help: "Azure ResourceManager ManagedDisks",
		},
		append(
			[]string{
				"resourceID",
				"subscriptionID",
				"resourceGroup",
				"managedDiskName",
				"location",
				"sku",
				"encrypted",
				"provisioningState",
			},
			azureResourceTags.prometheusLabels...,
		),
	)
	prometheus.MustRegister(m.prometheus.managedDisk)

	m.prometheus.managedDiskSize = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_manageddisk_size",
			Help: "Azure ResourceManager ManagedDisk size",
		},
		[]string{
			"resourceID",
			"subscriptionID",
			"managedDiskName",
		},
	)
	prometheus.MustRegister(m.prometheus.managedDiskSize)

	m.prometheus.managedDiskStats = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_manageddisk_status",
			Help: "Azure ResourceManager ManagedDisk status",
		},
		[]string{
			"resourceID",
			"subscriptionID",
			"managedDiskName",
			"type",
		},
	)
	prometheus.MustRegister(m.prometheus.managedDiskStats)
}

func (m *MetricsCollectorAzureRmStorage) Reset() {
	m.prometheus.storageAccount.Reset()
	m.prometheus.managedDisk.Reset()
	m.prometheus.managedDiskSize.Reset()
	m.prometheus.managedDiskStats.Reset()
}

func (m *MetricsCollectorAzureRmStorage) Collect(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	m.collectAzureStorageAccounts(ctx, logger, callback, subscription)
	m.collectAzureStorageManagedDisks(ctx, logger, callback, subscription)
}

func (m *MetricsCollectorAzureRmStorage) collectAzureStorageAccounts(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) (ipAddressList []string) {
	client := storage.NewAccountsClient(*subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer
	client.ResponseInspector = azureResponseInspector(&subscription)

	list, err := client.ListComplete(ctx)
	if err != nil {
		logger.Panic(err)
	}

	infoMetric := prometheusCommon.NewMetricsList()

	for list.NotDone() {
		val := list.Value()

		infoLabels := prometheus.Labels{
			"resourceID":         to.String(val.ID),
			"subscriptionID":     to.String(subscription.SubscriptionID),
			"resourceGroup":      extractResourceGroupFromAzureId(to.String(val.ID)),
			"storageAccountName": to.String(val.Name),
			"location":           to.String(val.Location),
			"httpsOnly":          boolPtrToString(val.EnableHTTPSTrafficOnly),
			"sku":                string(val.Sku.Name),
			"accessTier":         string(val.AccessTier),
			"encrypted":          boolToString(val.Encryption.KeySource != ""),
			"provisioningState":  string(val.ProvisioningState),
		}
		infoLabels = azureResourceTags.appendPrometheusLabel(infoLabels, val.Tags)
		infoMetric.AddInfo(infoLabels)

		if list.NextWithContext(ctx) != nil {
			break
		}
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.storageAccount)
	}

	return
}

func (m *MetricsCollectorAzureRmStorage) collectAzureStorageManagedDisks(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) (ipAddressList []string) {
	client := compute.NewDisksClient(*subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer
	client.ResponseInspector = azureResponseInspector(&subscription)

	list, err := client.List(ctx)
	if err != nil {
		logger.Panic(err)
	}

	infoMetric := prometheusCommon.NewMetricsList()
	sizeMetric := prometheusCommon.NewMetricsList()
	statusMetric := prometheusCommon.NewMetricsList()

	for _, val := range list.Values() {
		infoLabels := prometheus.Labels{
			"resourceID":        to.String(val.ID),
			"subscriptionID":    to.String(subscription.SubscriptionID),
			"resourceGroup":     extractResourceGroupFromAzureId(to.String(val.ID)),
			"managedDiskName":   to.String(val.Name),
			"location":          to.String(val.Location),
			"sku":               string(val.Sku.Name),
			"encrypted":         boolToString(false),
			"provisioningState": to.String(val.ProvisioningState),
		}

		if val.EncryptionSettingsCollection != nil {
			if val.EncryptionSettingsCollection.Enabled != nil {
				infoLabels["encrypted"] = boolPtrToString(val.EncryptionSettingsCollection.Enabled)
			}
		}

		infoLabels = azureResourceTags.appendPrometheusLabel(infoLabels, val.Tags)
		infoMetric.AddInfo(infoLabels)

		if val.DiskSizeGB != nil {
			sizeMetric.Add(prometheus.Labels{
				"resourceID":      to.String(val.ID),
				"subscriptionID":  to.String(subscription.SubscriptionID),
				"managedDiskName": to.String(val.Name),
			}, float64(*val.DiskSizeGB)*1073741824)
		}

		if val.DiskIOPSReadWrite != nil {
			statusMetric.Add(prometheus.Labels{
				"resourceID":      to.String(val.ID),
				"subscriptionID":  to.String(subscription.SubscriptionID),
				"managedDiskName": to.String(val.Name),
				"type":            "DiskIOPSReadWrite",
			}, float64(*val.DiskIOPSReadWrite))
		}

		if val.DiskMBpsReadWrite != nil {
			statusMetric.Add(prometheus.Labels{
				"resourceID":      to.String(val.ID),
				"subscriptionID":  to.String(subscription.SubscriptionID),
				"managedDiskName": to.String(val.Name),
				"type":            "DiskMBpsReadWrite",
			}, float64(*val.DiskMBpsReadWrite))
		}
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.managedDisk)
		sizeMetric.GaugeSet(m.prometheus.managedDiskSize)
		statusMetric.GaugeSet(m.prometheus.managedDiskStats)
	}

	return
}
