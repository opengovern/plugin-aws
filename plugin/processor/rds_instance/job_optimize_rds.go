package rds_instance

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/google/uuid"
	"github.com/kaytu-io/kaytu/pkg/utils"
	"github.com/kaytu-io/kaytu/preferences"
	"github.com/kaytu-io/plugin-aws/plugin/kaytu"
	"github.com/kaytu-io/plugin-aws/plugin/version"
)

type OptimizeRDSInstanceJob struct {
	processor *Processor
	item      RDSInstanceItem
}

func NewOptimizeRDSJob(processor *Processor, item RDSInstanceItem) *OptimizeRDSInstanceJob {
	return &OptimizeRDSInstanceJob{
		processor: processor,
		item:      item,
	}
}

func (j *OptimizeRDSInstanceJob) Id() string {
	return fmt.Sprintf("optimize_rds_%s", *j.item.Instance.DBInstanceIdentifier)
}
func (j *OptimizeRDSInstanceJob) Description() string {
	return fmt.Sprintf("Optimizing %s", *j.item.Instance.DBInstanceIdentifier)
}
func (j *OptimizeRDSInstanceJob) Run(ctx context.Context) error {
	if j.item.LazyLoadingEnabled {
		j.processor.jobQueue.Push(NewGetRDSInstanceMetricsJob(j.processor, j.item.Region, j.item.Instance))
		return nil
	}

	var clusterType kaytu.AwsRdsClusterType
	multiAZ := j.item.Instance.MultiAZ != nil && *j.item.Instance.MultiAZ
	readableStandbys := j.item.Instance.ReplicaMode == types.ReplicaModeOpenReadOnly
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
			HashedInstanceId:                   utils.HashString(*j.item.Instance.DBInstanceIdentifier),
			AvailabilityZone:                   *j.item.Instance.AvailabilityZone,
			InstanceType:                       *j.item.Instance.DBInstanceClass,
			Engine:                             *j.item.Instance.Engine,
			EngineVersion:                      *j.item.Instance.EngineVersion,
			LicenseModel:                       *j.item.Instance.LicenseModel,
			BackupRetentionPeriod:              j.item.Instance.BackupRetentionPeriod,
			ClusterType:                        clusterType,
			PerformanceInsightsEnabled:         *j.item.Instance.PerformanceInsightsEnabled,
			PerformanceInsightsRetentionPeriod: j.item.Instance.PerformanceInsightsRetentionPeriod,
			StorageType:                        j.item.Instance.StorageType,
			StorageSize:                        j.item.Instance.AllocatedStorage,
			StorageIops:                        j.item.Instance.Iops,
		},
		Metrics:     j.item.Metrics,
		Region:      j.item.Region,
		Preferences: preferences.Export(j.item.Preferences),
		Loading:     false,
	}
	if j.item.Instance.StorageThroughput != nil {
		floatThroughput := float64(*j.item.Instance.StorageThroughput)
		req.Instance.StorageThroughput = &floatThroughput
	}
	res, err := kaytu.RDSInstanceWastageRequest(req, j.processor.kaytuAcccessToken)
	if err != nil {
		return err
	}

	if res.RightSizing.Current.InstanceType == "" {
		j.item.OptimizationLoading = false
		j.processor.items.Set(*j.item.Instance.DBInstanceIdentifier, j.item)
		j.processor.publishOptimizationItem(j.item.ToOptimizationItem())
		j.processor.UpdateSummary(*j.item.Instance.DBInstanceIdentifier)
		return nil
	}

	j.item = RDSInstanceItem{
		Instance:            j.item.Instance,
		Region:              j.item.Region,
		OptimizationLoading: false,
		Preferences:         j.item.Preferences,
		Skipped:             false,
		SkipReason:          "",
		Metrics:             j.item.Metrics,
		Wastage:             *res,
	}
	j.processor.items.Set(*j.item.Instance.DBInstanceIdentifier, j.item)
	j.processor.publishOptimizationItem(j.item.ToOptimizationItem())
	j.processor.UpdateSummary(*j.item.Instance.DBInstanceIdentifier)
	return nil
}
