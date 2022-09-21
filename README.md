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

Normally no configuration is needed but can be customized using environment variables.

(to disable specific scrape collectors set them to `0` or set `SCRAPE_TIME` to `0` to disable all by default)

```
Usage:
  azure-resourcemanager-exporter [OPTIONS]

Application Options:
      --log.debug                         debug mode [$LOG_DEBUG]
      --log.trace                         trace mode [$LOG_TRACE]
      --log.json                          Switch log output to json format [$LOG_JSON]
      --azure.tenant=                     Azure tenant id [$AZURE_TENANT_ID]
      --azure.environment=                Azure environment name (default: AZUREPUBLICCLOUD) [$AZURE_ENVIRONMENT]
      --azure.subscription=               Azure subscription ID (space delimiter) [$AZURE_SUBSCRIPTION_ID]
      --azure.location=                   Azure locations (space delimiter) (default: westeurope, northeurope) [$AZURE_LOCATION]
      --azure.resourcegroup.tag=          Azure ResourceGroup tags (space delimiter) (default: owner) [$AZURE_RESOURCEGROUP_TAG]
      --azure.resource.tag=               Azure Resource tags (space delimiter) (default: owner) [$AZURE_RESOURCE_TAG]
      --scrape.time=                      Default scrape time (time.duration) (default: 5m) [$SCRAPE_TIME]
      --scrape.time.exporter=             Scrape time for exporter metrics (time.duration) (default: 10s) [$SCRAPE_TIME_EXPORTER]
      --scrape.time.general=              Scrape time for general metrics (time.duration) [$SCRAPE_TIME_GENERAL]
      --scrape.time.resource=             Scrape time for resource metrics  (time.duration) [$SCRAPE_TIME_RESOURCE]
      --scrape.time.quota=                Scrape time for quota metrics  (time.duration) [$SCRAPE_TIME_QUOTA]
      --scrape.time.security=             Scrape time for Security metrics (time.duration) [$SCRAPE_TIME_SECURITY]
      --scrape.time.resourcehealth=       Scrape time for ResourceHealth metrics (time.duration) [$SCRAPE_TIME_RESOURCEHEALTH]
      --scrape.time.iam=                  Scrape time for IAM metrics (time.duration) [$SCRAPE_TIME_IAM]
      --scrape.time.graph=                Scrape time for Graph metrics (time.duration) [$SCRAPE_TIME_GRAPH]
      --scrape.time.costs=                Scrape time for costs/consumtion metrics (time.duration; BETA) (default: 0)
                                          [$SCRAPE_TIME_COSTS]
      --resourcehealth.summary.maxlength= Max length of ResourceHealth summary label (0 = disable summary label) (default: 0)
                                          [$RESOURCEHEALTH_SUMMARY_MAXLENGTH]
      --graph.application.filter=         MS Graph application $filter query eg: startswith(displayName,'A')
                                          [$GRAPH_APPLICATION_FILTER]
      --costs.timeframe=                  Timeframe for cost reportings  (space delimiter) (default: MonthToDate, YearToDate)
                                          [$COSTS_TIMEFRAME]
      --costs.dimension=                  Dimensions for detailed cost metrics (eg
                                          'ResourceGroup','ResourceGroupName','ResourceLocation','ConsumedService','ResourceType-
                                          ','ResourceId','MeterId','BillingMonth','MeterCategory','MeterSubcategory','Meter','Ac-
                                          countName','DepartmentName','SubscriptionId','SubscriptionName','ServiceName','Service-
                                          Tier','EnrollmentAccountName','BillingAccountId','ResourceGuid','BillingPeriod','Invoi-
                                          ceNumber','ChargeType','PublisherType','ReservationId','ReservationName','Frequency','-
                                          PartNumber','CostAllocationRuleName','MarkupRuleName','PricingModel')  (space
                                          delimiter) (default: ResourceType, ResourceLocation) [$COSTS_DIMENSION]
      --portscan                          Enable portscan for public IPs [$PORTSCAN]
      --portscan.time=                    Portscan time (time.duration) (default: 3h) [$PORTSCAN_TIME]
      --portscan.parallel=                Portscan parallel scans (parallel * threads = concurrent gofuncs) (default: 2)
                                          [$PORTSCAN_PARALLEL]
      --portscan.threads=                 Portscan threads (concurrent port scans per IP) (default: 1000) [$PORTSCAN_THREADS]
      --portscan.timeout=                 Portscan timeout (seconds) (default: 5) [$PORTSCAN_TIMEOUT]
      --portscan.range=                   Portscan port range (first-last)  (space delimiter) (default: 1-65535)
                                          [$PORTSCAN_RANGE]
      --cache.path=                       Cache path [$CACHE_PATH]
      --server.bind=                      Server address (default: :8080) [$SERVER_BIND]
      --server.timeout.read=              Server read timeout (default: 5s) [$SERVER_TIMEOUT_READ]
      --server.timeout.write=             Server write timeout (default: 10s) [$SERVER_TIMEOUT_WRITE]

Help Options:
  -h, --help                              Show this help message
```

