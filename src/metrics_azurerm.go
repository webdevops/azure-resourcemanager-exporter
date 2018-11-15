package main

import (
	"sync"
	"time"
	"context"
	"github.com/prometheus/client_golang/prometheus"
)

type MetricCollectorAzureRm struct {
	prometheus struct {
		// general
		subscription *prometheus.GaugeVec
		resourceGroup *prometheus.GaugeVec

		// vm
		vm *prometheus.GaugeVec
		vmOs *prometheus.GaugeVec
		publicIp *prometheus.GaugeVec

		// api quota
		apiQuota *prometheus.GaugeVec

		// resource quota
		quota *prometheus.GaugeVec
		quotaCurrent *prometheus.GaugeVec
		quotaLimit *prometheus.GaugeVec

		// acr
		containerRegistry *prometheus.GaugeVec
		containerRegistryQuotaCurrent *prometheus.GaugeVec
		containerRegistryQuotaLimit *prometheus.GaugeVec

		// container instances
		containerInstance *prometheus.GaugeVec
		containerInstanceContainer *prometheus.GaugeVec
		containerInstanceContainerResource *prometheus.GaugeVec
		containerInstanceContainerPort *prometheus.GaugeVec

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

	m.prometheus.containerInstance = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_containerinstance_info",
			Help: "Azure ContainerInstance limit",
		},
		append(
			[]string{"resourceID", "subscriptionID", "location", "instanceName", "resourceGroup", "osType", "ipAdress"},
			prefixSlice(AZURE_RESOURCE_TAG_PREFIX, opts.AzureResourceTags)...
		),
	)

	m.prometheus.containerInstanceContainer = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_containerinstance_container",
			Help: "Azure ContainerInstance container",
		},
		[]string{"resourceID", "containerName", "containerImage", "livenessProbe", "readinessProbe"},
	)

	m.prometheus.containerInstanceContainerResource = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_containerinstance_container_resource",
			Help: "Azure ContainerInstance container resource",
		},
		[]string{"resourceID", "containerName", "type", "resource"},
	)

	m.prometheus.containerInstanceContainerPort = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_containerinstance_container_port",
			Help: "Azure ContainerInstance container port",
		},
		[]string{"resourceID", "containerName", "protocol"},
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
	prometheus.MustRegister(m.prometheus.containerInstance)
	prometheus.MustRegister(m.prometheus.containerInstanceContainer)
	prometheus.MustRegister(m.prometheus.containerInstanceContainerResource)
	prometheus.MustRegister(m.prometheus.containerInstanceContainerPort)
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
	ctx := context.Background()

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
			m.collectAzureSubscription(ctx, subscriptionId, callbackChannel)
			Logger.Verbose("subscription[%v]: finished Azure Subscription collection", subscriptionId)
		}(*subscription.SubscriptionID)

		// ResourceGroups
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			m.collectAzureResourceGroup(ctx, subscriptionId, callbackChannel)
			Logger.Verbose("subscription[%v]: finished Azure ResourceGroup collection", subscriptionId)
		}(*subscription.SubscriptionID)

		// VMs
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			m.collectAzureVm(ctx, subscriptionId, callbackChannel)
			Logger.Verbose("subscription[%v]: finished Azure VirtualMachine collection", subscriptionId)
		}(*subscription.SubscriptionID)

		// Public IPs
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			publicIpChannel <- m.collectAzurePublicIp(ctx, subscriptionId, callbackChannel)
			Logger.Verbose("subscription[%v]: finished Azure PublicIP collection", subscriptionId)
		}(*subscription.SubscriptionID)

		// Compute usage
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			m.collectAzureComputeUsage(ctx, subscriptionId, callbackChannel)
			Logger.Verbose("subscription[%v]: finished Azure ComputerUsage collection", subscriptionId)
		}(*subscription.SubscriptionID)

		// Network usage
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			m.collectAzureNetworkUsage(ctx, subscriptionId, callbackChannel)
			Logger.Verbose("subscription[%v]: finished Azure NetworkUsage collection", subscriptionId)
		}(*subscription.SubscriptionID)

		// Storage usage
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			m.collectAzureStorageUsage(ctx, subscriptionId, callbackChannel)
			Logger.Verbose("subscription[%v]: finished Azure StorageUsage collection", subscriptionId)
		}(*subscription.SubscriptionID)

		// ContainerRegistries usage
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			m.collectAzureContainerRegistries(ctx, subscriptionId, callbackChannel)
			Logger.Verbose("subscription[%v]: finished Azure ContainerRegistries collection", subscriptionId)
		}(*subscription.SubscriptionID)

		// ContainerInstances usage
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			m.collectAzureContainerInstances(ctx, subscriptionId, callbackChannel)
			Logger.Verbose("subscription[%v]: finished Azure ContainerInstances collection", subscriptionId)
		}(*subscription.SubscriptionID)

		// SecurityCompliance
		for _, location := range opts.AzureLocation {
			wg.Add(1)
			go func(subscriptionId, location string) {
				defer wg.Done()
				m.collectAzureSecurityCompliance(ctx, subscriptionId, location, callbackChannel)
				Logger.Verbose("subscription[%v]: finished Azure SecurityCompliance collection (%v)", subscriptionId, location)
			}(*subscription.SubscriptionID, location)
		}

		// AdvisorRecommendations
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			m.collectAzureAdvisorRecommendations(ctx, subscriptionId, callbackChannel)
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
		m.prometheus.containerInstance.Reset()
		m.prometheus.containerInstanceContainer.Reset()
		m.prometheus.containerInstanceContainerResource.Reset()
		m.prometheus.containerInstanceContainerPort.Reset()
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


func (m *MetricCollectorAzureRm) addAzureResourceTags(labels prometheus.Labels, tags map[string]*string) (prometheus.Labels) {
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
