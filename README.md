# Azure ResourceManager Exporter

[![license](https://img.shields.io/github/license/webdevops/azure-resourcemanager-exporter.svg)](https://github.com/webdevops/azure-resourcemanager-exporter/blob/master/LICENSE)
[![DockerHub](https://img.shields.io/badge/DockerHub-webdevops%2Fazure--resourcemanager--exporter-blue)](https://hub.docker.com/r/webdevops/azure-resourcemanager-exporter/)
[![Quay.io](https://img.shields.io/badge/Quay.io-webdevops%2Fazure--resourcemanager--exporter-blue)](https://quay.io/repository/webdevops/azure-resourcemanager-exporter)
[![Artifact Hub](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/azure-resourcemanager-exporter)](https://artifacthub.io/packages/search?repo=azure-resourcemanager-exporter)

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

## Configuration

```
Usage:
  azure-resourcemanager-exporter [OPTIONS]

Application Options:
      --log.debug             debug mode [$LOG_DEBUG]
      --log.devel             development mode [$LOG_DEVEL]
      --log.json              Switch log output to json format [$LOG_JSON]
      --config=               Path to config file [$CONFIG]
      --azure.tenant=         Azure tenant id [$AZURE_TENANT_ID]
      --azure.environment=    Azure environment name (default: AZUREPUBLICCLOUD) [$AZURE_ENVIRONMENT]
      --cache.path=           Cache path (to folder, file://path... or azblob://storageaccount.blob.core.windows.net/containername or
                              k8scm://{namespace}/{configmap}}) [$CACHE_PATH]
      --server.bind=          Server address (default: :8080) [$SERVER_BIND]
      --server.timeout.read=  Server read timeout (default: 5s) [$SERVER_TIMEOUT_READ]
      --server.timeout.write= Server write timeout (default: 10s) [$SERVER_TIMEOUT_WRITE]

Help Options:
  -h, --help                  Show this help message
```

for Azure API authentication (using ENV vars) see https://docs.microsoft.com/en-us/azure/developer/go/azure-sdk-authentication

## Config file

see [`example.yaml`](example.yaml)

## Deprecations/old resource metrics

Please use [`azure-resourcegraph-exporter`](https://github.com/webdevops/azure-resourcegraph-exporter) for exporting resources.
This exporter is using Azure ResourceGraph queries and not wasting Azure API calls for fetching metrics.

`azure-resourcegraph-exporter` provides a way how metrics can be build by using Kusto queries.

## Azure permissions

This exporter needs `Reader` permissions on subscription level.

## Metrics

| Metric                                      | Collector  | Description                                                                                 |
|---------------------------------------------|------------|---------------------------------------------------------------------------------------------|
| `azurerm_stats`                             | Exporter   | General exporter stats                                                                      |
| `azurerm_costs_budget_info`                 | Costs      | Azure CostManagement bugdet information                                                     |
| `azurerm_costs_budget_current`              | Costs      | Current value of CostManagemnet budget usage                                                |
| `azurerm_costs_budget_limit`                | Costs      | Limit of CostManagemnet budget                                                              |
| `azurerm_costs_budget_usage`                | Costs      | Percentage of usage of CostManagemnet budget                                                |
| `azurerm_costs_{queryName}`                 | Costs      | Costs query result (see `example.yaml`)                                                     |
| `azurerm_subscription_info`                 | General    | Azure Subscription details (ID, name, ...)                                                  |
| `azurerm_resource_health`                   | Health     | Azure Resource health information                                                           |
| `azurerm_iam_roleassignment_info`           | IAM        | Azure IAM RoleAssignment information                                                        |
| `azurerm_iam_roledefinition_info`           | IAM        | Azure IAM RoleDefinition information                                                        |
| `azurerm_iam_principal_info`                | IAM        | Azure IAM Principal information                                                             |
| `azurerm_quota_info`                        | Quota      | Azure RM quota details (readable name, scope, ...)                                          |
| `azurerm_quota_current`                     | Quota      | Azure RM quota current (current value)                                                      |
| `azurerm_quota_limit`                       | Quota      | Azure RM quota limit (maximum limited value)                                                |
| `azurerm_quota_usage`                       | Quota      | Azure RM quota usage in percent                                                             |
| `azurerm_resourcegroup_info`                | Resource   | Azure ResourceGroup details (subscriptionID, name, various tags ...)                        |
| `azurerm_resource_info`                     | Resource   | Azure Resource information                                                                  |
| `azurerm_advisor_recommendation`            | Advisor    | Azure Advisor recommendation    |
| `azurerm_defender_secure_score_percentage`  | Defender   | Azure Defender secure score percerntage per Subscription                                    |
| `azurerm_defender_secure_score_max`         | Defender   | The maximum number of points you can gain by completing all recommendations within a control |
| `azurerm_defender_secure_score_current`     | Defender   | The current Azure Defender secure score                                                     |
| `azurerm_defender_compliance_score`         | Defender   | Azure Defender compliance score (based on applied Policies)                                 |
| `azurerm_defender_compliance_resources`     | Defender   | Azure Defender count of compliance resource in assessment                                   |
| `azurerm_defender_advisor_recommendation`   | Defender   | Azure Defender recommendations (eg. security findings)                                      |
| `azurerm_graph_app_info`                    | Graph      | AzureAD graph application information                                                       |
| `azurerm_graph_app_tag`                     | Graph      | AzureAD graph application tag                                                               |
| `azurerm_graph_app_credential`              | Graph      | AzureAD graph application credentials (create,expiry) information                           |
| `azurerm_graph_serviceprincipal_info`       | Graph      | AzureAD graph servicePrincipal information                                                  |
| `azurerm_graph_serviceprincipal_tag`        | Graph      | AzureAD graph servicePrincipal tag                                                          |
| `azurerm_graph_serviceprincipal_credential` | Graph      | AzureAD graph servicePrincipal credentials (create,expiry) information                      |
| `azurerm_publicip_info`                     | Portscan   | Azure PublicIP information                                                                  |
| `azurerm_publicip_portscan_status`          | Portscan   | Status of scanned ports (finished scan, elapsed time, updated timestamp)                    |
| `azurerm_publicip_portscan_port`            | Portscan   | List of opened ports per IP                                                                 |

### ResourceTags handling

see [armclient tagmanager documentation](https://github.com/webdevops/go-common/blob/main/azuresdk/README.md#tag-manager)

### AzureTracing metrics

see [armclient tracing documentation](https://github.com/webdevops/go-common/blob/main/azuresdk/README.md#azuretracing-metrics)

### Caching

see [prometheus collector cache documentation](https://github.com/webdevops/go-common/blob/main/prometheus/README.md#caching)

