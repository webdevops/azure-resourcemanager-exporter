package main

import (
	"fmt"
	"time"
	"context"
	"regexp"
	"strconv"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/network/mgmt/network"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/storage/mgmt/storage"
	"github.com/Azure/go-autorest/autorest"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	prometheusSubscription *prometheus.GaugeVec
	prometheusResourceGroup *prometheus.GaugeVec
	prometheusPublicIp *prometheus.GaugeVec
	prometheusApiQuota *prometheus.GaugeVec
	prometheusQuota *prometheus.GaugeVec
	prometheusQuotaCurrent *prometheus.GaugeVec
	prometheusQuotaLimit *prometheus.GaugeVec

	resourceGroupFromResourceIdRegExp = regexp.MustCompile("/resourceGroups/([^/]*)")
)

func initMetricsAzureRm() {
	prometheusSubscription = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_subscription_info",
			Help: "Azure ResourceManager subscription",
		},
		[]string{"subscriptionID", "subscriptionName", "spendingLimit", "quotaID", "locationPlacementID"},
	)

	prometheusResourceGroup = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_resourcegroup_info",
			Help: "Azure ResourceManager resourcegroups",
		},
		append(
			[]string{"subscriptionID", "resourceGroup", "location"},
			prefixSlice(AZURE_RESOURCEGROUP_TAG_PREFIX, opts.AzureResourceGroupTags)...
		),
	)

	prometheusPublicIp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_publicip_info",
			Help: "Azure ResourceManager public ip",
		},
		[]string{"subscriptionID", "resourceGroup", "location", "ipAddress", "ipAllocationMethod", "ipAdressVersion"},
	)

	prometheusApiQuota = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_ratelimit",
			Help: "Azure ResourceManager ratelimit",
		},
		[]string{"subscriptionID", "scope", "type"},
	)

	prometheusQuota = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_quota_info",
			Help: "Azure ResourceManager quota info",
		},
		[]string{"subscriptionID", "location", "scope", "quota", "quotaName"},
	)

	prometheusQuotaCurrent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_quota_current",
			Help: "Azure ResourceManager quota current value",
		},
		[]string{"subscriptionID", "location", "scope", "quota"},
	)

	prometheusQuotaLimit = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_quota_limit",
			Help: "Azure ResourceManager quota limit",
		},
		[]string{"subscriptionID", "location", "scope", "quota"},
	)


	prometheus.MustRegister(prometheusSubscription)
	prometheus.MustRegister(prometheusResourceGroup)
	prometheus.MustRegister(prometheusPublicIp)
	prometheus.MustRegister(prometheusApiQuota)
	prometheus.MustRegister(prometheusQuota)
	prometheus.MustRegister(prometheusQuotaCurrent)
	prometheus.MustRegister(prometheusQuotaLimit)

	go func() {
		for {
			go func() {
				probeCollect()
			}()
			time.Sleep(time.Duration(opts.ScrapeTime) * time.Second)
		}
	}()
}



