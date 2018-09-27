package main

import (
	"fmt"
	"sync"
	"time"
	"regexp"
	"strconv"
	"context"
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

// Create and setup metrics and collection
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
}

// Start backgrounded metrics collection
func startMetricsCollectionAzureRm() {
	go func() {
		for {
			go func() {
				runMetricsCollectionAzureRm()
			}()
			time.Sleep(opts.ScrapeTime)
		}
	}()
}

// Metrics run
func runMetricsCollectionAzureRm() {
	var wg sync.WaitGroup
	context := context.Background()

	publicIpChannel := make(chan []string)
	callbackChannel := make(chan func())

	for _, subscription := range AzureSubscriptions {
		Logger.Messsage(
			"Starting metrics update for Azure Subscription %v",
			*subscription.SubscriptionID,
		)

		// Subscription
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			collectAzureSubscription(context, subscriptionId, callbackChannel)
			Logger.Verbose("%v: finished Azure Subscription collection", subscriptionId)
		}(*subscription.SubscriptionID)

		// ResourceGroups
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			collectAzureResourceGroup(context, subscriptionId, callbackChannel)
			Logger.Verbose("%v: finished Azure ResourceGroup collection", subscriptionId)
		}(*subscription.SubscriptionID)

		// Public IPs
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			publicIpChannel <- collectAzurePublicIp(context, subscriptionId, callbackChannel)
			Logger.Verbose("%v: finished Azure PublicIP collection", subscriptionId)
		}(*subscription.SubscriptionID)

		// Compute usage
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			collectAzureComputeUsage(context, subscriptionId, callbackChannel)
			Logger.Verbose("%v: finished Azure ComputerUsage collection", subscriptionId)
		}(*subscription.SubscriptionID)

		// Network usage
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			// disabled due to
			// https://github.com/Azure/azure-sdk-for-go/issues/2340
			// https://github.com/Azure/azure-rest-api-specs/issues/1624
			//collectAzureNetworkUsage(context, subscriptionId, callbackChannel)
			Logger.Verbose("%v: finished Azure NetworkUsage collection (DISABLED -> AZURE BUG)", subscriptionId)
		}(*subscription.SubscriptionID)

		// Storage usage
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			collectAzureStorageUsage(context, subscriptionId, callbackChannel)
			Logger.Verbose("%v: finished Azure StorageUsage collection", subscriptionId)
		}(*subscription.SubscriptionID)
	}

	// process publicIP list and pass it to portscanner
	go func() {
		publicIpList := []string{}
		for ipAddressList := range publicIpChannel {
			publicIpList = append(publicIpList, ipAddressList...)
		}

		// update portscanner public ips
		if portscanner != nil {
			portscanner.SetIps(publicIpList)
			portscanner.Cleanup()
			portscanner.Enable()
		}
		Logger.Messsage("run: collected %v public IPs", len(publicIpList))
	}()

	// collect metrics (callbacks) and proceses them
	go func() {
		var callbackList []func()
		for callback := range callbackChannel {
			callbackList = append(callbackList, callback)
		}

		prometheusResourceGroup.Reset()
		prometheusPublicIp.Reset()
		for _, callback := range callbackList {
			callback()
		}

		Logger.Messsage("run: finished")
	}()

	// wait for all funcs
	wg.Wait()
	close(publicIpChannel)
	close(callbackChannel)
}

// Collect Azure Subscription metrics
func collectAzureSubscription(context context.Context, subscriptionId string, callback chan<- func()) {
	subscriptionClient := subscriptions.NewClient()
	subscriptionClient.Authorizer = AzureAuthorizer

	sub, err := subscriptionClient.Get(context, subscriptionId)
	if err != nil {
		panic(err)
	}

	callback <- func() {
		prometheusSubscription.With(
			prometheus.Labels{
				"subscriptionID":      *sub.SubscriptionID,
				"subscriptionName":    *sub.DisplayName,
				"spendingLimit":       string(sub.SubscriptionPolicies.SpendingLimit),
				"quotaID":             *sub.SubscriptionPolicies.QuotaID,
				"locationPlacementID": *sub.SubscriptionPolicies.LocationPlacementID,
			},
		).Set(1)
	}

	// subscription rate limits
	probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-subscription-reads", prometheus.Labels{"subscriptionID": subscriptionId, "scope": "subscription", "type": "read"}, callback)
	probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-subscription-resource-requests", prometheus.Labels{"subscriptionID": subscriptionId, "scope": "subscription", "type": "resource-requests"}, callback)
	probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-subscription-resource-entities-read", prometheus.Labels{"subscriptionID": subscriptionId, "scope": "subscription", "type": "resource-entities-read"}, callback)

	// tenant rate limits
	probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-tenant-reads", prometheus.Labels{"subscriptionID": subscriptionId, "scope": "tenant", "type": "read"}, callback)
	probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-tenant-resource-requests", prometheus.Labels{"subscriptionID": subscriptionId, "scope": "tenant", "type": "resource-requests"}, callback)
	probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-tenant-resource-entities-read", prometheus.Labels{"subscriptionID": subscriptionId, "scope": "tenant", "type": "resource-entities-read"}, callback)
}

