package ec2_instance

import (
	"fmt"
	types2 "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/utils"
	"github.com/opengovern/plugin-aws/plugin/processor/shared"
	golang2 "github.com/opengovern/plugin-aws/plugin/proto/src/golang"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"sort"
	"strings"
)

type EC2InstanceItem struct {
	Instance            types.Instance
	Image               *types.Image
	Region              string
	OptimizationLoading bool
	Preferences         []*golang.PreferenceItem
	Skipped             bool
	LazyLoadingEnabled  bool
	SkipReason          string
	Volumes             []types.Volume
	Metrics             map[string][]types2.Datapoint
	VolumeMetrics       map[string]map[string][]types2.Datapoint
	Wastage             *golang2.EC2InstanceOptimizationResponse
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
		Current: fmt.Sprintf("%d", i.Wastage.RightSizing.Current.Vcpu),
		Average: utils.Percentage(shared.WrappedToFloat64(i.Wastage.RightSizing.Vcpu.Avg)),
		Max:     utils.Percentage(shared.WrappedToFloat64(i.Wastage.RightSizing.Vcpu.Max)),
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
		Average: utils.Percentage(shared.WrappedToFloat64(i.Wastage.RightSizing.Memory.Avg)),
		Max:     utils.Percentage(shared.WrappedToFloat64(i.Wastage.RightSizing.Memory.Max)),
	}
	ebsProperty := &golang.Property{
		Key:     "EBS Bandwidth",
		Current: fmt.Sprintf("%s", i.Wastage.RightSizing.Current.EbsBandwidth),
		Average: PNetworkThroughputMBps(shared.WrappedToFloat64(i.Wastage.RightSizing.EbsBandwidth.Avg)),
		Max:     PNetworkThroughputMBps(shared.WrappedToFloat64(i.Wastage.RightSizing.EbsBandwidth.Max)),
	}
	iopsProperty := &golang.Property{
		Key:     "EBS IOPS",
		Current: fmt.Sprintf("%s", i.Wastage.RightSizing.Current.EbsIops),
		Average: fmt.Sprintf("%s io/s", utils.PFloat64ToString(shared.WrappedToFloat64(i.Wastage.RightSizing.EbsIops.Avg))),
		Max:     fmt.Sprintf("%s io/s", utils.PFloat64ToString(shared.WrappedToFloat64(i.Wastage.RightSizing.EbsIops.Max))),
	}
	if i.Wastage.RightSizing.EbsIops.Avg == nil {
		iopsProperty.Average = ""
	}
	if i.Wastage.RightSizing.EbsIops.Max == nil {
		iopsProperty.Max = ""
	}

	netThroughputProperty := &golang.Property{
		Key:     "  Throughput",
		Current: fmt.Sprintf("%s", i.Wastage.RightSizing.Current.NetworkThroughput),
		Average: utils.PNetworkThroughputMbps(shared.WrappedToFloat64(i.Wastage.RightSizing.NetworkThroughput.Avg)),
		Max:     utils.PNetworkThroughputMbps(shared.WrappedToFloat64(i.Wastage.RightSizing.NetworkThroughput.Max)),
	}
	enaProperty := &golang.Property{
		Key:     "  ENASupportChangeInInstanceType",
		Current: fmt.Sprintf("%s", i.Wastage.RightSizing.Current.EnaSupported),
	}

	costComponentPropertiesMap := make(map[string]*golang.Property)
	for k, v := range i.Wastage.RightSizing.Current.CostComponents {
		costComponentPropertiesMap[k] = &golang.Property{
			Key:     fmt.Sprintf("  %s", k),
			Current: fmt.Sprintf("$%.2f", v),
		}
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
		vCPUProperty.Recommended = fmt.Sprintf("%d", i.Wastage.RightSizing.Recommended.Vcpu)
		processorProperty.Recommended = i.Wastage.RightSizing.Recommended.Processor
		architectureProperty.Recommended = i.Wastage.RightSizing.Recommended.Architecture
		licenseCostProperty.Recommended = fmt.Sprintf("$%.2f", i.Wastage.RightSizing.Recommended.LicensePrice)
		memoryProperty.Recommended = fmt.Sprintf("%.1f GiB", i.Wastage.RightSizing.Recommended.Memory)
		ebsProperty.Recommended = i.Wastage.RightSizing.Recommended.EbsBandwidth
		iopsProperty.Recommended = i.Wastage.RightSizing.Recommended.EbsIops
		netThroughputProperty.Recommended = i.Wastage.RightSizing.Recommended.NetworkThroughput
		enaProperty.Recommended = i.Wastage.RightSizing.Recommended.EnaSupported
		for k, v := range i.Wastage.RightSizing.Recommended.CostComponents {
			if _, ok := costComponentPropertiesMap[k]; !ok {
				costComponentPropertiesMap[k] = &golang.Property{
					Key: fmt.Sprintf("  %s", k),
				}
			}
			costComponentPropertiesMap[k].Recommended = fmt.Sprintf("$%.2f", v)
		}
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

	if i.Image != nil && i.Image.EnaSupport != nil {
		enaSupported := "No"
		if *i.Image.EnaSupport {
			enaSupported = "Yes"
		}
		properties.Properties = append(properties.Properties, &golang.Property{
			Key:     "  ENASupportedByAMI",
			Current: enaSupported,
		})
	}

	costComponentProperties := make([]*golang.Property, 0, len(costComponentPropertiesMap))
	for _, v := range costComponentPropertiesMap {
		costComponentProperties = append(costComponentProperties, v)
	}
	sort.Slice(costComponentProperties, func(i, j int) bool {
		return costComponentProperties[i].Key < costComponentProperties[j].Key
	})
	properties.Properties = append(properties.Properties, &golang.Property{
		Key: "Cost Components",
	})
	properties.Properties = append(properties.Properties, costComponentProperties...)

	props[*i.Instance.InstanceId] = properties

	return &row, props
}

