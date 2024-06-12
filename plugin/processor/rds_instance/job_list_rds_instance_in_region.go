package rds_instance

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/google/uuid"
	"github.com/kaytu-io/kaytu/pkg/utils"
	"github.com/kaytu-io/kaytu/preferences"
	"github.com/kaytu-io/plugin-aws/plugin/kaytu"
	preferences2 "github.com/kaytu-io/plugin-aws/plugin/preferences"
	"github.com/kaytu-io/plugin-aws/plugin/version"
	"strings"
)

type ListRDSInstancesInRegionJob struct {
	region    string
	processor *Processor
}

func NewListRDSInstancesInRegionJob(processor *Processor, region string) *ListRDSInstancesInRegionJob {
	return &ListRDSInstancesInRegionJob{
		processor: processor,
		region:    region,
	}
}

func (j *ListRDSInstancesInRegionJob) Id() string {
	return fmt.Sprintf("list_rds_in_%s", j.region)
}
func (j *ListRDSInstancesInRegionJob) Description() string {
	return fmt.Sprintf("Listing all RDS Instances in %s", j.region)
}
func (j *ListRDSInstancesInRegionJob) Run() error {
	instances, err := j.processor.provider.ListRDSInstance(j.region)
	if err != nil {
		return err
	}

	for _, instance := range instances {
		if instance.DBClusterIdentifier != nil {
			continue
		}

		oi := RDSInstanceItem{
			Instance:            instance,
			Region:              j.region,
			OptimizationLoading: true,
			LazyLoadingEnabled:  false,
			Preferences:         preferences2.DefaultRDSPreferences,
		}

		if strings.Contains(strings.ToLower(*instance.Engine), "docdb") {
			oi.Skipped = true
			oi.SkipReason = "docdb instance"
		}

		if !oi.Skipped {
			j.processor.lazyloadCounter.Increment()
			if j.processor.lazyloadCounter.Get() > j.processor.configuration.RDSLazyLoad {
				oi.LazyLoadingEnabled = true
			}

			var clusterType kaytu.AwsRdsClusterType
			multiAZ := oi.Instance.MultiAZ != nil && *oi.Instance.MultiAZ
			readableStandbys := oi.Instance.ReplicaMode == types.ReplicaModeOpenReadOnly
			if multiAZ && readableStandbys {
				clusterType = kaytu.AwsRdsClusterTypeMultiAzTwoInstance
			} else if multiAZ {
				clusterType = kaytu.AwsRdsClusterTypeMultiAzOneInstance
			} else {
				clusterType = kaytu.AwsRdsClusterTypeSingleInstance
			}

			reqID := uuid.New().String()
			req := kaytu.AwsRdsWastageRequest{
				RequestId:      &reqID,
				CliVersion:     &version.VERSION,
				Identification: j.processor.identification,
				Instance: kaytu.AwsRds{
					HashedInstanceId:                   utils.HashString(*oi.Instance.DBInstanceIdentifier),
					AvailabilityZone:                   *oi.Instance.AvailabilityZone,
					InstanceType:                       *oi.Instance.DBInstanceClass,
					Engine:                             *oi.Instance.Engine,
					EngineVersion:                      *oi.Instance.EngineVersion,
					LicenseModel:                       *oi.Instance.LicenseModel,
					BackupRetentionPeriod:              oi.Instance.BackupRetentionPeriod,
					ClusterType:                        clusterType,
					PerformanceInsightsEnabled:         *oi.Instance.PerformanceInsightsEnabled,
					PerformanceInsightsRetentionPeriod: oi.Instance.PerformanceInsightsRetentionPeriod,
					StorageType:                        oi.Instance.StorageType,
					StorageSize:                        oi.Instance.AllocatedStorage,
					StorageIops:                        oi.Instance.Iops,
				},
				Metrics:     oi.Metrics,
				Region:      oi.Region,
				Preferences: preferences.Export(oi.Preferences),
				Loading:     true,
			}
			if oi.Instance.StorageThroughput != nil {
				floatThroughput := float64(*oi.Instance.StorageThroughput)
				req.Instance.StorageThroughput = &floatThroughput
			}
			_, err := kaytu.RDSInstanceWastageRequest(req, j.processor.kaytuAcccessToken)
			if err != nil {
				return err
			}
		}

		// just to show the loading
		j.processor.items.Set(*oi.Instance.DBInstanceIdentifier, oi)
		j.processor.publishOptimizationItem(oi.ToOptimizationItem())
		j.processor.UpdateSummary(*oi.Instance.DBInstanceIdentifier)
	}

	for _, instance := range instances {
		if instance.DBClusterIdentifier != nil {
			continue
		}

		if i, ok := j.processor.items.Get(*instance.DBInstanceIdentifier); ok && i.LazyLoadingEnabled {
			continue
		}

		j.processor.jobQueue.Push(NewGetRDSInstanceMetricsJob(j.processor, j.region, instance))
	}

	return nil
}
