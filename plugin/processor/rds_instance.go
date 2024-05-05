package processor

import (
	"fmt"
	types2 "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/google/uuid"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/utils"
	"github.com/kaytu-io/kaytu/preferences"
	"github.com/kaytu-io/plugin-aws/plugin/aws"
	"github.com/kaytu-io/plugin-aws/plugin/kaytu"
	preferences2 "github.com/kaytu-io/plugin-aws/plugin/preferences"
	"github.com/kaytu-io/plugin-aws/plugin/version"
	"os"
	"strings"
	"sync"
	"time"
)

type RDSInstanceProcessor struct {
	provider       *aws.AWS
	metricProvider *aws.CloudWatch
	identification map[string]string

	processWastageChan chan RDSInstanceItem
	items              map[string]RDSInstanceItem

	publishJob              func(result *golang.JobResult) *golang.JobResult
	publishError            func(error)
	publishOptimizationItem func(item *golang.OptimizationItem)
	kaytuAcccessToken       string
}

func NewRDSInstanceProcessor(
	provider *aws.AWS,
	metricProvider *aws.CloudWatch,
	identification map[string]string,
	publishJob func(result *golang.JobResult) *golang.JobResult,
	publishError func(error),
	publishOptimizationItem func(item *golang.OptimizationItem),
	kaytuAcccessToken string,
) *RDSInstanceProcessor {
	r := &RDSInstanceProcessor{
		provider:                provider,
		metricProvider:          metricProvider,
		identification:          identification,
		processWastageChan:      make(chan RDSInstanceItem, 1000),
		items:                   map[string]RDSInstanceItem{},
		publishJob:              publishJob,
		publishError:            publishError,
		publishOptimizationItem: publishOptimizationItem,
		kaytuAcccessToken:       kaytuAcccessToken,
	}
	go r.ProcessWastages()
	go r.ProcessAllRegions()
	return r
}

func (m *RDSInstanceProcessor) ProcessAllRegions() {
	defer func() {
		if r := recover(); r != nil {
			m.publishError(fmt.Errorf("%v", r))
		}
	}()

	job := m.publishJob(&golang.JobResult{Id: "list_rds_all_regions", Description: "Listing all available regions"})
	job.Done = true
	regions, err := m.provider.ListAllRegions()
	if err != nil {
		job.FailureMessage = err.Error()
		m.publishJob(job)
		return
	}
	m.publishJob(job)

	wg := sync.WaitGroup{}
	wg.Add(len(regions))
	for _, region := range regions {
		region := region
		go func() {
			defer wg.Done()
			m.ProcessRegion(region)
		}()
	}
	wg.Wait()
}

func (m *RDSInstanceProcessor) ProcessRegion(region string) {
	defer func() {
		if r := recover(); r != nil {
			m.publishError(fmt.Errorf("%v", r))
		}
	}()

	job := m.publishJob(&golang.JobResult{Id: fmt.Sprintf("region_rds_instances_%s", region), Description: "Listing all rds instances in " + region})
	job.Done = true

	instances, err := m.provider.ListRDSInstance(region)
	if err != nil {
		job.FailureMessage = err.Error()
		m.publishJob(job)
		return
	}
	m.publishJob(job)

	for _, instance := range instances {
		oi := RDSInstanceItem{
			Instance:            instance,
			Region:              region,
			OptimizationLoading: true,
			Preferences:         preferences2.DefaultRDSPreferences,
		}

		// just to show the loading
		m.items[*oi.Instance.DBInstanceIdentifier] = oi
		m.publishOptimizationItem(oi.ToOptimizationItem())
	}

	for _, instance := range instances {
		imjob := m.publishJob(&golang.JobResult{Id: fmt.Sprintf("rds_instance_%s_metrics", *instance.DBInstanceIdentifier), Description: fmt.Sprintf("getting metrics of %s", *instance.DBInstanceIdentifier)})
		imjob.Done = true
		startTime := time.Now().Add(-24 * 7 * time.Hour)
		endTime := time.Now()
		instanceMetrics := map[string][]types2.Datapoint{}
		cwMetrics, err := m.metricProvider.GetMetrics(
			region,
			"AWS/RDS",
			[]string{
				"CPUUtilization",
				"FreeableMemory",
				"FreeStorageSpace",
				"NetworkReceiveThroughput",
				"NetworkTransmitThroughput",
				"ReadIOPS",
				"ReadThroughput",
				"WriteIOPS",
				"WriteThroughput",
			},
			map[string][]string{
				"DBInstanceIdentifier": {*instance.DBInstanceIdentifier},
			},
			startTime, endTime,
			time.Hour,
			[]types2.Statistic{
				types2.StatisticAverage,
				types2.StatisticMaximum,
			},
		)
		if err != nil {
			imjob.FailureMessage = err.Error()
			m.publishJob(imjob)
			return
		}
		for k, v := range cwMetrics {
			instanceMetrics[k] = v
		}
		m.publishJob(imjob)

		oi := RDSInstanceItem{
			Instance:            instance,
			Metrics:             instanceMetrics,
			Region:              region,
			OptimizationLoading: true,
			Preferences:         preferences2.DefaultRDSPreferences,
		}

		m.items[*oi.Instance.DBInstanceIdentifier] = oi
		m.publishOptimizationItem(oi.ToOptimizationItem())
		if !oi.Skipped {
			m.processWastageChan <- oi
		}
	}
}

