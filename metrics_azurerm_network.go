package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/network/mgmt/network"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/Azure/go-autorest/autorest/to"
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
	prometheus.MustRegister(m.prometheus.vnet)

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
	prometheus.MustRegister(m.prometheus.vnetAddress)

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
	prometheus.MustRegister(m.prometheus.vnetSubnet)

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
	prometheus.MustRegister(m.prometheus.vnetSubnetAddress)

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
	prometheus.MustRegister(m.prometheus.nic)

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
	prometheus.MustRegister(m.prometheus.nicIp)

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
				"ipAddressVersion",
			},
			azureResourceTags.prometheusLabels...,
		),
	)
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
	client := network.NewVirtualNetworksClientWithBaseURI(azureEnvironment.ResourceManagerEndpoint, *subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer
	client.ResponseInspector = azureResponseInspector(&subscription)

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
			"vnetID":         to.String(val.ID),
			"subscriptionID": to.String(subscription.SubscriptionID),
			"resourceGroup":  extractResourceGroupFromAzureId(to.String(val.ID)),
			"vnetName":       to.String(val.Name),
		}
		infoLabels = azureResourceTags.appendPrometheusLabel(infoLabels, val.Tags)
		vnetMetric.AddInfo(infoLabels)

		if val.AddressSpace != nil && val.AddressSpace.AddressPrefixes != nil {
			for _, addressRange := range *val.AddressSpace.AddressPrefixes {
				vnetAddressMetric.AddInfo(prometheus.Labels{
					"vnetID":         to.String(val.ID),
					"subscriptionID": to.String(subscription.SubscriptionID),
					"addressRange":   addressRange,
				})
			}
		}

		// SUBNETS
		if val.Subnets != nil {
			for _, subnet := range *val.Subnets {
				vnetSubnetMetric.AddInfo(prometheus.Labels{
					"vnetID":         to.String(val.ID),
					"subnetID":       to.String(subnet.ID),
					"subscriptionID": to.String(subscription.SubscriptionID),
					"subnetName":     to.String(subnet.Name),
				})

				if subnet.AddressPrefix != nil {
					vnetSubnetAddressMetric.AddInfo(prometheus.Labels{
						"vnetID":         to.String(val.ID),
						"subnetID":       to.String(subnet.ID),
						"subscriptionID": to.String(subscription.SubscriptionID),
						"addressRange":   to.String(subnet.AddressPrefix),
					})
				}

				if subnet.AddressPrefixes != nil {
					for _, addressRange := range *subnet.AddressPrefixes {
						vnetSubnetAddressMetric.AddInfo(prometheus.Labels{
							"vnetID":         to.String(val.ID),
							"subnetID":       to.String(subnet.ID),
							"subscriptionID": to.String(subscription.SubscriptionID),
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
	client := network.NewInterfacesClientWithBaseURI(azureEnvironment.ResourceManagerEndpoint, *subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer
	client.ResponseInspector = azureResponseInspector(&subscription)

	list, err := client.ListAllComplete(ctx)
	if err != nil {
		logger.Panic(err)
	}

	infoMetric := prometheusCommon.NewMetricsList()
	ipConfigMetric := prometheusCommon.NewMetricsList()

	for list.NotDone() {
		val := list.Value()

		infoLabels := prometheus.Labels{
			"resourceID":                  to.String(val.ID),
			"subscriptionID":              to.String(subscription.SubscriptionID),
			"resourceGroup":               extractResourceGroupFromAzureId(*val.ID),
			"name":                        to.String(val.Name),
			"macAddress":                  to.String(val.MacAddress),
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
					"resourceID":       to.String(val.ID),
					"subscriptionID":   to.String(subscription.SubscriptionID),
					"subnetID":         vnetSubnetID,
					"isPrimary":        boolPtrToString(ipconf.Primary),
					"ipAddress":        to.String(ipconf.PrivateIPAddress),
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
	client := network.NewPublicIPAddressesClientWithBaseURI(azureEnvironment.ResourceManagerEndpoint, *subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer
	client.ResponseInspector = azureResponseInspector(&subscription)

	list, err := client.ListAll(ctx)
	if err != nil {
		logger.Panic(err)
	}

	infoMetric := prometheusCommon.NewMetricsList()

	for _, val := range list.Values() {
		location := to.String(val.Location)
		ipAddress := ""
		ipAllocationMethod := string(val.PublicIPAllocationMethod)
		ipAddressVersion := string(val.PublicIPAddressVersion)
		gaugeValue := float64(1)

		if val.IPAddress != nil {
			ipAddress = to.String(val.IPAddress)
		} else {
			ipAddress = "not allocated"
			gaugeValue = 0
		}

		infoLabels := prometheus.Labels{
			"resourceID":         to.String(val.ID),
			"subscriptionID":     *subscription.SubscriptionID,
			"resourceGroup":      extractResourceGroupFromAzureId(to.String(val.ID)),
			"location":           location,
			"ipAddress":          ipAddress,
			"ipAllocationMethod": ipAllocationMethod,
			"ipAddressVersion":   ipAddressVersion,
		}
		infoLabels = azureResourceTags.appendPrometheusLabel(infoLabels, val.Tags)

		infoMetric.Add(infoLabels, gaugeValue)
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.publicIp)
	}
}
