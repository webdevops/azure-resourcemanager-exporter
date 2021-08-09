Azure ResourceManager Exporter
==============================

[![license](https://img.shields.io/github/license/webdevops/azure-resourcemanager-exporter.svg)](https://github.com/webdevops/azure-resourcemanager-exporter/blob/master/LICENSE)
[![DockerHub](https://img.shields.io/badge/DockerHub-webdevops%2Fazure--resourcemanager--exporter-blue)](https://hub.docker.com/r/webdevops/azure-resourcemanager-exporter/)
[![Quay.io](https://img.shields.io/badge/Quay.io-webdevops%2Fazure--resourcemanager--exporter-blue)](https://quay.io/repository/webdevops/azure-resourcemanager-exporter)

Prometheus exporter for Azure information.


## Features

- Uses of official [Azure SDK for go](https://github.com/Azure/azure-sdk-for-go)
- Supports all Azure environments (Azure public cloud, Azure governmant cloud, Azure china cloud, ...) via Azure SDK configuration


- Docker image is based on [Google's distroless](https://github.com/GoogleContainerTools/distroless) static image to reduce attack surface (no shell, no other binaries inside image)
- Available via Docker Hub and Quay (see badges on top)
- Can run non-root and with readonly root filesystem, doesn't need any capabilities (you can safely use `drop: ["All"]`)
- Publishes Azure API rate limit metrics (when exporter sends Azure API requests)

useful with additional exporters:

- [azure-resourcegraph-exporter](https://github.com/webdevops/azure-resourcegraph-exporter) for exporting Azure resource information from Azure ResourceGraph API with custom Kusto queries (get the tags from resources and ResourceGroups with this exporter)
- [azure-metrics-exporter](https://github.com/webdevops/azure-metrics-exporter) for exporting Azure Monitor metrics
- [azure-keyvault-exporter](https://github.com/webdevops/azure-keyvault-exporter) for exporting Azure KeyVault information (eg expiry date for secrets, certificates and keys)
- [azure-loganalytics-exporter](https://github.com/webdevops/azure-loganalytics-exporter) for exporting Azure LogAnalytics workspace information with custom Kusto queries (eg ingestion rate or application error count)

Configuration
-------------

Normally no configuration is needed but can be customized using environment variables.

(to disable specific scrape collectors set them to `0` or set `SCRAPE_TIME` to `0` to disable all by default)

| Environment variable              | DefaultValue                | Description                                                       |
|-----------------------------------|-----------------------------|-------------------------------------------------------------------|
| `AZURE_TENANT_ID`                 | `empty`                     | Azure Tenant IDs (**required**)                                   |
| `AZURE_SUBSCRIPTION_ID`           | `empty`                     | Azure Subscription IDs (empty for auto lookup)                    |
| `AZURE_LOCATION`                  | `westeurope`, `northeurope` | Azure location for usage statitics                                |
| `SCRAPE_TIME`                     | `5m`                        | Default scrape time (time.Duration) between Azure API collections |
| `SCRAPE_TIME_GENERAL`             | -> SCRAPE_TIME              | Scrape time for General metrics                                   |
| `SCRAPE_RATELIMIT_READ`           | `2m`                        | Scrape time for Azure rate limit read metrics                     |
| `SCRAPE_RATELIMIT_WRITE`          | `5m`                        | Scrape time for Azure rate limit write metrics (needs tag permissions on subscriptions, see below) |
| `SCRAPE_TIME_RESOURCE`            | -> SCRAPE_TIME              | Scrape time for Resource metrics [*Deprecated*](README.md#Deprecations) |
| `SCRAPE_TIME_STORAGE`             | -> SCRAPE_TIME              | Scrape time for Storage metrics [*Deprecated*](README.md#Deprecations) |
| `SCRAPE_TIME_QUOTA`               | -> SCRAPE_TIME              | Scrape time for Quota metrics                                     |
| `SCRAPE_TIME_CONTAINERREGISTRY`   | -> SCRAPE_TIME              | Scrape time for ContainerRegistry metrics [*Deprecated*](README.md#Deprecations) |
| `SCRAPE_TIME_CONTAINERINSTANCE`   | -> SCRAPE_TIME              | Scrape time for ContainerInstance metrics [*Deprecated*](README.md#Deprecations) |
| `SCRAPE_TIME_EVENTHUB`            | `30m`        `              | Scrape time for Eventhub metrics [*Deprecated*](README.md#Deprecations) |
| `SCRAPE_TIME_SECURITY`            | -> SCRAPE_TIME              | Scrape time for Security metrics                                  |
| `SCRAPE_TIME_HEALTH`              | -> SCRAPE_TIME              | Scrape time for Health metrics                                    |
| `SCRAPE_TIME_IAM`                 | -> SCRAPE_TIME              | Scrape time for AzurAD IAM (roledefinitions, rolebindings, principals) metrics  |
| `SCRAPE_TIME_GRAPH`               | -> SCRAPE_TIME              | Scrape time for AzurAD Graph metrics                              |
| `SERVER_BIND`                     | `:8080`                     | IP/Port binding                                                   |
| `AZURE_RESOURCEGROUP_TAG`         | `owner`                     | Tags which should be included (methods available eg. `owner:lower` will transform content lowercase, methods: `lower`, `upper`, `title`)  |
| `AZURE_RESOURCE_TAG`              | `owner`                     | Tags which should be included (methods available eg. `owner:lower` will transform content lowercase, methods: `lower`, `upper`, `title`)  |
| `PORTSCAN`                        | `0`                         | Enables portscanner for public IPs (experimental)                 |
| `PORTSCAN_RANGE`                  | `1-65535`                   | Port range to scan (single port or range, mutliple ranges possible -> space as seperator)  |
| `PORTSCAN_TIME`                   | `3h`                        | Time (time.Duration) between portscanner runs                     |
| `PORTSCAN_PARALLEL`               | `2`                         | Parallel IPs which are scanned at the same time                   |
| `PORTSCAN_THREADS`                | `1000`                      | Number of threads per IP (parallel scanned ports)                 |
| `PORTSCAN_TIMEOUT`                | `5`                         | Timeout (seconds) for each port                                   |
| `GRAPH_APPLICATION_FILTER`        | `empty`                     | AzureAD graph application filter, eg. `startswith(displayName,'foo')` |

for Azure API authentication (using ENV vars) see https://github.com/Azure/azure-sdk-for-go#authentication

Deprecations
------------

Please use [`azure-resourcegraph-exporter`](https://github.com/webdevops/azure-resourcegraph-exporter) for exporting resources.
This exporter is using Azure ResourceGraph queries and not wasting Azure API calls for fetching metrics.

`azure-resourcegraph-exporter` provides a way how metrics can be build by using Kusto querys.

Azure permissions
-----------------

This exporter needs `Reader` permissions on subscription level.

For Azure write rate limits it tries to tag the subscription with an empty tag set (actually no changes).
For this operation it needs `Microsoft.Resources/tags/write` on scope `/subscription/*`.

To disable write rate limits set `SCRAPE_RATELIMIT_WRITE` to `0`.

Metrics
-------

| Metric                                         | Collector           | Description                                                                           |
|------------------------------------------------|---------------------|---------------------------------------------------------------------------------------|
| `azurerm_stats`                                | Exporter            | General exporter stats                                                                |
| `azurerm_subscription_info`                    | General             | Azure Subscription details (ID, name, ...)                                            |
| `azurerm_resourcegroup_info`                   | Resource            | Azure ResourceGroup details (subscriptionID, name, various tags ...)                  |
| `azurerm_resource_info`                        | Resource            | Azure Resource information                                                            |
| `azurerm_ratelimit`                            | *all* (if detected) | Azure API ratelimit (left calls)                                                      |
| `azurerm_quota`                                | Quota               | Azure RM quota details (readable name, scope, ...)                                    |
| `azurerm_quota_current`                        | Quota               | Azure RM quota current (current value)                                                |
| `azurerm_quota_limit`                          | Quota               | Azure RM quota limit (maximum limited value)                                          |
| `azurerm_publicip_portscan_status`             | Portscan            | Status of scanned ports (finished scan, elapsed time, updated timestamp)              |
| `azurerm_publicip_portscan_port`               | Portscan            | List of opend ports per IP                                                            |
| `azurerm_securitycenter_compliance`            | Security            | Azure SecurityCenter compliance status                                                |
| `azurerm_advisor_recommendation`               | Security            | Azure Adisor recommendations (eg. security findings)                                  |
| `azurerm_resource_health`                      | Health              | Azure Resource health information                                                     |
| `azurerm_iam_roleassignment_info`              | IAM                 | Azure IAM RoleAssignment information                                                  |
| `azurerm_iam_roledefinition_info`              | IAM                 | Azure IAM RoleDefinition information                                                  |
| `azurerm_iam_principal_info`                   | IAM                 | Azure IAM Principal information                                                       |
| `azurerm_graph_app_info`                       | Graph               | AzureAD graph application information                                                 |
| `azurerm_graph_app_credential`                 | Graph               | AzureAD graph application credentials (create,expiry) information                     |
