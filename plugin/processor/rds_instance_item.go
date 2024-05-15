package processor

import (
	"fmt"
	types2 "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/utils"
	"github.com/kaytu-io/plugin-aws/plugin/kaytu"
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
	Wastage kaytu.AwsRdsWastageResponse
}

func (i RDSInstanceItem) RDSInstanceDevice() []*golang.Device {
	ec2InstanceCompute := &golang.Device{
		Properties:   nil,
		DeviceId:     fmt.Sprintf("%s-compute", *i.Instance.DBInstanceIdentifier),
		ResourceType: "RDS Instance Compute",
		Runtime:      "730 hours",
		CurrentCost:  i.Wastage.RightSizing.Current.ComputeCost,
	}
	ec2InstanceStorage := &golang.Device{
		Properties:   nil,
		DeviceId:     fmt.Sprintf("%s-storage", *i.Instance.DBInstanceIdentifier),
		ResourceType: "RDS Instance Storage",
		Runtime:      "730 hours",
		CurrentCost:  i.Wastage.RightSizing.Current.StorageCost,
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
		Current: fmt.Sprintf("%d", i.Wastage.RightSizing.Current.VCPU),
		Average: utils.Percentage(i.Wastage.RightSizing.VCPU.Avg),
		Max:     utils.Percentage(i.Wastage.RightSizing.VCPU.Max),
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
		Average: utils.MemoryUsagePercentageByFreeSpace(i.Wastage.RightSizing.FreeMemoryBytes.Avg, float64(i.Wastage.RightSizing.Current.MemoryGb)),
		Max:     utils.MemoryUsagePercentageByFreeSpace(i.Wastage.RightSizing.FreeMemoryBytes.Min, float64(i.Wastage.RightSizing.Current.MemoryGb)),
	}
	storageTypeProperty := &golang.Property{
		Key:     "Type",
		Current: utils.PString(i.Wastage.RightSizing.Current.StorageType),
	}
	storageSizeProperty := &golang.Property{
		Key:     "Size",
		Current: utils.SizeByteToGB(i.Wastage.RightSizing.Current.StorageSize),
		Average: utils.StorageUsagePercentageByFreeSpace(i.Wastage.RightSizing.FreeStorageBytes.Avg, i.Wastage.RightSizing.Current.StorageSize),
		Max:     utils.StorageUsagePercentageByFreeSpace(i.Wastage.RightSizing.FreeStorageBytes.Min, i.Wastage.RightSizing.Current.StorageSize),
	}
	storageIOPSProperty := &golang.Property{
		Key:     "IOPS",
		Current: utils.PInt32ToString(i.Wastage.RightSizing.Current.StorageIops),
		Average: fmt.Sprintf("%s io/s", utils.PFloat64ToString(i.Wastage.RightSizing.StorageIops.Avg)),
		Max:     fmt.Sprintf("%s io/s", utils.PFloat64ToString(i.Wastage.RightSizing.StorageIops.Max)),
	}
	if storageIOPSProperty.Current != "" {
		storageIOPSProperty.Current = fmt.Sprintf("%s io/s", storageIOPSProperty.Current)
	} else {
		storageIOPSProperty.Current = "N/A"
	}
	// current number is in MB/s, so we need to convert it to bytes/s so matches the other values
	if i.Wastage.RightSizing.Current.StorageThroughput != nil {
		v := *i.Wastage.RightSizing.Current.StorageThroughput * 1024.0 * 1024.0
		i.Wastage.RightSizing.Current.StorageThroughput = &v
	}
	storageThroughputProperty := &golang.Property{
		Key:     "Throughput",
		Current: utils.PStorageThroughputMbps(i.Wastage.RightSizing.Current.StorageThroughput),
		Average: utils.PStorageThroughputMbps(i.Wastage.RightSizing.StorageThroughputBytes.Avg),
		Max:     utils.PStorageThroughputMbps(i.Wastage.RightSizing.StorageThroughputBytes.Max),
	}

	if i.Wastage.RightSizing.Recommended != nil {
		processorProperty.Recommended = i.Wastage.RightSizing.Recommended.Processor
		architectureProperty.Recommended = i.Wastage.RightSizing.Recommended.Architecture
		ec2InstanceCompute.RightSizedCost = i.Wastage.RightSizing.Recommended.ComputeCost
		ec2InstanceStorage.RightSizedCost = i.Wastage.RightSizing.Recommended.StorageCost
		regionProperty.Recommended = i.Wastage.RightSizing.Recommended.Region
		instanceSizeProperty.Recommended = i.Wastage.RightSizing.Recommended.InstanceType
		engineProperty.Recommended = i.Wastage.RightSizing.Recommended.Engine
		engineVerProperty.Recommended = i.Wastage.RightSizing.Recommended.EngineVersion
		clusterTypeProperty.Recommended = string(i.Wastage.RightSizing.Recommended.ClusterType)
		vCPUProperty.Recommended = fmt.Sprintf("%d", i.Wastage.RightSizing.Recommended.VCPU)
		memoryProperty.Recommended = fmt.Sprintf("%d GiB", i.Wastage.RightSizing.Recommended.MemoryGb)
		storageTypeProperty.Recommended = utils.PString(i.Wastage.RightSizing.Recommended.StorageType)
		storageSizeProperty.Recommended = utils.SizeByteToGB(i.Wastage.RightSizing.Recommended.StorageSize)
		storageIOPSProperty.Recommended = utils.PInt32ToString(i.Wastage.RightSizing.Recommended.StorageIops)
		if storageIOPSProperty.Recommended != "" {
			storageIOPSProperty.Recommended = fmt.Sprintf("%s io/s", storageIOPSProperty.Recommended)
		} else {
			storageIOPSProperty.Recommended = "N/A"
		}
		// Recommended number is in MB/s, so we need to convert it to bytes/s so matches the other values
		if i.Wastage.RightSizing.Recommended.StorageThroughput != nil {
			v := *i.Wastage.RightSizing.Recommended.StorageThroughput * 1024.0 * 1024.0
			i.Wastage.RightSizing.Recommended.StorageThroughput = &v
		}
		storageThroughputProperty.Recommended = utils.PStorageThroughputMbps(i.Wastage.RightSizing.Recommended.StorageThroughput)
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

	return []*golang.Device{ec2InstanceCompute, ec2InstanceStorage}
}

func (i RDSInstanceItem) Devices() []*golang.Device {
	return i.RDSInstanceDevice()
}

func (i RDSInstanceItem) ToOptimizationItem() *golang.OptimizationItem {
	oi := &golang.OptimizationItem{
		Id:                 *i.Instance.DBInstanceIdentifier,
		ResourceType:       *i.Instance.DBInstanceClass,
		Region:             i.Region,
		Devices:            i.Devices(),
		Preferences:        i.Preferences,
		Description:        i.Wastage.RightSizing.Description,
		Loading:            i.OptimizationLoading,
		Skipped:            i.Skipped,
		SkipReason:         i.SkipReason,
		LazyLoadingEnabled: i.LazyLoadingEnabled,
	}

	if i.Instance.Engine != nil {
		oi.Platform = *i.Instance.Engine
	}
	//for _, t := range i.Instance.Tags {
	//	if t.Key != nil && strings.ToLower(*t.Key) == "name" && t.Value != nil {
	//		oi.Name = *t.Value
	//	}
	//}
	if oi.Name == "" {
		oi.Name = *i.Instance.DBInstanceIdentifier
	}

	return oi
}
