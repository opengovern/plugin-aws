package processor

import (
	"fmt"
	types2 "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/utils"
	"github.com/kaytu-io/plugin-aws/plugin/kaytu"
	"strings"
)

type EC2InstanceItem struct {
	Instance            types.Instance
	Region              string
	OptimizationLoading bool
	Preferences         []*golang.PreferenceItem
	Skipped             bool
	LazyLoadingEnabled  bool
	SkipReason          string
	Volumes             []types.Volume
	Metrics             map[string][]types2.Datapoint
	VolumeMetrics       map[string]map[string][]types2.Datapoint
	Wastage             kaytu.EC2InstanceWastageResponse
}

func (i EC2InstanceItem) EC2InstanceDevice() *golang.Device {
	ec2Instance := &golang.Device{
		Properties:   nil,
		DeviceId:     *i.Instance.InstanceId,
		ResourceType: "EC2 Instance",
		Runtime:      "730 hours",
		CurrentCost:  i.Wastage.RightSizing.Current.Cost,
	}
	regionProperty := &golang.Property{
		Key:     "Region",
		Current: i.Wastage.RightSizing.Current.Region,
	}
	instanceSizeProperty := &golang.Property{
		Key:     "Instance Size",
		Current: i.Wastage.RightSizing.Current.InstanceType,
	}
	vCPUProperty := &golang.Property{
		Key:     "  vCPU",
		Current: fmt.Sprintf("%d", i.Wastage.RightSizing.Current.VCPU),
		Average: utils.Percentage(i.Wastage.RightSizing.VCPU.Avg),
		Max:     utils.Percentage(i.Wastage.RightSizing.VCPU.Max),
	}
	processorProperty := &golang.Property{
		Key:     "  Processor(s)",
		Current: i.Wastage.RightSizing.Current.Processor,
	}
	architectureProperty := &golang.Property{
		Key:     "  Architecture",
		Current: i.Wastage.RightSizing.Current.Architecture,
	}
	licenseCostProperty := &golang.Property{
		Key:     "  License Cost",
		Current: fmt.Sprintf("$%.2f", i.Wastage.RightSizing.Current.LicensePrice),
	}
	memoryProperty := &golang.Property{
		Key:     "  Memory",
		Current: fmt.Sprintf("%.1f GiB", i.Wastage.RightSizing.Current.Memory),
		Average: utils.Percentage(i.Wastage.RightSizing.Memory.Avg),
		Max:     utils.Percentage(i.Wastage.RightSizing.Memory.Max),
	}
	ebsProperty := &golang.Property{
		Key:     "EBS Bandwidth",
		Current: fmt.Sprintf("%s", i.Wastage.RightSizing.Current.EBSBandwidth),
		Average: utils.PNetworkThroughputMbps(i.Wastage.RightSizing.EBSBandwidth.Avg),
		Max:     utils.PNetworkThroughputMbps(i.Wastage.RightSizing.EBSBandwidth.Max),
	}
	iopsProperty := &golang.Property{
		Key:     "EBS IOPS",
		Current: fmt.Sprintf("%s", i.Wastage.RightSizing.Current.EBSIops),
		Average: fmt.Sprintf("%s io/s", utils.PFloat64ToString(i.Wastage.RightSizing.EBSIops.Avg)),
		Max:     fmt.Sprintf("%s io/s", utils.PFloat64ToString(i.Wastage.RightSizing.EBSIops.Max)),
	}
	if i.Wastage.RightSizing.EBSIops.Avg == nil {
		iopsProperty.Average = ""
	}
	if i.Wastage.RightSizing.EBSIops.Max == nil {
		iopsProperty.Max = ""
	}

	netThroughputProperty := &golang.Property{
		Key:     "  Throughput",
		Current: fmt.Sprintf("%s", i.Wastage.RightSizing.Current.NetworkThroughput),
		Average: utils.PNetworkThroughputMbps(i.Wastage.RightSizing.NetworkThroughput.Avg),
		Max:     utils.PNetworkThroughputMbps(i.Wastage.RightSizing.NetworkThroughput.Max),
	}
	enaProperty := &golang.Property{
		Key:     "  ENA",
		Current: fmt.Sprintf("%s", i.Wastage.RightSizing.Current.ENASupported),
	}

	if i.Wastage.RightSizing.Recommended != nil {
		ec2Instance.RightSizedCost = i.Wastage.RightSizing.Recommended.Cost
		regionProperty.Recommended = i.Wastage.RightSizing.Recommended.Region
		instanceSizeProperty.Recommended = i.Wastage.RightSizing.Recommended.InstanceType
		vCPUProperty.Recommended = fmt.Sprintf("%d", i.Wastage.RightSizing.Recommended.VCPU)
		processorProperty.Recommended = i.Wastage.RightSizing.Recommended.Processor
		architectureProperty.Recommended = i.Wastage.RightSizing.Recommended.Architecture
		licenseCostProperty.Recommended = fmt.Sprintf("$%.2f", i.Wastage.RightSizing.Recommended.LicensePrice)
		memoryProperty.Recommended = fmt.Sprintf("%.1f GiB", i.Wastage.RightSizing.Recommended.Memory)
		ebsProperty.Recommended = i.Wastage.RightSizing.Recommended.EBSBandwidth
		iopsProperty.Recommended = i.Wastage.RightSizing.Recommended.EBSIops
		netThroughputProperty.Recommended = i.Wastage.RightSizing.Recommended.NetworkThroughput
		enaProperty.Recommended = i.Wastage.RightSizing.Recommended.ENASupported
	}
	ec2Instance.Properties = append(ec2Instance.Properties, regionProperty)
	ec2Instance.Properties = append(ec2Instance.Properties, instanceSizeProperty)
	ec2Instance.Properties = append(ec2Instance.Properties, &golang.Property{
		Key: "Compute",
	})
	ec2Instance.Properties = append(ec2Instance.Properties, vCPUProperty)
	ec2Instance.Properties = append(ec2Instance.Properties, processorProperty)
	ec2Instance.Properties = append(ec2Instance.Properties, architectureProperty)
	ec2Instance.Properties = append(ec2Instance.Properties, licenseCostProperty)
	ec2Instance.Properties = append(ec2Instance.Properties, memoryProperty)
	ec2Instance.Properties = append(ec2Instance.Properties, ebsProperty)
	ec2Instance.Properties = append(ec2Instance.Properties, iopsProperty)
	ec2Instance.Properties = append(ec2Instance.Properties, &golang.Property{
		Key: "Network Performance",
	})
	ec2Instance.Properties = append(ec2Instance.Properties, netThroughputProperty)
	ec2Instance.Properties = append(ec2Instance.Properties, enaProperty)

	return ec2Instance
}

