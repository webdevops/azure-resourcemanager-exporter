Azure ResourceManager Exporter
==============================

[![license](https://img.shields.io/github/license/webdevops/azure-resourcemanager-exporter.svg)](https://github.com/webdevops/azure-resourcemanager-exporter/blob/master/LICENSE)
[![Docker](https://img.shields.io/badge/docker-webdevops%2Fazure--resourcemanager--exporter-blue.svg?longCache=true&style=flat&logo=docker)](https://hub.docker.com/r/webdevops/azure-resourcemanager-exporter/)
[![Docker Build Status](https://img.shields.io/docker/build/webdevops/azure-resourcemanager-exporter.svg)](https://hub.docker.com/r/webdevops/azure-resourcemanager-exporter/)

Prometheus exporter for Azure API ratelimit (currently read only) and quota usage. It also exports Public IPs and opend ports (portscanner).

Configuration
-------------

Normally no configuration is needed but can be customized using environment variables.

| Environment variable              | DefaultValue                | Description                                                       |
|-----------------------------------|-----------------------------|-------------------------------------------------------------------|
| `AZURE_SUBSCRIPTION_ID`           | `empty`                     | Azure Subscription IDs (empty for auto lookup)                    |
| `AZURE_LOCATION`                  | `westeurope`, `northeurope` | Azure location for usage statitics                                |
| `SCRAPE_TIME`                     | `5m`                        | Time (time.Duration) between Azure API collections                |
| `SERVER_BIND`                     | `:8080`                     | IP/Port binding                                                   |
| `AZURE_RESOURCE_TAG`              | `owner`                     | Tags which should be included                                     |
| `PORTSCAN`                        | `0`                         | Enables portscanner for public IPs (experimental)                 |
| `PORTSCAN_RANGE`                  | `1-65535`                   | Port range to scan (single port or range, mutliple ranges possible -> space as seperator)  |
| `PORTSCAN_TIME`                   | `3h`                        | Time (time.Duration) between portscanner runs                     |
| `PORTSCAN_PARALLEL`               | `2`                         | Parallel IPs which are scanned at the same time                   |
| `PORTSCAN_THREADS`                | `1000`                      | Number of threads per IP (parallel scanned ports)                 |
| `PORTSCAN_TIMEOUT`                | `5`                         | Timeout (seconds) for each port                                   |

for Azure API authentication (using ENV vars) see https://github.com/Azure/azure-sdk-for-go#authentication

Metrics
-------

| Metric                                         | Description                                                                           |
|------------------------------------------------|---------------------------------------------------------------------------------------|
| `azurerm_subscription_info`                    | Azure Subscription details (ID, name, ...)                                            |
| `azurerm_resourcegroup_info`                   | Azure ResourceGroup details (subscriptionID, name, various tags ...)                  |
| `azurerm_publicip_info`                        | Azure Public IPs details (subscriptionID, resourceGroup, ipAdress, ipVersion, ...)    |
| `azurerm_ratelimit`                            | Azure API ratelimit (left calls)                                                      |
| `azurerm_quota`                                | Azure RM quota details (readable name, scope, ...)                                    |
| `azurerm_quota_current`                        | Azure RM quota current (current value)                                                |
| `azurerm_quota_limit`                          | Azure RM quota limit (maximum limited value)                                          |
| `azurerm_publicip_portscan_status`             | Status of scanned ports (finished scan, elapsed time, updated timestamp)              |
| `azurerm_publicip_portscan_port`               | List of opend ports per IP                                                            |
| `azurerm_containerregistry_info`               | List of Container registries                                                          |
| `azurerm_containerregistry_quota_current`      | Quota usage of Container registries                                                   |
| `azurerm_containerregistry_quota_limit`        | Quota limit of Container registries                                                   |
| `azurerm_containerinstance_info`               | List of Container instances                                                           |
| `azurerm_containerinstance_container`          | List of containers of container instances (container groups)                          |
| `azurerm_containerinstance_container_resource` | Container resource (request / limit) per container                                    |
| `azurerm_containerinstance_container_port`     | Container ports per container                                                         |
| `azurerm_securitycenter_compliance`            | Azure SecurityCenter compliance status                                                |
| `azurerm_advisor_recommendation`               | Azure Adisor recommendations (eg. security findings)                                  |
