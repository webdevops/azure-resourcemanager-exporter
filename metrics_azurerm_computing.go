package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/network/mgmt/network"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/prometheus/client_golang/prometheus"
)

type MetricsCollectorAzureRmComputing struct {
	CollectorProcessorGeneral

	prometheus struct {
		vm       *prometheus.GaugeVec
		vmOs     *prometheus.GaugeVec
		publicIp *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorAzureRmComputing) Setup(collector *CollectorGeneral) {
	m.CollectorReference = collector

	m.prometheus.vm = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_vm_info",
			Help: "Azure ResourceManager VMs",
		},
		append(
			[]string{
				"resourceID",
				"subscriptionID",
				"location",
				"resourceGroup",
				"vmID",
				"vmName",
				"vmType",
				"vmSize",
				"vmProvisioningState",
			},
			opts.azureResourceTags.prometheusLabels...,
		),
	)

	m.prometheus.vmOs = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_vm_os",
			Help: "Azure ResourceManager VM OS",
		},
		[]string{
			"vmID",
			"imagePublisher",
			"imageSku",
			"imageOffer",
			"imageVersion",
		},
	)

	m.prometheus.publicIp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_publicip_info",
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
			opts.azureResourceTags.prometheusLabels...,
		),
	)

	prometheus.MustRegister(m.prometheus.vm)
	prometheus.MustRegister(m.prometheus.vmOs)
	prometheus.MustRegister(m.prometheus.publicIp)
}

func (m *MetricsCollectorAzureRmComputing) Reset() {
	m.prometheus.vm.Reset()
	m.prometheus.vmOs.Reset()
	m.prometheus.publicIp.Reset()
}

func (m *MetricsCollectorAzureRmComputing) Collect(ctx context.Context, callback chan<- func(), subscription subscriptions.Subscription) {
	m.collectAzureVm(ctx, callback, subscription)
	m.collectAzurePublicIp(ctx, callback, subscription)
}

// Collect Azure PublicIP metrics
func (m *MetricsCollectorAzureRmComputing) collectAzurePublicIp(ctx context.Context, callback chan<- func(), subscription subscriptions.Subscription) (ipAddressList []string) {
	client := network.NewPublicIPAddressesClient(*subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer

	list, err := client.ListAll(ctx)
	if err != nil {
		panic(err)
	}

	infoMetric := MetricCollectorList{}

	for _, val := range list.Values() {
		location := *val.Location
		ipAddress := ""
		ipAllocationMethod := string(val.PublicIPAllocationMethod)
		ipAdressVersion := string(val.PublicIPAddressVersion)
		gaugeValue := float64(1)

		if val.IPAddress != nil {
			ipAddress = stringPtrToString(val.IPAddress)
			ipAddressList = append(ipAddressList, ipAddress)
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
		infoLabels = opts.azureResourceTags.appendPrometheusLabel(infoLabels, val.Tags)

		infoMetric.Add(infoLabels, gaugeValue)
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.publicIp)
	}

	return
}

func (m *MetricsCollectorAzureRmComputing) collectAzureVm(ctx context.Context, callback chan<- func(), subscription subscriptions.Subscription) {
	client := compute.NewVirtualMachinesClient(*subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer

	list, err := client.ListAllComplete(ctx, "")

	if err != nil {
		panic(err)
	}

	infoMetric := MetricCollectorList{}
	osMetric := MetricCollectorList{}

	for list.NotDone() {
		val := list.Value()

		infoLabels := prometheus.Labels{
			"resourceID":          *val.ID,
			"subscriptionID":      *subscription.SubscriptionID,
			"location":            stringPtrToString(val.Location),
			"resourceGroup":       extractResourceGroupFromAzureId(*val.ID),
			"vmID":                stringPtrToString(val.VMID),
			"vmName":              stringPtrToString(val.Name),
			"vmType":              stringPtrToString(val.Type),
			"vmSize":              string(val.VirtualMachineProperties.HardwareProfile.VMSize),
			"vmProvisioningState": stringPtrToString(val.ProvisioningState),
		}
		infoLabels = opts.azureResourceTags.appendPrometheusLabel(infoLabels, val.Tags)
		infoMetric.AddInfo(infoLabels)

		if val.StorageProfile != nil {
			osMetric.AddInfo(prometheus.Labels{
				"vmID":           *val.VMID,
				"imagePublisher": stringPtrToString(val.StorageProfile.ImageReference.Publisher),
				"imageSku":       stringPtrToString(val.StorageProfile.ImageReference.Sku),
				"imageOffer":     stringPtrToString(val.StorageProfile.ImageReference.Offer),
				"imageVersion":   stringPtrToString(val.StorageProfile.ImageReference.Version),
			})
		}

		if list.NextWithContext(ctx) != nil {
			break
		}
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.vm)
		osMetric.GaugeSet(m.prometheus.vmOs)
	}
}
