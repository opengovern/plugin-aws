package processor

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	types2 "github.com/aws/aws-sdk-go-v2/service/cloudwatch/types"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/google/uuid"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/utils"
	"github.com/kaytu-io/kaytu/preferences"
	aws2 "github.com/kaytu-io/plugin-aws/plugin/aws"
	kaytu2 "github.com/kaytu-io/plugin-aws/plugin/kaytu"
	preferences2 "github.com/kaytu-io/plugin-aws/plugin/preferences"
	"github.com/kaytu-io/plugin-aws/plugin/version"
	"strings"
	"sync"
	"time"
)

type EC2InstanceProcessor struct {
	provider           *aws2.AWS
	metricProvider     *aws2.CloudWatch
	identification     map[string]string
	processWastageChan chan EC2InstanceItem
	items              sync.Map
	//items                   map[string]EC2InstanceItem
	publishJob              func(result *golang.JobResult) *golang.JobResult
	publishError            func(error)
	publishOptimizationItem func(item *golang.OptimizationItem)
	publishResultsReady     func(bool)
	kaytuAcccessToken       string

	processRegionJobsFinished      map[string]bool
	processRegionJobsFinishedMutex sync.RWMutex

	lazyLoadCounter int
	lazyLoadMutex   sync.RWMutex
}

func NewEC2InstanceProcessor(
	prv *aws2.AWS,
	metric *aws2.CloudWatch,
	identification map[string]string,
	publishJob func(result *golang.JobResult) *golang.JobResult,
	publishError func(error),
	publishOptimizationItem func(item *golang.OptimizationItem),
	publishResultsReady func(bool),
	kaytuAcccessToken string,
) *EC2InstanceProcessor {
	r := &EC2InstanceProcessor{
		processWastageChan:        make(chan EC2InstanceItem, 1000),
		provider:                  prv,
		metricProvider:            metric,
		identification:            identification,
		items:                     sync.Map{}, // map[string]EC2InstanceItem{},
		publishJob:                publishJob,
		publishOptimizationItem:   publishOptimizationItem,
		publishError:              publishError,
		publishResultsReady:       publishResultsReady,
		kaytuAcccessToken:         kaytuAcccessToken,
		processRegionJobsFinished: map[string]bool{},
	}
	for i := 0; i < 4; i++ {
		go r.processWastages()
	}
	go r.processAllRegions()
	r.processRegionJobsFinishedMutex.Lock()
	r.processRegionJobsFinished["start"] = false
	r.processRegionJobsFinishedMutex.Unlock()

	go r.SendResultsReadyMessage()
	return r
}

func (m *EC2InstanceProcessor) processAllRegions() {
	defer func() {
		if r := recover(); r != nil {
			m.publishError(fmt.Errorf("%v", r))
		}
	}()

	configuration, err := kaytu2.ConfigurationRequest()
	if err != nil {
		m.publishError(err)
		return
	}

	job := m.publishJob(&golang.JobResult{Id: "list_all_regions", Description: "Listing all available regions"})
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
		m.processRegionJobsFinishedMutex.Lock()
		m.processRegionJobsFinished[region] = false
		m.processRegionJobsFinished["start"] = true
		m.processRegionJobsFinishedMutex.Unlock()
		region := region
		go func() {
			defer wg.Done()
			m.processRegion(region, configuration)
		}()
	}
	wg.Wait()
}

