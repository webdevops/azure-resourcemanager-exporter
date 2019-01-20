package old

import (
	"sync"
	"time"
	"context"
	"github.com/prometheus/client_golang/prometheus"
)

type MetricCollectorAzureRm struct {
	prometheus struct {


		// resource meta
		resourceHealth *prometheus.GaugeVec

		// vm
		vm *prometheus.GaugeVec
		vmOs *prometheus.GaugeVec
		publicIp *prometheus.GaugeVec


		// resource quota
		quota *prometheus.GaugeVec
		quotaCurrent *prometheus.GaugeVec
		quotaLimit *prometheus.GaugeVec

		// acr


		// database


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

	m.initGeneral()
	m.initQuota()
	m.initResourceHealth()
	m.initVm()
	m.initContainerRegistries()
	m.initContainerInstances()
	m.initSecurity()
	m.initDatabase()

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

		// ResourceHealth
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			m.collectAzureResourceHealth(ctx, subscriptionId, callbackChannel)
			Logger.Verbose("subscription[%v]: finished Azure ResourceHealth collection", subscriptionId)
		}(*subscription.SubscriptionID)

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

		// ResourceHealth
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			m.collectAzureResourceHealth(ctx, subscriptionId, callbackChannel)
			Logger.Verbose("subscription[%v]: finished Azure ResourceHealth collection", subscriptionId)
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

		// ContainerRegistries
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			m.collectAzureContainerRegistries(ctx, subscriptionId, callbackChannel)
			Logger.Verbose("subscription[%v]: finished Azure ContainerRegistries collection", subscriptionId)
		}(*subscription.SubscriptionID)

		// ContainerInstances
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			m.collectAzureContainerInstances(ctx, subscriptionId, callbackChannel)
			Logger.Verbose("subscription[%v]: finished Azure ContainerInstances collection", subscriptionId)
		}(*subscription.SubscriptionID)

		// Databases
		wg.Add(1)
		go func(subscriptionId string) {
			defer wg.Done()
			m.collectAzureDatabasePostgresql(ctx, subscriptionId, callbackChannel)
			Logger.Verbose("subscription[%v]: finished Azure Database Postgresql collection", subscriptionId)

			m.collectAzureDatabaseMysql(ctx, subscriptionId, callbackChannel)
			Logger.Verbose("subscription[%v]: finished Azure Database MySQL collection", subscriptionId)
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
		m.prometheus.resourceHealth.Reset()
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
		m.prometheus.database.Reset()
		m.prometheus.databaseStatus.Reset()
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
