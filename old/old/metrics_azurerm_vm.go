package old

import (
	"context"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/compute/mgmt/compute"
	"github.com/Azure/azure-sdk-for-go/profiles/latest/network/mgmt/network"
	"github.com/prometheus/client_golang/prometheus"
)

func (m *MetricCollectorAzureRm) initVm() {
	m.prometheus.vm = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_vm_info",
			Help: "Azure ResourceManager VMs",
		},
		append(
			[]string{"resourceID", "subscriptionID", "location", "resourceGroup", "vmID", "vmName", "vmType", "vmSize", "vmProvisioningState"},
			prefixSlice(AZURE_RESOURCE_TAG_PREFIX, opts.AzureResourceTags)...
		),
	)

	m.prometheus.vmOs = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_vm_os",
			Help: "Azure ResourceManager VM OS",
		},
		[]string{"vmID", "imagePublisher", "imageSku", "imageOffer", "imageVersion"},
	)

	m.prometheus.publicIp = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "azurerm_publicip_info",
			Help: "Azure ResourceManager public ip",
		},
		append(
			[]string{"resourceID", "subscriptionID", "resourceGroup", "location", "ipAddress", "ipAllocationMethod", "ipAdressVersion"},
			prefixSlice(AZURE_RESOURCE_TAG_PREFIX, opts.AzureResourceTags)...
		),
	)

	prometheus.MustRegister(m.prometheus.vm)
	prometheus.MustRegister(m.prometheus.vmOs)
	prometheus.MustRegister(m.prometheus.publicIp)
}

// Collect Azure PublicIP metrics
func (m *MetricCollectorAzureRm) collectAzurePublicIp(ctx context.Context, subscriptionId string, callback chan<- func()) (ipAddressList []string) {
	client := network.NewPublicIPAddressesClient(subscriptionId)
	client.Authorizer = AzureAuthorizer

	list, err := client.ListAll(ctx)
	if err != nil {
		panic(err)
	}

	infoMetric := prometheusMetricsList{}

	for _, val:= range list.Values() {
		location := *val.Location
		ipAddress := ""
		ipAllocationMethod := string(val.PublicIPAllocationMethod)
		ipAdressVersion := string(val.PublicIPAddressVersion)
		gaugeValue := float64(1)

		if val.IPAddress != nil {
			ipAddress = *val.IPAddress
			ipAddressList = append(ipAddressList, ipAddress)
		} else {
			ipAddress = "not allocated"
			gaugeValue = 0
		}

		infoLabels := prometheus.Labels{
			"resourceID": *val.ID,
			"subscriptionID":     subscriptionId,
			"resourceGroup":      extractResourceGroupFromAzureId(*val.ID),
			"location":           location,
			"ipAddress":          ipAddress,
			"ipAllocationMethod": ipAllocationMethod,
			"ipAdressVersion":    ipAdressVersion,
		}
		infoLabels = m.addAzureResourceTags(infoLabels, val.Tags)

		infoMetric.Add(infoLabels, gaugeValue)
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.publicIp)
	}

	return
}


func (m *MetricCollectorAzureRm) collectAzureVm(ctx context.Context, subscriptionId string, callback chan<- func()) {
	client := compute.NewVirtualMachinesClient(subscriptionId)
	client.Authorizer = AzureAuthorizer

	list, err := client.ListAllComplete(ctx)

	if err != nil {
		panic(err)
	}

	infoMetric := prometheusMetricsList{}
	osMetric := prometheusMetricsList{}

	for list.NotDone() {
		val := list.Value()

		infoLabels := prometheus.Labels{
			"resourceID": *val.ID,
			"subscriptionID": subscriptionId,
			"location": *val.Location,
			"resourceGroup": extractResourceGroupFromAzureId(*val.ID),
			"vmID": *val.VMID,
			"vmName": *val.Name,
			"vmType": *val.Type,
			"vmSize": string(val.VirtualMachineProperties.HardwareProfile.VMSize),
			"vmProvisioningState": *val.ProvisioningState,
		}
		infoLabels = m.addAzureResourceTags(infoLabels, val.Tags)

		osLabels := prometheus.Labels{
			"vmID": *val.VMID,
			"imagePublisher": *val.StorageProfile.ImageReference.Publisher,
			"imageSku": *val.StorageProfile.ImageReference.Sku,
			"imageOffer": *val.StorageProfile.ImageReference.Offer,
			"imageVersion": *val.StorageProfile.ImageReference.Version,
		}

		infoMetric.Add(infoLabels, 1)
		osMetric.Add(osLabels, 1)

		if list.NextWithContext(ctx) != nil {
			break
		}
	}

	callback <- func() {
		infoMetric.GaugeSet(m.prometheus.vm)
		osMetric.GaugeSet(m.prometheus.vmOs)
	}
}
