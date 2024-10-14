package rds_instance

import (
	"fmt"
	types2 "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/utils"
	"github.com/opengovern/plugin-aws/plugin/processor/shared"
	golang2 "github.com/opengovern/plugin-aws/plugin/proto/src/golang"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"sort"
	"strings"
	"time"
)

type RDSInstanceItem struct {
	Instance            types.DBInstance
	Region              string
	OptimizationLoading bool
	Preferences         []*golang.PreferenceItem
	Skipped             bool
	LazyLoadingEnabled  bool
	SkipReason          string

	Metrics map[string][]types2.Datapoint
	Wastage *golang2.RDSInstanceOptimizationResponse
}

func (i RDSInstanceItem) RDSInstanceDevice() ([]*golang.ChartRow, map[string]*golang.Properties) {
	props := make(map[string]*golang.Properties)
	computeProps := &golang.Properties{}
	storageProps := &golang.Properties{}

	computeRow := golang.ChartRow{
		RowId:  fmt.Sprintf("%s-compute", *i.Instance.DBInstanceIdentifier),
		Values: make(map[string]*golang.ChartRowItem),
	}
	computeRow.Values["resource_id"] = &golang.ChartRowItem{
		Value: fmt.Sprintf("%s-compute", *i.Instance.DBInstanceIdentifier),
	}
	computeRow.Values["resource_name"] = &golang.ChartRowItem{
		Value: *i.Instance.DBInstanceIdentifier,
	}
	computeRow.Values["resource_type"] = &golang.ChartRowItem{
		Value: "RDS Instance Compute",
	}
	computeRow.Values["runtime"] = &golang.ChartRowItem{
		Value: "730 hours",
	}
	computeRow.Values["current_cost"] = &golang.ChartRowItem{
		Value: utils.FormatPriceFloat(i.Wastage.RightSizing.Current.ComputeCost),
	}

	storageRow := golang.ChartRow{
		RowId:  fmt.Sprintf("%s-storage", *i.Instance.DBInstanceIdentifier),
		Values: make(map[string]*golang.ChartRowItem),
	}
	storageRow.Values["resource_id"] = &golang.ChartRowItem{
		Value: fmt.Sprintf("%s-storage", *i.Instance.DBInstanceIdentifier),
	}
	storageRow.Values["resource_name"] = &golang.ChartRowItem{
		Value: *i.Instance.DBInstanceIdentifier,
	}
	storageRow.Values["resource_type"] = &golang.ChartRowItem{
		Value: "RDS Instance Storage",
	}
	storageRow.Values["runtime"] = &golang.ChartRowItem{
		Value: "730 hours",
	}
	storageRow.Values["current_cost"] = &golang.ChartRowItem{
		Value: utils.FormatPriceFloat(i.Wastage.RightSizing.Current.StorageCost),
	}

	regionProperty := &golang.Property{
		Key:     "Region",
		Current: i.Wastage.RightSizing.Current.Region,
	}
	instanceSizeProperty := &golang.Property{
		Key:     "Instance Size",
		Current: i.Wastage.RightSizing.Current.InstanceType,
	}
	engineProperty := &golang.Property{
		Key:     "Engine",
		Current: i.Wastage.RightSizing.Current.Engine,
	}
	engineVerProperty := &golang.Property{
		Key:     "Engine Version",
		Current: i.Wastage.RightSizing.Current.EngineVersion,
	}
	clusterTypeProperty := &golang.Property{
		Key:     "Cluster Type",
		Current: string(i.Wastage.RightSizing.Current.ClusterType),
	}
	vCPUProperty := &golang.Property{
		Key:     "vCPU",
		Current: fmt.Sprintf("%d", i.Wastage.RightSizing.Current.Vcpu),
		Average: utils.Percentage(shared.WrappedToFloat64(i.Wastage.RightSizing.Vcpu.Avg)),
		Max:     utils.Percentage(shared.WrappedToFloat64(i.Wastage.RightSizing.Vcpu.Max)),
	}
	processorProperty := &golang.Property{
		Key:     "Processor(s)",
		Current: i.Wastage.RightSizing.Current.Processor,
	}
	architectureProperty := &golang.Property{
		Key:     "Architecture",
		Current: i.Wastage.RightSizing.Current.Architecture,
	}
	memoryProperty := &golang.Property{
		Key:     "Memory",
		Current: fmt.Sprintf("%d GiB", i.Wastage.RightSizing.Current.MemoryGb),
		Average: utils.MemoryUsagePercentageByFreeSpace(shared.WrappedToFloat64(i.Wastage.RightSizing.FreeMemoryBytes.Avg), float64(i.Wastage.RightSizing.Current.MemoryGb)),
	}
	storageTypeProperty := &golang.Property{
		Key:     "Type",
		Current: utils.PString(shared.WrappedToString(i.Wastage.RightSizing.Current.StorageType)),
	}
	storageSizeProperty := &golang.Property{
		Key:     "Size",
		Current: utils.SizeByteToGB(shared.WrappedToInt32(i.Wastage.RightSizing.Current.StorageSize)),
		Average: utils.StorageUsagePercentageByFreeSpace(shared.WrappedToFloat64(i.Wastage.RightSizing.FreeStorageBytes.Avg), shared.WrappedToInt32(i.Wastage.RightSizing.Current.StorageSize)),
		Max:     utils.StorageUsagePercentageByFreeSpace(shared.WrappedToFloat64(i.Wastage.RightSizing.FreeStorageBytes.Min), shared.WrappedToInt32(i.Wastage.RightSizing.Current.StorageSize)),
	}
	if strings.Contains(strings.ToLower(i.Wastage.RightSizing.Current.Engine), "aurora") {
		avgPercentage := (*shared.WrappedToFloat64(i.Wastage.RightSizing.VolumeBytesUsed.Avg) / (1024.0 * 1024.0 * 1024.0)) / float64(*shared.WrappedToInt32(i.Wastage.RightSizing.Current.StorageSize)) * 100
		maxPercentage := (*shared.WrappedToFloat64(i.Wastage.RightSizing.VolumeBytesUsed.Max) / (1024.0 * 1024.0 * 1024.0)) / float64(*shared.WrappedToInt32(i.Wastage.RightSizing.Current.StorageSize)) * 100
		storageSizeProperty.Average = utils.Percentage(&avgPercentage)
		storageSizeProperty.Max = utils.Percentage(&maxPercentage)
	}
	storageIOPSProperty := &golang.Property{
		Key:     "IOPS",
		Current: utils.PInt32ToString(shared.WrappedToInt32(i.Wastage.RightSizing.Current.StorageIops)),
		Average: fmt.Sprintf("%s io/s", utils.PFloat64ToString(shared.WrappedToFloat64(i.Wastage.RightSizing.StorageIops.Avg))),
		Max:     fmt.Sprintf("%s io/s", utils.PFloat64ToString(shared.WrappedToFloat64(i.Wastage.RightSizing.StorageIops.Max))),
	}
	if storageIOPSProperty.Current != "" {
		storageIOPSProperty.Current = fmt.Sprintf("%s io/s", storageIOPSProperty.Current)
	} else {
		storageIOPSProperty.Current = ""
	}
	if i.Wastage.RightSizing.StorageIops.Avg == nil {
		storageIOPSProperty.Average = ""
	}
	if i.Wastage.RightSizing.StorageIops.Max == nil {
		storageIOPSProperty.Max = ""
	}
	// current number is in MB/s, so we need to convert it to bytes/s so matches the other values
	if i.Wastage.RightSizing.Current.StorageThroughput != nil {
		v := *shared.WrappedToFloat64(i.Wastage.RightSizing.Current.StorageThroughput) * 1024.0 * 1024.0
		i.Wastage.RightSizing.Current.StorageThroughput = shared.Float64ToWrapper(&v)
	}
	storageThroughputProperty := &golang.Property{
		Key:     "Throughput",
		Current: utils.PStorageThroughputMbps(shared.WrappedToFloat64(i.Wastage.RightSizing.Current.StorageThroughput)),
		Average: utils.PStorageThroughputMbps(shared.WrappedToFloat64(i.Wastage.RightSizing.StorageThroughput.Avg)),
		Max:     utils.PStorageThroughputMbps(shared.WrappedToFloat64(i.Wastage.RightSizing.StorageThroughput.Max)),
	}
	runtimeProperty := &golang.Property{
		Key:     "RuntimeHours",
		Current: fmt.Sprintf("%.0f", time.Now().Sub(*i.Instance.InstanceCreateTime).Hours()),
		Hidden:  true,
	}

	computeCostComponentPropertiesMap := make(map[string]*golang.Property)
	for k, v := range i.Wastage.RightSizing.Current.ComputeCostComponents {
		computeCostComponentPropertiesMap[k] = &golang.Property{
			Key:     fmt.Sprintf("  %s", k),
			Current: fmt.Sprintf("$%.2f", v),
		}
	}
	storageCostComponentPropertiesMap := make(map[string]*golang.Property)
	for k, v := range i.Wastage.RightSizing.Current.StorageCostComponents {
		storageCostComponentPropertiesMap[k] = &golang.Property{
			Key:     fmt.Sprintf("  %s", k),
			Current: fmt.Sprintf("$%.2f", v),
		}
	}

	if i.Wastage.RightSizing.Recommended != nil {
		processorProperty.Recommended = i.Wastage.RightSizing.Recommended.Processor
		architectureProperty.Recommended = i.Wastage.RightSizing.Recommended.Architecture
		computeRow.Values["right_sized_cost"] = &golang.ChartRowItem{
			Value: utils.FormatPriceFloat(i.Wastage.RightSizing.Recommended.ComputeCost),
		}
		computeRow.Values["savings"] = &golang.ChartRowItem{
			Value: utils.FormatPriceFloat(i.Wastage.RightSizing.Current.ComputeCost - i.Wastage.RightSizing.Recommended.ComputeCost),
		}
		storageRow.Values["right_sized_cost"] = &golang.ChartRowItem{
			Value: utils.FormatPriceFloat(i.Wastage.RightSizing.Recommended.StorageCost),
		}
		storageRow.Values["savings"] = &golang.ChartRowItem{
			Value: utils.FormatPriceFloat(i.Wastage.RightSizing.Current.StorageCost - i.Wastage.RightSizing.Recommended.StorageCost),
		}
		regionProperty.Recommended = i.Wastage.RightSizing.Recommended.Region
		instanceSizeProperty.Recommended = i.Wastage.RightSizing.Recommended.InstanceType
		engineProperty.Recommended = i.Wastage.RightSizing.Recommended.Engine
		engineVerProperty.Recommended = i.Wastage.RightSizing.Recommended.EngineVersion
		clusterTypeProperty.Recommended = string(i.Wastage.RightSizing.Recommended.ClusterType)
		vCPUProperty.Recommended = fmt.Sprintf("%d", i.Wastage.RightSizing.Recommended.Vcpu)
		memoryProperty.Recommended = fmt.Sprintf("%d GiB", i.Wastage.RightSizing.Recommended.MemoryGb)
		storageTypeProperty.Recommended = utils.PString(shared.WrappedToString(i.Wastage.RightSizing.Recommended.StorageType))
		storageSizeProperty.Recommended = utils.SizeByteToGB(shared.WrappedToInt32(i.Wastage.RightSizing.Recommended.StorageSize))
		storageIOPSProperty.Recommended = utils.PInt32ToString(shared.WrappedToInt32(i.Wastage.RightSizing.Recommended.StorageIops))
		if storageIOPSProperty.Recommended != "" {
			storageIOPSProperty.Recommended = fmt.Sprintf("%s io/s", storageIOPSProperty.Recommended)
		} else {
			storageIOPSProperty.Recommended = "N/A"
		}
		// Recommended number is in MB/s, so we need to convert it to bytes/s so matches the other values
		if i.Wastage.RightSizing.Recommended.StorageThroughput != nil {
			v := *shared.WrappedToFloat64(i.Wastage.RightSizing.Recommended.StorageThroughput) * 1024.0 * 1024.0
			i.Wastage.RightSizing.Recommended.StorageThroughput = shared.Float64ToWrapper(&v)
		}
		storageThroughputProperty.Recommended = utils.PStorageThroughputMbps(shared.WrappedToFloat64(i.Wastage.RightSizing.Recommended.StorageThroughput))

		for k, v := range i.Wastage.RightSizing.Recommended.ComputeCostComponents {
			if _, ok := computeCostComponentPropertiesMap[k]; !ok {
				computeCostComponentPropertiesMap[k] = &golang.Property{
					Key:         fmt.Sprintf("  %s", k),
					Recommended: fmt.Sprintf("$%.2f", v),
				}
			} else {
				computeCostComponentPropertiesMap[k].Recommended = fmt.Sprintf("$%.2f", v)
			}
		}
		for k, v := range i.Wastage.RightSizing.Recommended.StorageCostComponents {
			if _, ok := storageCostComponentPropertiesMap[k]; !ok {
				storageCostComponentPropertiesMap[k] = &golang.Property{
					Key:         fmt.Sprintf("  %s", k),
					Recommended: fmt.Sprintf("$%.2f", v),
				}
			} else {
				storageCostComponentPropertiesMap[k].Recommended = fmt.Sprintf("$%.2f", v)
			}
		}
	}
	computeProps.Properties = append(computeProps.Properties, regionProperty)
	computeProps.Properties = append(computeProps.Properties, instanceSizeProperty)
	computeProps.Properties = append(computeProps.Properties, engineProperty)
	computeProps.Properties = append(computeProps.Properties, engineVerProperty)
	computeProps.Properties = append(computeProps.Properties, clusterTypeProperty)
	computeProps.Properties = append(computeProps.Properties, vCPUProperty)
	computeProps.Properties = append(computeProps.Properties, memoryProperty)
	computeProps.Properties = append(computeProps.Properties, processorProperty)
	computeProps.Properties = append(computeProps.Properties, architectureProperty)

	computeCostComponentProperties := make([]*golang.Property, 0, len(computeCostComponentPropertiesMap))
	for _, v := range computeCostComponentPropertiesMap {
		computeCostComponentProperties = append(computeCostComponentProperties, v)
	}
	sort.Slice(computeCostComponentProperties, func(i, j int) bool {
		return computeCostComponentProperties[i].Key < computeCostComponentProperties[j].Key
	})
	computeProps.Properties = append(computeProps.Properties, &golang.Property{
		Key: "Cost Components",
	})
	computeProps.Properties = append(computeProps.Properties, computeCostComponentProperties...)

	storageProps.Properties = append(storageProps.Properties, regionProperty)
	storageProps.Properties = append(storageProps.Properties, storageTypeProperty)
	storageProps.Properties = append(storageProps.Properties, storageSizeProperty)
	storageProps.Properties = append(storageProps.Properties, storageIOPSProperty)
	storageProps.Properties = append(storageProps.Properties, storageThroughputProperty)
	storageProps.Properties = append(storageProps.Properties, runtimeProperty)

	volumeTypeModification := &golang.Property{
		Key:         "Volume Type Modification",
		Recommended: "No",
	}
	if storageTypeProperty.Current != storageTypeProperty.Recommended {
		volumeTypeModification.Recommended = "Yes"
	}
	volumeSizeModification := &golang.Property{
		Key:         "Volume Size Modification",
		Recommended: "No",
	}
	if storageSizeProperty.Current != storageSizeProperty.Recommended {
		volumeSizeModification.Recommended = "Yes"
	}
	storageProps.Properties = append(storageProps.Properties, volumeTypeModification)
	storageProps.Properties = append(storageProps.Properties, volumeSizeModification)

	storageCostComponentProperties := make([]*golang.Property, 0, len(storageCostComponentPropertiesMap))
	for _, v := range storageCostComponentPropertiesMap {
		storageCostComponentProperties = append(storageCostComponentProperties, v)
	}
	sort.Slice(storageCostComponentProperties, func(i, j int) bool {
		return storageCostComponentProperties[i].Key < storageCostComponentProperties[j].Key
	})
	storageProps.Properties = append(storageProps.Properties, &golang.Property{
		Key: "Cost Components",
	})
	storageProps.Properties = append(storageProps.Properties, storageCostComponentProperties...)

	props[computeRow.RowId] = computeProps
	props[storageRow.RowId] = storageProps

	return []*golang.ChartRow{&computeRow, &storageRow}, props
}

