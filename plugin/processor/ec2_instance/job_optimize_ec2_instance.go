package ec2_instance

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/google/uuid"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/kaytu/pkg/utils"
	"github.com/kaytu-io/kaytu/preferences"
	kaytu2 "github.com/kaytu-io/plugin-aws/plugin/kaytu"
	"github.com/kaytu-io/plugin-aws/plugin/processor/shared"
	golang2 "github.com/kaytu-io/plugin-aws/plugin/proto/src/golang"
	"github.com/kaytu-io/plugin-aws/plugin/version"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type OptimizeEC2InstanceJob struct {
	processor *Processor
	item      EC2InstanceItem
}

func NewOptimizeEC2InstanceJob(processor *Processor, item EC2InstanceItem) *OptimizeEC2InstanceJob {
	return &OptimizeEC2InstanceJob{
		processor: processor,
		item:      item,
	}
}

func (j *OptimizeEC2InstanceJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          fmt.Sprintf("optimize_ec2_instance_%s", *j.item.Instance.InstanceId),
		Description: fmt.Sprintf("Optimizing %s", *j.item.Instance.InstanceId),
		MaxRetry:    3,
	}
}

func (j *OptimizeEC2InstanceJob) Run(ctx context.Context) error {
	if j.item.LazyLoadingEnabled {
		j.processor.jobQueue.Push(NewGetEC2InstanceMetricsJob(j.processor, j.item.Region, j.item.Instance, j.item.Image))
		return nil
	}

	var monitoring *types.MonitoringState
	if j.item.Instance.Monitoring != nil {
		monitoring = &j.item.Instance.Monitoring.State
	}
	var placement *kaytu2.EC2Placement
	if j.item.Instance.Placement != nil {
		placement = &kaytu2.EC2Placement{
			Tenancy: j.item.Instance.Placement.Tenancy,
		}
		if j.item.Instance.Placement.AvailabilityZone != nil {
			placement.AvailabilityZone = *j.item.Instance.Placement.AvailabilityZone
		}
		if j.item.Instance.Placement.HostId != nil {
			placement.HashedHostId = utils.HashString(*j.item.Instance.Placement.HostId)
		}
	}
	platform := ""
	if j.item.Instance.PlatformDetails != nil {
		platform = *j.item.Instance.PlatformDetails
	}

	var volumes []*golang2.EC2Volume
	for _, v := range j.item.Volumes {
		volumes = append(volumes, toEBSVolume(v))
	}
	reqID := uuid.New().String()

	var newMonitoring *wrapperspb.StringValue
	var newPlacement *golang2.EC2Placement
	if monitoring != nil {
		newMonitoring = wrapperspb.String(string(*monitoring))
	}
	if placement != nil {
		newPlacement = &golang2.EC2Placement{
			HashedHostId:     placement.HashedHostId,
			Tenancy:          string(placement.Tenancy),
			AvailabilityZone: placement.AvailabilityZone,
		}
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

	volumeMetrics := make(map[string]*golang2.VolumeMetrics)
	for vol, ms := range j.item.VolumeMetrics {
		volumeM := make(map[string]*golang2.Metric)
		for k, v := range ms {
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
			volumeM[k] = &golang2.Metric{
				Metric: data,
			}
		}
		volumeMetrics[vol] = &golang2.VolumeMetrics{
			Metrics: volumeM,
		}
	}

	grpcCtx := metadata.NewOutgoingContext(ctx, metadata.Pairs("workspace-name", "kaytu"))
	grpcCtx, cancel := context.WithTimeout(grpcCtx, shared.GrpcOptimizeRequestTimeout)
	defer cancel()
	res, err := j.processor.client.EC2InstanceOptimization(grpcCtx, &golang2.EC2InstanceOptimizationRequest{
		RequestId:      wrapperspb.String(reqID),
		CliVersion:     wrapperspb.String(version.VERSION),
		Identification: j.processor.identification,
		Instance: &golang2.EC2Instance{
			HashedInstanceId:  utils.HashString(*j.item.Instance.InstanceId),
			State:             string(j.item.Instance.State.Name),
			InstanceType:      string(j.item.Instance.InstanceType),
			Platform:          platform,
			ThreadsPerCore:    *j.item.Instance.CpuOptions.ThreadsPerCore,
			CoreCount:         *j.item.Instance.CpuOptions.CoreCount,
			EbsOptimized:      *j.item.Instance.EbsOptimized,
			InstanceLifecycle: string(j.item.Instance.InstanceLifecycle),
			Monitoring:        newMonitoring,
			Placement:         newPlacement,
			UsageOperation:    *j.item.Instance.UsageOperation,
			Tenancy:           string(j.item.Instance.Placement.Tenancy),
		},
		Volumes:       volumes,
		VolumeCount:   int64(len(volumes)),
		Metrics:       metrics,
		VolumeMetrics: volumeMetrics,
		Region:        j.item.Region,
		Preferences:   preferencesMap,
		Loading:       false,
	})
	if err != nil {
		return err
	}

	if res.RightSizing.Current.InstanceType == "" {
		j.item.OptimizationLoading = false
		j.processor.items.Set(*j.item.Instance.InstanceId, j.item)
		j.processor.publishOptimizationItem(j.item.ToOptimizationItem())
		j.processor.UpdateSummary(*j.item.Instance.InstanceId)
		return nil
	}

	j.item = EC2InstanceItem{
		Instance:            j.item.Instance,
		Image:               j.item.Image,
		Region:              j.item.Region,
		OptimizationLoading: false,
		Preferences:         j.item.Preferences,
		Skipped:             false,
		SkipReason:          "",
		Volumes:             j.item.Volumes,
		Metrics:             j.item.Metrics,
		VolumeMetrics:       j.item.VolumeMetrics,
		Wastage:             res,
	}
	j.processor.items.Set(*j.item.Instance.InstanceId, j.item)
	j.processor.publishOptimizationItem(j.item.ToOptimizationItem())
	j.processor.UpdateSummary(*j.item.Instance.InstanceId)
	return nil
}
