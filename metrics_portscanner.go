package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/network/mgmt/network"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/go-autorest/autorest/to"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"os"
)

type MetricsCollectorPortscanner struct {
	CollectorProcessorCustom

	portscanner *Portscanner

	prometheus struct {
		publicIpInfo            *prometheus.GaugeVec
		publicIpPortscanStatus  *prometheus.GaugeVec
		publicIpPortscanUpdated *prometheus.GaugeVec
		publicIpPortscanPort    *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorPortscanner) Setup(collector *CollectorCustom) {
	m.CollectorReference = collector

	m.portscanner = &Portscanner{}
	m.portscanner.Init()

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
	prometheus.MustRegister(m.prometheus.publicIpInfo)

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
	prometheus.MustRegister(m.prometheus.publicIpPortscanStatus)

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
	prometheus.MustRegister(m.prometheus.publicIpPortscanPort)

	m.portscanner.Callbacks.FinishScan = func(c *Portscanner) {
		m.logger().Infof("finished for %v IPs", len(m.portscanner.PublicIps))

		if opts.Cache.Path != "" {
			m.logger().Infof("saved to cache")
			m.portscanner.CacheSave(opts.Cache.Path)
		}
	}

	m.portscanner.Callbacks.StartupScan = func(c *Portscanner) {
		m.logger().Infof(
			"starting for %v IPs (parallel:%v, threads per run:%v, timeout:%vs, portranges:%v)",
			len(c.PublicIps),
			opts.Portscan.Parallel,
			opts.Portscan.Threads,
			opts.Portscan.Timeout,
			portscanPortRange,
		)

		m.prometheus.publicIpPortscanStatus.Reset()
	}

	m.portscanner.Callbacks.StartScanIpAdress = func(c *Portscanner, pip network.PublicIPAddress) {
		ipAddress := to.String(pip.IPAddress)

		m.logger().WithField("ipAddress", ipAddress).Infof("start port scanning")

		// set the ipAdress to be scanned
		m.prometheus.publicIpPortscanStatus.With(prometheus.Labels{
			"ipAddress": ipAddress,
			"type":      "finished",
		}).Set(0)
	}

	m.portscanner.Callbacks.FinishScanIpAdress = func(c *Portscanner, pip network.PublicIPAddress, elapsed float64) {
		ipAddress := to.String(pip.IPAddress)

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

	if opts.Cache.Path != "" {
		if _, err := os.Stat(opts.Cache.Path); !os.IsNotExist(err) {
			m.logger().Infof("load from cache")
			m.portscanner.CacheLoad(opts.Cache.Path)
		}
	}
}

func (m *MetricsCollectorPortscanner) Collect(ctx context.Context, logger *log.Entry) {
	publicIpList := m.fetchPublicIpAdresses(ctx, logger, m.CollectorReference.AzureSubscriptions)

	m.portscanner.SetAzurePublicIpList(publicIpList)

	if len(publicIpList) > 0 {
		m.portscanner.Start()
	}
}

func (m *MetricsCollectorPortscanner) fetchPublicIpAdresses(ctx context.Context, logger *log.Entry, subscriptions []subscriptions.Subscription) (pipList []network.PublicIPAddress) {
	logger.Info("collecting public ips")

	for _, val := range subscriptions {
		subscription := val
		contextLogger := logger.WithField("azureSubscription", subscription)

		client := network.NewPublicIPAddressesClientWithBaseURI(azureEnvironment.ResourceManagerEndpoint, *subscription.SubscriptionID)
		decorateAzureAutorest(&client.Client)

		list, err := client.ListAll(ctx)
		if err != nil {
			contextLogger.Panic(err)
		}

		for _, val := range list.Values() {
			if val.IPAddress != nil {
				pipList = append(pipList, val)
			}
		}
	}

	m.prometheus.publicIpInfo.Reset()
	for _, pip := range pipList {
		m.prometheus.publicIpInfo.With(prometheus.Labels{
			"subscriptionID":   extractSubscriptionIdFromAzureId(to.String(pip.ID)),
			"resourceID":       toResourceId(pip.ID),
			"resourceGroup":    extractResourceGroupFromAzureId(to.String(pip.ID)),
			"name":             to.String(pip.Name),
			"ipAddressVersion": string(pip.PublicIPAddressVersion),
			"ipAddress":        to.String(pip.IPAddress),
		}).Set(1)
	}

	return pipList
}
