package rds_cluster

import (
	"context"
	"fmt"
	types2 "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/kaytu/pkg/utils"
	"strings"
	"time"
)

type GetRDSClusterMetricsJob struct {
	cluster   types.DBCluster
	instances []types.DBInstance
	region    string

	processor *Processor
}

func NewGetRDSInstanceMetricsJob(processor *Processor, region string, cluster types.DBCluster, instances []types.DBInstance) *GetRDSClusterMetricsJob {
	return &GetRDSClusterMetricsJob{
		processor: processor,
		cluster:   cluster,
		instances: instances,
		region:    region,
	}
}

func (j *GetRDSClusterMetricsJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          fmt.Sprintf("get_rds_cluster_metrics_%s", *j.cluster.DBClusterIdentifier),
		Description: fmt.Sprintf("Getting metrics of %s", *j.cluster.DBClusterIdentifier),
		MaxRetry:    0,
	}
}

func (j *GetRDSClusterMetricsJob) Run(ctx context.Context) error {
	allMetrics := map[string]map[string][]types2.Datapoint{}
	for _, instance := range j.instances {
		isAurora := j.cluster.DBClusterIdentifier != nil && strings.Contains(strings.ToLower(*j.cluster.Engine), "aurora")
		allMetrics[utils.HashString(*instance.DBInstanceIdentifier)] = map[string][]types2.Datapoint{}
		cwMetrics, err := j.processor.metricProvider.GetDayByDayMetrics(
			ctx,
			j.region,
			"AWS/RDS",
			[]string{
				"CPUUtilization",
				"FreeableMemory",
			},
			map[string][]string{
				"DBInstanceIdentifier": {*instance.DBInstanceIdentifier},
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
			allMetrics[utils.HashString(*instance.DBInstanceIdentifier)][k] = v
		}

		cwMetrics, err = j.processor.metricProvider.GetDayByDayMetrics(
			ctx,
			j.region,
			"AWS/RDS",
			[]string{
				"FreeStorageSpace",
				"NetworkReceiveThroughput",
				"NetworkTransmitThroughput",
			},
			map[string][]string{
				"DBInstanceIdentifier": {*instance.DBInstanceIdentifier},
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
		for k, v := range cwMetrics {
			allMetrics[utils.HashString(*instance.DBInstanceIdentifier)][k] = v
		}

		var volumeThroughput map[string][]types2.Datapoint
		var iopsMetrics map[string][]types2.Datapoint
		var clusterMetrics map[string][]types2.Datapoint
		if !isAurora {
			volumeThroughput, err = j.processor.metricProvider.GetDayByDayMetrics(
				ctx,
				j.region,
				"AWS/RDS",
				[]string{
					"ReadThroughput",
					"WriteThroughput",
				},
				map[string][]string{
					"DBInstanceIdentifier": {*instance.DBInstanceIdentifier},
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
			iopsMetrics, err = j.processor.metricProvider.GetDayByDayMetrics(
				ctx,
				j.region,
				"AWS/RDS",
				[]string{
					"ReadIOPS",
					"WriteIOPS",
				},
				map[string][]string{
					"DBInstanceIdentifier": {*instance.DBInstanceIdentifier},
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
		} else {
			volumeThroughput, err = j.processor.metricProvider.GetDayByDayMetrics(
				ctx,
				j.region,
				"AWS/RDS",
				[]string{
					"ReadThroughput",
					"WriteThroughput",
				},
				map[string][]string{
					"DBClusterIdentifier": {*instance.DBClusterIdentifier},
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
			iopsMetrics, err = j.processor.metricProvider.GetDayByDayMetrics(
				ctx,
				j.region,
				"AWS/RDS",
				[]string{
					"ReadIOPS",
					"WriteIOPS",
				},
				map[string][]string{
					"DBClusterIdentifier": {*instance.DBClusterIdentifier},
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
			clusterMetrics, err = j.processor.metricProvider.GetDayByDayMetrics(
				ctx,
				j.region,
				"AWS/RDS",
				[]string{
					"VolumeBytesUsed",
				},
				map[string][]string{
					"DBClusterIdentifier": {*instance.DBClusterIdentifier},
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
		for k, val := range volumeThroughput {
			volumeThroughput[k] = val
		}
		for k, val := range iopsMetrics {
			iopsMetrics[k] = val
		}

		hashedIdentifier := utils.HashString(*instance.DBInstanceIdentifier)
		for k, v := range cwMetrics {
			allMetrics[hashedIdentifier][k] = v
		}
		for k, v := range iopsMetrics {
			allMetrics[hashedIdentifier][k] = v
		}
		for k, v := range volumeThroughput {
			allMetrics[hashedIdentifier][k] = v
		}
		if clusterMetrics != nil {
			for k, v := range clusterMetrics {
				allMetrics[hashedIdentifier][k] = v
			}
		}
	}

	oi := RDSClusterItem{
		Cluster:             j.cluster,
		Instances:           j.instances,
		Region:              j.region,
		OptimizationLoading: true,
		Preferences:         j.processor.defaultPreferences,
		Skipped:             false,
		LazyLoadingEnabled:  false,
		SkipReason:          "",
		Metrics:             allMetrics,
	}

	j.processor.items.Set(*oi.Cluster.DBClusterIdentifier, oi)
	j.processor.publishOptimizationItem(oi.ToOptimizationItem())
	j.processor.UpdateSummary(*oi.Cluster.DBClusterIdentifier)
	if !oi.Skipped && !oi.LazyLoadingEnabled {
		j.processor.jobQueue.Push(NewOptimizeRDSJob(j.processor, oi))
	}
	return nil
}
