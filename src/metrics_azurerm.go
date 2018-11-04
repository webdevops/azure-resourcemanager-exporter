package main

import (
	"fmt"
	"time"
	"sync"
	"context"
	"strconv"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/advisor/mgmt/advisor"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/containerregistry/mgmt/containerregistry"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/network/mgmt/network"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/resources"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/storage/mgmt/storage"
	"github.com/Azure/azure-sdk-for-go/profiles/preview/preview/security/mgmt/security"
	"github.com/Azure/go-autorest/autorest"
	"github.com/prometheus/client_golang/prometheus"
)

type MetricCollectorAzureRm struct {
	prometheus struct {
		subscription *prometheus.GaugeVec
		resourceGroup *prometheus.GaugeVec
		vm *prometheus.GaugeVec
		vmOs *prometheus.GaugeVec
		publicIp *prometheus.GaugeVec
		apiQuota *prometheus.GaugeVec
		quota *prometheus.GaugeVec
		quotaCurrent *prometheus.GaugeVec
		quotaLimit *prometheus.GaugeVec
		containerRegistry *prometheus.GaugeVec
		containerRegistryQuotaCurrent *prometheus.GaugeVec
		containerRegistryQuotaLimit *prometheus.GaugeVec

		// compliance
		securitycenterCompliance *prometheus.GaugeVec
		advisorRecommendations *prometheus.GaugeVec

		// portscanner
		publicIpPortscanStatus *prometheus.GaugeVec
		publicIpPortscanUpdated *prometheus.GaugeVec
		publicIpPortscanPort *prometheus.GaugeVec
	}

	portscanner *Portscanner
	enablePortscanner bool
}

