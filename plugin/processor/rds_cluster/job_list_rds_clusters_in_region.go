package rds_cluster

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/google/uuid"
	"github.com/kaytu-io/kaytu/pkg/utils"
	"github.com/kaytu-io/kaytu/preferences"
	"github.com/kaytu-io/plugin-aws/plugin/kaytu"
	preferences2 "github.com/kaytu-io/plugin-aws/plugin/preferences"
	"github.com/kaytu-io/plugin-aws/plugin/version"
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

		if cluster.ServerlessV2ScalingConfiguration != nil {
			oi.Skipped = true
			oi.SkipReason = "serverless cluster"
		}

		if !oi.Skipped {
			j.processor.lazyloadCounter.Increment()
			if j.processor.lazyloadCounter.Get() > j.processor.configuration.RDSLazyLoad {
				oi.LazyLoadingEnabled = true
			}

			reqID := uuid.New().String()
			var reqInstances []kaytu.AwsRds
			for _, i := range oi.Instances {
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

				reqInstances = append(reqInstances, rdsInstance)
			}

			req := kaytu.AwsClusterWastageRequest{
				RequestId:      &reqID,
				CliVersion:     &version.VERSION,
				Identification: j.processor.identification,
				Cluster: kaytu.AwsRdsCluster{
					HashedClusterId: utils.HashString(*oi.Cluster.DBClusterIdentifier),
					Engine:          *oi.Cluster.Engine,
				},
				Instances:   reqInstances,
				Metrics:     oi.Metrics,
				Region:      oi.Region,
				Preferences: preferences.Export(oi.Preferences),
				Loading:     true,
			}
			_, err := kaytu.RDSClusterWastageRequest(req, j.processor.kaytuAcccessToken)
			if err != nil {
				return err
			}
		}

		// just to show the loading
		j.processor.items.Set(*oi.Cluster.DBClusterIdentifier, oi)
		j.processor.publishOptimizationItem(oi.ToOptimizationItem())
	}

	for _, cluster := range clusters {
		if i, ok := j.processor.items.Get(*cluster.DBClusterIdentifier); ok && (i.LazyLoadingEnabled || i.Skipped) {
			continue
		}

		oi, _ := j.processor.items.Get(*cluster.DBClusterIdentifier)
		if oi.Skipped {
			continue
		}
		j.processor.jobQueue.Push(NewGetRDSInstanceMetricsJob(j.processor, j.region, cluster, oi.Instances))
	}

	return nil
}
