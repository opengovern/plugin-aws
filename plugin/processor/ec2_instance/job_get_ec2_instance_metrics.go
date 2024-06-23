package ec2_instance

import (
	"context"
	"fmt"
	types2 "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/kaytu-io/kaytu/pkg/utils"
	aws2 "github.com/kaytu-io/plugin-aws/plugin/aws"
	"time"
)

type GetEC2InstanceMetricsJob struct {
	instance types.Instance
	image    *types.Image
	region   string

	processor *Processor
}

func NewGetEC2InstanceMetricsJob(processor *Processor, region string, instance types.Instance, image *types.Image) *GetEC2InstanceMetricsJob {
	return &GetEC2InstanceMetricsJob{
		processor: processor,
		instance:  instance,
		image:     image,
		region:    region,
	}
}

func (j *GetEC2InstanceMetricsJob) Id() string {
	return fmt.Sprintf("get_ec2_instance_metrics_%s", *j.instance.InstanceId)
}
func (j *GetEC2InstanceMetricsJob) Description() string {
	return fmt.Sprintf("Getting metrics of %s", *j.instance.InstanceId)
}
func (j *GetEC2InstanceMetricsJob) Run(ctx context.Context) error {
	isAutoScaling := false
	for _, tag := range j.instance.Tags {
		if *tag.Key == "aws:autoscaling:groupName" && tag.Value != nil && *tag.Value != "" {
			isAutoScaling = true
		}
	}
	if j.instance.State.Name != types.InstanceStateNameRunning ||
		j.instance.InstanceLifecycle == types.InstanceLifecycleTypeSpot ||
		isAutoScaling {
		return nil
	}

	volumes, err := j.processor.provider.ListAttachedVolumes(ctx, j.region, j.instance)
	if err != nil {
		return err
	}

	instanceMetrics := map[string][]types2.Datapoint{}

	cwMetrics, err := j.processor.metricProvider.GetDayByDayMetrics(
		ctx,
		j.region,
		"AWS/EC2",
		[]string{
			"CPUUtilization",
		},
		map[string][]string{
			"InstanceId": {*j.instance.InstanceId},
		},
		j.processor.observabilityDays,
		time.Minute,
		nil,
		[]string{"tm99"},
	)
	if err != nil {
		return err
	}
	for k, v := range cwMetrics {
		for idx, vv := range v {
			tmp := vv.ExtendedStatistics["tm99"]
			vv.Average = &tmp
			v[idx] = vv
		}

		instanceMetrics[k] = v
	}

	cwPerSecondMetrics, err := j.processor.metricProvider.GetDayByDayMetrics(
		ctx,
		j.region,
		"AWS/EC2",
		[]string{
			"NetworkIn",
			"NetworkOut",
		},
		map[string][]string{
			"InstanceId": {*j.instance.InstanceId},
		},
		j.processor.observabilityDays,
		time.Minute,
		[]types2.Statistic{
			types2.StatisticSum,
			types2.StatisticSampleCount,
		},
		nil,
	)
	if err != nil {
		return err
	}
	for k, v := range cwPerSecondMetrics {
		instanceMetrics[k] = aws2.GetDatapointsAvgFromSum(v, 60)
	}

	cwaMetrics, err := j.processor.metricProvider.GetDayByDayMetrics(
		ctx,
		j.region,
		"CWAgent",
		[]string{
			"mem_used_percent",
		},
		map[string][]string{
			"InstanceId": {*j.instance.InstanceId},
		},
		j.processor.observabilityDays,
		time.Minute,
		[]types2.Statistic{
			types2.StatisticAverage,
			types2.StatisticMaximum,
		},
		nil,
	)
	if err != nil {
		return err
	}
	for k, v := range cwaMetrics {
		instanceMetrics[k] = v
	}

	var volumeIDs []string
	for _, v := range j.instance.BlockDeviceMappings {
		if v.Ebs != nil {
			volumeIDs = append(volumeIDs, *v.Ebs.VolumeId)
		}
	}

	volumeMetrics := map[string]map[string][]types2.Datapoint{}
	for _, v := range volumeIDs {
		volumeMetricsMap, err := j.processor.metricProvider.GetDayByDayMetrics(
			ctx,
			j.region,
			"AWS/EBS",
			[]string{
				"VolumeReadBytes",
				"VolumeWriteBytes",
			},
			map[string][]string{
				"VolumeId": {v},
			},
			j.processor.observabilityDays,
			time.Minute,
			[]types2.Statistic{
				types2.StatisticSum,
				types2.StatisticSampleCount,
			},
			nil,
		)
		if err != nil {
			return err
		}

		for k, val := range volumeMetricsMap {
			volumeMetricsMap[k] = aws2.GetDatapointsAvgFromSumPeriod(val, int32(time.Minute/time.Second))
		}

		volumeIops, err := j.processor.metricProvider.GetDayByDayMetrics(
			ctx,
			j.region,
			"AWS/EBS",
			[]string{
				"VolumeReadOps",
				"VolumeWriteOps",
			},
			map[string][]string{
				"VolumeId": {v},
			},
			j.processor.observabilityDays,
			time.Minute,
			[]types2.Statistic{
				types2.StatisticSum,
			},
			nil,
		)
		if err != nil {
			return err
		}

		for k, val := range volumeIops {
			val = aws2.GetDatapointsAvgFromSumPeriod(val, int32(time.Minute/time.Second))
			volumeMetricsMap[k] = val
		}

		// Hash v
		hashedId := utils.HashString(v)
		volumeMetrics[hashedId] = volumeMetricsMap
	}

	oi := EC2InstanceItem{
		Instance:            j.instance,
		Image:               j.image,
		Volumes:             volumes,
		Metrics:             instanceMetrics,
		VolumeMetrics:       volumeMetrics,
		Region:              j.region,
		OptimizationLoading: true,
		LazyLoadingEnabled:  false,
		Preferences:         j.processor.defaultPreferences,
	}
	if j.instance.State.Name != types.InstanceStateNameRunning ||
		j.instance.InstanceLifecycle == types.InstanceLifecycleTypeSpot ||
		isAutoScaling {
		oi.OptimizationLoading = false
		oi.Skipped = true
		reason := ""
		if j.instance.State.Name != types.InstanceStateNameRunning {
			reason = "not running"
		} else if j.instance.InstanceLifecycle == types.InstanceLifecycleTypeSpot {
			reason = "spot instance"
		} else if isAutoScaling {
			reason = "auto-scaling group instance"
		}
		if len(reason) > 0 {
			oi.SkipReason = reason
		}
	}
	j.processor.items.Set(*oi.Instance.InstanceId, oi)
	j.processor.publishOptimizationItem(oi.ToOptimizationItem())
	j.processor.UpdateSummary(*oi.Instance.InstanceId)
	if !oi.Skipped && !oi.LazyLoadingEnabled {
		j.processor.jobQueue.Push(NewOptimizeEC2InstanceJob(j.processor, oi))
	}
	return nil
}
