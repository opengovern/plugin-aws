package ec2_instance

import (
	"fmt"
	types2 "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/utils"
	"github.com/kaytu-io/plugin-aws/plugin/kaytu"
	"google.golang.org/protobuf/types/known/wrapperspb"
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

func (i EC2InstanceItem) EC2InstanceDevice() (*golang.ChartRow, map[string]*golang.Properties) {
	var name string
	for _, t := range i.Instance.Tags {
		if t.Key != nil && strings.ToLower(*t.Key) == "name" && t.Value != nil {
			name = *t.Value
		}
	}
	if name == "" {
		name = *i.Instance.InstanceId
	}

	row := golang.ChartRow{
		RowId:  *i.Instance.InstanceId,
		Values: make(map[string]*golang.ChartRowItem),
	}
	row.RowId = *i.Instance.InstanceId
	row.Values["resource_id"] = &golang.ChartRowItem{
		Value: *i.Instance.InstanceId,
	}
	row.Values["resource_name"] = &golang.ChartRowItem{
		Value: name,
	}
	row.Values["resource_type"] = &golang.ChartRowItem{
		Value: "EC2 Instance",
	}
	row.Values["runtime"] = &golang.ChartRowItem{
		Value: "730 hours",
	}
	row.Values["current_cost"] = &golang.ChartRowItem{
		Value: utils.FormatPriceFloat(i.Wastage.RightSizing.Current.Cost),
	}

	props := make(map[string]*golang.Properties)
	properties := &golang.Properties{}

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
		Average: PNetworkThroughputMBps(i.Wastage.RightSizing.EBSBandwidth.Avg),
		Max:     PNetworkThroughputMBps(i.Wastage.RightSizing.EBSBandwidth.Max),
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
		Key:     "  ENA Support",
		Current: fmt.Sprintf("%s", i.Wastage.RightSizing.Current.ENASupported),
	}

	if i.Wastage.RightSizing.Recommended != nil {
		row.Values["right_sized_cost"] = &golang.ChartRowItem{
			Value: utils.FormatPriceFloat(i.Wastage.RightSizing.Recommended.Cost),
		}
		row.Values["savings"] = &golang.ChartRowItem{
			Value: utils.FormatPriceFloat(i.Wastage.RightSizing.Current.Cost - i.Wastage.RightSizing.Recommended.Cost),
		}
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
	properties.Properties = append(properties.Properties, regionProperty)
	properties.Properties = append(properties.Properties, instanceSizeProperty)
	properties.Properties = append(properties.Properties, &golang.Property{
		Key: "Compute",
	})
	properties.Properties = append(properties.Properties, vCPUProperty)
	properties.Properties = append(properties.Properties, processorProperty)
	properties.Properties = append(properties.Properties, architectureProperty)
	properties.Properties = append(properties.Properties, licenseCostProperty)
	properties.Properties = append(properties.Properties, memoryProperty)
	properties.Properties = append(properties.Properties, ebsProperty)
	properties.Properties = append(properties.Properties, iopsProperty)
	properties.Properties = append(properties.Properties, &golang.Property{
		Key: "Network Performance",
	})
	properties.Properties = append(properties.Properties, netThroughputProperty)
	properties.Properties = append(properties.Properties, enaProperty)
	props[*i.Instance.InstanceId] = properties

	return &row, props
}

func (i EC2InstanceItem) EBSVolumeDevice(v types.Volume, vs kaytu.EBSVolumeRecommendation) (*golang.ChartRow, map[string]*golang.Properties) {
	var name string
	for _, t := range i.Instance.Tags {
		if t.Key != nil && strings.ToLower(*t.Key) == "name" && t.Value != nil {
			name = *t.Value
		}
	}
	if name == "" {
		name = *v.VolumeId
	}

	row := golang.ChartRow{
		RowId:  *v.VolumeId,
		Values: make(map[string]*golang.ChartRowItem),
	}
	row.RowId = *v.VolumeId
	row.Values["resource_id"] = &golang.ChartRowItem{
		Value: *v.VolumeId,
	}
	row.Values["resource_name"] = &golang.ChartRowItem{
		Value: name,
	}
	row.Values["resource_type"] = &golang.ChartRowItem{
		Value: "EBS Volume",
	}
	row.Values["runtime"] = &golang.ChartRowItem{
		Value: "730 hours",
	}
	row.Values["current_cost"] = &golang.ChartRowItem{
		Value: utils.FormatPriceFloat(vs.Current.Cost),
	}

	props := make(map[string]*golang.Properties)
	properties := &golang.Properties{}

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
		Average: PNetworkThroughputMBps(vs.Throughput.Avg),
	}
	baselineThroughputProp := &golang.Property{
		Key:     "  Baseline Throughput",
		Current: PNetworkThroughputMBps(&vs.Current.BaselineThroughput),
	}
	provisionedThroughputProp := &golang.Property{
		Key:     "  Provisioned Throughput",
		Current: PNetworkThroughputMBps(vs.Current.ProvisionedThroughput),
	}

	if vs.Recommended != nil {
		row.Values["right_sized_cost"] = &golang.ChartRowItem{
			Value: utils.FormatPriceFloat(vs.Recommended.Cost),
		}
		row.Values["savings"] = &golang.ChartRowItem{
			Value: utils.FormatPriceFloat(vs.Current.Cost - vs.Recommended.Cost),
		}
		storageTierProp.Recommended = string(vs.Recommended.Tier)
		volumeSizeProp.Recommended = utils.SizeByteToGB(vs.Recommended.VolumeSize)
		iopsProp.Recommended = fmt.Sprintf("%d", vs.Recommended.IOPS())
		baselineIOPSProp.Recommended = fmt.Sprintf("%d", vs.Recommended.BaselineIOPS)
		provisionedIOPSProp.Recommended = utils.PInt32ToString(vs.Recommended.ProvisionedIOPS)
		throughputProp.Recommended = fmt.Sprintf("%.2f", vs.Recommended.Throughput())
		baselineThroughputProp.Recommended = utils.NetworkThroughputMbps(vs.Recommended.BaselineThroughput)
		provisionedThroughputProp.Recommended = utils.PNetworkThroughputMbps(vs.Recommended.ProvisionedThroughput)
	}

	volumeTypeModification := &golang.Property{
		Key:         "Volume Type Modification",
		Recommended: "No",
	}
	if storageTierProp.Current != storageTierProp.Recommended {
		volumeTypeModification.Recommended = "Yes"
	}
	volumeSizeModification := &golang.Property{
		Key:         "Volume Size Modification",
		Recommended: "No",
	}
	if volumeSizeProp.Current != volumeSizeProp.Recommended {
		volumeSizeModification.Recommended = "Yes"
	}

	properties.Properties = append(properties.Properties, storageTierProp)
	properties.Properties = append(properties.Properties, volumeSizeProp)
	properties.Properties = append(properties.Properties, iopsProp)
	properties.Properties = append(properties.Properties, baselineIOPSProp)
	properties.Properties = append(properties.Properties, provisionedIOPSProp)
	properties.Properties = append(properties.Properties, throughputProp)
	properties.Properties = append(properties.Properties, baselineThroughputProp)
	properties.Properties = append(properties.Properties, provisionedThroughputProp)
	props[*v.VolumeId] = properties

	return &row, props
}

