package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/storage/mgmt/storage"
	"github.com/prometheus/client_golang/prometheus"
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
			Help: "Azure ResourceManager StorageACcount",
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
			opts.azureResourceTags.prometheusLabels...,
		),
	)

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
			opts.azureResourceTags.prometheusLabels...,
		),
	)

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

	prometheus.MustRegister(m.prometheus.storageAccount)
	prometheus.MustRegister(m.prometheus.managedDisk)
	prometheus.MustRegister(m.prometheus.managedDiskSize)
	prometheus.MustRegister(m.prometheus.managedDiskStats)
}

func (m *MetricsCollectorAzureRmStorage) Reset() {
	m.prometheus.storageAccount.Reset()
	m.prometheus.managedDisk.Reset()
	m.prometheus.managedDiskSize.Reset()
	m.prometheus.managedDiskStats.Reset()
}

func (m *MetricsCollectorAzureRmStorage) Collect(ctx context.Context, callback chan<- func(), subscription subscriptions.Subscription) {
	m.collectAzureStorageAccounts(ctx, callback, subscription)
	m.collectAzureStorageManagedDisks(ctx, callback, subscription)
}

func (m *MetricsCollectorAzureRmStorage) collectAzureStorageAccounts(ctx context.Context, callback chan<- func(), subscription subscriptions.Subscription) (ipAddressList []string) {
	client := storage.NewAccountsClient(*subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer

	list, err := client.ListComplete(ctx)
	if err != nil {
		panic(err)
	}

	infoMetric := MetricCollectorList{}

	for list.NotDone() {
		val := list.Value()

		infoLabels := prometheus.Labels{
			"resourceID":         *val.ID,
			"subscriptionID":     *subscription.SubscriptionID,
			"resourceGroup":      extractResourceGroupFromAzureId(*val.ID),
			"storageAccountName": *val.Name,
			"location":           *val.Location,
			"httpsOnly":          boolToString(*val.EnableHTTPSTrafficOnly),
			"sku":                string(val.Sku.Name),
			"accessTier":         string(val.AccessTier),
			"encrypted":          boolToString(val.Encryption.KeySource != ""),
			"provisioningState":  string(val.ProvisioningState),
		}
		infoLabels = opts.azureResourceTags.appendPrometheusLabel(infoLabels, val.Tags)
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

func (m *MetricsCollectorAzureRmStorage) collectAzureStorageManagedDisks(ctx context.Context, callback chan<- func(), subscription subscriptions.Subscription) (ipAddressList []string) {
	client := compute.NewDisksClient(*subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer

	list, err := client.List(ctx)
	if err != nil {
		panic(err)
	}

	infoMetric := MetricCollectorList{}
	sizeMetric := MetricCollectorList{}
	statusMetric := MetricCollectorList{}

	for _, val := range list.Values() {
		infoLabels := prometheus.Labels{
			"resourceID":        *val.ID,
			"subscriptionID":    *subscription.SubscriptionID,
			"resourceGroup":     extractResourceGroupFromAzureId(*val.ID),
			"managedDiskName":   stringPtrToString(val.Name),
			"location":          stringPtrToString(val.Location),
			"sku":               string(val.Sku.Name),
			"encrypted":         boolToString(false),
			"provisioningState": stringPtrToString(val.ProvisioningState),
		}

		if val.EncryptionSettingsCollection != nil {
			if val.EncryptionSettingsCollection.Enabled != nil {
				infoLabels["encrypted"] = boolToString(*val.EncryptionSettingsCollection.Enabled)
			}
		}

		infoLabels = opts.azureResourceTags.appendPrometheusLabel(infoLabels, val.Tags)
		infoMetric.AddInfo(infoLabels)

		if val.DiskSizeGB != nil {
			sizeMetric.Add(prometheus.Labels{
				"resourceID":      *val.ID,
				"subscriptionID":  *subscription.SubscriptionID,
				"managedDiskName": stringPtrToString(val.Name),
			}, float64(*val.DiskSizeGB)*1073741824)
		}

		if val.DiskIOPSReadWrite != nil {
			statusMetric.Add(prometheus.Labels{
				"resourceID":      *val.ID,
				"subscriptionID":  *subscription.SubscriptionID,
				"managedDiskName": stringPtrToString(val.Name),
				"type":            "DiskIOPSReadWrite",
			}, float64(*val.DiskIOPSReadWrite))
		}

		if val.DiskMBpsReadWrite != nil {
			statusMetric.Add(prometheus.Labels{
				"resourceID":      *val.ID,
				"subscriptionID":  *subscription.SubscriptionID,
				"managedDiskName": stringPtrToString(val.Name),
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
