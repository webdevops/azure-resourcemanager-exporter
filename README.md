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
| `SCRAPE_TIME`                     | `2m`                        | Time (time.Duration) between API calls                                            |
| `SERVER_BIND`                     | `:8080`                     | IP/Port binding                                                   |
| `AZURE_RESOURCEGROUP_TAG`         | `owner`                     | Tags which should be included                                     |
| `PORTSCAN`                        | `0`                         | Enables portscanner for public IPs (experimental)                 |
| `PORTSCAN_RANGE`                  | `1-65535`                   | Port range to scan (single port or range, mutliple ranges possible -> space as seperator)  |
| `PORTSCAN_TIME`                   | `30m`                       | Time (time.Duration) between portscanner runs                           |
| `PORTSCAN_PARALLEL`               | `2`                         | Parallel IPs which are scanned at the same time                   |
| `PORTSCAN_THREADS`                | `1000`                      | Number of threads per IP (parallel scanned ports)                 |
| `PORTSCAN_TIMEOUT`                | `5`                         | Timeout (seconds) for each port                                   |

for Azure API authentication (using ENV vars) see https://github.com/Azure/azure-sdk-for-go#authentication

Metrics
-------

| Metric                              | Description                                                                           |
|-------------------------------------|---------------------------------------------------------------------------------------|
| `azurerm_subscription_info`         | Azure Subscription details (ID, name, ...)                                            |
| `azurerm_resourcegroup_info`        | Azure ResourceGroup details (subscriptionID, name, various tags ...)                  |
| `azurerm_publicip_info`             | Azure Public IPs details (subscriptionID, resourceGroup, ipAdress, ipVersion, ...)    |
| `azurerm_ratelimit`                 | Azure API ratelimit (left calls)                                                      |
| `azurerm_quota`                     | Azure RM quota details (readable name, scope, ...)                                    |
| `azurerm_quota_current`             | Azure RM quota current (current value)                                                |
| `azurerm_quota_limit`               | Azure RM quota limit (maximum limited value)                                          |
| `azurerm_publicip_portscan_status`  | Status of scanned ports (finished scan, elapsed time, updated timestamp)              |
| `azurerm_publicip_portscan_port`    | List of opend ports per IP                                                            |
