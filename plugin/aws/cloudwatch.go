package aws

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudwatch"
	types2 "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"time"
)

type CloudWatch struct {
	cfg aws.Config
}

func NewCloudWatch(cfg aws.Config) (*CloudWatch, error) {
	return &CloudWatch{cfg: cfg}, nil
}

func (cw *CloudWatch) GetMetrics(
	region string,
	namespace string,
	metricNames []string,
	filters map[string][]string,
	startTime, endTime time.Time,
	interval time.Duration,
	statistics []types2.Statistic,
	extendedStatistics []string,
) (map[string][]types2.Datapoint, error) {
	localCfg := cw.cfg
	localCfg.Region = region

	metrics := map[string][]types2.Datapoint{}

	ctx := context.Background()
	cloudwatchClient := cloudwatch.NewFromConfig(localCfg)
	var dimensionFilters []types2.DimensionFilter
	for k, v := range filters {
		dimensionFilters = append(dimensionFilters, types2.DimensionFilter{
			Name:  aws.String(k),
			Value: aws.String(v[0]),
		})
	}

	var dimensions []types2.Dimension
	for k, v := range filters {
		dimensions = append(dimensions, types2.Dimension{
			Name:  aws.String(k),
			Value: aws.String(v[0]),
		})
	}

	for _, metricName := range metricNames {
		// Create input for GetMetricStatistics
		input := &cloudwatch.GetMetricStatisticsInput{
			EndTime:            aws.Time(endTime),
			MetricName:         aws.String(metricName),
			Namespace:          aws.String(namespace),
			Period:             aws.Int32(int32(interval.Seconds())),
			StartTime:          aws.Time(startTime),
			Dimensions:         dimensions,
			ExtendedStatistics: extendedStatistics,
			Statistics:         statistics,
		}

		// Get metric data
		resp, err := cloudwatchClient.GetMetricStatistics(ctx, input)
		if err != nil {
			return nil, err
		}

		metrics[metricName] = resp.Datapoints
	}
	return metrics, nil
}

func (cw *CloudWatch) GetDayByDayMetrics(
	region string,
	namespace string,
	metricNames []string,
	filters map[string][]string,
	days int,
	interval time.Duration,
	statistics []types2.Statistic,
	extendedStatistics []string,
) (map[string][]types2.Datapoint, error) {
	datapoints := make(map[string][]types2.Datapoint)
	for i := 1; i <= days; i++ {
		startTime := time.Now().Add(-time.Duration(24*i) * time.Hour)
		endTime := time.Now().Add(-time.Duration(24*(i-1)) * time.Hour)
		metrics, err := cw.GetMetrics(region, namespace, metricNames, filters, startTime, endTime, interval, statistics, extendedStatistics)
		if err != nil {
			return nil, err
		}
		for k, v := range metrics {
			if _, ok := datapoints[k]; ok {
				datapoints[k] = append(datapoints[k], v...)
			} else {
				datapoints[k] = []types2.Datapoint{}
				datapoints[k] = append(datapoints[k], v...)
			}
		}
	}
	return datapoints, nil
}

func GetDatapointsAvgFromSum(dps []types2.Datapoint, period int32) []types2.Datapoint {
	for i, dp := range dps {
		avg := (*dp.Sum) / (*dp.SampleCount * float64(period))
		dp.Average = &avg
		dps[i] = dp
	}
	return dps
}

func GetDatapointsAvgFromSumPeriod(dps []types2.Datapoint, period int32) []types2.Datapoint {
	for i, dp := range dps {
		avg := (*dp.Sum) / (float64(period))
		dp.Average = &avg
		dps[i] = dp
	}
	return dps
}
