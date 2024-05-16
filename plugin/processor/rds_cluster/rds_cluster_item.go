package rds_cluster

import (
	"fmt"
	types2 "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/utils"
	"github.com/kaytu-io/plugin-aws/plugin/kaytu"
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
	Wastage kaytu.AwsClusterWastageResponse
}

func (c RDSClusterItem) RDSInstanceDevice() []*golang.Device {
	var devices []*golang.Device
	for _, i := range c.Instances {
		ec2InstanceCompute := &golang.Device{
			Properties:   nil,
			DeviceId:     fmt.Sprintf("%s-compute", *i.DBInstanceIdentifier),
			ResourceType: "RDS Instance Compute",
			Runtime:      "730 hours",
			CurrentCost:  c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Current.ComputeCost,
		}
		ec2InstanceStorage := &golang.Device{
			Properties:   nil,
			DeviceId:     fmt.Sprintf("%s-storage", *i.DBInstanceIdentifier),
			ResourceType: "RDS Instance Storage",
			Runtime:      "730 hours",
			CurrentCost:  c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Current.StorageCost,
		}
		regionProperty := &golang.Property{
			Key:     "Region",
			Current: c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Current.Region,
		}
		instanceSizeProperty := &golang.Property{
			Key:     "Instance Size",
			Current: c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Current.InstanceType,
		}
		engineProperty := &golang.Property{
			Key:     "Engine",
			Current: c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Current.Engine,
		}
		engineVerProperty := &golang.Property{
			Key:     "Engine Version",
			Current: c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Current.EngineVersion,
		}
		clusterTypeProperty := &golang.Property{
			Key:     "Cluster Type",
			Current: string(c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Current.ClusterType),
		}
		vCPUProperty := &golang.Property{
			Key:     "vCPU",
			Current: fmt.Sprintf("%d", c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Current.VCPU),
			Average: utils.Percentage(c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].VCPU.Avg),
			Max:     utils.Percentage(c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].VCPU.Max),
		}
		processorProperty := &golang.Property{
			Key:     "Processor(s)",
			Current: c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Current.Processor,
		}
		architectureProperty := &golang.Property{
			Key:     "Architecture",
			Current: c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Current.Architecture,
		}
		memoryProperty := &golang.Property{
			Key:     "Memory",
			Current: fmt.Sprintf("%d GiB", c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Current.MemoryGb),
			Average: utils.MemoryUsagePercentageByFreeSpace(c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].FreeMemoryBytes.Avg, float64(c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Current.MemoryGb)),
			Max:     utils.MemoryUsagePercentageByFreeSpace(c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].FreeMemoryBytes.Min, float64(c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Current.MemoryGb)),
		}
		storageTypeProperty := &golang.Property{
			Key:     "Type",
			Current: utils.PString(c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Current.StorageType),
		}
		storageSizeProperty := &golang.Property{
			Key:     "Size",
			Current: utils.SizeByteToGB(c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Current.StorageSize),
			Average: utils.StorageUsagePercentageByFreeSpace(c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].FreeStorageBytes.Avg, c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Current.StorageSize),
			Max:     utils.StorageUsagePercentageByFreeSpace(c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].FreeStorageBytes.Min, c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Current.StorageSize),
		}
		if strings.Contains(strings.ToLower(c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Current.Engine), "aurora") {
			avgPercentage := (*c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].VolumeBytesUsed.Avg / (1024.0 * 1024.0 * 1024.0)) / float64(*c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Current.StorageSize) * 100
			maxPercentage := (*c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].VolumeBytesUsed.Max / (1024.0 * 1024.0 * 1024.0)) / float64(*c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Current.StorageSize) * 100
			storageSizeProperty.Average = utils.Percentage(&avgPercentage)
			storageSizeProperty.Max = utils.Percentage(&maxPercentage)
		}
		storageIOPSProperty := &golang.Property{
			Key:     "IOPS",
			Current: utils.PInt32ToString(c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Current.StorageIops),
			Average: fmt.Sprintf("%s io/s", utils.PFloat64ToString(c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].StorageIops.Avg)),
			Max:     fmt.Sprintf("%s io/s", utils.PFloat64ToString(c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].StorageIops.Max)),
		}
		if storageIOPSProperty.Current != "" {
			storageIOPSProperty.Current = fmt.Sprintf("%s io/s", storageIOPSProperty.Current)
		} else {
			storageIOPSProperty.Current = "N/A"
		}
		// current number is in MB/s, so we need to convert it to bytes/s so matches the other values
		if c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Current.StorageThroughput != nil {
			v := c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)]
			tmp := *v.Current.StorageThroughput * 1024.0 * 1024.0
			v.Current.StorageThroughput = &tmp
			c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)] = v
		}
		storageThroughputProperty := &golang.Property{
			Key:     "Throughput",
			Current: utils.PStorageThroughputMbps(c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Current.StorageThroughput),
			Average: utils.PStorageThroughputMbps(c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].StorageThroughput.Avg),
			Max:     utils.PStorageThroughputMbps(c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].StorageThroughput.Max),
		}

		if c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Recommended != nil {
			processorProperty.Recommended = c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Recommended.Processor
			architectureProperty.Recommended = c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Recommended.Architecture
			ec2InstanceCompute.RightSizedCost = c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Recommended.ComputeCost
			ec2InstanceStorage.RightSizedCost = c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Recommended.StorageCost
			regionProperty.Recommended = c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Recommended.Region
			instanceSizeProperty.Recommended = c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Recommended.InstanceType
			engineProperty.Recommended = c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Recommended.Engine
			engineVerProperty.Recommended = c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Recommended.EngineVersion
			clusterTypeProperty.Recommended = string(c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Recommended.ClusterType)
			vCPUProperty.Recommended = fmt.Sprintf("%d", c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Recommended.VCPU)
			memoryProperty.Recommended = fmt.Sprintf("%d GiB", c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Recommended.MemoryGb)
			storageTypeProperty.Recommended = utils.PString(c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Recommended.StorageType)
			storageSizeProperty.Recommended = utils.SizeByteToGB(c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Recommended.StorageSize)
			storageIOPSProperty.Recommended = utils.PInt32ToString(c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Recommended.StorageIops)
			if storageIOPSProperty.Recommended != "" {
				storageIOPSProperty.Recommended = fmt.Sprintf("%s io/s", storageIOPSProperty.Recommended)
			} else {
				storageIOPSProperty.Recommended = "N/A"
			}
			// Recommended number is in MB/s, so we need to convert it to bytes/s so matches the other values
			if c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Recommended.StorageThroughput != nil {
				v := *c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Recommended.StorageThroughput * 1024.0 * 1024.0
				c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Recommended.StorageThroughput = &v
			}
			storageThroughputProperty.Recommended = utils.PStorageThroughputMbps(c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Recommended.StorageThroughput)
		}
		ec2InstanceCompute.Properties = append(ec2InstanceCompute.Properties, regionProperty)
		ec2InstanceCompute.Properties = append(ec2InstanceCompute.Properties, instanceSizeProperty)
		ec2InstanceCompute.Properties = append(ec2InstanceCompute.Properties, engineProperty)
		ec2InstanceCompute.Properties = append(ec2InstanceCompute.Properties, engineVerProperty)
		ec2InstanceCompute.Properties = append(ec2InstanceCompute.Properties, clusterTypeProperty)
		ec2InstanceStorage.Properties = append(ec2InstanceStorage.Properties, regionProperty)
		//ec2InstanceStorage.Properties = append(ec2Instance.Properties, &golang.Property{
		//	Key: "Compute",
		//})
		ec2InstanceCompute.Properties = append(ec2InstanceCompute.Properties, vCPUProperty)
		ec2InstanceCompute.Properties = append(ec2InstanceCompute.Properties, memoryProperty)
		ec2InstanceCompute.Properties = append(ec2InstanceCompute.Properties, processorProperty)
		ec2InstanceCompute.Properties = append(ec2InstanceCompute.Properties, architectureProperty)
		//ec2Instance.Properties = append(ec2Instance.Properties, &golang.Property{
		//	Key: "Storage",
		//})
		ec2InstanceStorage.Properties = append(ec2InstanceStorage.Properties, storageTypeProperty)
		ec2InstanceStorage.Properties = append(ec2InstanceStorage.Properties, storageSizeProperty)
		ec2InstanceStorage.Properties = append(ec2InstanceStorage.Properties, storageIOPSProperty)
		ec2InstanceStorage.Properties = append(ec2InstanceStorage.Properties, storageThroughputProperty)

		devices = append(devices, ec2InstanceCompute, ec2InstanceStorage)
	}

	return devices
}

func (i RDSClusterItem) Devices() []*golang.Device {
	return i.RDSInstanceDevice()
}

func (i RDSClusterItem) ToOptimizationItem() *golang.OptimizationItem {
	oi := &golang.OptimizationItem{
		Id:                 *i.Cluster.DBClusterIdentifier,
		ResourceType:       *i.Cluster.Engine,
		Region:             i.Region,
		Devices:            i.Devices(),
		Preferences:        i.Preferences,
		Description:        "", //c.Wastage.RightSizing[utils.HashString(*i.DBInstanceIdentifier)].Description, //TODO-Saleh
		Loading:            i.OptimizationLoading,
		Skipped:            i.Skipped,
		SkipReason:         i.SkipReason,
		LazyLoadingEnabled: i.LazyLoadingEnabled,
	}

	if i.Cluster.Engine != nil {
		oi.Platform = *i.Cluster.Engine
	}
	//for _, t := range i.Tags {
	//	if t.Key != nil && strings.ToLower(*t.Key) == "name" && t.Value != nil {
	//		oi.Name = *t.Value
	//	}
	//}
	if oi.Name == "" {
		oi.Name = *i.Cluster.DBClusterIdentifier
	}

	return oi
}