func probeCollect() {
	context := context.Background()

	prometheusResourceGroup.Reset()
	prometheusPublicIp.Reset()

	for _, subscription := range AzureSubscriptions {
		subscriptionClient := subscriptions.NewClient()
		subscriptionClient.Authorizer = AzureAuthorizer

		sub, err := subscriptionClient.Get(context, *subscription.SubscriptionID)
		if err != nil {
			panic(err)
		}

		//---------------------------------------
		// Subscription
		//---------------------------------------

		prometheusSubscription.With(
			prometheus.Labels{
				"subscriptionID": *sub.SubscriptionID,
				"subscriptionName": *sub.DisplayName,
				"spendingLimit": string(sub.SubscriptionPolicies.SpendingLimit),
				"quotaID": *sub.SubscriptionPolicies.QuotaID,
				"locationPlacementID": *sub.SubscriptionPolicies.LocationPlacementID,
			},
		).Set(1)

		// subscription rate limits
		probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-subscription-reads", prometheus.Labels{"subscriptionID": *subscription.SubscriptionID, "scope": "subscription", "type": "read"})
		probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-subscription-resource-requests", prometheus.Labels{"subscriptionID": *subscription.SubscriptionID, "scope": "subscription", "type": "resource-requests"})
		probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-subscription-resource-entities-read", prometheus.Labels{"subscriptionID": *subscription.SubscriptionID, "scope": "subscription", "type": "resource-entities-read"})

		// tenant rate limits
		probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-tenant-reads", prometheus.Labels{"subscriptionID": *subscription.SubscriptionID, "scope": "tenant", "type": "read"})
		probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-tenant-resource-requests", prometheus.Labels{"subscriptionID": *subscription.SubscriptionID, "scope": "tenant", "type": "resource-requests"})
		probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-tenant-resource-entities-read", prometheus.Labels{"subscriptionID": *subscription.SubscriptionID, "scope": "tenant", "type": "resource-entities-read"})

		//---------------------------------------
		// ResourceGroups
		//---------------------------------------

		resourceGroupClient := resources.NewGroupsClient(*subscription.SubscriptionID)
		resourceGroupClient.Authorizer = AzureAuthorizer

		resourceGroupResult, err := resourceGroupClient.ListComplete(context, "", nil)
		if err != nil {
			panic(err)
		}

		for _, item := range *resourceGroupResult.Response().Value {
			rgLabels := prometheus.Labels{
				"subscriptionID": *sub.SubscriptionID,
				"resourceGroup": *item.Name,
				"location": *item.Location,
			}

			for _, rgTag := range opts.AzureResourceGroupTags {
				rgTabLabel := AZURE_RESOURCEGROUP_TAG_PREFIX + rgTag

				if _, ok := item.Tags[rgTag]; ok {
					rgLabels[rgTabLabel] = *item.Tags[rgTag]
				} else {
					rgLabels[rgTabLabel] = ""
				}
			}
			prometheusResourceGroup.With(rgLabels).Set(1)
		}

		//---------------------------------------
		// Public IPs
		//---------------------------------------

		netPublicIpClient := network.NewPublicIPAddressesClient(*sub.SubscriptionID)
		netPublicIpClient.Authorizer = AzureAuthorizer

		list, err := netPublicIpClient.ListAll(context)
		if err != nil {
			panic(err)
		}

		publicIpList := []string{}
		for _, val := range list.Values() {
			ipAdress := ""

			if val.IPAddress != nil {
				ipAdress = *val.IPAddress
				publicIpList = append(publicIpList, ipAdress)
			} else {
				ipAdress = "none"
			}

			resourceGroup := ""
			rgSubMatch := resourceGroupFromResourceIdRegExp.FindStringSubmatch(*val.ID)

			if len(rgSubMatch) >= 1 {
				resourceGroup = rgSubMatch[1]
			}

			prometheusPublicIp.With(prometheus.Labels{
				"subscriptionID": *sub.SubscriptionID,
				"resourceGroup": resourceGroup,
				"location": *val.Location,
				"ipAddress": ipAdress,
				"ipAllocationMethod": string(val.PublicIPAllocationMethod),
				"ipAdressVersion": string(val.PublicIPAddressVersion),
			}).Set(1)
		}

		// update portscanner public ips
		if portscanner != nil {
			portscanner.SetIps(publicIpList)
			portscanner.Cleanup()
		}

		//---------------------------------------
		// Computer usage
		//---------------------------------------

		computeClient := compute.NewUsageClient(*sub.SubscriptionID)
		computeClient.Authorizer = AzureAuthorizer
		for _, location := range opts.AzureLocation {
			list, err := computeClient.List(context, location)

			if err != nil {
				panic(err)
			}

			for _, val := range list.Values() {
				labels := prometheus.Labels{"subscriptionID": *sub.SubscriptionID, "location": location, "scope": "compute", "quota": *val.Name.Value}
				infoLabels := prometheus.Labels{"subscriptionID": *sub.SubscriptionID, "location": location, "scope": "compute", "quota": *val.Name.Value, "quotaName": *val.Name.LocalizedValue}
				prometheusQuota.With(infoLabels).Set(1)
				prometheusQuotaCurrent.With(labels).Set(float64(*val.CurrentValue))
				prometheusQuotaLimit.With(labels).Set(float64(*val.Limit))
			}
		}

		//---------------------------------------
		// Network usage
		//---------------------------------------

		// network usage
		// disabled due to
		// https://github.com/Azure/azure-sdk-for-go/issues/2340
		// https://github.com/Azure/azure-rest-api-specs/issues/1624
		//networkClient := network.NewUsagesClient(*sub.SubscriptionID)
		//networkClient.Authorizer = AzureAuthorizer
		//for _, location := range opts.AzureLocation {
		//	list, err := networkClient.List(context, location)
		//
		//	if err != nil {
		//		panic(err)
		//	}
		//
		//	for _, val := range list.Values() {
		//      labels := prometheus.Labels{"subscriptionID": *sub.SubscriptionID, "location": location, "scope": "storage", "quota": *val.Name.Value}
		//      infoLabels := prometheus.Labels{"subscriptionID": *sub.SubscriptionID, "location": location, "scope": "storage", "quota": *val.Name.Value, "quotaName": *val.Name.LocalizedValue}
		//    	prometheusQuota.With(infoLabels).Set(1)
		//		prometheusQuotaCurrent.With(labels).Set(float64(*val.CurrentValue))
		//		prometheusQuotaLimit.With(labels).Set(float64(*val.Limit))
		//	}
		//}


		//---------------------------------------
		// Storage usage
		//---------------------------------------

		storageClient := storage.NewUsageClient(*sub.SubscriptionID)
		storageClient.Authorizer = AzureAuthorizer
		for _, location := range opts.AzureLocation {
			list, err := storageClient.List(context)

			if err != nil {
				panic(err)
			}

			for _, val := range *list.Value {
				labels := prometheus.Labels{"subscriptionID": *sub.SubscriptionID, "location": location, "scope": "storage", "quota": *val.Name.Value}
				infoLabels := prometheus.Labels{"subscriptionID": *sub.SubscriptionID, "location": location, "scope": "storage", "quota": *val.Name.Value, "quotaName": *val.Name.LocalizedValue}
				prometheusQuota.With(infoLabels).Set(1)
				prometheusQuotaCurrent.With(labels).Set(float64(*val.CurrentValue))
				prometheusQuotaLimit.With(labels).Set(float64(*val.Limit))
			}
		}
	}
}

// read header and set prometheus api quota (if found)
func probeProcessHeader(response autorest.Response, header string, labels prometheus.Labels) {
	if val := response.Header.Get(header); val != "" {
		valFloat, err := strconv.ParseFloat(val, 64)

		if err == nil {
			prometheusApiQuota.With(labels).Set(valFloat)
		} else {
			ErrorLogger.Error(fmt.Sprintf("Failed to parse value '%v':", val), err)
		}
	}
}