// Create and setup metrics and collection
func (m *MetricCollectorAzureRm) Init(enablePortscanner bool) {
	m.enablePortscanner = enablePortscanner

	m.prometheus.subscription = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_subscription_info",
			Help: "Azure ResourceManager subscription",
		},
		[]string{"resourceID", "subscriptionID", "subscriptionName", "spendingLimit", "quotaID", "locationPlacementID"},
	)

	m.prometheus.resourceGroup = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_resourcegroup_info",
			Help: "Azure ResourceManager resourcegroups",
		},
		append(
			[]string{"resourceID", "subscriptionID", "resourceGroup", "location"},
			prefixSlice(AZURE_RESOURCE_TAG_PREFIX, opts.AzureResourceTags)...
		),
	)

	m.prometheus.vm = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_vm_info",
			Help: "Azure ResourceManager VMs",
		},
		append(
			[]string{"resourceID", "subscriptionID", "location", "resourceGroup", "vmID", "vmName", "vmType", "vmSize", "vmProvisioningState"},
			prefixSlice(AZURE_RESOURCE_TAG_PREFIX, opts.AzureResourceTags)...
		),
	)

	m.prometheus.vmOs = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_vm_os",
			Help: "Azure ResourceManager VM OS",
		},
		[]string{"vmID", "imagePublisher", "imageSku", "imageOffer", "imageVersion"},
	)

	m.prometheus.publicIp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_publicip_info",
			Help: "Azure ResourceManager public ip",
		},
		append(
			[]string{"resourceID", "subscriptionID", "resourceGroup", "location", "ipAddress", "ipAllocationMethod", "ipAdressVersion"},
			prefixSlice(AZURE_RESOURCE_TAG_PREFIX, opts.AzureResourceTags)...
		),
	)

	m.prometheus.apiQuota = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_ratelimit",
			Help: "Azure ResourceManager ratelimit",
		},
		[]string{"subscriptionID", "scope", "type"},
	)

	m.prometheus.quota = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_quota_info",
			Help: "Azure ResourceManager quota info",
		},
		[]string{"subscriptionID", "location", "scope", "quota", "quotaName"},
	)

	m.prometheus.quotaCurrent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_quota_current",
			Help: "Azure ResourceManager quota current value",
		},
		[]string{"subscriptionID", "location", "scope", "quota"},
	)

	m.prometheus.quotaLimit = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_quota_limit",
			Help: "Azure ResourceManager quota limit",
		},
		[]string{"subscriptionID", "location", "scope", "quota"},
	)

	m.prometheus.containerRegistry = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_containerregistry_info",
			Help: "Azure ContainerRegistry limit",
		},
		append(
			[]string{"resourceID", "subscriptionID", "location", "registryName", "resourceGroup", "adminUserEnabled", "skuName", "skuTier"},
			prefixSlice(AZURE_RESOURCE_TAG_PREFIX, opts.AzureResourceTags)...
		),
	)

	m.prometheus.containerRegistryQuotaCurrent = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_containerregistry_quota_current",
			Help: "Azure ContainerRegistry quota current",
		},
		[]string{"subscriptionID", "registryName", "quotaName", "quotaUnit"},
	)

	m.prometheus.containerRegistryQuotaLimit = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_containerregistry_quota_limit",
			Help: "Azure ContainerRegistry quota limit",
		},
		[]string{"subscriptionID", "registryName", "quotaName", "quotaUnit"},
	)

	m.prometheus.securitycenterCompliance = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_securitycenter_compliance",
			Help: "Azure Audit SecurityCenter compliance status",
		},
		[]string{"subscriptionID", "assessmentType"},
	)

	m.prometheus.advisorRecommendations = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_advisor_recommendation",
			Help: "Azure Audit Advisor recommendation",
		},
		[]string{"subscriptionID", "category", "resourceType", "resourceName", "resourceGroup", "impact", "risk"},
	)

	prometheus.MustRegister(m.prometheus.subscription)
	prometheus.MustRegister(m.prometheus.resourceGroup)
	prometheus.MustRegister(m.prometheus.vm)
	prometheus.MustRegister(m.prometheus.vmOs)
	prometheus.MustRegister(m.prometheus.publicIp)
	prometheus.MustRegister(m.prometheus.apiQuota)
	prometheus.MustRegister(m.prometheus.quota)
	prometheus.MustRegister(m.prometheus.quotaCurrent)
	prometheus.MustRegister(m.prometheus.quotaLimit)
	prometheus.MustRegister(m.prometheus.containerRegistry)
	prometheus.MustRegister(m.prometheus.containerRegistryQuotaCurrent)
	prometheus.MustRegister(m.prometheus.containerRegistryQuotaLimit)
	prometheus.MustRegister(m.prometheus.securitycenterCompliance)
	prometheus.MustRegister(m.prometheus.advisorRecommendations)

	if m.enablePortscanner {
		m.initPortscanner()
	}
}

// Start backgrounded metrics collection
func (m *MetricCollectorAzureRm) Start() {
	go func() {
		for {
			go func() {
				m.collect()
			}()

			Logger.Messsage("run: sleeping %v", opts.ScrapeTime.String())
			time.Sleep(opts.ScrapeTime)
		}
	}()

	if m.enablePortscanner {
		m.startPortscanner()
	}
}