func (i EC2InstanceItem) Devices() ([]*golang.ChartRow, map[string]*golang.Properties) {
	var deviceRows []*golang.ChartRow
	deviceProps := make(map[string]*golang.Properties)
	ec2Rows, ec2Props := i.EC2InstanceDevice()
	deviceRows = append(deviceRows, ec2Rows)
	for k, v := range ec2Props {
		deviceProps[k] = v
	}
	for _, v := range i.Volumes {
		vs, ok := i.Wastage.VolumeRightSizing[utils.HashString(*v.VolumeId)]
		if !ok {
			continue
		}

		ebsRows, ebsProps := i.EBSVolumeDevice(v, vs)

		deviceRows = append(deviceRows, ebsRows)
		for k, val := range ebsProps {
			deviceProps[k] = val
		}
	}
	return deviceRows, deviceProps
}

func (i EC2InstanceItem) ToOptimizationItem() *golang.ChartOptimizationItem {
	var name, platform string
	for _, t := range i.Instance.Tags {
		if t.Key != nil && strings.ToLower(*t.Key) == "name" && t.Value != nil {
			name = *t.Value
		}
	}
	if name == "" {
		name = *i.Instance.InstanceId
	}
	if i.Instance.PlatformDetails != nil {
		platform = *i.Instance.PlatformDetails
	}

	deviceRows, deviceProps := i.Devices()

	status := ""
	if i.Skipped {
		status = fmt.Sprintf("skipped - %s", i.SkipReason)
	} else if i.LazyLoadingEnabled && !i.OptimizationLoading {
		status = "press enter to load"
	} else if i.OptimizationLoading {
		status = "loading"
	} else if i.Wastage.RightSizing.Recommended != nil {
		totalSaving := 0.0
		totalCurrentCost := 0.0
		for _, v := range i.Wastage.VolumeRightSizing {
			totalSaving += v.Current.Cost - v.Recommended.Cost
			totalCurrentCost += v.Current.Cost
		}
		totalSaving += i.Wastage.RightSizing.Current.Cost - i.Wastage.RightSizing.Recommended.Cost
		totalCurrentCost += i.Wastage.RightSizing.Current.Cost
		status = fmt.Sprintf("%s (%.2f%%)", utils.FormatPriceFloat(totalSaving), (totalSaving/totalCurrentCost)*100)
	}

	oi := &golang.ChartOptimizationItem{
		OverviewChartRow: &golang.ChartRow{
			RowId: *i.Instance.InstanceId,
			Values: map[string]*golang.ChartRowItem{
				"x_kaytu_right_arrow": {
					Value: "→",
				},
				"resource_id": {
					Value: *i.Instance.InstanceId,
				},
				"resource_name": {
					Value: name,
				},
				"resource_type": {
					Value: string(i.Instance.InstanceType),
				},
				"region": {
					Value: i.Region,
				},
				"platform": {
					Value: platform,
				},
				"total_saving": {
					Value: status,
				},
			},
		},
		DevicesChartRows:   deviceRows,
		DevicesProperties:  deviceProps,
		Preferences:        i.Preferences,
		Description:        i.Wastage.RightSizing.Description,
		Loading:            i.OptimizationLoading,
		Skipped:            i.Skipped,
		SkipReason:         nil,
		LazyLoadingEnabled: i.LazyLoadingEnabled,
	}
	if i.SkipReason != "" {
		oi.SkipReason = &wrapperspb.StringValue{Value: i.SkipReason}
	}

	return oi
}

func PNetworkThroughputMBps(v *float64) string {
	if v == nil {
		return ""
	}
	vv := *v / (1024 * 1024)
	return fmt.Sprintf("%.2f MB/s", vv)
}