func (m *EC2InstanceProcessor) processRegion(region string, configuration *kaytu2.Configuration) {
	defer func() {
		m.processRegionJobsFinishedMutex.Lock()
		m.processRegionJobsFinished[region] = true
		m.processRegionJobsFinishedMutex.Unlock()
		if r := recover(); r != nil {
			m.publishError(fmt.Errorf("%v", r))
		}
	}()

	job := m.publishJob(&golang.JobResult{Id: fmt.Sprintf("region_ec2_instances_%s", region), Description: "Listing all ec2 instances in " + region})
	job.Done = true

	instances, err := m.provider.ListInstances(region)
	if err != nil {
		job.FailureMessage = err.Error()
		m.publishJob(job)
		return
	}
	m.publishJob(job)

	for _, instance := range instances {
		oi := EC2InstanceItem{
			Instance:            instance,
			Region:              region,
			OptimizationLoading: true,
			LazyLoadingEnabled:  false,
			Preferences:         preferences2.DefaultEC2Preferences,
		}

		isAutoScaling := false
		for _, tag := range instance.Tags {
			if *tag.Key == "aws:autoscaling:groupName" && tag.Value != nil && *tag.Value != "" {
				isAutoScaling = true
			}
		}
		if instance.State.Name != types.InstanceStateNameRunning ||
			instance.InstanceLifecycle == types.InstanceLifecycleTypeSpot ||
			isAutoScaling {
			oi.OptimizationLoading = false
			oi.Skipped = true
			reason := ""
			if instance.State.Name != types.InstanceStateNameRunning {
				reason = "not running"
			} else if instance.InstanceLifecycle == types.InstanceLifecycleTypeSpot {
				reason = "spot instance"
			} else if isAutoScaling {
				reason = "auto-scaling group instance"
			}
			if len(reason) > 0 {
				oi.SkipReason = reason
			}
		}

		if !oi.Skipped {
			m.lazyLoadMutex.Lock()
			m.lazyLoadCounter++
			if m.lazyLoadCounter > configuration.EC2LazyLoad {
				oi.LazyLoadingEnabled = true
			}
			m.lazyLoadMutex.Unlock()
		}

		if !oi.Skipped {
			var monitoring *types.MonitoringState
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
			_, err := kaytu2.Ec2InstanceWastageRequest(kaytu2.EC2InstanceWastageRequest{
				RequestId:      &reqID,
				CliVersion:     &version.VERSION,
				Identification: m.identification,
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
			}, m.kaytuAcccessToken)
			if err != nil {
				if strings.Contains(err.Error(), "please login") {
					m.publishError(err)
					return
				}
				job.FailureMessage = err.Error()
				m.publishJob(job)
				return
			}
		}

		// just to show the loading
		m.items.Store(*oi.Instance.InstanceId, oi)
		m.publishOptimizationItem(oi.ToOptimizationItem())
	}

	for _, instance := range instances {
		i, ok := m.items.Load(*instance.InstanceId)
		if ok && (i.(EC2InstanceItem).LazyLoadingEnabled || !i.(EC2InstanceItem).OptimizationLoading) {
			continue
		}

		//TODO-Saleh since we're doing these one by one if user runs the lazy loading item it gets re-run here as well because lazy loading enabled is false now.
		m.processInstance(instance, region)
	}
}

