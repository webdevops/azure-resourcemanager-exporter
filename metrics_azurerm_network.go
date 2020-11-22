package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/network/mgmt/network"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	prometheusCommon "github.com/webdevops/go-prometheus-common"
)

type MetricsCollectorAzureRmNetwork struct {
	CollectorProcessorGeneral

	prometheus struct {
		vnet              *prometheus.GaugeVec
		vnetAddress       *prometheus.GaugeVec
		vnetSubnet        *prometheus.GaugeVec
		vnetSubnetAddress *prometheus.GaugeVec
		nic               *prometheus.GaugeVec
		nicIp             *prometheus.GaugeVec
		publicIp          *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorAzureRmNetwork) Setup(collector *CollectorGeneral) {
	m.CollectorReference = collector

	m.prometheus.vnet = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_network_vnet_info",
			Help: "Azure ResourceManager Virtual Network",
		},
		append(
			[]string{
				"vnetID",
				"subscriptionID",
				"resourceGroup",
				"vnetName",
			},
			azureResourceTags.prometheusLabels...,
		),
	)

	m.prometheus.vnetAddress = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_network_vnet_address",
			Help: "Azure ResourceManager Virtual Network address range",
		},
		[]string{
			"vnetID",
			"subscriptionID",
			"addressRange",
		},
	)

	m.prometheus.vnetSubnet = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_network_vnet_subnet",
			Help: "Azure ResourceManager Virtual Network subnet",
		},
		[]string{
			"vnetID",
			"subnetID",
			"subscriptionID",
			"subnetName",
		},
	)

	m.prometheus.vnetSubnetAddress = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_network_vnet_subnet_address",
			Help: "Azure ResourceManager Virtual Network subnet address range",
		},
		[]string{
			"vnetID",
			"subnetID",
			"subscriptionID",
			"addressRange",
		},
	)

	m.prometheus.nic = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_network_interface_info",
			Help: "Azure ResourceManager Network network interface card",
		},
		append(
			[]string{
				"resourceID",
				"subscriptionID",
				"resourceGroup",
				"name",
				"macAddress",
				"isPrimary",
				"enableIPForwarding",
				"enableAcceleratedNetworking",
			},
			azureResourceTags.prometheusLabels...,
		),
	)

	m.prometheus.nicIp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_network_interface_ip",
			Help: "Azure ResourceManager Network network interface ip config",
		},
		[]string{
			"resourceID",
			"subscriptionID",
			"subnetID",
			"isPrimary",
			"ipAddress",
			"ipAddressVersion",
			"allocationMethod",
		},
	)

	m.prometheus.publicIp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_network_publicip_info",
			Help: "Azure ResourceManager public ip",
		},
		append(
			[]string{
				"resourceID",
				"subscriptionID",
				"resourceGroup",
				"location",
				"ipAddress",
				"ipAllocationMethod",
				"ipAdressVersion",
			},
			azureResourceTags.prometheusLabels...,
		),
	)

	prometheus.MustRegister(m.prometheus.vnet)
	prometheus.MustRegister(m.prometheus.vnetAddress)
	prometheus.MustRegister(m.prometheus.vnetSubnet)
	prometheus.MustRegister(m.prometheus.vnetSubnetAddress)
	prometheus.MustRegister(m.prometheus.nic)
	prometheus.MustRegister(m.prometheus.nicIp)
	prometheus.MustRegister(m.prometheus.publicIp)
}

func (m *MetricsCollectorAzureRmNetwork) Reset() {
	m.prometheus.vnet.Reset()
	m.prometheus.vnetAddress.Reset()
	m.prometheus.vnetSubnet.Reset()
	m.prometheus.vnetSubnetAddress.Reset()
	m.prometheus.nic.Reset()
	m.prometheus.nicIp.Reset()
	m.prometheus.publicIp.Reset()
}

func (m *MetricsCollectorAzureRmNetwork) Collect(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	m.collectAzureVnet(ctx, logger, callback, subscription)
	m.collectAzureNics(ctx, logger, callback, subscription)
	m.collectAzurePublicIp(ctx, logger, callback, subscription)
}