// Metrics run
func (m *MetricCollectorAzureRm) collect() {
	var wg sync.WaitGroup
	context := context.Background()

	publicIpChannel := make(chan []string)
	callbackChannel := make(chan func())

	for _, subscription := range AzureSubscriptions {
		Logger.Messsage(
			"subscription[%v]: starting metrics collection",
			*subscription.SubscriptionID,
		)

		// Subscription
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			m.collectAzureSubscription(context, subscriptionId, callbackChannel)
			Logger.Verbose("subscription[%v]: finished Azure Subscription collection", subscriptionId)
		}(*subscription.SubscriptionID)

		// ResourceGroups
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			m.collectAzureResourceGroup(context, subscriptionId, callbackChannel)
			Logger.Verbose("subscription[%v]: finished Azure ResourceGroup collection", subscriptionId)
		}(*subscription.SubscriptionID)

		// VMs
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			m.collectAzureVm(context, subscriptionId, callbackChannel)
			Logger.Verbose("subscription[%v]: finished Azure VirtualMachine collection", subscriptionId)
		}(*subscription.SubscriptionID)

		// Public IPs
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			publicIpChannel <- m.collectAzurePublicIp(context, subscriptionId, callbackChannel)
			Logger.Verbose("subscription[%v]: finished Azure PublicIP collection", subscriptionId)
		}(*subscription.SubscriptionID)

		// Compute usage
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			m.collectAzureComputeUsage(context, subscriptionId, callbackChannel)
			Logger.Verbose("subscription[%v]: finished Azure ComputerUsage collection", subscriptionId)
		}(*subscription.SubscriptionID)

		// Network usage
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			m.collectAzureNetworkUsage(context, subscriptionId, callbackChannel)
			Logger.Verbose("subscription[%v]: finished Azure NetworkUsage collection", subscriptionId)
		}(*subscription.SubscriptionID)

		// Storage usage
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			m.collectAzureStorageUsage(context, subscriptionId, callbackChannel)
			Logger.Verbose("subscription[%v]: finished Azure StorageUsage collection", subscriptionId)
		}(*subscription.SubscriptionID)

		// ContainerRegistries usage
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			m.collectAzureContainerRegistries(context, subscriptionId, callbackChannel)
			Logger.Verbose("subscription[%v]: finished Azure ContainerRegistries collection", subscriptionId)
		}(*subscription.SubscriptionID)

		// SecurityCompliance
		for _, location := range opts.AzureLocation {
			wg.Add(1)
			go func(subscriptionId, location string) {
				defer wg.Done()
				m.collectAzureSecurityCompliance(context, subscriptionId, location, callbackChannel)
				Logger.Verbose("subscription[%v]: finished Azure SecurityCompliance collection (%v)", subscriptionId, location)
			}(*subscription.SubscriptionID, location)
		}

		// AdvisorRecommendations
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			m.collectAzureAdvisorRecommendations(context, subscriptionId, callbackChannel)
			Logger.Verbose("subscription[%v]: finished Azure AdvisorRecommendations collection", subscriptionId)
		}(*subscription.SubscriptionID)
	}

	// process publicIP list and pass it to portscanner
	go func() {
		publicIpList := []string{}
		for ipAddressList := range publicIpChannel {
			publicIpList = append(publicIpList, ipAddressList...)
		}

		// update portscanner public ips
		if m.portscanner != nil {
			m.portscanner.SetIps(publicIpList)
			m.portscanner.Cleanup()
			m.portscanner.Enable()
		}
		Logger.Messsage("run: collected %v public IPs", len(publicIpList))
	}()

	// collect metrics (callbacks) and proceses them
	go func() {
		var callbackList []func()
		for callback := range callbackChannel {
			callbackList = append(callbackList, callback)
		}

		m.prometheus.subscription.Reset()
		m.prometheus.resourceGroup.Reset()
		m.prometheus.vm.Reset()
		m.prometheus.vmOs.Reset()
		m.prometheus.publicIp.Reset()
		m.prometheus.apiQuota.Reset()
		m.prometheus.quota.Reset()
		m.prometheus.quotaCurrent.Reset()
		m.prometheus.quotaLimit.Reset()
		m.prometheus.containerRegistry.Reset()
		m.prometheus.containerRegistryQuotaCurrent.Reset()
		m.prometheus.containerRegistryQuotaLimit.Reset()
		m.prometheus.securitycenterCompliance.Reset()
		m.prometheus.advisorRecommendations.Reset()
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
func (m *MetricCollectorAzureRm) collectAzureSubscription(context context.Context, subscriptionId string, callback chan<- func()) {
	subscriptionClient := subscriptions.NewClient()
	subscriptionClient.Authorizer = AzureAuthorizer

	sub, err := subscriptionClient.Get(context, subscriptionId)
	if err != nil {
		panic(err)
	}

	callback <- func() {
		m.prometheus.subscription.With(
			prometheus.Labels{
				"resourceID": *sub.ID,
				"subscriptionID":      *sub.SubscriptionID,
				"subscriptionName":    *sub.DisplayName,
				"spendingLimit":       string(sub.SubscriptionPolicies.SpendingLimit),
				"quotaID":             *sub.SubscriptionPolicies.QuotaID,
				"locationPlacementID": *sub.SubscriptionPolicies.LocationPlacementID,
			},
		).Set(1)
	}

	// subscription rate limits
	m.probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-subscription-reads", prometheus.Labels{"subscriptionID": subscriptionId, "scope": "subscription", "type": "read"}, callback)
	m.probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-subscription-resource-requests", prometheus.Labels{"subscriptionID": subscriptionId, "scope": "subscription", "type": "resource-requests"}, callback)
	m.probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-subscription-resource-entities-read", prometheus.Labels{"subscriptionID": subscriptionId, "scope": "subscription", "type": "resource-entities-read"}, callback)

	// tenant rate limits
	m.probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-tenant-reads", prometheus.Labels{"subscriptionID": subscriptionId, "scope": "tenant", "type": "read"}, callback)
	m.probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-tenant-resource-requests", prometheus.Labels{"subscriptionID": subscriptionId, "scope": "tenant", "type": "resource-requests"}, callback)
	m.probeProcessHeader(sub.Response, "x-ms-ratelimit-remaining-tenant-resource-entities-read", prometheus.Labels{"subscriptionID": subscriptionId, "scope": "tenant", "type": "resource-entities-read"}, callback)
}

func (m *MetricCollectorAzureRm) addAzureResourceTags(tags map[string]*string, labels prometheus.Labels) (prometheus.Labels) {
	for _, rgTag := range opts.AzureResourceTags {
		rgTabLabel := AZURE_RESOURCE_TAG_PREFIX + rgTag

		if _, ok := tags[rgTag]; ok {
			labels[rgTabLabel] = *tags[rgTag]
		} else {
			labels[rgTabLabel] = ""
		}
	}

	return labels
}

// Collect Azure ResourceGroup metrics
func (m *MetricCollectorAzureRm) collectAzureResourceGroup(context context.Context, subscriptionId string, callback chan<- func()) {
	resourceGroupClient := resources.NewGroupsClient(subscriptionId)
	resourceGroupClient.Authorizer = AzureAuthorizer

	resourceGroupResult, err := resourceGroupClient.ListComplete(context, "", nil)
	if err != nil {
		panic(err)
	}

	for _, item := range *resourceGroupResult.Response().Value {
		infoLabels := prometheus.Labels{
			"resourceID": *item.ID,
			"subscriptionID": subscriptionId,
			"resourceGroup": *item.Name,
			"location": *item.Location,
		}
		infoLabels = m.addAzureResourceTags(item.Tags, infoLabels)

		callback <- func() {
			m.prometheus.resourceGroup.With(infoLabels).Set(1)
		}
	}
}

// Collect Azure PublicIP metrics
func (m *MetricCollectorAzureRm) collectAzurePublicIp(context context.Context, subscriptionId string, callback chan<- func()) (ipAddressList []string) {
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

		infoLabels := prometheus.Labels{
			"resourceID": *val.ID,
			"subscriptionID":     subscriptionId,
			"resourceGroup":      extractResourceGroupFromAzureId(*val.ID),
			"location":           location,
			"ipAddress":          ipAddress,
			"ipAllocationMethod": ipAllocationMethod,
			"ipAdressVersion":    ipAdressVersion,
		}
		infoLabels = m.addAzureResourceTags(val.Tags, infoLabels)


		callback <- func() {
			m.prometheus.publicIp.With(infoLabels).Set(gaugeValue)
		}
	}

	return
}

