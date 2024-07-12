package rds_cluster

import (
	"fmt"
	types2 "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/utils"
	"github.com/kaytu-io/plugin-aws/plugin/processor/shared"
	golang2 "github.com/kaytu-io/plugin-aws/plugin/proto/src/golang"
	"google.golang.org/protobuf/types/known/wrapperspb"
	"sort"
	"strings"
)

type RDSClusterItem struct {
	Cluster             types.DBCluster
	Instances           []types.DBInstance
	Region              string
	OptimizationLoading bool
	Preferences         []*golang.PreferenceItem
	Skipped             bool
	LazyLoadingEnabled  bool
	SkipReason          string

	Metrics map[string]map[string][]types2.Datapoint
	Wastage *golang2.RDSClusterOptimizationResponse
}

func (c RDSClusterItem) RDSInstanceDevice() ([]*golang.ChartRow, map[string]*golang.Properties) {
	var deviceRows []*golang.ChartRow
	deviceProps := make(map[string]*golang.Properties)

	for _, i := range c.Instances {
		hashedId := utils.HashString(*i.DBInstanceIdentifier)
		computeProps := &golang.Properties{}
		storageProps := &golang.Properties{}

		computeRow := golang.ChartRow{
			RowId:  fmt.Sprintf("%s-compute", *i.DBInstanceIdentifier),
			Values: make(map[string]*golang.ChartRowItem),
		}
		computeRow.Values["resource_id"] = &golang.ChartRowItem{
			Value: fmt.Sprintf("%s-compute", *i.DBInstanceIdentifier),
		}
		computeRow.Values["resource_name"] = &golang.ChartRowItem{
			Value: fmt.Sprintf("%s-compute", *i.DBInstanceIdentifier),
		}
		computeRow.Values["resource_type"] = &golang.ChartRowItem{
			Value: "RDS Instance Compute",
		}
		computeRow.Values["runtime"] = &golang.ChartRowItem{
			Value: "730 hours",
		}
		computeRow.Values["current_cost"] = &golang.ChartRowItem{
			Value: utils.FormatPriceFloat(c.Wastage.RightSizing[hashedId].Current.ComputeCost),
		}

		storageRow := golang.ChartRow{
			RowId:  fmt.Sprintf("%s-storage", *i.DBInstanceIdentifier),
			Values: make(map[string]*golang.ChartRowItem),
		}
		storageRow.Values["resource_id"] = &golang.ChartRowItem{
			Value: fmt.Sprintf("%s-storage", *i.DBInstanceIdentifier),
		}
		storageRow.Values["resource_name"] = &golang.ChartRowItem{
			Value: *i.DBInstanceIdentifier,
		}
		storageRow.Values["resource_type"] = &golang.ChartRowItem{
			Value: "RDS Instance Storage",
		}
		storageRow.Values["runtime"] = &golang.ChartRowItem{
			Value: "730 hours",
		}
		storageRow.Values["current_cost"] = &golang.ChartRowItem{
			Value: utils.FormatPriceFloat(c.Wastage.RightSizing[hashedId].Current.StorageCost),
		}

		regionProperty := &golang.Property{
			Key:     "Region",
			Current: c.Wastage.RightSizing[hashedId].Current.Region,
		}
		instanceSizeProperty := &golang.Property{
			Key:     "Instance Size",
			Current: c.Wastage.RightSizing[hashedId].Current.InstanceType,
		}
		engineProperty := &golang.Property{
			Key:     "Engine",
			Current: c.Wastage.RightSizing[hashedId].Current.Engine,
		}
		engineVerProperty := &golang.Property{
			Key:     "Engine Version",
			Current: c.Wastage.RightSizing[hashedId].Current.EngineVersion,
		}
		clusterTypeProperty := &golang.Property{
			Key:     "Cluster Type",
			Current: string(c.Wastage.RightSizing[hashedId].Current.ClusterType),
		}
		vCPUProperty := &golang.Property{
			Key:     "vCPU",
			Current: fmt.Sprintf("%d", c.Wastage.RightSizing[hashedId].Current.Vcpu),
			Average: utils.Percentage(shared.WrappedToFloat64(c.Wastage.RightSizing[hashedId].Vcpu.Avg)),
			Max:     utils.Percentage(shared.WrappedToFloat64(c.Wastage.RightSizing[hashedId].Vcpu.Max)),
		}
		processorProperty := &golang.Property{
			Key:     "Processor(s)",
			Current: c.Wastage.RightSizing[hashedId].Current.Processor,
		}
		architectureProperty := &golang.Property{
			Key:     "Architecture",
			Current: c.Wastage.RightSizing[hashedId].Current.Architecture,
		}
		memoryProperty := &golang.Property{
			Key:     "Memory",
			Current: fmt.Sprintf("%d GiB", c.Wastage.RightSizing[hashedId].Current.MemoryGb),
			Average: utils.MemoryUsagePercentageByFreeSpace(shared.WrappedToFloat64(c.Wastage.RightSizing[hashedId].FreeMemoryBytes.Avg), float64(c.Wastage.RightSizing[hashedId].Current.MemoryGb)),
			Max:     utils.MemoryUsagePercentageByFreeSpace(shared.WrappedToFloat64(c.Wastage.RightSizing[hashedId].FreeMemoryBytes.Min), float64(c.Wastage.RightSizing[hashedId].Current.MemoryGb)),
		}
		storageTypeProperty := &golang.Property{
			Key:     "Type",
			Current: utils.PString(shared.WrappedToString(c.Wastage.RightSizing[hashedId].Current.StorageType)),
		}
		storageSizeProperty := &golang.Property{
			Key:     "Size",
			Current: utils.SizeByteToGB(shared.WrappedToInt32(c.Wastage.RightSizing[hashedId].Current.StorageSize)),
			Average: utils.StorageUsagePercentageByFreeSpace(shared.WrappedToFloat64(c.Wastage.RightSizing[hashedId].FreeStorageBytes.Avg), shared.WrappedToInt32(c.Wastage.RightSizing[hashedId].Current.StorageSize)),
			Max:     utils.StorageUsagePercentageByFreeSpace(shared.WrappedToFloat64(c.Wastage.RightSizing[hashedId].FreeStorageBytes.Min), shared.WrappedToInt32(c.Wastage.RightSizing[hashedId].Current.StorageSize)),
		}

		computeCostComponentPropertiesMap := make(map[string]*golang.Property)
		for k, v := range c.Wastage.RightSizing[hashedId].Current.ComputeCostComponents {
			computeCostComponentPropertiesMap[k] = &golang.Property{
				Key:     fmt.Sprintf("  %s", k),
				Current: fmt.Sprintf("$%.2f", v),
			}
		}
		storageCostComponentPropertiesMap := make(map[string]*golang.Property)
		for k, v := range c.Wastage.RightSizing[hashedId].Current.StorageCostComponents {
			storageCostComponentPropertiesMap[k] = &golang.Property{
				Key:     fmt.Sprintf("  %s", k),
				Current: fmt.Sprintf("$%.2f", v),
			}
		}

		if strings.Contains(strings.ToLower(c.Wastage.RightSizing[hashedId].Current.Engine), "aurora") {
			avgPercentage := (*shared.WrappedToFloat64(c.Wastage.RightSizing[hashedId].VolumeBytesUsed.Avg) / (1024.0 * 1024.0 * 1024.0)) / float64(*shared.WrappedToInt32(c.Wastage.RightSizing[hashedId].Current.StorageSize)) * 100
			maxPercentage := (*shared.WrappedToFloat64(c.Wastage.RightSizing[hashedId].VolumeBytesUsed.Max) / (1024.0 * 1024.0 * 1024.0)) / float64(*shared.WrappedToInt32(c.Wastage.RightSizing[hashedId].Current.StorageSize)) * 100
			storageSizeProperty.Average = utils.Percentage(&avgPercentage)
			storageSizeProperty.Max = utils.Percentage(&maxPercentage)
		}
		storageIOPSProperty := &golang.Property{
			Key:     "IOPS",
			Current: utils.PInt32ToString(shared.WrappedToInt32(c.Wastage.RightSizing[hashedId].Current.StorageIops)),
			Average: fmt.Sprintf("%s io/s", utils.PFloat64ToString(shared.WrappedToFloat64(c.Wastage.RightSizing[hashedId].StorageIops.Avg))),
			Max:     fmt.Sprintf("%s io/s", utils.PFloat64ToString(shared.WrappedToFloat64(c.Wastage.RightSizing[hashedId].StorageIops.Max))),
		}
		if storageIOPSProperty.Current != "" {
			storageIOPSProperty.Current = fmt.Sprintf("%s io/s", storageIOPSProperty.Current)
		} else {
			storageIOPSProperty.Current = ""
		}
		if c.Wastage.RightSizing[hashedId].StorageIops.Avg == nil {
			storageIOPSProperty.Average = ""
		}
		if c.Wastage.RightSizing[hashedId].StorageIops.Max == nil {
			storageIOPSProperty.Max = ""
		}
		// current number is in MB/s, so we need to convert it to bytes/s so matches the other values
		if c.Wastage.RightSizing[hashedId].Current.StorageThroughput != nil {
			v := c.Wastage.RightSizing[hashedId]
			tmp := *shared.WrappedToFloat64(v.Current.StorageThroughput) * 1024.0 * 1024.0
			v.Current.StorageThroughput = shared.Float64ToWrapper(&tmp)
			c.Wastage.RightSizing[hashedId] = v
		}
		storageThroughputProperty := &golang.Property{
			Key:     "Throughput",
			Current: utils.PStorageThroughputMbps(shared.WrappedToFloat64(c.Wastage.RightSizing[hashedId].Current.StorageThroughput)),
			Average: utils.PStorageThroughputMbps(shared.WrappedToFloat64(c.Wastage.RightSizing[hashedId].StorageThroughput.Avg)),
			Max:     utils.PStorageThroughputMbps(shared.WrappedToFloat64(c.Wastage.RightSizing[hashedId].StorageThroughput.Max)),
		}

		if c.Wastage.RightSizing[hashedId].Recommended != nil {
			processorProperty.Recommended = c.Wastage.RightSizing[hashedId].Recommended.Processor
			architectureProperty.Recommended = c.Wastage.RightSizing[hashedId].Recommended.Architecture
			computeRow.Values["right_sized_cost"] = &golang.ChartRowItem{
				Value: utils.FormatPriceFloat(c.Wastage.RightSizing[hashedId].Recommended.ComputeCost),
			}
			computeRow.Values["savings"] = &golang.ChartRowItem{
				Value: utils.FormatPriceFloat(c.Wastage.RightSizing[hashedId].Current.ComputeCost - c.Wastage.RightSizing[hashedId].Recommended.ComputeCost),
			}
			storageRow.Values["right_sized_cost"] = &golang.ChartRowItem{
				Value: utils.FormatPriceFloat(c.Wastage.RightSizing[hashedId].Recommended.StorageCost),
			}
			storageRow.Values["savings"] = &golang.ChartRowItem{
				Value: utils.FormatPriceFloat(c.Wastage.RightSizing[hashedId].Current.StorageCost - c.Wastage.RightSizing[hashedId].Recommended.StorageCost),
			}
			regionProperty.Recommended = c.Wastage.RightSizing[hashedId].Recommended.Region
			instanceSizeProperty.Recommended = c.Wastage.RightSizing[hashedId].Recommended.InstanceType
			engineProperty.Recommended = c.Wastage.RightSizing[hashedId].Recommended.Engine
			engineVerProperty.Recommended = c.Wastage.RightSizing[hashedId].Recommended.EngineVersion
			clusterTypeProperty.Recommended = string(c.Wastage.RightSizing[hashedId].Recommended.ClusterType)
			vCPUProperty.Recommended = fmt.Sprintf("%d", c.Wastage.RightSizing[hashedId].Recommended.Vcpu)
			memoryProperty.Recommended = fmt.Sprintf("%d GiB", c.Wastage.RightSizing[hashedId].Recommended.MemoryGb)
			storageTypeProperty.Recommended = utils.PString(shared.WrappedToString(c.Wastage.RightSizing[hashedId].Recommended.StorageType))
			storageSizeProperty.Recommended = utils.SizeByteToGB(shared.WrappedToInt32(c.Wastage.RightSizing[hashedId].Recommended.StorageSize))
			storageIOPSProperty.Recommended = utils.PInt32ToString(shared.WrappedToInt32(c.Wastage.RightSizing[hashedId].Recommended.StorageIops))
			if storageIOPSProperty.Recommended != "" {
				storageIOPSProperty.Recommended = fmt.Sprintf("%s io/s", storageIOPSProperty.Recommended)
			} else {
				storageIOPSProperty.Recommended = "N/A"
			}
			// Recommended number is in MB/s, so we need to convert it to bytes/s so matches the other values
			if c.Wastage.RightSizing[hashedId].Recommended.StorageThroughput != nil {
				v := *shared.WrappedToFloat64(c.Wastage.RightSizing[hashedId].Recommended.StorageThroughput) * 1024.0 * 1024.0
				c.Wastage.RightSizing[hashedId].Recommended.StorageThroughput = shared.Float64ToWrapper(&v)
			}
			storageThroughputProperty.Recommended = utils.PStorageThroughputMbps(shared.WrappedToFloat64(c.Wastage.RightSizing[hashedId].Recommended.StorageThroughput))

			for k, v := range c.Wastage.RightSizing[hashedId].Recommended.ComputeCostComponents {
				if _, ok := computeCostComponentPropertiesMap[k]; !ok {
					computeCostComponentPropertiesMap[k] = &golang.Property{
						Key: fmt.Sprintf("  %s", k),
					}
				}
				computeCostComponentPropertiesMap[k].Recommended = fmt.Sprintf("$%.2f", v)
			}
			for k, v := range c.Wastage.RightSizing[hashedId].Recommended.StorageCostComponents {
				if _, ok := storageCostComponentPropertiesMap[k]; !ok {
					storageCostComponentPropertiesMap[k] = &golang.Property{
						Key: fmt.Sprintf("  %s", k),
					}
				}
				storageCostComponentPropertiesMap[k].Recommended = fmt.Sprintf("$%.2f", v)
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
		storageProps.Properties = append(storageProps.Properties, &golang.Property{
			Key:         "Description",
			Recommended: strings.TrimSpace(c.Wastage.RightSizing[hashedId].Description),
		})

		deviceProps[fmt.Sprintf("%s-compute", *i.DBInstanceIdentifier)] = computeProps
		deviceProps[fmt.Sprintf("%s-storage", *i.DBInstanceIdentifier)] = storageProps
		deviceRows = append(deviceRows, &computeRow, &storageRow)
	}

	return deviceRows, deviceProps
}

func (i RDSClusterItem) Devices() ([]*golang.ChartRow, map[string]*golang.Properties) {
	if i.Wastage == nil {
		return nil, nil
	}
	return i.RDSInstanceDevice()
}

func (i RDSClusterItem) ToOptimizationItem() *golang.ChartOptimizationItem {
	var platform string
	if i.Cluster.Engine != nil {
		platform = *i.Cluster.Engine
	}

	status := ""
	if i.Skipped {
		status = fmt.Sprintf("skipped - %s", i.SkipReason)
	} else if i.LazyLoadingEnabled && !i.OptimizationLoading {
		status = "press enter to load"
	} else if i.OptimizationLoading {
		status = "loading"
	} else {
		totalSaving := 0.0
		totalCurrentCost := 0.0
		for _, rs := range i.Wastage.RightSizing {
			totalSaving += rs.Current.ComputeCost - rs.Recommended.ComputeCost
			totalCurrentCost += rs.Current.ComputeCost
			totalSaving += rs.Current.StorageCost - rs.Recommended.StorageCost
			totalCurrentCost += rs.Current.StorageCost
			status = fmt.Sprintf("%s (%.2f%%)", utils.FormatPriceFloat(totalSaving), (totalSaving/totalCurrentCost)*100)
		}
	}

	deviceRows, deviceProps := i.Devices()

	oi := &golang.ChartOptimizationItem{
		OverviewChartRow: &golang.ChartRow{
			RowId: *i.Cluster.DBClusterIdentifier,
			Values: map[string]*golang.ChartRowItem{
				"x_kaytu_right_arrow": {
					Value: "→",
				},
				"resource_id": {
					Value: *i.Cluster.DBClusterIdentifier,
				},
				"resource_name": {
					Value: *i.Cluster.DBClusterIdentifier,
				},
				"resource_type": {
					Value: *i.Cluster.Engine,
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
		Description:        "", //c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Description, //TODO-Saleh
		Loading:            i.OptimizationLoading,
		Skipped:            i.Skipped,
		SkipReason:         nil,
		LazyLoadingEnabled: i.LazyLoadingEnabled,
	}
	if i.SkipReason != "" {
		oi.SkipReason = &wrapperspb.StringValue{Value: i.SkipReason}
	}
	//for _, t := range i.Tags {
	//	if t.Key != nil && strings.ToLower(*t.Key) == "name" && t.Value != nil {
	//		oi.Name = *t.Value
	//	}
	//}

	return oi
}