// Collect Azure NIC metrics
func (m *MetricsCollectorAzureRmNetwork) collectAzureVnet(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	client := network.NewVirtualNetworksClient(*subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer

	list, err := client.ListAllComplete(ctx)
	if err != nil {
		logger.Panic(err)
	}

	vnetMetric := prometheusCommon.NewMetricsList()
	vnetAddressMetric := prometheusCommon.NewMetricsList()
	vnetSubnetMetric := prometheusCommon.NewMetricsList()
	vnetSubnetAddressMetric := prometheusCommon.NewMetricsList()

	for list.NotDone() {
		val := list.Value()

		// VNET
		infoLabels := prometheus.Labels{
			"vnetID":         *val.ID,
			"subscriptionID": *subscription.SubscriptionID,
			"resourceGroup":  extractResourceGroupFromAzureId(*val.ID),
			"vnetName":       stringPtrToString(val.Name),
		}
		infoLabels = azureResourceTags.appendPrometheusLabel(infoLabels, val.Tags)
		vnetMetric.AddInfo(infoLabels)

		if val.AddressSpace != nil && val.AddressSpace.AddressPrefixes != nil {
			for _, addressRange := range *val.AddressSpace.AddressPrefixes {
				vnetAddressMetric.AddInfo(prometheus.Labels{
					"vnetID":         *val.ID,
					"subscriptionID": *subscription.SubscriptionID,
					"addressRange":   addressRange,
				})
			}
		}

		// SUBNETS
		if val.Subnets != nil {
			for _, subnet := range *val.Subnets {
				vnetSubnetMetric.AddInfo(prometheus.Labels{
					"vnetID":         *val.ID,
					"subnetID":       *subnet.ID,
					"subscriptionID": *subscription.SubscriptionID,
					"subnetName":     stringPtrToString(subnet.Name),
				})

				if subnet.AddressPrefix != nil {
					vnetSubnetAddressMetric.AddInfo(prometheus.Labels{
						"vnetID":         *val.ID,
						"subnetID":       *subnet.ID,
						"subscriptionID": *subscription.SubscriptionID,
						"addressRange":   *subnet.AddressPrefix,
					})
				}

				if subnet.AddressPrefixes != nil {
					for _, addressRange := range *subnet.AddressPrefixes {
						vnetSubnetAddressMetric.AddInfo(prometheus.Labels{
							"vnetID":         *val.ID,
							"subnetID":       *subnet.ID,
							"subscriptionID": *subscription.SubscriptionID,
							"addressRange":   addressRange,
						})
					}
				}
			}
		}

		if list.NextWithContext(ctx) != nil {
			break
		}
	}

	callback <- func() {
		vnetMetric.GaugeSet(m.prometheus.vnet)
		vnetAddressMetric.GaugeSet(m.prometheus.vnetAddress)
		vnetSubnetMetric.GaugeSet(m.prometheus.vnetSubnet)
		vnetSubnetAddressMetric.GaugeSet(m.prometheus.vnetSubnetAddress)
	}
}

// Collect Azure NIC metrics
func (m *MetricsCollectorAzureRmNetwork) collectAzureNics(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	client := network.NewInterfacesClient(*subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer

	list, err := client.ListAllComplete(ctx)
	if err != nil {
		logger.Panic(err)
	}

	infoMetric := prometheusCommon.NewMetricsList()
	ipConfigMetric := prometheusCommon.NewMetricsList()

	for list.NotDone() {
		val := list.Value()

		infoLabels := prometheus.Labels{
			"resourceID":                  *val.ID,
			"subscriptionID":              *subscription.SubscriptionID,
			"resourceGroup":               extractResourceGroupFromAzureId(*val.ID),
			"name":                        stringPtrToString(val.Name),
			"macAddress":                  stringPtrToString(val.MacAddress),
			"isPrimary":                   boolPtrToString(val.Primary),
			"enableIPForwarding":          boolPtrToString(val.EnableIPForwarding),
			"enableAcceleratedNetworking": boolPtrToString(val.EnableAcceleratedNetworking),
		}
		infoLabels = azureResourceTags.appendPrometheusLabel(infoLabels, val.Tags)
		infoMetric.AddInfo(infoLabels)

		if val.IPConfigurations != nil {
			for _, ipconf := range *val.IPConfigurations {
				vnetSubnetID := ""
				if ipconf.Subnet != nil && ipconf.Subnet.ID != nil {
					vnetSubnetID = *ipconf.Subnet.ID
				}

				ipConfigMetric.AddInfo(prometheus.Labels{
					"resourceID":       *val.ID,
					"subscriptionID":   *subscription.SubscriptionID,
					"subnetID":         vnetSubnetID,
					"isPrimary":        boolPtrToString(ipconf.Primary),
					"ipAddress":        stringPtrToString(ipconf.PrivateIPAddress),
					"ipAddressVersion": string(ipconf.PrivateIPAddressVersion),
					"allocationMethod": string(ipconf.PrivateIPAllocationMethod),
				})
			}
		}

		if list.NextWithContext(ctx) != nil {
			break
		}
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.nic)
		ipConfigMetric.GaugeSet(m.prometheus.nicIp)
	}
}

// Collect Azure PublicIP metrics
func (m *MetricsCollectorAzureRmNetwork) collectAzurePublicIp(ctx context.Context, logger *log.Entry, callback chan<- func(), subscription subscriptions.Subscription) {
	client := network.NewPublicIPAddressesClient(*subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer

	list, err := client.ListAll(ctx)
	if err != nil {
		logger.Panic(err)
	}

	infoMetric := prometheusCommon.NewMetricsList()

	for _, val := range list.Values() {
		location := *val.Location
		ipAddress := ""
		ipAllocationMethod := string(val.PublicIPAllocationMethod)
		ipAdressVersion := string(val.PublicIPAddressVersion)
		gaugeValue := float64(1)

		if val.IPAddress != nil {
			ipAddress = stringPtrToString(val.IPAddress)
		} else {
			ipAddress = "not allocated"
			gaugeValue = 0
		}

		infoLabels := prometheus.Labels{
			"resourceID":         *val.ID,
			"subscriptionID":     *subscription.SubscriptionID,
			"resourceGroup":      extractResourceGroupFromAzureId(*val.ID),
			"location":           location,
			"ipAddress":          ipAddress,
			"ipAllocationMethod": ipAllocationMethod,
			"ipAdressVersion":    ipAdressVersion,
		}
		infoLabels = azureResourceTags.appendPrometheusLabel(infoLabels, val.Tags)

		infoMetric.Add(infoLabels, gaugeValue)
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.publicIp)
	}
}
