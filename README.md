Azure ResourceManager Exporter
==============================

[![license](https://img.shields.io/github/license/mblaschke/azure-resourcemanager-exporter.svg)](https://github.com/mblaschke/azure-resourcemanager-exporter/blob/master/LICENSE)
[![Docker](https://img.shields.io/badge/docker-mblaschke%2Fazure--scheduledevents--exporter-blue.svg?longCache=true&style=flat&logo=docker)](https://hub.docker.com/r/mblaschke/azure-resourcemanager-exporter/)
[![Docker Build Status](https://img.shields.io/docker/build/mblaschke/azure-resourcemanager-exporter.svg)](https://hub.docker.com/r/mblaschke/azure-resourcemanager-exporter/)

Prometheus exporter for Azure API ratelimit (currently read only)

Configuration
-------------

Normally no configuration is needed but can be customized using environment variables.

| Environment variable     | DefaultValue                | Description                                                       |
|--------------------------|-----------------------------|-------------------------------------------------------------------|
| `AZURE_SUBSCRIPTION_ID`  | `empty`                     | Azure Subscription IDs (empty for auto lookup)                    |
| `SCRAPE_TIME`            | `120`                       | Time between API calls                                            |
| `SERVER_BIND`            | `:8080`                     | IP/Port binding                                                   |

for Azure API authentication (using ENV vars) see https://github.com/Azure/azure-sdk-for-go#authentication
