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

	paginator := cloudwatch.NewListMetricsPaginator(cloudwatchClient, &cloudwatch.ListMetricsInput{
		Namespace:  aws.String(namespace),
		Dimensions: dimensionFilters,
	})
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, err
		}

		for _, p := range page.Metrics {
			if p.MetricName == nil {
				continue
			}

			exists := false
			for _, mn := range metricNames {
				if *p.MetricName == mn {
					exists = true
					break
				}
			}

			if !exists {
				continue
			}

			// Create input for GetMetricStatistics
			input := &cloudwatch.GetMetricStatisticsInput{
				Namespace:  aws.String(namespace),
				MetricName: p.MetricName,
				Dimensions: dimensions,
				StartTime:  aws.Time(startTime),
				EndTime:    aws.Time(endTime),
				Period:     aws.Int32(int32(interval.Seconds())),
				Statistics: statistics,
			}

			// Get metric data
			resp, err := cloudwatchClient.GetMetricStatistics(ctx, input)
			if err != nil {
				return nil, err
			}

			metrics[*p.MetricName] = resp.Datapoints
		}
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
) (map[string][]types2.Datapoint, error) {
	datapoints := make(map[string][]types2.Datapoint)
	for i := 1; i <= days; i++ {
		startTime := time.Now().Add(-time.Duration(24*i) * time.Hour)
		endTime := time.Now().Add(-time.Duration(24*(i-1)) * time.Hour)
		metrics, err := cw.GetMetrics(region, namespace, metricNames, filters, startTime, endTime, interval, statistics)
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
		avg := (*dp.Sum) / float64(period)
		dp.Average = &avg
		dps[i] = dp
	}
	return dps
}
