package rds_instance

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/google/uuid"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/kaytu/pkg/utils"
	"github.com/kaytu-io/kaytu/preferences"
	"github.com/opengovern/plugin-aws/plugin/kaytu"
	"github.com/opengovern/plugin-aws/plugin/processor/shared"
	golang2 "github.com/opengovern/plugin-aws/plugin/proto/src/golang"
	"github.com/opengovern/plugin-aws/plugin/version"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/wrapperspb"
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

func (j *OptimizeRDSInstanceJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          fmt.Sprintf("optimize_rds_%s", *j.item.Instance.DBInstanceIdentifier),
		Description: fmt.Sprintf("Optimizing %s", *j.item.Instance.DBInstanceIdentifier),
		MaxRetry:    3,
	}
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

	var storageThroughput *float64
	if j.item.Instance.StorageThroughput != nil {
		floatThroughput := float64(*j.item.Instance.StorageThroughput)
		storageThroughput = &floatThroughput
	}

	preferencesMap := map[string]*wrapperspb.StringValue{}
	for k, v := range preferences.Export(j.item.Preferences) {
		preferencesMap[k] = nil
		if v != nil {
			preferencesMap[k] = wrapperspb.String(*v)
		}
	}

	metrics := make(map[string]*golang2.Metric)
	for k, v := range j.item.Metrics {
		var data []*golang2.Datapoint
		for _, d := range v {
			data = append(data, &golang2.Datapoint{
				Average:     shared.Float64ToWrapper(d.Average),
				Maximum:     shared.Float64ToWrapper(d.Maximum),
				Minimum:     shared.Float64ToWrapper(d.Minimum),
				SampleCount: shared.Float64ToWrapper(d.SampleCount),
				Sum:         shared.Float64ToWrapper(d.Sum),
				Timestamp:   shared.TimeToTimestamp(d.Timestamp),
			})
		}
		metrics[k] = &golang2.Metric{
			Metric: data,
		}
	}

	grpcCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("workspace-name", "kaytu"))
	grpcCtx, cancel := context.WithTimeout(grpcCtx, shared.GrpcOptimizeRequestTimeout)
	defer cancel()
	res, err := j.processor.client.RDSInstanceOptimization(grpcCtx, &golang2.RDSInstanceOptimizationRequest{
		RequestId:      wrapperspb.String(reqID),
		CliVersion:     wrapperspb.String(version.VERSION),
		Identification: j.processor.identification,
		Instance: &golang2.RDSInstance{
			HashedInstanceId:                   utils.HashString(*j.item.Instance.DBInstanceIdentifier),
			AvailabilityZone:                   *j.item.Instance.AvailabilityZone,
			InstanceType:                       *j.item.Instance.DBInstanceClass,
			Engine:                             *j.item.Instance.Engine,
			EngineVersion:                      *j.item.Instance.EngineVersion,
			LicenseModel:                       *j.item.Instance.LicenseModel,
			BackupRetentionPeriod:              shared.Int32ToWrapper(j.item.Instance.BackupRetentionPeriod),
			ClusterType:                        string(clusterType),
			PerformanceInsightsEnabled:         *j.item.Instance.PerformanceInsightsEnabled,
			PerformanceInsightsRetentionPeriod: shared.Int32ToWrapper(j.item.Instance.PerformanceInsightsRetentionPeriod),
			StorageType:                        shared.StringToWrapper(j.item.Instance.StorageType),
			StorageSize:                        shared.Int32ToWrapper(j.item.Instance.AllocatedStorage),
			StorageIops:                        shared.Int32ToWrapper(j.item.Instance.Iops),
			StorageThroughput:                  shared.Float64ToWrapper(storageThroughput),
		},
		Metrics:     metrics,
		Region:      j.item.Region,
		Preferences: preferencesMap,
		Loading:     false,
	})
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
		Wastage:             res,
	}
	j.processor.items.Set(*j.item.Instance.DBInstanceIdentifier, j.item)
	j.processor.publishOptimizationItem(j.item.ToOptimizationItem())
	j.processor.UpdateSummary(*j.item.Instance.DBInstanceIdentifier)
	return nil
}
