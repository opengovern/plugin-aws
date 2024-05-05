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
	SkipReason          string

	Metrics map[string][]types2.Datapoint
	Wastage kaytu.AwsRdsWastageResponse
}

func (i RDSInstanceItem) RDSInstanceDevice() *golang.Device {
	ec2Instance := &golang.Device{
		Properties:   nil,
		DeviceId:     *i.Instance.DBInstanceIdentifier,
		ResourceType: "RDS Instance",
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
		Key:     "  vCPU",
		Current: fmt.Sprintf("%d", i.Wastage.RightSizing.Current.VCPU),
		Average: utils.Percentage(i.Wastage.RightSizing.VCPU.Avg),
		Max:     utils.Percentage(i.Wastage.RightSizing.VCPU.Max),
	}
	memoryProperty := &golang.Property{
		Key:     "  Memory",
		Current: fmt.Sprintf("%d GiB", i.Wastage.RightSizing.Current.MemoryGb),
		Average: utils.MemoryUsagePercentageByFreeSpace(i.Wastage.RightSizing.FreeMemoryBytes.Avg, float64(i.Wastage.RightSizing.Current.MemoryGb)),
		Max:     utils.MemoryUsagePercentageByFreeSpace(i.Wastage.RightSizing.FreeMemoryBytes.Min, float64(i.Wastage.RightSizing.Current.MemoryGb)),
	}
	storageTypeProperty := &golang.Property{
		Key:     "  Type",
		Current: utils.PString(i.Wastage.RightSizing.Current.StorageType),
	}
	storageSizeProperty := &golang.Property{
		Key:     "  Size",
		Current: utils.SizeByteToGB(i.Wastage.RightSizing.Current.StorageSize),
		Average: utils.StorageUsagePercentageByFreeSpace(i.Wastage.RightSizing.FreeStorageBytes.Avg, i.Wastage.RightSizing.Current.StorageSize),
		Max:     utils.StorageUsagePercentageByFreeSpace(i.Wastage.RightSizing.FreeStorageBytes.Min, i.Wastage.RightSizing.Current.StorageSize),
	}
	storageIOPSProperty := &golang.Property{
		Key:     "  IOPS",
		Current: fmt.Sprintf("%d", i.Wastage.RightSizing.Current.StorageIops),
		Average: fmt.Sprintf("%s io/s", utils.PFloat64ToString(i.Wastage.RightSizing.StorageIops.Avg)),
		Max:     fmt.Sprintf("%s io/s", utils.PFloat64ToString(i.Wastage.RightSizing.StorageIops.Max)),
	}
	storageThroughputProperty := &golang.Property{
		Key:     "  Throughput",
		Current: utils.PStorageThroughputMbps(i.Wastage.RightSizing.Current.StorageThroughput),
		Average: utils.PStorageThroughputMbps(i.Wastage.RightSizing.StorageThroughputBytes.Avg),
		Max:     utils.PStorageThroughputMbps(i.Wastage.RightSizing.StorageThroughputBytes.Max),
	}

	if i.Wastage.RightSizing.Recommended != nil {
		ec2Instance.RightSizedCost = i.Wastage.RightSizing.Recommended.Cost
		regionProperty.Recommended = i.Wastage.RightSizing.Recommended.Region
		instanceSizeProperty.Recommended = i.Wastage.RightSizing.Recommended.InstanceType
		engineProperty.Recommended = i.Wastage.RightSizing.Recommended.Engine
		engineVerProperty.Recommended = i.Wastage.RightSizing.Recommended.EngineVersion
		clusterTypeProperty.Recommended = string(i.Wastage.RightSizing.Recommended.ClusterType)
		vCPUProperty.Recommended = fmt.Sprintf("%d", i.Wastage.RightSizing.Recommended.VCPU)
		memoryProperty.Recommended = fmt.Sprintf("%d GiB", i.Wastage.RightSizing.Recommended.MemoryGb)
		storageTypeProperty.Recommended = utils.PString(i.Wastage.RightSizing.Recommended.StorageType)
		storageSizeProperty.Recommended = utils.SizeByteToGB(i.Wastage.RightSizing.Recommended.StorageSize)
		storageIOPSProperty.Recommended = fmt.Sprintf("%s io/s", utils.PInt32ToString(i.Wastage.RightSizing.Recommended.StorageIops))
		storageThroughputProperty.Recommended = utils.PStorageThroughputMbps(i.Wastage.RightSizing.Recommended.StorageThroughput)
	}
	ec2Instance.Properties = append(ec2Instance.Properties, regionProperty)
	ec2Instance.Properties = append(ec2Instance.Properties, instanceSizeProperty)
	ec2Instance.Properties = append(ec2Instance.Properties, engineProperty)
	ec2Instance.Properties = append(ec2Instance.Properties, engineVerProperty)
	ec2Instance.Properties = append(ec2Instance.Properties, clusterTypeProperty)
	ec2Instance.Properties = append(ec2Instance.Properties, &golang.Property{
		Key: "Compute",
	})
	ec2Instance.Properties = append(ec2Instance.Properties, vCPUProperty)
	ec2Instance.Properties = append(ec2Instance.Properties, memoryProperty)
	ec2Instance.Properties = append(ec2Instance.Properties, &golang.Property{
		Key: "Storage",
	})
	ec2Instance.Properties = append(ec2Instance.Properties, storageTypeProperty)
	ec2Instance.Properties = append(ec2Instance.Properties, storageSizeProperty)
	ec2Instance.Properties = append(ec2Instance.Properties, storageIOPSProperty)
	ec2Instance.Properties = append(ec2Instance.Properties, storageThroughputProperty)

	return ec2Instance
}

func (i RDSInstanceItem) Devices() []*golang.Device {
	return []*golang.Device{i.RDSInstanceDevice()}
}

func (i RDSInstanceItem) ToOptimizationItem() *golang.OptimizationItem {
	oi := &golang.OptimizationItem{
		Id:           *i.Instance.DBInstanceIdentifier,
		ResourceType: *i.Instance.DBInstanceClass,
		Region:       i.Region,
		Devices:      i.Devices(),
		Preferences:  i.Preferences,
		Description:  i.Wastage.RightSizing.Description,
		Loading:      i.OptimizationLoading,
		Skipped:      i.Skipped,
		SkipReason:   i.SkipReason,
	}

	//if i.Instance.PlatformDetails != nil {
	//	oi.Platform = *i.Instance.PlatformDetails
	//}
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
