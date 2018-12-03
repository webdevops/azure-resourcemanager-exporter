package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/network/mgmt/network"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/prometheus/client_golang/prometheus"
	"os"
)

type MetricsCollectorPortscanner struct {
	CollectorProcessorCustom

	portscanner *Portscanner

	prometheus struct {
		publicIpPortscanStatus *prometheus.GaugeVec
		publicIpPortscanUpdated *prometheus.GaugeVec
		publicIpPortscanPort *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorPortscanner) Setup(collector *CollectorCustom) {
	m.CollectorReference = collector

	m.portscanner = &Portscanner{}
	m.portscanner.Init()

	m.prometheus.publicIpPortscanStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_publicip_portscan_status",
			Help: "Azure ResourceManager public ip portscan status",
		},
		[]string{"ipAddress", "type"},
	)

	m.prometheus.publicIpPortscanPort = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_publicip_portscan_port",
			Help: "Azure ResourceManager public ip port",
		},
		[]string{"ipAddress", "protocol", "port", "description"},
	)

	prometheus.MustRegister(m.prometheus.publicIpPortscanStatus)
	prometheus.MustRegister(m.prometheus.publicIpPortscanPort)

	m.portscanner.Callbacks.FinishScan = func(c *Portscanner) {
		Logger.Messsage("portscan: finished for %v IPs", len(m.portscanner.PublicIps))

		if opts.CachePath != "" {
			Logger.Messsage("portscan: saved to cache")
			m.portscanner.CacheSave(opts.CachePath)
		}
	}

	m.portscanner.Callbacks.StartupScan = func(c *Portscanner) {
		Logger.Messsage(
			"portscan: starting for %v IPs (parallel:%v, threads per run:%v, timeout:%vs, portranges:%v)",
			len(c.PublicIps),
			opts.PortscanPrallel,
			opts.PortscanThreads,
			opts.PortscanTimeout,
			opts.portscanPortRange,
		)

		m.prometheus.publicIpPortscanStatus.Reset()
	}

	m.portscanner.Callbacks.StartScanIpAdress = func(c *Portscanner, ipAddress string) {
		Logger.Messsage("portscan[%v]: start port scanning", ipAddress)

		// set the ipAdress to be scanned
		m.prometheus.publicIpPortscanStatus.With(prometheus.Labels{
			"ipAddress": ipAddress,
			"type": "finished",
		}).Set(0)
	}

	m.portscanner.Callbacks.FinishScanIpAdress = func(c *Portscanner, ipAddress string, elapsed float64) {
		// set ipAddess to be finsihed
		m.prometheus.publicIpPortscanStatus.With(prometheus.Labels{
			"ipAddress": ipAddress,
			"type": "finished",
		}).Set(1)

		// set the elapsed time
		m.prometheus.publicIpPortscanStatus.With(prometheus.Labels{
			"ipAddress": ipAddress,
			"type": "elapsed",
		}).Set(elapsed)

		// set update time
		m.prometheus.publicIpPortscanStatus.With(prometheus.Labels{
			"ipAddress": ipAddress,
			"type": "updated",
		}).SetToCurrentTime()
	}

	m.portscanner.Callbacks.ResultCleanup = func(c *Portscanner) {
		m.prometheus.publicIpPortscanPort.Reset()
	}

	m.portscanner.Callbacks.ResultPush = func(c *Portscanner, result PortscannerResult) {
		m.prometheus.publicIpPortscanPort.With(result.Labels).Set(result.Value)
	}

	if opts.CachePath != "" {
		if _, err := os.Stat(opts.CachePath); !os.IsNotExist(err) {
			Logger.Messsage("portscan: load from cache")
			m.portscanner.CacheLoad(opts.CachePath)
		}
	}
}

func (m *MetricsCollectorPortscanner) Collect(ctx context.Context) {
	ipAdressList := m.fetchPublicIpAdresses(ctx, m.CollectorReference.AzureSubscriptions)

	m.portscanner.SetIps(ipAdressList)

	if len(ipAdressList) > 0 {
		m.portscanner.Start()
	}
}

func (m *MetricsCollectorPortscanner) fetchPublicIpAdresses(ctx context.Context, subscriptions []subscriptions.Subscription) (ipAddressList []string) {
	Logger.Messsage(
		"portscan: collecting public ips",
	)

	for _, subscription := range subscriptions {

		client := network.NewPublicIPAddressesClient(*subscription.SubscriptionID)
		client.Authorizer = AzureAuthorizer

		list, err := client.ListAll(ctx)
		if err != nil {
			panic(err)
		}

		for _, val:= range list.Values() {
			if val.IPAddress != nil {
				ipAddressList = append(ipAddressList, *val.IPAddress)
			}
		}
	}

	return ipAddressList
}