func (m *EC2InstanceProcessor) processInstance(instance types.Instance, region string) {
	isAutoScaling := false
	for _, tag := range instance.Tags {
		if *tag.Key == "aws:autoscaling:groupName" && tag.Value != nil && *tag.Value != "" {
			isAutoScaling = true
		}
	}
	if instance.State.Name != types.InstanceStateNameRunning ||
		instance.InstanceLifecycle == types.InstanceLifecycleTypeSpot ||
		isAutoScaling {
		return
	}

	vjob := m.publishJob(&golang.JobResult{Id: fmt.Sprintf("volumes_%s", *instance.InstanceId), Description: fmt.Sprintf("getting volumes of %s", *instance.InstanceId)})
	vjob.Done = true

	volumes, err := m.provider.ListAttachedVolumes(region, instance)
	if err != nil {
		vjob.FailureMessage = err.Error()
		m.publishJob(vjob)
		return
	}
	m.publishJob(vjob)

	imjob := m.publishJob(&golang.JobResult{Id: fmt.Sprintf("instance_%s_metrics", *instance.InstanceId), Description: fmt.Sprintf("getting metrics of %s", *instance.InstanceId)})
	imjob.Done = true
	startTime := time.Now().Add(-24 * 7 * time.Hour)
	endTime := time.Now()
	instanceMetrics := map[string][]types2.Datapoint{}

	cwMetrics, err := m.metricProvider.GetMetrics(
		region,
		"AWS/EC2",
		[]string{
			"CPUUtilization",
		},
		map[string][]string{
			"InstanceId": {*instance.InstanceId},
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

	cwPerSecondMetrics, err := m.metricProvider.GetMetrics(
		region,
		"AWS/EC2",
		[]string{
			"NetworkIn",
			"NetworkOut",
		},
		map[string][]string{
			"InstanceId": {*instance.InstanceId},
		},
		startTime, endTime,
		time.Hour,
		[]types2.Statistic{
			types2.StatisticSum,
		},
	)
	if err != nil {
		imjob.FailureMessage = err.Error()
		m.publishJob(imjob)
		return
	}
	for k, v := range cwPerSecondMetrics {
		cwPerSecondMetrics[k] = aws2.GetDatapointsAvgFromSum(v, int32(time.Hour/time.Second))
		instanceMetrics[k] = cwPerSecondMetrics[k]
	}

	cwaMetrics, err := m.metricProvider.GetMetrics(
		region,
		"CWAgent",
		[]string{
			"mem_used_percent",
		},
		map[string][]string{
			"InstanceId": {*instance.InstanceId},
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
	for k, v := range cwaMetrics {
		instanceMetrics[k] = v
	}
	m.publishJob(imjob)

	ivjob := m.publishJob(&golang.JobResult{Id: fmt.Sprintf("volume_%s_metrics", *instance.InstanceId), Description: fmt.Sprintf("getting volume metrics of %s", *instance.InstanceId)})
	ivjob.Done = true

	var volumeIDs []string
	for _, v := range instance.BlockDeviceMappings {
		if v.Ebs != nil {
			volumeIDs = append(volumeIDs, *v.Ebs.VolumeId)
		}
	}

	volumeMetrics := map[string]map[string][]types2.Datapoint{}
	for _, v := range volumeIDs {
		volumeMetricsMap, err := m.metricProvider.GetMetrics(
			region,
			"AWS/EBS",
			[]string{
				"VolumeReadBytes",
				"VolumeWriteBytes",
			},
			map[string][]string{
				"VolumeId": {v},
			},
			startTime, endTime,
			time.Hour,
			[]types2.Statistic{
				types2.StatisticSum,
			},
		)
		if err != nil {
			ivjob.FailureMessage = err.Error()
			m.publishJob(ivjob)
			return
		}

		for k, val := range volumeMetricsMap {
			volumeMetricsMap[k] = aws2.GetDatapointsAvgFromSum(val, int32(time.Hour/time.Second))
		}

		volumeIops, err := m.metricProvider.GetDayByDayMetrics(
			region,
			"AWS/EBS",
			[]string{
				"VolumeReadOps",
				"VolumeWriteOps",
			},
			map[string][]string{
				"VolumeId": {v},
			},
			7,
			time.Minute,
			[]types2.Statistic{
				types2.StatisticSum,
			},
		)
		if err != nil {
			ivjob.FailureMessage = err.Error()
			m.publishJob(ivjob)
			return
		}

		for k, val := range volumeIops {
			val = aws2.GetDatapointsAvgFromSum(val, int32(time.Minute/time.Second))
			volumeMetricsMap[k] = val
		}

		// Hash v
		hashedId := utils.HashString(v)
		volumeMetrics[hashedId] = volumeMetricsMap
	}
	m.publishJob(ivjob)

	oi := EC2InstanceItem{
		Instance:            instance,
		Volumes:             volumes,
		Metrics:             instanceMetrics,
		VolumeMetrics:       volumeMetrics,
		Region:              region,
		OptimizationLoading: true,
		LazyLoadingEnabled:  false,
		Preferences:         preferences2.DefaultEC2Preferences,
	}
	if instance.State.Name != types.InstanceStateNameRunning ||
		instance.InstanceLifecycle == types.InstanceLifecycleTypeSpot ||
		isAutoScaling {
		oi.OptimizationLoading = false
		oi.Skipped = true
		reason := ""
		if instance.State.Name != types.InstanceStateNameRunning {
			reason = "not running"
		} else if instance.InstanceLifecycle == types.InstanceLifecycleTypeSpot {
			reason = "spot instance"
		} else if isAutoScaling {
			reason = "auto-scaling group instance"
		}
		if len(reason) > 0 {
			oi.SkipReason = reason
		}
	}
	m.items.Store(*oi.Instance.InstanceId, oi)
	m.publishOptimizationItem(oi.ToOptimizationItem())
	if !oi.Skipped && !oi.LazyLoadingEnabled {
		m.processWastageChan <- oi
	}
}

func (m *EC2InstanceProcessor) SendResultsReadyMessage() {
	for {
		ready := true
		time.Sleep(time.Second)
		m.processRegionJobsFinishedMutex.RLock()
		for _, f := range m.processRegionJobsFinished {
			if !f {
				ready = false
				break
			}
		}
		m.processRegionJobsFinishedMutex.RUnlock()
		m.items.Range(func(key, value any) bool {
			if value.(EC2InstanceItem).OptimizationLoading {
				ready = false
				return false
			}
			return true
		})
		m.publishResultsReady(ready)
	}
}

func (m *EC2InstanceProcessor) processWastages() {
	for item := range m.processWastageChan {
		m.wastageWorker(item)
	}
}

func (m *EC2InstanceProcessor) wastageWorker(item EC2InstanceItem) {
	defer func() {
		if r := recover(); r != nil {
			m.publishError(fmt.Errorf("%v", r))
		}
	}()

	if item.LazyLoadingEnabled {
		m.processInstance(item.Instance, item.Region)
		return
	}

	job := m.publishJob(&golang.JobResult{Id: fmt.Sprintf("wastage_%s", *item.Instance.InstanceId), Description: fmt.Sprintf("Evaluating usage data for %s", *item.Instance.InstanceId)})
	job.Done = true

	var monitoring *types.MonitoringState
	if item.Instance.Monitoring != nil {
		monitoring = &item.Instance.Monitoring.State
	}
	var placement *kaytu2.EC2Placement
	if item.Instance.Placement != nil {
		placement = &kaytu2.EC2Placement{
			Tenancy: item.Instance.Placement.Tenancy,
		}
		if item.Instance.Placement.AvailabilityZone != nil {
			placement.AvailabilityZone = *item.Instance.Placement.AvailabilityZone
		}
		if item.Instance.Placement.HostId != nil {
			placement.HashedHostId = utils.HashString(*item.Instance.Placement.HostId)
		}
	}
	platform := ""
	if item.Instance.PlatformDetails != nil {
		platform = *item.Instance.PlatformDetails
	}

	var volumes []kaytu2.EC2Volume
	for _, v := range item.Volumes {
		volumes = append(volumes, toEBSVolume(v))
	}
	reqID := uuid.New().String()

	res, err := kaytu2.Ec2InstanceWastageRequest(kaytu2.EC2InstanceWastageRequest{
		RequestId:      &reqID,
		CliVersion:     &version.VERSION,
		Identification: m.identification,
		Instance: kaytu2.EC2Instance{
			HashedInstanceId:  utils.HashString(*item.Instance.InstanceId),
			State:             item.Instance.State.Name,
			InstanceType:      item.Instance.InstanceType,
			Platform:          platform,
			ThreadsPerCore:    *item.Instance.CpuOptions.ThreadsPerCore,
			CoreCount:         *item.Instance.CpuOptions.CoreCount,
			EbsOptimized:      *item.Instance.EbsOptimized,
			InstanceLifecycle: item.Instance.InstanceLifecycle,
			Monitoring:        monitoring,
			Placement:         placement,
			UsageOperation:    *item.Instance.UsageOperation,
			Tenancy:           item.Instance.Placement.Tenancy,
		},
		Volumes:       volumes,
		VolumeCount:   len(volumes),
		Metrics:       item.Metrics,
		VolumeMetrics: item.VolumeMetrics,
		Region:        item.Region,
		Preferences:   preferences.Export(item.Preferences),
		Loading:       false,
	}, m.kaytuAcccessToken)
	if err != nil {
		if strings.Contains(err.Error(), "please login") {
			m.publishError(err)
			return
		}
		job.FailureMessage = err.Error()
		m.publishJob(job)
		return
	}
	m.publishJob(job)

	if res.RightSizing.Current.InstanceType == "" {
		item.OptimizationLoading = false
		m.items.Store(*item.Instance.InstanceId, item)
		m.publishOptimizationItem(item.ToOptimizationItem())
		return
	}

	item = EC2InstanceItem{
		Instance:            item.Instance,
		Region:              item.Region,
		OptimizationLoading: false,
		Preferences:         item.Preferences,
		Skipped:             false,
		SkipReason:          "",
		Volumes:             item.Volumes,
		Metrics:             item.Metrics,
		VolumeMetrics:       item.VolumeMetrics,
		Wastage:             *res,
	}
	m.items.Store(*item.Instance.InstanceId, item)
	m.publishOptimizationItem(item.ToOptimizationItem())
}

func (m *EC2InstanceProcessor) ReEvaluate(id string, items []*golang.PreferenceItem) {
	vv, ok := m.items.Load(id)
	var v EC2InstanceItem
	if ok {
		v = vv.(EC2InstanceItem)
	}
	v.Preferences = items
	m.items.Store(id, v)
	m.processWastageChan <- v
}

func toEBSVolume(v types.Volume) kaytu2.EC2Volume {
	var throughput *float64
	if v.Throughput != nil {
		throughput = aws.Float64(float64(*v.Throughput))
	}

	return kaytu2.EC2Volume{
		HashedVolumeId:   utils.HashString(*v.VolumeId),
		VolumeType:       v.VolumeType,
		Size:             v.Size,
		Iops:             v.Iops,
		AvailabilityZone: v.AvailabilityZone,
		Throughput:       throughput,
	}
}
