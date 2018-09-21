package main

import (
	"sync"
	"time"
	"strconv"
	"github.com/remeh/sizedwaitgroup"
	scanner "github.com/anvie/port-scanner"
	"github.com/prometheus/client_golang/prometheus"
)

type PortscannerResult struct {
	IpAddress string
	Labels prometheus.Labels
	Value float64
}

type Portscanner struct {
	List map[string][]PortscannerResult
	PublicIps map[string]string
	Enabled bool
	mux sync.Mutex

	Callbacks struct {
		StartupScan func(c *Portscanner)
		FinishScan func(c *Portscanner)
		StartScanIpAdress func(c *Portscanner, ipAddress string)
		FinishScanIpAdress func(c *Portscanner, ipAddress string, elapsed float64)
		ResultCleanup func(c *Portscanner)
		ResultPush func(c *Portscanner, result PortscannerResult)
	}
}

func (c *Portscanner) Init() {
	c.Enabled = false
	c.List = map[string][]PortscannerResult{}
	c.PublicIps = map[string]string{}

	portscanner.Callbacks.StartupScan = func(c *Portscanner) {}
	portscanner.Callbacks.FinishScan = func(c *Portscanner) {}
	portscanner.Callbacks.StartScanIpAdress = func(c *Portscanner, ipAddress string) {}
	portscanner.Callbacks.FinishScanIpAdress = func(c *Portscanner, ipAddress string, elapsed float64) {}
	portscanner.Callbacks.ResultCleanup = func(c *Portscanner) {}
	portscanner.Callbacks.ResultPush = func(c *Portscanner, result PortscannerResult) {}
}

func (c *Portscanner) Enable() {
	c.Enabled = true
}

func (c *Portscanner) SetIps(ipAddresses []string) {
	c.mux.Lock()

	// build map
	ipAddressList := map[string]string{}
	for _, ipAddress := range ipAddresses {
		ipAddressList[ipAddress] = ipAddress
	}
	
	c.PublicIps = ipAddressList
	c.mux.Unlock()
}

func (c *Portscanner) addResults(ipAddress string, results []PortscannerResult) {
	// update result cache and update prometheus
	c.mux.Lock()
	c.List[ipAddress] = results
	c.pushResults()
	c.mux.Unlock()
}

func (c *Portscanner) Cleanup() {
	// cleanup
	c.mux.Lock()

	orphanedIpList := []string{}
	for ipAddress,_ := range c.List {
		if _, ok := c.PublicIps[ipAddress]; !ok {
			orphanedIpList = append(orphanedIpList, ipAddress)
		}
	}

	// delete oprhaned IPs
	for _, ipAddress := range orphanedIpList {
		delete(c.List, ipAddress)
	}

	c.mux.Unlock()
}

func (c *Portscanner) Publish() {
	c.mux.Lock()
	c.Callbacks.ResultCleanup(c)
	c.pushResults()
	c.mux.Unlock()
}

func (c *Portscanner) pushResults() {
	for _, results := range c.List {
		for _, result := range results {
			portscanner.Callbacks.ResultPush(c, result)
		}
	}
}

func (c *Portscanner) Start() {
	portscanTimeout := time.Duration(opts.PortscanTimeout) * time.Second

	c.Callbacks.StartupScan(c)

	// cleanup and update prometheus again
	portscanner.Cleanup()
	portscanner.Publish()

	swg := sizedwaitgroup.New(opts.PortscanPrallel)
	for _, ipAddress := range portscanner.PublicIps {
		swg.Add()
		go func(ipAddress string, portscanTimeout time.Duration) {
			defer swg.Done()

			c.Callbacks.StartScanIpAdress(c, ipAddress)
			
			results, elapsed := c.scanIp(ipAddress, portscanTimeout)

			c.Callbacks.FinishScanIpAdress(c, ipAddress, elapsed)
			
			portscanner.addResults(ipAddress, results)
		}(ipAddress, portscanTimeout)
	}

	// wait for all port scanners
	swg.Wait()

	// cleanup and update prometheus again
	portscanner.Cleanup()
	portscanner.Publish()

	c.Callbacks.FinishScan(c)
}

func (c *Portscanner) scanIp(ipAddress string, portscanTimeout time.Duration) (result []PortscannerResult, elapsed float64) {
	startTime := time.Now().Unix()

	// check if public ip is still owned
	if _, ok := c.PublicIps[ipAddress]; !ok {
		return
	}

	ps := scanner.NewPortScanner(ipAddress, portscanTimeout, opts.PortscanThreads)

	for _, portrange := range opts.portscanPortRange {
		openedPorts := ps.GetOpenedPort(portrange.FirstPort, portrange.LastPort)

		for _, port := range openedPorts {
			Logger.Verbose("Found IP %v with opend port %v", ipAddress, port)
			result = append(
				result,
				PortscannerResult{
					IpAddress: ipAddress,
					Labels: prometheus.Labels{
						"ipAddress": ipAddress,
						"protocol": "TCP",
						"port": strconv.Itoa(port),
						"description": "",
					},
					Value: 1,
				},
			)
		}
	}

	elapsed = float64(time.Now().Unix() - startTime)

	return result, elapsed
}