func (i RDSInstanceItem) Devices() ([]*golang.ChartRow, map[string]*golang.Properties) {
	if i.Wastage == nil {
		return nil, nil
	}
	return i.RDSInstanceDevice()
}

func (i RDSInstanceItem) ToOptimizationItem() *golang.ChartOptimizationItem {
	var platform string
	if i.Instance.Engine != nil {
		platform = *i.Instance.Engine
	}

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
		totalSaving += i.Wastage.RightSizing.Current.ComputeCost - i.Wastage.RightSizing.Recommended.ComputeCost
		totalCurrentCost += i.Wastage.RightSizing.Current.ComputeCost
		totalSaving += i.Wastage.RightSizing.Current.StorageCost - i.Wastage.RightSizing.Recommended.StorageCost
		totalCurrentCost += i.Wastage.RightSizing.Current.StorageCost
		status = fmt.Sprintf("%s (%.2f%%)", utils.FormatPriceFloat(totalSaving), (totalSaving/totalCurrentCost)*100)
	}

	deviceRows, deviceProps := i.Devices()

	oi := &golang.ChartOptimizationItem{
		OverviewChartRow: &golang.ChartRow{
			RowId: *i.Instance.DBInstanceIdentifier,
			Values: map[string]*golang.ChartRowItem{
				"x_kaytu_right_arrow": {
					Value: "→",
				},
				"resource_id": {
					Value: *i.Instance.DBInstanceIdentifier,
				},
				"resource_name": {
					Value: *i.Instance.DBInstanceIdentifier,
				},
				"resource_type": {
					Value: *i.Instance.DBInstanceClass,
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
