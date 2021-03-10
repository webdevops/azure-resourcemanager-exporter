Azure ResourceManager Exporter
==============================

[![license](https://img.shields.io/github/license/webdevops/azure-resourcemanager-exporter.svg)](https://github.com/webdevops/azure-resourcemanager-exporter/blob/master/LICENSE)
[![DockerHub](https://img.shields.io/badge/DockerHub-webdevops%2Fazure--resourcemanager--exporter-blue)](https://hub.docker.com/r/webdevops/azure-resourcemanager-exporter/)
[![Quay.io](https://img.shields.io/badge/Quay.io-webdevops%2Fazure--resourcemanager--exporter-blue)](https://quay.io/repository/webdevops/azure-resourcemanager-exporter)

Prometheus exporter for Azure Resources and information.

Installation
-------------

[Helm 3](https://helm.sh/) must be installed to use the chart. Once Helm is set up properly, add the repo as follows:

```shell
$ helm repo add azure-resourcemanager-exporter https://carrefour-group.github.io/azure-resourcemanager-exporter
$ helm repo update
```

Install Chart
-------------
The chart can be installed as follows:
```shell
$ helm install [RELEASE_NAME] azure-resourcemanager-exporter/azure-resourcemanager-exporter
```

Uninstall Chart
-------------
To uninstall the chart 

```shell
$ helm uninstall [RELEASE_NAME] 
```

Exporter Configuration
---------------------

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
| `SCRAPE_TIME_EVENTHUB`            | `30m`        `              | Scrape time for Eventhub metrics                                  |
| `SCRAPE_TIME_SECURITY`            | -> SCRAPE_TIME              | Scrape time for Security metrics                                  |
| `SCRAPE_TIME_HEALTH`              | -> SCRAPE_TIME              | Scrape time for Health metrics                                    |
| `SCRAPE_TIME_IAM`                 | -> SCRAPE_TIME              | Scrape time for AzurAD IAM (roledefinitions, rolebindings, principals) metrics  |
| `SCRAPE_TIME_GRAPH`               | -> SCRAPE_TIME              | Scrape time for AzurAD Graph metrics                              |
| `SERVER_BIND`                     | `:8080`                     | IP/Port binding                                                   |
| `AZURE_RESOURCE_GROUP_TAG`        | `owner`                     | Tags which should be included (methods available eg. `owner:lower` will transform content lowercase, methods: `lower`, `upper`, `title`)  |
| `AZURE_RESOURCE_TAG`              | `owner`                     | Tags which should be included (methods available eg. `owner:lower` will transform content lowercase, methods: `lower`, `upper`, `title`)  |
| `PORTSCAN`                        | `0`                         | Enables portscanner for public IPs (experimental)                 |
| `PORTSCAN_RANGE`                  | `1-65535`                   | Port range to scan (single port or range, mutliple ranges possible -> space as seperator)  |
| `PORTSCAN_TIME`                   | `3h`                        | Time (time.Duration) between portscanner runs                     |
| `PORTSCAN_PARALLEL`               | `2`                         | Parallel IPs which are scanned at the same time                   |
| `PORTSCAN_THREADS`                | `1000`                      | Number of threads per IP (parallel scanned ports)                 |
| `PORTSCAN_TIMEOUT`                | `5`                         | Timeout (seconds) for each port                                   |
| `GRAPH_APPLICATION_FILTER`        | `empty`                     | AzureAD graph application filter, eg. `startswith(displayName,'foo')` |

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
| `azurerm_vm_info`                              | Computing         | Azure VM information                                                                  |
| `azurerm_vm_os`                                | Computing         | Azure VM base image information                                                       |
| `azurerm_vm_nic`                               | Computing         | Azure VM network card information                                                     |
| `azurerm_vmss_info`                            | Computing         | Azure VMSS base image information                                                     |
| `azurerm_vmss_capacity`                        | Computing         | Azure VMSS capacity (number of instances)                                             |
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
| `azurerm_eventhub_namespace_info`              | Eventhub          | Eventhub namespace info                                                               |
| `azurerm_eventhub_namespace_status`            | Eventhub          | Eventhub namespace status (maximumThroughputUnits)                                    |
| `azurerm_eventhub_namespace_eventhub_info`     | Eventhub          | Eventhub namespace eventhub info                                                      |
| `azurerm_eventhub_namespace_eventhub_status`   | Eventhub          | Eventhub namespace eventhub status (partitionCount, messageRetentionInDays)           |
| `azurerm_securitycenter_compliance`            | Security          | Azure SecurityCenter compliance status                                                |
| `azurerm_advisor_recommendation`               | Security          | Azure Adisor recommendations (eg. security findings)                                  |
| `azurerm_resource_info`                        | Resource          | Azure Resource information                                                            |
| `azurerm_resource_health`                      | Health            | Azure Resource health information                                                     |
| `azurerm_storageaccount_info`                  | Storage           | Azure StorageAccount information                                                      |
| `azurerm_manageddisk_info`                     | Storage           | Azure ManagedDisk information                                                         |
| `azurerm_manageddisk_size`                     | Storage           | Azure ManagedDisk size                                                                |
| `azurerm_manageddisk_status`                   | Storage           | Azure ManagedDisk stats information                                                   |
| `azurerm_iam_roleassignment_info`              | IAM               | Azure IAM RoleAssignment information                                                  |
| `azurerm_iam_roledefinition_info`              | IAM               | Azure IAM RoleDefinition information                                                  |
| `azurerm_iam_principal_info`                   | IAM               | Azure IAM Principal information                                                       |
| `azurerm_graph_app_info`                       | Graph             | AzureAD graph application information                                                 |
| `azurerm_graph_app_credential`                 | Graph             | AzureAD graph application credentials (create,expiry) information                     |


Contributing
------------
We welcome any contributions from the community with open arms. If you're planning a new feature, please file an issue to discuss first.

How to Release
--------------
To release a new version of the helm chart, you need to bump:
 * `appVersion` and `version` fields in `Chart.yaml` file.
 * `image.tag` in `values.yaml` file.
