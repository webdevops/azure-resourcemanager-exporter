package main

import (
	"encoding/json"
	scanner "github.com/anvie/port-scanner"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/remeh/sizedwaitgroup"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"strconv"
	"sync"
	"time"
)

type PortscannerResult struct {
	IpAddress string
	Labels    prometheus.Labels
	Value     float64
}

type Portscanner struct {
	List      map[string][]PortscannerResult
	PublicIps map[string]string
	Enabled   bool `json:"-"`
	mux       sync.Mutex

	logger *log.Entry

	Callbacks struct {
		StartupScan        func(c *Portscanner)
		FinishScan         func(c *Portscanner)
		StartScanIpAdress  func(c *Portscanner, ipAddress string)
		FinishScanIpAdress func(c *Portscanner, ipAddress string, elapsed float64)
		ResultCleanup      func(c *Portscanner)
		ResultPush         func(c *Portscanner, result PortscannerResult)
	} `json:"-"`
}

func (c *Portscanner) Init() {
	c.Enabled = false
	c.List = map[string][]PortscannerResult{}
	c.PublicIps = map[string]string{}

	c.logger = log.WithField("component", "portscanner")

	c.Callbacks.StartupScan = func(c *Portscanner) {}
	c.Callbacks.FinishScan = func(c *Portscanner) {}
	c.Callbacks.StartScanIpAdress = func(c *Portscanner, ipAddress string) {}
	c.Callbacks.FinishScanIpAdress = func(c *Portscanner, ipAddress string, elapsed float64) {}
	c.Callbacks.ResultCleanup = func(c *Portscanner) {}
	c.Callbacks.ResultPush = func(c *Portscanner, result PortscannerResult) {}
}

func (c *Portscanner) Enable() {
	c.Enabled = true
}

func (c *Portscanner) CacheLoad(path string) {
	c.mux.Lock()

	file, err := os.Open(path)
	if err != nil {
		c.logger.Panic(err)
	}
	defer file.Close()

	jsonContent, _ := ioutil.ReadAll(file)
	err = json.Unmarshal(jsonContent, &c)
	if err != nil {
		c.logger.Errorf("failed to load portscanner cache: %v", err)
	}

	c.mux.Unlock()

	// cleanup and update prometheus again
	c.Cleanup()
	c.Publish()
}

func (c *Portscanner) CacheSave(path string) {
	c.mux.Lock()

	jsonData, _ := json.Marshal(c)
	err := ioutil.WriteFile(path, jsonData, 0644)
	if err != nil {
		c.logger.Panic(err)
	}

	c.mux.Unlock()
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
	for ipAddress, _ := range c.List {
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
			c.Callbacks.ResultPush(c, result)
		}
	}
}

func (c *Portscanner) Start() {
	portscanTimeout := time.Duration(opts.Portscan.Timeout) * time.Second

	c.Callbacks.StartupScan(c)

	// cleanup and update prometheus again
	c.Cleanup()
	c.Publish()

	swg := sizedwaitgroup.New(opts.Portscan.Prallel)
	for _, ipAddress := range c.PublicIps {
		swg.Add()
		go func(ipAddress string, portscanTimeout time.Duration) {
			defer swg.Done()

			c.Callbacks.StartScanIpAdress(c, ipAddress)

			results, elapsed := c.scanIp(ipAddress, portscanTimeout)

			c.Callbacks.FinishScanIpAdress(c, ipAddress, elapsed)

			c.addResults(ipAddress, results)
		}(ipAddress, portscanTimeout)
	}

	// wait for all port scanners
	swg.Wait()

	// cleanup and update prometheus again
	c.Cleanup()
	c.Publish()

	c.Callbacks.FinishScan(c)
}

func (c *Portscanner) scanIp(ipAddress string, portscanTimeout time.Duration) (result []PortscannerResult, elapsed float64) {
	startTime := time.Now().Unix()

	contextLogger := c.logger.WithField("ipAddress", ipAddress)

	// check if public ip is still owned
	if _, ok := c.PublicIps[ipAddress]; !ok {
		return
	}

	ps := scanner.NewPortScanner(ipAddress, portscanTimeout, opts.Portscan.Threads)

	for _, portrange := range portscanPortRange {
		openedPorts := ps.GetOpenedPort(portrange.FirstPort, portrange.LastPort)

		for _, port := range openedPorts {
			contextLogger.WithField("port", port).Debugf("detected open port %v", ipAddress, port)
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
