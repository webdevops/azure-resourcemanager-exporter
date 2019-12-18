package main

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/resources/mgmt/subscriptions"
	"github.com/prometheus/client_golang/prometheus"
)

type MetricsCollectorAzureRmCompute struct {
	CollectorProcessorGeneral

	prometheus struct {
		vm       *prometheus.GaugeVec
		vmOs     *prometheus.GaugeVec
		vmNic    *prometheus.GaugeVec
	}
}

func (m *MetricsCollectorAzureRmCompute) Setup(collector *CollectorGeneral) {
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
	m.prometheus.vmNic = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_vm_nic",
			Help: "Azure ResourceManager VM NIC",
		},
		[]string{
			"vmID",
			"resourceID",
			"isPrimary",
		},
	)

	prometheus.MustRegister(m.prometheus.vm)
	prometheus.MustRegister(m.prometheus.vmOs)
}

func (m *MetricsCollectorAzureRmCompute) Reset() {
	m.prometheus.vm.Reset()
	m.prometheus.vmOs.Reset()
	m.prometheus.vmNic.Reset()
}

func (m *MetricsCollectorAzureRmCompute) Collect(ctx context.Context, callback chan<- func(), subscription subscriptions.Subscription) {
	m.collectAzureVm(ctx, callback, subscription)
}

func (m *MetricsCollectorAzureRmCompute) collectAzureVm(ctx context.Context, callback chan<- func(), subscription subscriptions.Subscription) {
	client := compute.NewVirtualMachinesClient(*subscription.SubscriptionID)
	client.Authorizer = AzureAuthorizer

	list, err := client.ListAllComplete(ctx, "")

	if err != nil {
		panic(err)
	}

	infoMetric := MetricCollectorList{}
	osMetric := MetricCollectorList{}
	nicMetric := MetricCollectorList{}

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

		if val.NetworkProfile != nil && val.NetworkProfile.NetworkInterfaces != nil {
			for _, nic := range *val.NetworkProfile.NetworkInterfaces {
				nicMetric.AddInfo(prometheus.Labels{
					"vmID":           *val.VMID,
					"resourceID":     stringPtrToString(nic.ID),
					"isPrimary":      boolPtrToString(nic.Primary),
				})
			}
		}

		if list.NextWithContext(ctx) != nil {
			break
		}
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.vm)
		osMetric.GaugeSet(m.prometheus.vmOs)
		nicMetric.GaugeSet(m.prometheus.vmNic)
	}
}
