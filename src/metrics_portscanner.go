package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"time"
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
		Logger.Messsage("portscan: finished for %v IPs", len(portscanner.PublicIps))
	}

	portscanner.Callbacks.StartupScan = func(c *Portscanner) {
		Logger.Messsage(
			"portscan: starting for %v IPs (parallel:%v, threads per run:%v, timeout:%vs, portranges:%v)",
			len(c.PublicIps),
			opts.PortscanPrallel,
			opts.PortscanThreads,
			opts.PortscanTimeout,
			opts.portscanPortRange,
		)

		prometheusPublicIpPortscanStatus.Reset()
	}

	portscanner.Callbacks.StartScanIpAdress = func(c *Portscanner, ipAddress string) {
		Logger.Messsage("portscan[%v]: start port scanning", ipAddress)

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
		}).SetToCurrentTime()
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
			sleepDuration := opts.PortscanTime
			if portscanner.Enabled && len(portscanner.PublicIps) > 0 {
				portscanner.Start()
			} else {
				if firstStart {
					// short delayed first time start
					sleepDuration = time.Duration(30 * time.Second)
					firstStart = false
				} else {
					// wait for next scrape time
					sleepDuration = opts.ScrapeTime
				}
			}

			Logger.Messsage("portscanner: sleeping %v", sleepDuration.String())
			time.Sleep(sleepDuration)
		}
	}()
}
