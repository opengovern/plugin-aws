package rds_instance

import (
	"fmt"
	types2 "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/kaytu-io/plugin-aws/plugin/aws"
	preferences2 "github.com/kaytu-io/plugin-aws/plugin/preferences"
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

func (j *GetRDSInstanceMetricsJob) Id() string {
	return fmt.Sprintf("get_rds_metrics_%s", *j.instance.DBInstanceIdentifier)
}
func (j *GetRDSInstanceMetricsJob) Description() string {
	return fmt.Sprintf("Getting metrics of %s", *j.instance.DBInstanceIdentifier)
}
func (j *GetRDSInstanceMetricsJob) Run() error {
	startTime := time.Now().Add(-24 * 1 * time.Hour)
	endTime := time.Now()

	instanceMetrics := map[string][]types2.Datapoint{}
	cwTM99Metrics, err := j.processor.metricProvider.GetMetrics(
		j.region,
		"AWS/RDS",
		[]string{
			"CPUUtilization",
			"FreeableMemory",
		},
		map[string][]string{
			"DBInstanceIdentifier": {*j.instance.DBInstanceIdentifier},
		},
		startTime, endTime,
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

	cwMetrics, err := j.processor.metricProvider.GetMetrics(
		j.region,
		"AWS/RDS",
		[]string{
			"FreeStorageSpace",
		},
		map[string][]string{
			"DBInstanceIdentifier": {*j.instance.DBInstanceIdentifier},
		},
		startTime, endTime,
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

	throughputMetrics, err := j.processor.metricProvider.GetMetrics(
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
		startTime, endTime,
		time.Minute,
		[]types2.Statistic{
			types2.StatisticSum,
		},
		nil,
	)
	if err != nil {
		return err
	}

	for k, val := range throughputMetrics {
		throughputMetrics[k] = aws.GetDatapointsAvgFromSum(val, 1)
	}

	iopsMetrics, err := j.processor.metricProvider.GetDayByDayMetrics(
		j.region,
		"AWS/RDS",
		[]string{
			"ReadIOPS",
			"WriteIOPS",
		},
		map[string][]string{
			"DBInstanceIdentifier": {*j.instance.DBInstanceIdentifier},
		},
		1,
		time.Minute,
		[]types2.Statistic{
			types2.StatisticSum,
		},
		nil,
	)
	if err != nil {
		return err
	}
	for k, val := range iopsMetrics {
		iopsMetrics[k] = aws.GetDatapointsAvgFromSum(val, 1)
	}

	var clusterMetrics map[string][]types2.Datapoint
	if j.instance.DBClusterIdentifier != nil && strings.Contains(strings.ToLower(*j.instance.Engine), "aurora") {
		clusterMetrics, err = j.processor.metricProvider.GetMetrics(
			j.region,
			"AWS/RDS",
			[]string{
				"VolumeBytesUsed",
			},
			map[string][]string{
				"DBClusterIdentifier": {*j.instance.DBClusterIdentifier},
			},
			startTime, endTime,
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
		Preferences:         preferences2.DefaultRDSPreferences,
	}

	j.processor.items.Set(*oi.Instance.DBInstanceIdentifier, oi)
	j.processor.publishOptimizationItem(oi.ToOptimizationItem())
	if !oi.Skipped && !oi.LazyLoadingEnabled {
		j.processor.jobQueue.Push(NewOptimizeRDSJob(j.processor, oi))
	}
	return nil
}
