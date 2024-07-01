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
	"github.com/kaytu-io/plugin-aws/plugin/version"
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

	var volumes []kaytu2.EC2Volume
	for _, v := range j.item.Volumes {
		volumes = append(volumes, toEBSVolume(v))
	}
	reqID := uuid.New().String()
	req := kaytu2.EC2InstanceWastageRequest{
		RequestId:      &reqID,
		CliVersion:     &version.VERSION,
		Identification: j.processor.identification,
		Instance: kaytu2.EC2Instance{
			HashedInstanceId:  utils.HashString(*j.item.Instance.InstanceId),
			State:             j.item.Instance.State.Name,
			InstanceType:      j.item.Instance.InstanceType,
			Platform:          platform,
			ThreadsPerCore:    *j.item.Instance.CpuOptions.ThreadsPerCore,
			CoreCount:         *j.item.Instance.CpuOptions.CoreCount,
			EbsOptimized:      *j.item.Instance.EbsOptimized,
			InstanceLifecycle: j.item.Instance.InstanceLifecycle,
			Monitoring:        monitoring,
			Placement:         placement,
			UsageOperation:    *j.item.Instance.UsageOperation,
			Tenancy:           j.item.Instance.Placement.Tenancy,
		},
		Volumes:       volumes,
		VolumeCount:   len(volumes),
		Metrics:       j.item.Metrics,
		VolumeMetrics: j.item.VolumeMetrics,
		Region:        j.item.Region,
		Preferences:   preferences.Export(j.item.Preferences),
		Loading:       false,
	}
	res, err := kaytu2.Ec2InstanceWastageRequest(ctx, req, j.processor.kaytuAcccessToken)
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
		Wastage:             *res,
	}
	j.processor.items.Set(*j.item.Instance.InstanceId, j.item)
	j.processor.publishOptimizationItem(j.item.ToOptimizationItem())
	j.processor.UpdateSummary(*j.item.Instance.InstanceId)
	return nil
}
