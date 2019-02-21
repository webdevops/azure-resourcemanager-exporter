Azure ResourceManager Exporter
==============================

[![license](https://img.shields.io/github/license/webdevops/azure-resourcemanager-exporter.svg)](https://github.com/webdevops/azure-resourcemanager-exporter/blob/master/LICENSE)
[![Docker](https://img.shields.io/badge/docker-webdevops%2Fazure--resourcemanager--exporter-blue.svg?longCache=true&style=flat&logo=docker)](https://hub.docker.com/r/webdevops/azure-resourcemanager-exporter/)
[![Docker Build Status](https://img.shields.io/docker/build/webdevops/azure-resourcemanager-exporter.svg)](https://hub.docker.com/r/webdevops/azure-resourcemanager-exporter/)

Prometheus exporter for Azure Resources and informations.

Configuration
-------------

Normally no configuration is needed but can be customized using environment variables.

| Environment variable              | DefaultValue                | Description                                                       |
|-----------------------------------|-----------------------------|-------------------------------------------------------------------|
| `AZURE_SUBSCRIPTION_ID`           | `empty`                     | Azure Subscription IDs (empty for auto lookup)                    |
| `AZURE_LOCATION`                  | `westeurope`, `northeurope` | Azure location for usage statitics                                |
| `SCRAPE_TIME`                     | `5m`                        | Default scrape time (time.Duration) between Azure API collections |
| `SCRAPE_TIME_GENERAL`             | -> SCRAPE_TIME              | Scrape time for General metrics                                   |
| `SCRAPE_TIME_RESOURCE`            | -> SCRAPE_TIME              | Scrape time for Resource metrics                                  |
| `SCRAPE_TIME_STORAGE`             | -> SCRAPE_TIME              | Scrape time for Storage metrics                                   |
| `SCRAPE_TIME_QUOTA`               | -> SCRAPE_TIME              | Scrape time for Quota metrics                                     |
| `SCRAPE_TIME_CONTAINERREGISTRY`   | -> SCRAPE_TIME              | Scrape time for ContainerRegistry metrics                         |
| `SCRAPE_TIME_CONTAINERINSTANCE`   | -> SCRAPE_TIME              | Scrape time for ContainerInstance metrics                         |
| `SCRAPE_TIME_SECURITY`            | -> SCRAPE_TIME              | Scrape time for Security metrics                                  |
| `SCRAPE_TIME_HEALTH`              | -> SCRAPE_TIME              | Scrape time for Health metrics                                    |
| `SERVER_BIND`                     | `:8080`                     | IP/Port binding                                                   |
| `AZURE_RESOURCE_GROUP_TAG`        | `owner`                     | Tags which should be included (methods available eg. `owner:lower` will transform content lowercase, methods: `lower`, `upper`, `title`)  |
| `AZURE_RESOURCE_TAG`              | `owner`                     | Tags which should be included (methods available eg. `owner:lower` will transform content lowercase, methods: `lower`, `upper`, `title`)  |
| `PORTSCAN`                        | `0`                         | Enables portscanner for public IPs (experimental)                 |
| `PORTSCAN_RANGE`                  | `1-65535`                   | Port range to scan (single port or range, mutliple ranges possible -> space as seperator)  |
| `PORTSCAN_TIME`                   | `3h`                        | Time (time.Duration) between portscanner runs                     |
| `PORTSCAN_PARALLEL`               | `2`                         | Parallel IPs which are scanned at the same time                   |
| `PORTSCAN_THREADS`                | `1000`                      | Number of threads per IP (parallel scanned ports)                 |
| `PORTSCAN_TIMEOUT`                | `5`                         | Timeout (seconds) for each port                                   |

for Azure API authentication (using ENV vars) see https://github.com/Azure/azure-sdk-for-go#authentication

Metrics
-------

| Metric                                         | Collector         | Description                                                                           |
|------------------------------------------------|-------------------|---------------------------------------------------------------------------------------|
| `azurerm_stats`                                | Exporter          | General exporter stats                                                                |
| `azurerm_subscription_info`                    | General           | Azure Subscription details (ID, name, ...)                                            |
| `azurerm_resourcegroup_info`                   | General           | Azure ResourceGroup details (subscriptionID, name, various tags ...)                  |
| `azurerm_ratelimit`                            | General           | Azure API ratelimit (left calls)                                                      |
| `azurerm_quota`                                | Quota             | Azure RM quota details (readable name, scope, ...)                                    |
| `azurerm_quota_current`                        | Quota             | Azure RM quota current (current value)                                                |
| `azurerm_quota_limit`                          | Quota             | Azure RM quota limit (maximum limited value)                                          |
| `azurerm_vm_info`                              | Computing         | Azure VM informations                                                                 |
| `azurerm_vm_os`                                | Computing         | Azure VM base image informations                                                      |
| `azurerm_publicip_info`                        | Computing         | Azure Public IPs details (subscriptionID, resourceGroup, ipAdress, ipVersion, ...)    |
| `azurerm_publicip_portscan_status`             | Computing         | Status of scanned ports (finished scan, elapsed time, updated timestamp)              |
| `azurerm_publicip_portscan_port`               | Portscan          | List of opend ports per IP                                                            |
| `azurerm_containerregistry_info`               | ContainerRegistry | List of Container registries                                                          |
| `azurerm_containerregistry_quota_current`      | ContainerRegistry | Quota usage of Container registries                                                   |
| `azurerm_containerregistry_quota_limit`        | ContainerRegistry | Quota limit of Container registries                                                   |
| `azurerm_containerinstance_info`               | ContainerInstance | List of Container instances                                                           |
| `azurerm_containerinstance_container`          | ContainerInstance | List of containers of container instances (container groups)                          |
| `azurerm_containerinstance_container_resource` | ContainerInstance | Container resource (request / limit) per container                                    |
| `azurerm_containerinstance_container_port`     | ContainerInstance | Container ports per container                                                         |
| `azurerm_securitycenter_compliance`            | Security          | Azure SecurityCenter compliance status                                                |
| `azurerm_advisor_recommendation`               | Security          | Azure Adisor recommendations (eg. security findings)                                  |
| `azurerm_resource_info`                        | Resource          | Azure Resource informations                                                           |
| `azurerm_resource_health`                      | Health            | Azure Resource health informations                                                    |
| `azurerm_storageaccount_info`                  | Storage           | Azure StorageAccount informations                                                     |
| `azurerm_manageddisk_info`                     | Storage           | Azure ManagedDisk informations                                                        |
| `azurerm_manageddisk_size`                     | Storage           | Azure ManagedDisk size                                                                |
| `azurerm_manageddisk_status`                   | Storage           | Azure ManagedDisk stats informations                                                  |