// Collect Azure ComputeUsage metrics
func (m *MetricCollectorAzureRm) collectAzureComputeUsage(context context.Context, subscriptionId string, callback chan<- func()) {
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

			labels := prometheus.Labels{
				"subscriptionID": subscriptionId,
				"location": location,
				"scope": "compute",
				"quota": quotaName,
			}

			infoLabels := prometheus.Labels{
				"subscriptionID": subscriptionId,
				"location": location,
				"scope": "compute",
				"quota": quotaName,
				"quotaName": quotaNameLocalized,
			}

			callback <- func() {
				m.prometheus.quota.With(infoLabels).Set(1)
				m.prometheus.quotaCurrent.With(labels).Set(currentValue)
				m.prometheus.quotaLimit.With(labels).Set(limitValue)
			}
		}
	}
}

// Collect Azure NetworkUsage metrics
func (m *MetricCollectorAzureRm) collectAzureNetworkUsage(context context.Context, subscriptionId string, callback chan<- func()) {
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

			labels := prometheus.Labels{
				"subscriptionID": subscriptionId,
				"location": location,
				"scope": "network",
				"quota": quotaName,
			}

			infoLabels := prometheus.Labels{
				"subscriptionID": subscriptionId,
				"location": location,
				"scope": "network",
				"quota": quotaName,
				"quotaName": quotaNameLocalized,
			}

			callback <- func() {
				m.prometheus.quota.With(infoLabels).Set(1)
				m.prometheus.quotaCurrent.With(labels).Set(currentValue)
				m.prometheus.quotaLimit.With(labels).Set(limitValue)
			}
		}
	}
}

