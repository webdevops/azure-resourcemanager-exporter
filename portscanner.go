package main

import (
	"strconv"
	"sync"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork"
	scanner "github.com/anvie/port-scanner"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/remeh/sizedwaitgroup"
	"github.com/webdevops/go-common/utils/to"
	"go.uber.org/zap"
)

type PortscannerResult struct {
	IpAddress string
	Labels    prometheus.Labels
	Value     float64
}

type Portscanner struct {
	Data    *PortscannerData
	Enabled bool `json:"-"`
	mux     sync.Mutex

	logger *zap.SugaredLogger

	Callbacks struct {
		RestoreCache       func(c *Portscanner) interface{}
		StoreCache         func(c *Portscanner, data interface{})
		StartupScan        func(c *Portscanner)
		FinishScan         func(c *Portscanner)
		StartScanIpAdress  func(c *Portscanner, pip armnetwork.PublicIPAddress)
		FinishScanIpAdress func(c *Portscanner, pip armnetwork.PublicIPAddress, elapsed float64)
		ResultCleanup      func(c *Portscanner)
		ResultPush         func(c *Portscanner, result PortscannerResult)
	} `json:"-"`
}

type PortscannerData struct {
	List      map[string][]PortscannerResult
	PublicIps map[string]*armnetwork.PublicIPAddress
}

func (c *Portscanner) Init() {
	c.Enabled = false
	c.Data = &PortscannerData{
		List:      map[string][]PortscannerResult{},
		PublicIps: map[string]*armnetwork.PublicIPAddress{},
	}

	c.logger = logger.With(zap.String("component", "portscanner"))

	c.Callbacks.RestoreCache = func(c *Portscanner) interface{} { return nil }
	c.Callbacks.StoreCache = func(c *Portscanner, data interface{}) {}
	c.Callbacks.StartupScan = func(c *Portscanner) {}
	c.Callbacks.FinishScan = func(c *Portscanner) {}
	c.Callbacks.StartScanIpAdress = func(c *Portscanner, pip armnetwork.PublicIPAddress) {}
	c.Callbacks.FinishScanIpAdress = func(c *Portscanner, pip armnetwork.PublicIPAddress, elapsed float64) {}
	c.Callbacks.ResultCleanup = func(c *Portscanner) {}
	c.Callbacks.ResultPush = func(c *Portscanner, result PortscannerResult) {}
}

func (c *Portscanner) Enable() {
	c.Enabled = true
}

func (c *Portscanner) CacheLoad() {
	c.mux.Lock()

	if val := c.Callbacks.RestoreCache(c); val != nil {
		if data, ok := val.(*PortscannerData); ok {
			c.logger.Infof(`restored state from cache`)
			c.Data = data

			if c.Data.List == nil {
				c.Data.List = map[string][]PortscannerResult{}
			}

			if c.Data.PublicIps == nil {
				c.Data.PublicIps = map[string]*armnetwork.PublicIPAddress{}
			}
		} else {
			c.logger.Errorf("failed to load portscanner cache")
		}
	}

	c.mux.Unlock()

	// cleanup and update prometheus again
	c.Cleanup()
	c.Publish()
}

func (c *Portscanner) CacheSave() {
	c.mux.Lock()
	c.Callbacks.StoreCache(c, c.Data)
	c.mux.Unlock()
}

func (c *Portscanner) SetAzurePublicIpList(pipList []*armnetwork.PublicIPAddress) {
	c.mux.Lock()

	// build map
	ipAddressList := map[string]*armnetwork.PublicIPAddress{}
	for _, pip := range pipList {
		ipAddress := to.String(pip.Properties.IPAddress)
		ipAddressList[ipAddress] = pip
	}

	c.Data.PublicIps = ipAddressList
	c.mux.Unlock()
}

func (c *Portscanner) addResults(pip armnetwork.PublicIPAddress, results []PortscannerResult) {
	ipAddress := to.String(pip.Properties.IPAddress)
	// update result cache and update prometheus
	c.mux.Lock()
	c.Data.List[ipAddress] = results
	c.pushResults()
	c.mux.Unlock()
}

func (c *Portscanner) Cleanup() {
	// cleanup
	c.mux.Lock()

	orphanedIpList := []string{}
	for ipAddress := range c.Data.List {
		if _, ok := c.Data.PublicIps[ipAddress]; !ok {
			orphanedIpList = append(orphanedIpList, ipAddress)
		}
	}

	// delete oprhaned IPs
	for _, ipAddress := range orphanedIpList {
		delete(c.Data.List, ipAddress)
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
	for _, results := range c.Data.List {
		for _, result := range results {
			c.Callbacks.ResultPush(c, result)
		}
	}
}

func (c *Portscanner) Start() {
	portscanTimeout := time.Duration(Opts.Portscan.Timeout) * time.Second

	c.Callbacks.StartupScan(c)

	// cleanup and update prometheus again
	c.Cleanup()
	c.Publish()

	swg := sizedwaitgroup.New(Opts.Portscan.Parallel)
	for _, pip := range c.Data.PublicIps {
		swg.Add()
		go func(pip armnetwork.PublicIPAddress, portscanTimeout time.Duration) {
			defer swg.Done()

			c.Callbacks.StartScanIpAdress(c, pip)

			results, elapsed := c.scanIp(pip, portscanTimeout)

			c.Callbacks.FinishScanIpAdress(c, pip, elapsed)

			c.addResults(pip, results)
		}(*pip, portscanTimeout)
	}

	// wait for all port scanners
	swg.Wait()

	// cleanup and update prometheus again
	c.Cleanup()
	c.Publish()

	c.Callbacks.FinishScan(c)
}

func (c *Portscanner) scanIp(pip armnetwork.PublicIPAddress, portscanTimeout time.Duration) (result []PortscannerResult, elapsed float64) {
	ipAddress := to.String(pip.Properties.IPAddress)
	startTime := time.Now().Unix()

	contextLogger := c.logger.With(zap.String("ipAddress", ipAddress))

	// check if public ip is still owned
	if _, ok := c.Data.PublicIps[ipAddress]; !ok {
		return
	}

	ps := scanner.NewPortScanner(ipAddress, portscanTimeout, Opts.Portscan.Threads)

	for _, portrange := range portscanPortRange {
		openedPorts := ps.GetOpenedPort(portrange.FirstPort, portrange.LastPort)

		for _, port := range openedPorts {
			contextLogger.With(zap.Int("port", port)).Debugf("detected open port %v", port)
			result = append(
				result,
				PortscannerResult{
					IpAddress: ipAddress,
					Labels: prometheus.Labels{
						"ipAddress":   ipAddress,
						"protocol":    "TCP",
						"port":        strconv.Itoa(port),
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
