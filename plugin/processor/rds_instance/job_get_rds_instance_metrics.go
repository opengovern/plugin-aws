package rds_instance

import (
	"context"
	"fmt"
	types2 "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"strings"
	"time"
)

type GetRDSInstanceMetricsJob struct {
	instance types.DBInstance
	region   string

	processor *Processor
}

func NewGetRDSInstanceMetricsJob(processor *Processor, region string, instance types.DBInstance) *GetRDSInstanceMetricsJob {
	return &GetRDSInstanceMetricsJob{
		processor: processor,
		instance:  instance,
		region:    region,
	}
}

func (j *GetRDSInstanceMetricsJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          fmt.Sprintf("get_rds_metrics_%s", *j.instance.DBInstanceIdentifier),
		Description: fmt.Sprintf("Getting metrics of %s", *j.instance.DBInstanceIdentifier),
		MaxRetry:    0,
	}
}

func (j *GetRDSInstanceMetricsJob) Run(ctx context.Context) error {
	instanceMetrics := map[string][]types2.Datapoint{}
	cwTM99Metrics, err := j.processor.metricProvider.GetDayByDayMetrics(
		ctx,
		j.region,
		"AWS/RDS",
		[]string{
			"CPUUtilization",
			"FreeableMemory",
		},
		map[string][]string{
			"DBInstanceIdentifier": {*j.instance.DBInstanceIdentifier},
		},
		j.processor.observabilityDays,
		time.Minute,
		nil,
		[]string{"tm99"},
	)
	if err != nil {
		return err
	}
	for k, v := range cwTM99Metrics {
		for idx, vv := range v {
			tmp := vv.ExtendedStatistics["tm99"]
			vv.Average = &tmp
			v[idx] = vv
		}

		instanceMetrics[k] = v
	}

	cwMetrics, err := j.processor.metricProvider.GetDayByDayMetrics(
		ctx,
		j.region,
		"AWS/RDS",
		[]string{
			"FreeStorageSpace",
		},
		map[string][]string{
			"DBInstanceIdentifier": {*j.instance.DBInstanceIdentifier},
		},
		j.processor.observabilityDays,
		time.Minute,
		[]types2.Statistic{
			types2.StatisticAverage,
			types2.StatisticMaximum,
			types2.StatisticMinimum,
		},
		nil,
	)
	if err != nil {
		return err
	}

	throughputMetrics, err := j.processor.metricProvider.GetDayByDayMetrics(
		ctx,
		j.region,
		"AWS/RDS",
		[]string{
			"ReadThroughput",
			"WriteThroughput",
			"NetworkReceiveThroughput",
			"NetworkTransmitThroughput",
		},
		map[string][]string{
			"DBInstanceIdentifier": {*j.instance.DBInstanceIdentifier},
		},
		j.processor.observabilityDays,
		time.Minute,
		[]types2.Statistic{
			types2.StatisticAverage,
			types2.StatisticMaximum,
			types2.StatisticMinimum,
		},
		nil,
	)
	if err != nil {
		return err
	}

	for k, val := range throughputMetrics {
		throughputMetrics[k] = val
	}

	iopsMetrics, err := j.processor.metricProvider.GetDayByDayMetrics(
		ctx,
		j.region,
		"AWS/RDS",
		[]string{
			"ReadIOPS",
			"WriteIOPS",
		},
		map[string][]string{
			"DBInstanceIdentifier": {*j.instance.DBInstanceIdentifier},
		},
		j.processor.observabilityDays,
		time.Minute,
		[]types2.Statistic{
			types2.StatisticAverage,
			types2.StatisticMaximum,
			types2.StatisticMinimum,
		},
		nil,
	)
	if err != nil {
		return err
	}
	for k, val := range iopsMetrics {
		iopsMetrics[k] = val
	}

	var clusterMetrics map[string][]types2.Datapoint
	if j.instance.DBClusterIdentifier != nil && strings.Contains(strings.ToLower(*j.instance.Engine), "aurora") {
		clusterMetrics, err = j.processor.metricProvider.GetDayByDayMetrics(
			ctx,
			j.region,
			"AWS/RDS",
			[]string{
				"VolumeBytesUsed",
			},
			map[string][]string{
				"DBClusterIdentifier": {*j.instance.DBClusterIdentifier},
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
	}

	for k, v := range cwMetrics {
		instanceMetrics[k] = v
	}
	for k, v := range iopsMetrics {
		instanceMetrics[k] = v
	}
	for k, v := range throughputMetrics {
		instanceMetrics[k] = v
	}
	if clusterMetrics != nil {
		for k, v := range clusterMetrics {
			instanceMetrics[k] = v
		}
	}

	oi := RDSInstanceItem{
		Instance:            j.instance,
		Metrics:             instanceMetrics,
		Region:              j.region,
		OptimizationLoading: true,
		LazyLoadingEnabled:  false,
		Preferences:         j.processor.defaultPreferences,
	}

	j.processor.items.Set(*oi.Instance.DBInstanceIdentifier, oi)
	j.processor.publishOptimizationItem(oi.ToOptimizationItem())
	j.processor.UpdateSummary(*oi.Instance.DBInstanceIdentifier)
	if !oi.Skipped && !oi.LazyLoadingEnabled {
		j.processor.jobQueue.Push(NewOptimizeRDSJob(j.processor, oi))
	}
	return nil
}