// Collect Azure StorageUsage metrics
func (m *MetricCollectorAzureRm) collectAzureStorageUsage(context context.Context, subscriptionId string, callback chan<- func()) {
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

			labels := prometheus.Labels{
				"subscriptionID": subscriptionId,
				"location": location,
				"scope": "storage",
				"quota": quotaName,
			}

			infoLabels := prometheus.Labels{
				"subscriptionID": subscriptionId,
				"location": location,
				"scope": "storage",
				"quota": quotaName,
				"quotaName": quotaNameLocalized,
			}

			callback <- func() {
				m.prometheus.quota.With(infoLabels).Set(1)
				m.prometheus.quotaCurrent.With(labels).Set(currentValue)
				m.prometheus.quotaLimit.With(labels).Set(limitValue)
			}
		}
	}
}

func (m *MetricCollectorAzureRm) collectAzureVm(context context.Context, subscriptionId string, callback chan<- func()) {
	computeClient := compute.NewVirtualMachinesClient(subscriptionId)
	computeClient.Authorizer = AzureAuthorizer

	list, err := computeClient.ListAllComplete(context)

	if err != nil {
		panic(err)
	}


	for list.NotDone() {
		val := list.Value()

		infoLabels := prometheus.Labels{
			"resourceID": *val.ID,
			"subscriptionID": subscriptionId,
			"location": *val.Location,
			"resourceGroup": extractResourceGroupFromAzureId(*val.ID),
			"vmID": *val.VMID,
			"vmName": *val.Name,
			"vmType": *val.Type,
			"vmSize": string(val.VirtualMachineProperties.HardwareProfile.VMSize),
			"vmProvisioningState": *val.ProvisioningState,
		}
		infoLabels = m.addAzureResourceTags(val.Tags, infoLabels)

		osLabels := prometheus.Labels{
			"vmID": *val.VMID,
			"imagePublisher": *val.StorageProfile.ImageReference.Publisher,
			"imageSku": *val.StorageProfile.ImageReference.Sku,
			"imageOffer": *val.StorageProfile.ImageReference.Offer,
			"imageVersion": *val.StorageProfile.ImageReference.Version,
		}

		callback <- func() {
			m.prometheus.vm.With(infoLabels).Set(1)
			m.prometheus.vmOs.With(osLabels).Set(1)
		}

		if list.Next() != nil {
			break
		}
	}
}


