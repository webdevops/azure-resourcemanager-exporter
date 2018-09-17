package main

import (
	"time"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	prometheusPublicIpPortscanStatus *prometheus.GaugeVec
	prometheusPublicIpPortscanUpdated *prometheus.GaugeVec
	prometheusPublicIpPortscanPort *prometheus.GaugeVec

	portscanner *Portscanner
)

// Create and setup metrics and collection
func initMetricsPortscanner() {
	portscanner = &Portscanner{}
	portscanner.Init()

	prometheusPublicIpPortscanStatus = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_publicip_portscan_status",
			Help: "Azure ResourceManager public ip portscan status",
		},
		[]string{"ipAddress", "type"},
	)

	prometheusPublicIpPortscanPort = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_publicip_portscan_port",
			Help: "Azure ResourceManager public ip port",
		},
		[]string{"ipAddress", "protocol", "port", "description"},
	)

	prometheus.MustRegister(prometheusPublicIpPortscanStatus)
	prometheus.MustRegister(prometheusPublicIpPortscanPort)


	portscanner.Callbacks.FinishScan = func(c *Portscanner) {
		Logger.Messsage("Finished portscan for %v IPs", len(portscanner.PublicIps))
	}

	portscanner.Callbacks.StartupScan = func(c *Portscanner) {
		Logger.Messsage(
			"Starting portscan for %v IPs (parallel:%v, threads per run:%v, timeout:%vs, portranges:%v)",
			len(c.PublicIps),
			opts.PortscanPrallel,
			opts.PortscanThreads,
			opts.PortscanTimeout,
			opts.portscanPortRange,
		)
	}

	portscanner.Callbacks.StartScanIpAdress = func(c *Portscanner, ipAddress string) {
		Logger.Messsage("Start port scanning for %v", ipAddress)

		// set the ipAdress to be scanned
		prometheusPublicIpPortscanStatus.With(prometheus.Labels{
			"ipAddress": ipAddress,
			"type": "finished",
		}).Set(0)
	}

	portscanner.Callbacks.FinishScanIpAdress = func(c *Portscanner, ipAddress string, elapsed float64) {
		// set ipAddess to be finsihed
		prometheusPublicIpPortscanStatus.With(prometheus.Labels{
			"ipAddress": ipAddress,
			"type": "finished",
		}).Set(1)

		// set the elapsed time
		prometheusPublicIpPortscanStatus.With(prometheus.Labels{
			"ipAddress": ipAddress,
			"type": "elapsed",
		}).Set(elapsed)

		// set update time
		prometheusPublicIpPortscanStatus.With(prometheus.Labels{
			"ipAddress": ipAddress,
			"type": "updated",
		}).Set(float64(time.Now().Unix()))
	}

	portscanner.Callbacks.ResultCleanup = func(c *Portscanner) {
		prometheusPublicIpPortscanPort.Reset()
	}

	portscanner.Callbacks.ResultPush = func(c *Portscanner, result PortscannerResult) {
		prometheusPublicIpPortscanPort.With(result.Labels).Set(result.Value)
	}
}

// Start backgrounded metrics collection
func startMetricsCollectionPortscanner() {
	firstStart := true
	go func() {
		for {
			if portscanner.Enabled && len(portscanner.PublicIps) > 0 {
				portscanner.Start()
				time.Sleep(opts.PortscanTime * time.Second)
			} else {
				if firstStart {
					// short delayed first time start
					time.Sleep(time.Duration(10) * time.Second)
				} else {
					// longer delayed restart
					time.Sleep(opts.ScrapeTime + 5)
				}
			}
		}
	}()
}
