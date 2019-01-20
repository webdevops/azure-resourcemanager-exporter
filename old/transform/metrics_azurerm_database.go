package old

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/mysql/mgmt/mysql"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/postgresql/mgmt/postgresql"
	"github.com/prometheus/client_golang/prometheus"
)

func (m *MetricCollectorAzureRm) initDatabase() {
	m.prometheus.database = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_containerregistry_database_info",
			Help: "Azure Database info",
		},
		append(
			[]string{"resourceID", "subscriptionID", "location", "type", "serverName", "resourceGroup", "version", "skuName", "skuTier", "fqdn", "sslEnforcement", "geoRedundantBackup"},
			prefixSlice(AZURE_RESOURCE_TAG_PREFIX, opts.AzureResourceTags)...
		),
	)

	m.prometheus.databaseStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_containerregistry_database_status",
			Help: "Azure Database status informations",
		},
		[]string{"resourceID", "type"},
	)

	prometheus.MustRegister(m.prometheus.database)
	prometheus.MustRegister(m.prometheus.databaseStatus)
}

func (m *MetricCollectorAzureRm) collectAzureDatabasePostgresql(ctx context.Context, subscriptionId string, callback chan<- func()) {
	client := postgresql.NewServersClient(subscriptionId)
	client.Authorizer = AzureAuthorizer

	list, err := client.List(ctx)

	if err != nil {
		panic(err)
	}

	infoMetric := MetricCollectorList{}
	statusMetric := MetricCollectorList{}

	for _, val := range *list.Value {
		skuName := ""
		skuTier := ""

		if val.Sku != nil {
			skuName = string(*val.Sku.Name)
			skuTier = string(val.Sku.Tier)
		}

		infoLabels := prometheus.Labels{
			"resourceID": *val.ID,
			"subscriptionID": subscriptionId,
			"location": *val.Location,
			"type": "postgresql",
			"serverName": *val.Name,
			"resourceGroup": extractResourceGroupFromAzureId(*val.ID),
			"skuName": skuName,
			"skuTier": skuTier,
			"version": string(val.Version),
			"fqdn": *val.FullyQualifiedDomainName,
			"sslEnforcement": string(val.SslEnforcement),
			"geoRedundantBackup": string(val.StorageProfile.GeoRedundantBackup),
		}
		infoLabels = addAzureResourceTags(infoLabels, val.Tags)
		infoMetric.Add(infoLabels, 1)

		statusMetric.Add(prometheus.Labels{
			"resourceID": *val.ID,
			"type": "backupRetentionDays",
		}, float64(*val.StorageProfile.BackupRetentionDays))

		statusMetric.Add(prometheus.Labels{
			"resourceID": *val.ID,
			"type": "storage",
		}, float64(*val.StorageProfile.StorageMB * 1048576))
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.database)
		statusMetric.GaugeSet(m.prometheus.databaseStatus)
	}
}

func (m *MetricCollectorAzureRm) collectAzureDatabaseMysql(ctx context.Context, subscriptionId string, callback chan<- func()) {
	client := mysql.NewServersClient(subscriptionId)
	client.Authorizer = AzureAuthorizer

	list, err := client.List(ctx)

	if err != nil {
		panic(err)
	}

	infoMetric := MetricCollectorList{}
	statusMetric := MetricCollectorList{}

	for _, val := range *list.Value {
		skuName := ""
		skuTier := ""

		if val.Sku != nil {
			skuName = string(*val.Sku.Name)
			skuTier = string(val.Sku.Tier)
		}

		infoLabels := prometheus.Labels{
			"resourceID": *val.ID,
			"subscriptionID": subscriptionId,
			"location": *val.Location,
			"serverName": *val.Name,
			"type": "mysql",
			"resourceGroup": extractResourceGroupFromAzureId(*val.ID),
			"skuName": skuName,
			"skuTier": skuTier,
			"version": string(val.Version),
			"fqdn": *val.FullyQualifiedDomainName,
			"sslEnforcement": string(val.SslEnforcement),
			"geoRedundantBackup": string(val.StorageProfile.GeoRedundantBackup),
		}
		infoLabels = addAzureResourceTags(infoLabels, val.Tags)
		infoMetric.Add(infoLabels, 1)

		statusMetric.Add(prometheus.Labels{
			"resourceID": *val.ID,
			"type": "backupRetentionDays",
		}, float64(*val.StorageProfile.BackupRetentionDays))

		statusMetric.Add(prometheus.Labels{
			"resourceID": *val.ID,
			"type": "storage",
		}, float64(*val.StorageProfile.StorageMB * 1048576))
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.database)
		statusMetric.GaugeSet(m.prometheus.databaseStatus)
	}
}
