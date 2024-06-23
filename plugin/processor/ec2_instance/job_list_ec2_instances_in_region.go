package ec2_instance

import (
	"context"
	"fmt"
	types2 "github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/google/uuid"
	"github.com/kaytu-io/kaytu/pkg/utils"
	"github.com/kaytu-io/kaytu/preferences"
	kaytu2 "github.com/kaytu-io/plugin-aws/plugin/kaytu"
	"github.com/kaytu-io/plugin-aws/plugin/version"
)

type ListEC2InstancesInRegionJob struct {
	region    string
	processor *Processor
}

func NewListEC2InstancesInRegionJob(processor *Processor, region string) *ListEC2InstancesInRegionJob {
	return &ListEC2InstancesInRegionJob{
		processor: processor,
		region:    region,
	}
}

func (j *ListEC2InstancesInRegionJob) Id() string {
	return fmt.Sprintf("list_ec2_instances_in_%s", j.region)
}
func (j *ListEC2InstancesInRegionJob) Description() string {
	return fmt.Sprintf("Listing all EC2 Instances in %s", j.region)
}
func (j *ListEC2InstancesInRegionJob) Run(ctx context.Context) error {
	instances, err := j.processor.provider.ListInstances(ctx, j.region)
	if err != nil {
		return err
	}

	for _, instance := range instances {
		oi := EC2InstanceItem{
			Instance:            instance,
			Region:              j.region,
			OptimizationLoading: true,
			LazyLoadingEnabled:  false,
			Preferences:         j.processor.defaultPreferences,
		}

		isAutoScaling := false
		for _, tag := range instance.Tags {
			if *tag.Key == "aws:autoscaling:groupName" && tag.Value != nil && *tag.Value != "" {
				isAutoScaling = true
			}
		}
		if instance.State.Name != types2.InstanceStateNameRunning ||
			instance.InstanceLifecycle == types2.InstanceLifecycleTypeSpot ||
			isAutoScaling {
			oi.OptimizationLoading = false
			oi.Skipped = true
			reason := ""
			if instance.State.Name != types2.InstanceStateNameRunning {
				if instance.State.Name == types2.InstanceStateNameTerminated || instance.State.Name == types2.InstanceStateNameStopped {
					continue
				}
				reason = "not running"
			} else if instance.InstanceLifecycle == types2.InstanceLifecycleTypeSpot {
				reason = "spot instance"
			} else if isAutoScaling {
				reason = "auto-scaling group instance"
			}
			if len(reason) > 0 {
				oi.SkipReason = reason
			}
		}

		if !oi.Skipped {
			j.processor.lazyloadCounter.Add(1)
			if j.processor.lazyloadCounter.Load() > uint32(j.processor.configuration.EC2LazyLoad) {
				oi.LazyLoadingEnabled = true
			}
		}

		if !oi.Skipped {
			var monitoring *types2.MonitoringState
			if oi.Instance.Monitoring != nil {
				monitoring = &oi.Instance.Monitoring.State
			}
			var placement *kaytu2.EC2Placement
			if oi.Instance.Placement != nil {
				placement = &kaytu2.EC2Placement{
					Tenancy: oi.Instance.Placement.Tenancy,
				}
				if oi.Instance.Placement.AvailabilityZone != nil {
					placement.AvailabilityZone = *oi.Instance.Placement.AvailabilityZone
				}
				if oi.Instance.Placement.HostId != nil {
					placement.HashedHostId = utils.HashString(*oi.Instance.Placement.HostId)
				}
			}
			platform := ""
			if oi.Instance.PlatformDetails != nil {
				platform = *oi.Instance.PlatformDetails
			}
			reqID := uuid.New().String()
			_, err := kaytu2.Ec2InstanceWastageRequest(ctx, kaytu2.EC2InstanceWastageRequest{
				RequestId:      &reqID,
				CliVersion:     &version.VERSION,
				Identification: j.processor.identification,
				Instance: kaytu2.EC2Instance{
					HashedInstanceId:  utils.HashString(*oi.Instance.InstanceId),
					State:             oi.Instance.State.Name,
					InstanceType:      oi.Instance.InstanceType,
					Platform:          platform,
					ThreadsPerCore:    *oi.Instance.CpuOptions.ThreadsPerCore,
					CoreCount:         *oi.Instance.CpuOptions.CoreCount,
					EbsOptimized:      *oi.Instance.EbsOptimized,
					InstanceLifecycle: oi.Instance.InstanceLifecycle,
					Monitoring:        monitoring,
					Placement:         placement,
					UsageOperation:    *oi.Instance.UsageOperation,
					Tenancy:           oi.Instance.Placement.Tenancy,
				},
				Volumes:       nil,
				VolumeCount:   len(instance.BlockDeviceMappings),
				Metrics:       nil,
				VolumeMetrics: nil,
				Region:        oi.Region,
				Preferences:   preferences.Export(oi.Preferences),
				Loading:       true,
			}, j.processor.kaytuAcccessToken)
			if err != nil {
				return err
			}
		}

		// just to show the loading
		j.processor.items.Set(*oi.Instance.InstanceId, oi)
		j.processor.publishOptimizationItem(oi.ToOptimizationItem())
		j.processor.UpdateSummary(*oi.Instance.InstanceId)
	}

	for _, instance := range instances {
		i, ok := j.processor.items.Get(*instance.InstanceId)
		if ok && (i.LazyLoadingEnabled || !i.OptimizationLoading || i.Skipped) {
			continue
		}

		//TODO-Saleh since we're doing these one by one if user runs the lazy loading item it gets re-run here as well because lazy loading enabled is false now.
		j.processor.jobQueue.Push(NewGetEC2InstanceMetricsJob(j.processor, j.region, instance))
	}

	return nil
}