func (i EC2InstanceItem) EBSVolumeDevice(v types.Volume, vs *golang2.EBSVolumeRecommendation) (*golang.ChartRow, map[string]*golang.Properties) {
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
		Current: utils.SizeByteToGB(shared.WrappedToInt32(vs.Current.VolumeSize)),
	}
	iopsProp := &golang.Property{
		Key:     "IOPS",
		Current: fmt.Sprintf("%d", getRightsizingEBSVolumeIOPS(vs.Current)),
		Average: utils.PFloat64ToString(shared.WrappedToFloat64(vs.Iops.Avg)),
		Max:     utils.PFloat64ToString(shared.WrappedToFloat64(vs.Iops.Max)),
	}
	baselineIOPSProp := &golang.Property{
		Key:     "  Baseline IOPS",
		Current: fmt.Sprintf("%d", vs.Current.BaselineIops),
	}
	provisionedIOPSProp := &golang.Property{
		Key:     "  Provisioned IOPS",
		Current: utils.PInt32ToString(shared.WrappedToInt32(vs.Current.ProvisionedIops)),
	}
	throughputProp := &golang.Property{
		Key:     "Throughput (MB/s)",
		Current: fmt.Sprintf("%.2f", getRightsizingEBSVolumeThroughput(vs.Current)),
		Average: PNetworkThroughputMBps(shared.WrappedToFloat64(vs.Throughput.Avg)),
	}
	baselineThroughputProp := &golang.Property{
		Key:     "  Baseline Throughput",
		Current: PNetworkThroughputMBps(&vs.Current.BaselineThroughput),
	}
	provisionedThroughputProp := &golang.Property{
		Key:     "  Provisioned Throughput",
		Current: PNetworkThroughputMBps(shared.WrappedToFloat64(vs.Current.ProvisionedThroughput)),
	}
	costComponentPropertiesMap := make(map[string]*golang.Property)
	for k, vv := range vs.Current.CostComponents {
		costComponentPropertiesMap[k] = &golang.Property{
			Key:     fmt.Sprintf("  %s", k),
			Current: fmt.Sprintf("$%.2f", vv),
		}
	}

	if vs.Recommended != nil {
		row.Values["right_sized_cost"] = &golang.ChartRowItem{
			Value: utils.FormatPriceFloat(vs.Recommended.Cost),
		}
		row.Values["savings"] = &golang.ChartRowItem{
			Value: utils.FormatPriceFloat(vs.Current.Cost - vs.Recommended.Cost),
		}
		storageTierProp.Recommended = vs.Recommended.Tier
		volumeSizeProp.Recommended = utils.SizeByteToGB(shared.WrappedToInt32(vs.Recommended.VolumeSize))
		iopsProp.Recommended = fmt.Sprintf("%d", getRightsizingEBSVolumeIOPS(vs.Recommended))
		baselineIOPSProp.Recommended = fmt.Sprintf("%d", vs.Recommended.BaselineIops)
		provisionedIOPSProp.Recommended = utils.PInt32ToString(shared.WrappedToInt32(vs.Recommended.ProvisionedIops))
		throughputProp.Recommended = fmt.Sprintf("%.2f", getRightsizingEBSVolumeThroughput(vs.Recommended))
		baselineThroughputProp.Recommended = utils.NetworkThroughputMbps(vs.Recommended.BaselineThroughput)
		provisionedThroughputProp.Recommended = utils.PNetworkThroughputMbps(shared.WrappedToFloat64(vs.Recommended.ProvisionedThroughput))
		for k, vv := range vs.Recommended.CostComponents {
			if _, ok := costComponentPropertiesMap[k]; !ok {
				costComponentPropertiesMap[k] = &golang.Property{
					Key: fmt.Sprintf("  %s", k),
				}
			}
			costComponentPropertiesMap[k].Recommended = fmt.Sprintf("$%.2f", vv)
		}
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
	properties.Properties = append(properties.Properties, volumeTypeModification)
	properties.Properties = append(properties.Properties, volumeSizeModification)

	costComponentProperties := make([]*golang.Property, 0, len(costComponentPropertiesMap))
	for _, vv := range costComponentPropertiesMap {
		costComponentProperties = append(costComponentProperties, vv)
	}
	sort.Slice(costComponentProperties, func(i, j int) bool {
		return costComponentProperties[i].Key < costComponentProperties[j].Key
	})
	properties.Properties = append(properties.Properties, &golang.Property{
		Key: "Cost Components",
	})
	properties.Properties = append(properties.Properties, costComponentProperties...)

	properties.Properties = append(properties.Properties, &golang.Property{
		Key:         "Description",
		Recommended: vs.Description,
	})

	props[*v.VolumeId] = properties

	return &row, props
}

func (i EC2InstanceItem) Devices() ([]*golang.ChartRow, map[string]*golang.Properties) {
	var deviceRows []*golang.ChartRow
	deviceProps := make(map[string]*golang.Properties)

	if i.Wastage != nil {
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
		Loading:            i.OptimizationLoading,
		Skipped:            i.Skipped,
		SkipReason:         nil,
		LazyLoadingEnabled: i.LazyLoadingEnabled,
	}
	if i.SkipReason != "" {
		oi.SkipReason = &wrapperspb.StringValue{Value: i.SkipReason}
	}
	if i.Wastage != nil && i.Wastage.RightSizing != nil {
		oi.Description = i.Wastage.RightSizing.Description
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

func getRightsizingEBSVolumeIOPS(v *golang2.RightsizingEBSVolume) int32 {
	val := v.BaselineIops
	if v.ProvisionedIops != nil {
		val += v.ProvisionedIops.GetValue()
	}
	return val
}

func getRightsizingEBSVolumeThroughput(v *golang2.RightsizingEBSVolume) float64 {
	val := v.BaselineThroughput
	if v.ProvisionedThroughput != nil {
		val += v.ProvisionedThroughput.GetValue()
	}
	return val
}