// Collect Azure ResourceGroup metrics
func collectAzureResourceGroup(context context.Context, subscriptionId string, callback chan<- func()) {
	resourceGroupClient := resources.NewGroupsClient(subscriptionId)
	resourceGroupClient.Authorizer = AzureAuthorizer

	resourceGroupResult, err := resourceGroupClient.ListComplete(context, "", nil)
	if err != nil {
		panic(err)
	}

	for _, item := range *resourceGroupResult.Response().Value {
		rgLabels := prometheus.Labels{
			"subscriptionID": subscriptionId,
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

		callback <- func() {
			prometheusResourceGroup.With(rgLabels).Set(1)
		}
	}
}

// Collect Azure PublicIP metrics
func collectAzurePublicIp(context context.Context, subscriptionId string, callback chan<- func()) (ipAddressList []string) {
	netPublicIpClient := network.NewPublicIPAddressesClient(subscriptionId)
	netPublicIpClient.Authorizer = AzureAuthorizer

	list, err := netPublicIpClient.ListAll(context)
	if err != nil {
		panic(err)
	}

	for _, val := range list.Values() {
		location := *val.Location
		ipAddress := ""
		ipAllocationMethod := string(val.PublicIPAllocationMethod)
		ipAdressVersion := string(val.PublicIPAddressVersion)
		gaugeValue := float64(1)

		if val.IPAddress != nil {
			ipAddress = *val.IPAddress
			ipAddressList = append(ipAddressList, ipAddress)
		} else {
			ipAddress = "not allocated"
			gaugeValue = 0
		}

		resourceGroup := ""
		rgSubMatch := resourceGroupFromResourceIdRegExp.FindStringSubmatch(*val.ID)

		if len(rgSubMatch) >= 1 {
			resourceGroup = rgSubMatch[1]
		}

		callback <- func() {
			prometheusPublicIp.With(prometheus.Labels{
				"subscriptionID":     subscriptionId,
				"resourceGroup":      resourceGroup,
				"location":           location,
				"ipAddress":          ipAddress,
				"ipAllocationMethod": ipAllocationMethod,
				"ipAdressVersion":    ipAdressVersion,
			}).Set(gaugeValue)
		}
	}

	return
}

// Collect Azure ComputeUsage metrics
func collectAzureComputeUsage(context context.Context, subscriptionId string, callback chan<- func()) {
	computeClient := compute.NewUsageClient(subscriptionId)
	computeClient.Authorizer = AzureAuthorizer
	for _, location := range opts.AzureLocation {
		list, err := computeClient.List(context, location)

		if err != nil {
			panic(err)
		}

		for _, val := range list.Values() {
			quotaName := *val.Name.Value
			quotaNameLocalized := *val.Name.LocalizedValue
			currentValue := float64(*val.CurrentValue)
			limitValue := float64(*val.Limit)

			labels := prometheus.Labels{"subscriptionID": subscriptionId, "location": location, "scope": "compute", "quota": quotaName}
			infoLabels := prometheus.Labels{"subscriptionID": subscriptionId, "location": location, "scope": "compute", "quota": quotaName, "quotaName": quotaNameLocalized}

			callback <- func() {
				prometheusQuota.With(infoLabels).Set(1)
				prometheusQuotaCurrent.With(labels).Set(currentValue)
				prometheusQuotaLimit.With(labels).Set(limitValue)
			}
		}
	}
}

// Collect Azure NetworkUsage metrics
func collectAzureNetworkUsage(context context.Context, subscriptionId string, callback chan<- func()) {
	networkClient := network.NewUsagesClient(subscriptionId)
	networkClient.Authorizer = AzureAuthorizer
	for _, location := range opts.AzureLocation {
		list, err := networkClient.List(context, location)

		if err != nil {
			panic(err)
		}

		for _, val := range list.Values() {
			quotaName := *val.Name.Value
			quotaNameLocalized := *val.Name.LocalizedValue
			currentValue := float64(*val.CurrentValue)
			limitValue := float64(*val.Limit)

			labels := prometheus.Labels{"subscriptionID": subscriptionId, "location": location, "scope": "storage", "quota": quotaName}
			infoLabels := prometheus.Labels{"subscriptionID": subscriptionId, "location": location, "scope": "storage", "quota": quotaName, "quotaName": quotaNameLocalized}

			callback <- func() {
				prometheusQuota.With(infoLabels).Set(1)
				prometheusQuotaCurrent.With(labels).Set(currentValue)
				prometheusQuotaLimit.With(labels).Set(limitValue)
			}
		}
	}
}

// Collect Azure StorageUsage metrics
func collectAzureStorageUsage(context context.Context, subscriptionId string, callback chan<- func()) {
	storageClient := storage.NewUsageClient(subscriptionId)
	storageClient.Authorizer = AzureAuthorizer
	for _, location := range opts.AzureLocation {
		list, err := storageClient.List(context)

		if err != nil {
			panic(err)
		}

		for _, val := range *list.Value {
			quotaName := *val.Name.Value
			quotaNameLocalized := *val.Name.LocalizedValue
			currentValue := float64(*val.CurrentValue)
			limitValue := float64(*val.Limit)

			labels := prometheus.Labels{"subscriptionID": subscriptionId, "location": location, "scope": "storage", "quota": quotaName}
			infoLabels := prometheus.Labels{"subscriptionID": subscriptionId, "location": location, "scope": "storage", "quota": quotaName, "quotaName": quotaNameLocalized}

			callback <- func() {
				prometheusQuota.With(infoLabels).Set(1)
				prometheusQuotaCurrent.With(labels).Set(currentValue)
				prometheusQuotaLimit.With(labels).Set(limitValue)
			}
		}
	}
}

// read header and set prometheus api quota (if found)
func probeProcessHeader(response autorest.Response, header string, labels prometheus.Labels, callback chan<- func()) {
	if val := response.Header.Get(header); val != "" {
		valFloat, err := strconv.ParseFloat(val, 64)

		if err == nil {
			callback <- func() {
				prometheusApiQuota.With(labels).Set(valFloat)
			}
		} else {
			ErrorLogger.Error(fmt.Sprintf("Failed to parse value '%v':", val), err)
		}
	}
}