func (i EC2InstanceItem) EBSVolumeDevice(v types.Volume, vs kaytu.EBSVolumeRecommendation) *golang.Device {
	volume := &golang.Device{
		Properties:   nil,
		DeviceId:     *v.VolumeId,
		ResourceType: "EBS Volume",
		Runtime:      "730 hours",
		CurrentCost:  vs.Current.Cost,
	}
	storageTierProp := &golang.Property{
		Key:     "  EBS Storage Tier",
		Current: string(vs.Current.Tier),
	}
	volumeSizeProp := &golang.Property{
		Key:     "  Volume Size (GB)",
		Current: utils.SizeByteToGB(vs.Current.VolumeSize),
	}
	iopsProp := &golang.Property{
		Key:     "IOPS",
		Current: fmt.Sprintf("%d", vs.Current.IOPS()),
		Average: utils.PFloat64ToString(vs.IOPS.Avg),
		Max:     utils.PFloat64ToString(vs.IOPS.Max),
	}
	baselineIOPSProp := &golang.Property{
		Key:     "  Baseline IOPS",
		Current: fmt.Sprintf("%d", vs.Current.BaselineIOPS),
	}
	provisionedIOPSProp := &golang.Property{
		Key:     "  Provisioned IOPS",
		Current: utils.PInt32ToString(vs.Current.ProvisionedIOPS),
	}
	throughputProp := &golang.Property{
		Key:     "Throughput (MB/s)",
		Current: fmt.Sprintf("%.2f", vs.Current.Throughput()),
		Average: utils.PNetworkThroughputMbps(vs.Throughput.Avg),
	}
	baselineThroughputProp := &golang.Property{
		Key:     "  Baseline Throughput",
		Current: utils.NetworkThroughputMbps(vs.Current.BaselineThroughput),
	}
	provisionedThroughputProp := &golang.Property{
		Key:     "  Provisioned Throughput",
		Current: utils.PNetworkThroughputMbps(vs.Current.ProvisionedThroughput),
	}

	if vs.Recommended != nil {
		volume.RightSizedCost = vs.Recommended.Cost
		storageTierProp.Recommended = string(vs.Recommended.Tier)
		volumeSizeProp.Recommended = utils.SizeByteToGB(vs.Recommended.VolumeSize)
		iopsProp.Recommended = fmt.Sprintf("%d", vs.Recommended.IOPS())
		baselineIOPSProp.Recommended = fmt.Sprintf("%d", vs.Recommended.BaselineIOPS)
		provisionedIOPSProp.Recommended = utils.PInt32ToString(vs.Recommended.ProvisionedIOPS)
		throughputProp.Recommended = fmt.Sprintf("%.2f", vs.Recommended.Throughput())
		baselineThroughputProp.Recommended = utils.NetworkThroughputMbps(vs.Recommended.BaselineThroughput)
		provisionedThroughputProp.Recommended = utils.PNetworkThroughputMbps(vs.Recommended.ProvisionedThroughput)
	}

	volume.Properties = append(volume.Properties, storageTierProp)
	volume.Properties = append(volume.Properties, volumeSizeProp)
	volume.Properties = append(volume.Properties, iopsProp)
	volume.Properties = append(volume.Properties, baselineIOPSProp)
	volume.Properties = append(volume.Properties, provisionedIOPSProp)
	volume.Properties = append(volume.Properties, throughputProp)
	volume.Properties = append(volume.Properties, baselineThroughputProp)
	volume.Properties = append(volume.Properties, provisionedThroughputProp)
	return volume
}

