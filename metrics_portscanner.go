package main

import (
	"os"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/resources/armsubscriptions"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/webdevops/go-common/azuresdk/armclient"
	"github.com/webdevops/go-common/prometheus/collector"
	"github.com/webdevops/go-common/utils/to"
)

type MetricsCollectorPortscanner struct {
	collector.Processor

	portscanner *Portscanner

	prometheus struct {
		publicIpInfo            *prometheus.GaugeVec
		publicIpPortscanStatus  *prometheus.GaugeVec
		publicIpPortscanUpdated *prometheus.GaugeVec
		publicIpPortscanPort    *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorPortscanner) Setup(collector *collector.Collector) {
	m.Processor.Setup(collector)

	m.portscanner = &Portscanner{}
	m.portscanner.Init()

	cachePath := opts.GetCachePath("portscan.json")

	m.prometheus.publicIpInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_publicip_info",
			Help: "Azure ResourceManager public ip resource information",
		},
		[]string{
			"subscriptionID",
			"resourceID",
			"resourceGroup",
			"name",
			"ipAddressVersion",
			"ipAddress",
		},
	)
	m.Collector.RegisterMetricList("publicIpInfo", m.prometheus.publicIpInfo, false)

	m.prometheus.publicIpPortscanStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_publicip_portscan_status",
			Help: "Azure ResourceManager public ip portscan status",
		},
		[]string{
			"ipAddress",
			"type",
		},
	)
	m.Collector.RegisterMetricList("publicIpPortscanStatus", m.prometheus.publicIpPortscanStatus, false)

	m.prometheus.publicIpPortscanPort = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_publicip_portscan_port",
			Help: "Azure ResourceManager public ip open port",
		},
		[]string{
			"ipAddress",
			"protocol",
			"port",
			"description",
		},
	)
	m.Collector.RegisterMetricList("publicIpPortscanPort", m.prometheus.publicIpPortscanPort, false)

	m.portscanner.Callbacks.FinishScan = func(c *Portscanner) {
		m.Logger().Infof("finished for %v IPs", len(m.portscanner.PublicIps))

		if cachePath != nil {
			m.Logger().Infof("saved to cache")
			m.portscanner.CacheSave(*cachePath)
		}
	}

	m.portscanner.Callbacks.StartupScan = func(c *Portscanner) {
		m.Logger().Infof(
			"starting for %v IPs (parallel:%v, threads per run:%v, timeout:%vs, portranges:%v)",
			len(c.PublicIps),
			opts.Portscan.Parallel,
			opts.Portscan.Threads,
			opts.Portscan.Timeout,
			portscanPortRange,
		)

		m.prometheus.publicIpPortscanStatus.Reset()
	}

	m.portscanner.Callbacks.StartScanIpAdress = func(c *Portscanner, pip armnetwork.PublicIPAddress) {
		ipAddress := to.StringLower(pip.Properties.IPAddress)

		m.Logger().WithField("ipAddress", ipAddress).Infof("start port scanning")

		// set the ipAdress to be scanned
		m.prometheus.publicIpPortscanStatus.With(prometheus.Labels{
			"ipAddress": ipAddress,
			"type":      "finished",
		}).Set(0)
	}

	m.portscanner.Callbacks.FinishScanIpAdress = func(c *Portscanner, pip armnetwork.PublicIPAddress, elapsed float64) {
		ipAddress := to.StringLower(pip.Properties.IPAddress)

		// set ipAddess to be finsihed
		m.prometheus.publicIpPortscanStatus.With(prometheus.Labels{
			"ipAddress": ipAddress,
			"type":      "finished",
		}).Set(1)

		// set the elapsed time
		m.prometheus.publicIpPortscanStatus.With(prometheus.Labels{
			"ipAddress": ipAddress,
			"type":      "elapsed",
		}).Set(elapsed)

		// set update time
		m.prometheus.publicIpPortscanStatus.With(prometheus.Labels{
			"ipAddress": ipAddress,
			"type":      "updated",
		}).SetToCurrentTime()
	}

	m.portscanner.Callbacks.ResultCleanup = func(c *Portscanner) {
		m.prometheus.publicIpPortscanPort.Reset()
	}

	m.portscanner.Callbacks.ResultPush = func(c *Portscanner, result PortscannerResult) {
		m.prometheus.publicIpPortscanPort.With(result.Labels).Set(result.Value)
	}

	if cachePath != nil {
		if _, err := os.Stat(*cachePath); !os.IsNotExist(err) {
			m.Logger().Infof("load from cache")
			m.portscanner.CacheLoad(*cachePath)
		}
	}
}

func (m *MetricsCollectorPortscanner) Reset() {
}

func (m *MetricsCollectorPortscanner) Collect(callback chan<- func()) {
	subscriptionList, err := AzureSubscriptionsIterator.ListSubscriptions()
	if err != nil {
		m.Logger().Panic(err)
	}

	publicIpList := m.fetchPublicIpAdresses(subscriptionList)
	m.portscanner.SetAzurePublicIpList(publicIpList)

	if len(publicIpList) > 0 {
		m.portscanner.Start()
	}
}

func (m *MetricsCollectorPortscanner) fetchPublicIpAdresses(subscriptions map[string]*armsubscriptions.Subscription) (pipList []*armnetwork.PublicIPAddress) {
	m.Logger().Info("collecting public ips")

	for _, val := range subscriptions {
		subscription := val
		contextLogger := m.Logger().WithField("azureSubscription", subscription)

		client, err := armnetwork.NewPublicIPAddressesClient(*subscription.SubscriptionID, AzureClient.GetCred(), AzureClient.NewArmClientOptions())
		if err != nil {
			contextLogger.Panic(err)
		}

		pager := client.NewListAllPager(nil)

		for pager.More() {
			result, err := pager.NextPage(m.Context())
			if err != nil {
				contextLogger.Panic(err)
			}

			if result.Value == nil {
				continue
			}

			for _, publicIp := range result.Value {
				if publicIp.Properties.IPAddress != nil {
					pipList = append(pipList, publicIp)
				}
			}
		}
	}

	m.prometheus.publicIpInfo.Reset()
	for _, pip := range pipList {
		resourceId := to.String(pip.ID)
		azureResource, _ := armclient.ParseResourceId(resourceId)

		m.prometheus.publicIpInfo.With(prometheus.Labels{
			"subscriptionID":   azureResource.Subscription,
			"resourceID":       to.StringLower(pip.ID),
			"resourceGroup":    azureResource.ResourceGroup,
			"name":             azureResource.ResourceName,
			"ipAddressVersion": stringToStringLower(string(*pip.Properties.PublicIPAddressVersion)),
			"ipAddress":        to.StringLower(pip.Properties.IPAddress),
		}).Set(1)
	}

	return pipList
}
