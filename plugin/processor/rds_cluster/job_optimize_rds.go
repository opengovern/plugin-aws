package rds_cluster

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

func (j *OptimizeRDSClusterJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          fmt.Sprintf("optimize_rds_cluster_%s", *j.item.Cluster.DBClusterIdentifier),
		Description: fmt.Sprintf("Optimizing %s", *j.item.Cluster.DBClusterIdentifier),
		MaxRetry:    3,
	}
}

func (j *OptimizeRDSClusterJob) Run(ctx context.Context) error {
	if j.item.LazyLoadingEnabled {
		j.processor.jobQueue.Push(NewGetRDSInstanceMetricsJob(j.processor, j.item.Region, j.item.Cluster, j.item.Instances))
		return nil
	}

	reqID := uuid.New().String()
	var instances []*golang2.RDSInstance
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

		rdsInstance := golang2.RDSInstance{
			HashedInstanceId:                   utils.HashString(*i.DBInstanceIdentifier),
			AvailabilityZone:                   *i.AvailabilityZone,
			InstanceType:                       *i.DBInstanceClass,
			Engine:                             *i.Engine,
			EngineVersion:                      *i.EngineVersion,
			LicenseModel:                       *i.LicenseModel,
			BackupRetentionPeriod:              shared.Int32ToWrapper(i.BackupRetentionPeriod),
			ClusterType:                        string(clusterType),
			PerformanceInsightsEnabled:         *i.PerformanceInsightsEnabled,
			PerformanceInsightsRetentionPeriod: shared.Int32ToWrapper(i.PerformanceInsightsRetentionPeriod),
			StorageType:                        shared.StringToWrapper(i.StorageType),
			StorageSize:                        shared.Int32ToWrapper(i.AllocatedStorage),
			StorageIops:                        shared.Int32ToWrapper(i.Iops),
		}
		if i.StorageThroughput != nil {
			floatThroughput := float64(*i.StorageThroughput)
			rdsInstance.StorageThroughput = shared.Float64ToWrapper(&floatThroughput)
		}

		instances = append(instances, &rdsInstance)
	}

	preferencesMap := map[string]*wrapperspb.StringValue{}
	for k, v := range preferences.Export(j.item.Preferences) {
		preferencesMap[k] = nil
		if v != nil {
			preferencesMap[k] = wrapperspb.String(*v)
		}
	}

	metrics := make(map[string]*golang2.RDSClusterMetrics)
	for instance, m := range j.item.Metrics {
		instanceMetrics := make(map[string]*golang2.Metric)
		for k, v := range m {
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
			instanceMetrics[k] = &golang2.Metric{
				Metric: data,
			}
		}
		metrics[instance] = &golang2.RDSClusterMetrics{
			Metrics: instanceMetrics,
		}
	}

	grpcCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("workspace-name", "kaytu"))
	grpcCtx, cancel := context.WithTimeout(grpcCtx, shared.GrpcOptimizeRequestTimeout)
	defer cancel()
	res, err := j.processor.client.RDSClusterOptimization(grpcCtx, &golang2.RDSClusterOptimizationRequest{
		RequestId:      wrapperspb.String(reqID),
		CliVersion:     wrapperspb.String(version.VERSION),
		Identification: j.processor.identification,
		Cluster: &golang2.RDSCluster{
			HashedClusterId: utils.HashString(*j.item.Cluster.DBClusterIdentifier),
			Engine:          *j.item.Cluster.Engine,
		},
		Instances:   instances,
		Metrics:     metrics,
		Region:      j.item.Region,
		Preferences: preferencesMap,
		Loading:     false,
	})
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
		Wastage:             res,
	}
	j.processor.items.Set(*j.item.Cluster.DBClusterIdentifier, j.item)
	j.processor.publishOptimizationItem(j.item.ToOptimizationItem())
	j.processor.UpdateSummary(*j.item.Cluster.DBClusterIdentifier)
	return nil
}