func (m *RDSInstanceProcessor) ProcessWastages() {
	for item := range m.processWastageChan {
		go m.WastageWorker(item)
	}
}

func (m *RDSInstanceProcessor) WastageWorker(item RDSInstanceItem) {
	defer func() {
		if r := recover(); r != nil {
			m.publishError(fmt.Errorf("%v", r))
		}
	}()

	job := m.publishJob(&golang.JobResult{Id: fmt.Sprintf("wastage_rds_%s", *item.Instance.DBInstanceIdentifier), Description: fmt.Sprintf("Evaluating RDS usage data for %s", *item.Instance.DBInstanceIdentifier)})
	job.Done = true

	var clusterType kaytu.AwsRdsClusterType
	multiAZ := item.Instance.MultiAZ != nil && *item.Instance.MultiAZ
	readableStandbys := item.Instance.ReplicaMode == types.ReplicaModeOpenReadOnly
	if multiAZ && readableStandbys {
		clusterType = kaytu.AwsRdsClusterTypeMultiAzTwoInstance
	} else if multiAZ {
		clusterType = kaytu.AwsRdsClusterTypeMultiAzOneInstance
	} else {
		clusterType = kaytu.AwsRdsClusterTypeSingleInstance
	}

	req := kaytu.AwsRdsWastageRequest{
		RequestId:      uuid.New().String(),
		CliVersion:     version.VERSION,
		Identification: m.identification,
		Instance: kaytu.AwsRds{
			HashedInstanceId:                   utils.HashString(*item.Instance.DBInstanceIdentifier),
			AvailabilityZone:                   *item.Instance.AvailabilityZone,
			InstanceType:                       *item.Instance.DBInstanceClass,
			Engine:                             *item.Instance.Engine,
			EngineVersion:                      *item.Instance.EngineVersion,
			LicenseModel:                       *item.Instance.LicenseModel,
			BackupRetentionPeriod:              item.Instance.BackupRetentionPeriod,
			ClusterType:                        clusterType,
			PerformanceInsightsEnabled:         *item.Instance.PerformanceInsightsEnabled,
			PerformanceInsightsRetentionPeriod: item.Instance.PerformanceInsightsRetentionPeriod,
			StorageType:                        item.Instance.StorageType,
			StorageSize:                        item.Instance.AllocatedStorage,
			StorageIops:                        item.Instance.Iops,
		},
		Metrics:     item.Metrics,
		Region:      item.Region,
		Preferences: preferences.Export(item.Preferences),
	}
	if item.Instance.StorageThroughput != nil {
		floatThroughput := float64(*item.Instance.StorageThroughput)
		req.Instance.StorageThroughput = &floatThroughput
	}
	res, err := kaytu.RDSInstanceWastageRequest(req, m.kaytuAcccessToken)
	if err != nil {
		if strings.Contains(err.Error(), "please login") {
			fmt.Println(err.Error())
			os.Exit(1)
			return
		}
		job.FailureMessage = err.Error()
		m.publishJob(job)
		return
	}
	m.publishJob(job)

	if res.RightSizing.Current.InstanceType == "" {
		item.OptimizationLoading = false
		m.items[*item.Instance.DBInstanceIdentifier] = item
		m.publishOptimizationItem(item.ToOptimizationItem())
		return
	}

	item = RDSInstanceItem{
		Instance:            item.Instance,
		Region:              item.Region,
		OptimizationLoading: false,
		Preferences:         item.Preferences,
		Skipped:             false,
		SkipReason:          "",
		Metrics:             item.Metrics,
		Wastage:             *res,
	}
	m.items[*item.Instance.DBInstanceIdentifier] = item
	m.publishOptimizationItem(item.ToOptimizationItem())
}

func (m *RDSInstanceProcessor) ReEvaluate(id string, items []*golang.PreferenceItem) {
	v := m.items[id]
	v.Preferences = items
	m.items[id] = v
	m.processWastageChan <- m.items[id]
}