func (m *MetricCollectorAzureRm) collectAzureContainerRegistries(context context.Context, subscriptionId string, callback chan<- func()) {
	acrClient := containerregistry.NewRegistriesClient(subscriptionId)
	acrClient.Authorizer = AzureAuthorizer

	list, err := acrClient.ListComplete(context)

	if err != nil {
		panic(err)
	}


	for list.NotDone() {
		val := list.Value()

		arcUsage, err := acrClient.ListUsages(context, extractResourceGroupFromAzureId(*val.ID), *val.Name)

		if err != nil {
			ErrorLogger.Error(fmt.Sprintf("subscription[%v]: unable to fetch ACR usage for %v", subscriptionId, *val.Name), err)
		}

		skuName := ""
		skuTier := ""

		if val.Sku != nil {
			skuName = string(val.Sku.Name)
			skuTier = string(val.Sku.Tier)
		}

		infoLabels := prometheus.Labels{
			"resourceID": *val.ID,
			"subscriptionID": subscriptionId,
			"location": *val.Location,
			"registryName": *val.Name,
			"resourceGroup": extractResourceGroupFromAzureId(*val.ID),
			"adminUserEnabled": boolToString(*val.AdminUserEnabled),
			"skuName": skuName,
			"skuTier": skuTier,
		}
		infoLabels = m.addAzureResourceTags(val.Tags, infoLabels)

		callback <- func() {
			m.prometheus.containerRegistry.With(infoLabels).Set(1)

			if arcUsage.Value != nil {
				for _, usage := range *arcUsage.Value {
					quotaLabels := prometheus.Labels{
						"subscriptionID": subscriptionId,
						"registryName": *val.Name,
						"quotaUnit": string(usage.Unit),
						"quotaName": *usage.Name,
					}

					m.prometheus.containerRegistryQuotaCurrent.With(quotaLabels).Set(float64(*usage.CurrentValue))
					m.prometheus.containerRegistryQuotaLimit.With(quotaLabels).Set(float64(*usage.Limit))
				}
			}
		}

		if list.Next() != nil {
			break
		}
	}
}


// read header and set prometheus api quota (if found)
func (m *MetricCollectorAzureRm) probeProcessHeader(response autorest.Response, header string, labels prometheus.Labels, callback chan<- func()) {
	if val := response.Header.Get(header); val != "" {
		valFloat, err := strconv.ParseFloat(val, 64)

		if err == nil {
			callback <- func() {
				m.prometheus.apiQuota.With(labels).Set(valFloat)
			}
		} else {
			ErrorLogger.Error(fmt.Sprintf("Failed to parse value '%v':", val), err)
		}
	}
}

func (m *MetricCollectorAzureRm) collectAzureSecurityCompliance(context context.Context, subscriptionId, location string, callback chan<- func()) {
	subscriptionResourceId := fmt.Sprintf("/subscriptions/%v", subscriptionId)
	complianceClient := security.NewCompliancesClient(subscriptionResourceId, location)
	complianceClient.Authorizer = AzureAuthorizer

	complienceResult, err := complianceClient.Get(context, subscriptionResourceId, time.Now().Format("2006-01-02Z"))
	if err != nil {
		ErrorLogger.Error(fmt.Sprintf("subscription[%v]", subscriptionId), err)
		return
	}

	if complienceResult.AssessmentResult != nil {
		for _, result := range *complienceResult.AssessmentResult {
			segmentType := ""
			if result.SegmentType != nil {
				segmentType = *result.SegmentType
			}

			infoLabels := prometheus.Labels{
				"subscriptionID": subscriptionId,
				"assessmentType": segmentType,
			}
			infoValue := *result.Percentage

			callback <- func() {
				m.prometheus.securitycenterCompliance.With(infoLabels).Set(infoValue)
			}
		}
	}
}

func (m *MetricCollectorAzureRm) collectAzureAdvisorRecommendations(context context.Context, subscriptionId string, callback chan<- func()) {
	advisorRecommendationsClient := advisor.NewRecommendationsClient(subscriptionId)
	advisorRecommendationsClient.Authorizer = AzureAuthorizer

	recommendationResult, err := advisorRecommendationsClient.ListComplete(context, "", nil, "")
	if err != nil {
		panic(err)
	}

	for _, item := range *recommendationResult.Response().Value {

		infoLabels := prometheus.Labels{
			"subscriptionID": subscriptionId,
			"category":       string(item.RecommendationProperties.Category),
			"resourceType":   *item.RecommendationProperties.ImpactedField,
			"resourceName":   *item.RecommendationProperties.ImpactedValue,
			"resourceGroup":  extractResourceGroupFromAzureId(*item.ID),
			"impact":         string(item.Impact),
			"risk":           string(item.Risk),
		}

		callback <- func() {
			m.prometheus.advisorRecommendations.With(infoLabels).Set(1)
		}
	}
}
