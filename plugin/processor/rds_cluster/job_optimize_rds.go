package rds_cluster

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/google/uuid"
	"github.com/kaytu-io/kaytu/pkg/utils"
	"github.com/kaytu-io/kaytu/preferences"
	"github.com/kaytu-io/plugin-aws/plugin/kaytu"
	"github.com/kaytu-io/plugin-aws/plugin/version"
)

type OptimizeRDSClusterJob struct {
	processor *Processor
	item      RDSClusterItem
}

func NewOptimizeRDSJob(processor *Processor, item RDSClusterItem) *OptimizeRDSClusterJob {
	return &OptimizeRDSClusterJob{
		processor: processor,
		item:      item,
	}
}

func (j *OptimizeRDSClusterJob) Id() string {
	return fmt.Sprintf("optimize_rds_cluster_%s", *j.item.Cluster.DBClusterIdentifier)
}
func (j *OptimizeRDSClusterJob) Description() string {
	return fmt.Sprintf("Optimizing %s", *j.item.Cluster.DBClusterIdentifier)
}
func (j *OptimizeRDSClusterJob) Run() error {
	if j.item.LazyLoadingEnabled {
		j.processor.jobQueue.Push(NewGetRDSInstanceMetricsJob(j.processor, j.item.Region, j.item.Cluster))
		return nil
	}

	reqID := uuid.New().String()
	var instances []kaytu.AwsRds
	for _, i := range j.item.Instances {
		var clusterType kaytu.AwsRdsClusterType
		multiAZ := i.MultiAZ != nil && *i.MultiAZ
		readableStandbys := i.ReplicaMode == types.ReplicaModeOpenReadOnly
		if multiAZ && readableStandbys {
			clusterType = kaytu.AwsRdsClusterTypeMultiAzTwoInstance
		} else if multiAZ {
			clusterType = kaytu.AwsRdsClusterTypeMultiAzOneInstance
		} else {
			clusterType = kaytu.AwsRdsClusterTypeSingleInstance
		}

		rdsInstance := kaytu.AwsRds{
			HashedInstanceId:                   utils.HashString(*i.DBInstanceIdentifier),
			AvailabilityZone:                   *i.AvailabilityZone,
			InstanceType:                       *i.DBInstanceClass,
			Engine:                             *i.Engine,
			EngineVersion:                      *i.EngineVersion,
			LicenseModel:                       *i.LicenseModel,
			BackupRetentionPeriod:              i.BackupRetentionPeriod,
			ClusterType:                        clusterType,
			PerformanceInsightsEnabled:         *i.PerformanceInsightsEnabled,
			PerformanceInsightsRetentionPeriod: i.PerformanceInsightsRetentionPeriod,
			StorageType:                        i.StorageType,
			StorageSize:                        i.AllocatedStorage,
			StorageIops:                        i.Iops,
		}
		if i.StorageThroughput != nil {
			floatThroughput := float64(*i.StorageThroughput)
			rdsInstance.StorageThroughput = &floatThroughput
		}

		instances = append(instances, rdsInstance)
	}

	req := kaytu.AwsClusterWastageRequest{
		RequestId:      &reqID,
		CliVersion:     &version.VERSION,
		Identification: j.processor.identification,
		Cluster: kaytu.AwsRdsCluster{
			HashedClusterId: utils.HashString(*j.item.Cluster.DBClusterIdentifier),
			Engine:          *j.item.Cluster.Engine,
		},
		Instances:   instances,
		Metrics:     j.item.Metrics,
		Region:      j.item.Region,
		Preferences: preferences.Export(j.item.Preferences),
		Loading:     false,
	}
	res, err := kaytu.RDSClusterWastageRequest(req, j.processor.kaytuAcccessToken)
	if err != nil {
		return err
	}

	//TODO-Saleh
	//if res.RightSizing.Current.InstanceType == "" {
	//	j.item.OptimizationLoading = false
	//	j.processor.items[*j.item.Cluster.DBClusterIdentifier] = j.item
	//	j.processor.publishOptimizationItem(j.item.ToOptimizationItem())
	//	return nil
	//}

	j.item = RDSClusterItem{
		Cluster:             j.item.Cluster,
		Instances:           j.item.Instances,
		Region:              j.item.Region,
		OptimizationLoading: false,
		Preferences:         j.item.Preferences,
		Skipped:             false,
		SkipReason:          "",
		Metrics:             j.item.Metrics,
		Wastage:             *res,
	}
	j.processor.items[*j.item.Cluster.DBClusterIdentifier] = j.item
	j.processor.publishOptimizationItem(j.item.ToOptimizationItem())
	return nil
}
