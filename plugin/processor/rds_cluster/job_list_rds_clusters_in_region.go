package rds_cluster

import (
	"fmt"
	preferences2 "github.com/kaytu-io/plugin-aws/plugin/preferences"
)

type ListRDSClustersInRegionJob struct {
	region    string
	processor *Processor
}

func NewListRDSInstancesInRegionJob(processor *Processor, region string) *ListRDSClustersInRegionJob {
	return &ListRDSClustersInRegionJob{
		processor: processor,
		region:    region,
	}
}

func (j *ListRDSClustersInRegionJob) Id() string {
	return fmt.Sprintf("list_rds_clusters_in_%s", j.region)
}
func (j *ListRDSClustersInRegionJob) Description() string {
	return fmt.Sprintf("Listing all RDS Clusters in %s", j.region)
}
func (j *ListRDSClustersInRegionJob) Run() error {
	clusters, err := j.processor.provider.ListRDSClusters(j.region)
	if err != nil {
		return err
	}

	for _, cluster := range clusters {
		instances, err := j.processor.provider.ListRDSInstanceByCluster(j.region, *cluster.DBClusterIdentifier)
		if err != nil {
			return err
		}

		oi := RDSClusterItem{
			Cluster:             cluster,
			Instances:           instances,
			Region:              j.region,
			OptimizationLoading: true,
			LazyLoadingEnabled:  false,
			Preferences:         preferences2.DefaultRDSPreferences,
		}

		if !oi.Skipped {
			j.processor.lazyloadCounter.Increment()
			if j.processor.lazyloadCounter.Get() > j.processor.configuration.RDSLazyLoad {
				oi.LazyLoadingEnabled = true
			}
			//TODO-Saleh
			//
			//var clusterType kaytu.AwsRdsClusterType
			//multiAZ := oi.Instance.MultiAZ != nil && *oi.Instance.MultiAZ
			//readableStandbys := oi.Instance.ReplicaMode == types.ReplicaModeOpenReadOnly
			//if multiAZ && readableStandbys {
			//	clusterType = kaytu.AwsRdsClusterTypeMultiAzTwoInstance
			//} else if multiAZ {
			//	clusterType = kaytu.AwsRdsClusterTypeMultiAzOneInstance
			//} else {
			//	clusterType = kaytu.AwsRdsClusterTypeSingleInstance
			//}
			//
			//reqID := uuid.New().String()
			//req := kaytu.AwsRdsWastageRequest{
			//	RequestId:      &reqID,
			//	CliVersion:     &version.VERSION,
			//	Identification: j.processor.identification,
			//	Instance: kaytu.AwsRds{
			//		HashedInstanceId:                   utils.HashString(*oi.Instance.DBInstanceIdentifier),
			//		AvailabilityZone:                   *oi.Instance.AvailabilityZone,
			//		InstanceType:                       *oi.Instance.DBInstanceClass,
			//		Engine:                             *oi.Instance.Engine,
			//		EngineVersion:                      *oi.Instance.EngineVersion,
			//		LicenseModel:                       *oi.Instance.LicenseModel,
			//		BackupRetentionPeriod:              oi.Instance.BackupRetentionPeriod,
			//		ClusterType:                        clusterType,
			//		PerformanceInsightsEnabled:         *oi.Instance.PerformanceInsightsEnabled,
			//		PerformanceInsightsRetentionPeriod: oi.Instance.PerformanceInsightsRetentionPeriod,
			//		StorageType:                        oi.Instance.StorageType,
			//		StorageSize:                        oi.Instance.AllocatedStorage,
			//		StorageIops:                        oi.Instance.Iops,
			//	},
			//	Metrics:     oi.Metrics,
			//	Region:      oi.Region,
			//	Preferences: preferences.Export(oi.Preferences),
			//	Loading:     true,
			//}
			//if oi.Instance.StorageThroughput != nil {
			//	floatThroughput := float64(*oi.Instance.StorageThroughput)
			//	req.Instance.StorageThroughput = &floatThroughput
			//}
			//_, err := kaytu.RDSInstanceWastageRequest(req, j.processor.kaytuAcccessToken)
			//if err != nil {
			//	return err
			//}
		}

		// just to show the loading
		j.processor.items[*oi.Cluster.DBClusterIdentifier] = oi
		j.processor.publishOptimizationItem(oi.ToOptimizationItem())
	}

	for _, cluster := range clusters {
		if i, ok := j.processor.items[*cluster.DBClusterIdentifier]; ok && i.LazyLoadingEnabled {
			continue
		}

		oi := j.processor.items[*cluster.DBClusterIdentifier]
		j.processor.jobQueue.Push(NewGetRDSInstanceMetricsJob(j.processor, j.region, cluster, oi.Instances))
	}

	return nil
}