func (i EC2InstanceItem) Devices() []*golang.Device {
	var devices []*golang.Device
	devices = append(devices, i.EC2InstanceDevice())
	for _, v := range i.Volumes {
		vs, ok := i.Wastage.VolumeRightSizing[utils.HashString(*v.VolumeId)]
		if !ok {
			continue
		}

		devices = append(devices, i.EBSVolumeDevice(v, vs))
	}
	return devices
}

func (i EC2InstanceItem) ToOptimizationItem() *golang.OptimizationItem {
	oi := &golang.OptimizationItem{
		Id:                 *i.Instance.InstanceId,
		ResourceType:       string(i.Instance.InstanceType),
		Region:             i.Region,
		Devices:            i.Devices(),
		Preferences:        i.Preferences,
		Description:        i.Wastage.RightSizing.Description,
		Loading:            i.OptimizationLoading,
		Skipped:            i.Skipped,
		SkipReason:         i.SkipReason,
		LazyLoadingEnabled: i.LazyLoadingEnabled,
	}

	if i.Instance.PlatformDetails != nil {
		oi.Platform = *i.Instance.PlatformDetails
	}
	for _, t := range i.Instance.Tags {
		if t.Key != nil && strings.ToLower(*t.Key) == "name" && t.Value != nil {
			oi.Name = *t.Value
		}
	}
	if oi.Name == "" {
		oi.Name = *i.Instance.InstanceId
	}

	return oi
}
