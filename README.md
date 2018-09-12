Azure ResourceManager Exporter
==============================

[![license](https://img.shields.io/github/license/mblaschke/azure-resourcemanager-exporter.svg)](https://github.com/mblaschke/azure-resourcemanager-exporter/blob/master/LICENSE)
[![Docker](https://img.shields.io/badge/docker-mblaschke%2Fazure--resourcemanager--exporter-blue.svg?longCache=true&style=flat&logo=docker)](https://hub.docker.com/r/mblaschke/azure-resourcemanager-exporter/)
[![Docker Build Status](https://img.shields.io/docker/build/mblaschke/azure-resourcemanager-exporter.svg)](https://hub.docker.com/r/mblaschke/azure-resourcemanager-exporter/)

Prometheus exporter for Azure API ratelimit (currently read only) and quota usage.

Configuration
-------------

Normally no configuration is needed but can be customized using environment variables.

| Environment variable              | DefaultValue                | Description                                                       |
|-----------------------------------|-----------------------------|-------------------------------------------------------------------|
| `AZURE_SUBSCRIPTION_ID`           | `empty`                     | Azure Subscription IDs (empty for auto lookup)                    |
| `AZURE_LOCATION`                  | `westeurope`, `northeurope` | Azure location for usage statitics                                |
| `SCRAPE_TIME`                     | `120`                       | Time between API calls                                            |
| `SERVER_BIND`                     | `:8080`                     | IP/Port binding                                                   |
| `AZURE_RESOURCEGROUP_TAGS`        | `owner`                     | Tags which should be included                                     |

for Azure API authentication (using ENV vars) see https://github.com/Azure/azure-sdk-for-go#authentication

Metrics
-------

| Environment variable              | Description                                                                           |
|-----------------------------------|---------------------------------------------------------------------------------------|
| `azurerm_subscription`            | Azure Subscription details (ID, name, ...)                                            |
| `azurerm_resourcegroup`           | Azure ResourceGroup details (subscriptionID, name, various tags ...)                  |
| `azurerm_publicip`                | Azure Public IPs details (subscriptionID, resourceGroup, ipAdress, ipVersion, ...)    |
| `azurerm_ratelimit`               | Azure API ratelimit (left calls)                                                      |
| `azurerm_quota`                   | Azure RM quota details (readable name, scope, ...)                                    |
| `azurerm_quota_current`           | Azure RM quota current (current value)                                                |
| `azurerm_quota_limit`             | Azure RM quota limit (maximum limited value)                                          |