for Azure API authentication (using ENV vars) see https://docs.microsoft.com/en-us/azure/developer/go/azure-sdk-authentication

## Deprecations/old resource metrics

Please use [`azure-resourcegraph-exporter`](https://github.com/webdevops/azure-resourcegraph-exporter) for exporting resources.
This exporter is using Azure ResourceGraph queries and not wasting Azure API calls for fetching metrics.

`azure-resourcegraph-exporter` provides a way how metrics can be build by using Kusto queries.

## Azure permissions

This exporter needs `Reader` permissions on subscription level.

For Azure write rate limits it tries to tag the subscription with an empty tag set (actually no changes).
For this operation it needs `Microsoft.Resources/tags/write` on scope `/subscription/*`.

To disable write rate limits set `SCRAPE_RATELIMIT_WRITE` to `0`.

## Metrics

| Metric                                         | Collector           | Description                                                                                                                       |
|------------------------------------------------|---------------------|-----------------------------------------------------------------------------------------------------------------------------------|
| `azurerm_stats`                                | Exporter            | General exporter stats                                                                                                            |
| `azurerm_consumtion_bugdet_info`               | Costs               | Azure CostManagement bugdet information                                                                                           |
| `azurerm_consumtion_bugdet_limit`              | Costs               | Limit of CostManagemnet budget                                                                                                    |
| `azurerm_consumtion_bugdet_current`            | Costs               | Current costs of CostManagement budget                                                                                            |
| `azurerm_consumtion_bugdet_usage`              | Costs               | Current budget usage in percentage                                                                                                |
| `azurerm_costmanagement_overall_usage`         | Costs               | CostManagement "usage" metric with timeframes by Subscription and ResourceGroup                                                   |
| `azurerm_costmanagement_overall_actualcost`    | Costs               | CostManagement "actualcosts" metric with timeframes by Subscription and ResourceGroup                                             |
| `azurerm_costmanagement_detail_usage`          | Costs               | CostManagement "usage" metric with timeframes by Subscription and ResourceGroup and cost dimensions (see `COSTS_DIMENSION`)       |
| `azurerm_costmanagement_detail_actualcost`     | Costs               | CostManagement "actualcosts" metric with timeframes by Subscription and ResourceGroup and cost dimensions (see `COSTS_DIMENSION`) |
| `azurerm_subscription_info`                    | General             | Azure Subscription details (ID, name, ...)                                                                                        |
| `azurerm_resource_health`                      | Health              | Azure Resource health information                                                                                                 |
| `azurerm_iam_roleassignment_info`              | IAM                 | Azure IAM RoleAssignment information                                                                                              |
| `azurerm_iam_roledefinition_info`              | IAM                 | Azure IAM RoleDefinition information                                                                                              |
| `azurerm_iam_principal_info`                   | IAM                 | Azure IAM Principal information                                                                                                   |
| `azurerm_quota_info`                           | Quota               | Azure RM quota details (readable name, scope, ...)                                                                                |
| `azurerm_quota_current`                        | Quota               | Azure RM quota current (current value)                                                                                            |
| `azurerm_quota_limit`                          | Quota               | Azure RM quota limit (maximum limited value)                                                                                      |
| `azurerm_quota_usage`                          | Quota               | Azure RM quota usage in percent                                                                                                   |
| `azurerm_resourcegroup_info`                   | Resource            | Azure ResourceGroup details (subscriptionID, name, various tags ...)                                                              |
| `azurerm_resource_info`                        | Resource            | Azure Resource information                                                                                                        |
| `azurerm_securitycenter_compliance`            | Security            | Azure SecurityCenter compliance status                                                                                            |
| `azurerm_advisor_recommendation`               | Security            | Azure Advisory recommendations (eg. security findings)                                                                            |
| `azurerm_graph_app_info`                       | Graph               | AzureAD graph application information                                                                                             |
| `azurerm_graph_app_credential`                 | Graph               | AzureAD graph application credentials (create,expiry) information                                                                 |
| `azurerm_publicip_info`                        | Portscan            | Azure PublicIP information                                                                                                        |
| `azurerm_publicip_portscan_status`             | Portscan            | Status of scanned ports (finished scan, elapsed time, updated timestamp)                                                          |
| `azurerm_publicip_portscan_port`               | Portscan            | List of opened ports per IP                                                                                                       |

### ResourceTags handling

Tag can be dynamically added to metrics and processed though filters

format is: `tagname?filter1` or `tagname?filter1&filter2`

| Tag filter | Description                 |
|------------|-----------------------------|
| `toLower`  | Lowercasing Azure tag value |
| `toUpper`  | Uppercasing Azure tag value |

### AzureTracing metrics

(with 22.2.0 and later)

Azuretracing metrics collects latency and latency from azure-sdk-for-go and creates metrics and is controllable using
environment variables (eg. setting buckets, disabling metrics or disable autoreset).

| Metric                                   | Description                                                                            |
|------------------------------------------|----------------------------------------------------------------------------------------|
| `azurerm_api_ratelimit`                  | Azure ratelimit metrics (only on /metrics, resets after query due to limited validity) |
| `azurerm_api_request_*`                  | Azure request count and latency as histogram                                           |

### Settings

| Environment variable                     | Example                            | Description                                                    |
|------------------------------------------|------------------------------------|----------------------------------------------------------------|
| `METRIC_AZURERM_API_REQUEST_BUCKETS`     | `1, 2.5, 5, 10, 30, 60, 90, 120`   | Sets buckets for `azurerm_api_request` histogram metric        |
| `METRIC_AZURERM_API_REQUEST_ENABLE`      | `false`                            | Enables/disables `azurerm_api_request_*` metric                |
| `METRIC_AZURERM_API_REQUEST_LABELS`      | `apiEndpoint, method, statusCode`  | Controls labels of `azurerm_api_request_*` metric              |
| `METRIC_AZURERM_API_RATELIMIT_ENABLE`    | `false`                            | Enables/disables `azurerm_api_ratelimit` metric                |
| `METRIC_AZURERM_API_RATELIMIT_AUTORESET` | `false`                            | Enables/disables `azurerm_api_ratelimit` autoreset after fetch |


| `azurerm_api_request` label | Status             | Description                                                                                              |
|-----------------------------|--------------------|----------------------------------------------------------------------------------------------------------|
| `apiEndpoint`               | enabled by default | hostname of endpoint (max 3 parts)                                                                       |
| `routingRegion`             | enabled by default | detected region for API call, either routing region from Azure Management API or Azure resource location |
| `subscriptionID`            | enabled by default | detected subscriptionID                                                                                  |
| `tenantID`                  | enabled by default | detected tenantID (extracted from jwt auth token)                                                        |
| `resourceProvider`          | enabled by default | detected Azure Management API provider                                                                   |
| `method`                    | enabled by default | HTTP method                                                                                              |
| `statusCode`                | enabled by default | HTTP status code                                                                                         |
