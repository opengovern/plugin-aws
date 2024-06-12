package ec2_instance

import (
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/kaytu/pkg/style"
	"github.com/kaytu-io/kaytu/pkg/utils"
	aws2 "github.com/kaytu-io/plugin-aws/plugin/aws"
	kaytu2 "github.com/kaytu-io/plugin-aws/plugin/kaytu"
	util "github.com/kaytu-io/plugin-aws/utils"
	"sync"
)

type Processor struct {
	provider                *aws2.AWS
	metricProvider          *aws2.CloudWatch
	identification          map[string]string
	items                   util.ConcurrentMap[string, EC2InstanceItem]
	publishOptimizationItem func(item *golang.ChartOptimizationItem)
	publishResultSummary    func(summary *golang.ResultSummary)
	kaytuAcccessToken       string
	jobQueue                *sdk.JobQueue
	configuration           *kaytu2.Configuration
	lazyloadCounter         *sdk.SafeCounter
	observabilityDays       int

	summary      map[string]EC2InstanceSummary
	summaryMutex sync.RWMutex
}

func NewProcessor(
	prv *aws2.AWS,
	metric *aws2.CloudWatch,
	identification map[string]string,
	publishOptimizationItem func(item *golang.ChartOptimizationItem),
	publishResultSummary func(summary *golang.ResultSummary),
	kaytuAcccessToken string,
	jobQueue *sdk.JobQueue,
	configurations *kaytu2.Configuration,
	lazyloadCounter *sdk.SafeCounter,
	observabilityDays int,
) *Processor {
	r := &Processor{
		provider:                prv,
		metricProvider:          metric,
		identification:          identification,
		items:                   util.NewMap[string, EC2InstanceItem](),
		publishOptimizationItem: publishOptimizationItem,
		publishResultSummary:    publishResultSummary,
		kaytuAcccessToken:       kaytuAcccessToken,
		jobQueue:                jobQueue,
		configuration:           configurations,
		lazyloadCounter:         lazyloadCounter,
		observabilityDays:       observabilityDays,

		summary:      map[string]EC2InstanceSummary{},
		summaryMutex: sync.RWMutex{},
	}
	jobQueue.Push(NewListAllRegionsJob(r))
	return r
}

func (m *Processor) ReEvaluate(id string, items []*golang.PreferenceItem) {
	v, _ := m.items.Get(id)
	v.Preferences = items
	m.items.Set(id, v)
	m.jobQueue.Push(NewOptimizeEC2InstanceJob(m, v))
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

func (m *Processor) ResultsSummary() *golang.ResultSummary {
	summary := &golang.ResultSummary{}
	var totalCost, savings float64
	m.summaryMutex.RLock()
	for _, item := range m.summary {
		totalCost += item.CurrentRuntimeCost
		savings += item.Savings
	}
	m.summaryMutex.RUnlock()
	summary.Message = fmt.Sprintf("Current runtime cost: %s, Savings: %s",
		style.CostStyle.Render(fmt.Sprintf("%s", utils.FormatPriceFloat(totalCost))), style.SavingStyle.Render(fmt.Sprintf("%s", utils.FormatPriceFloat(savings))))
	return summary
}

func (m *Processor) UpdateSummary(itemId string) {
	i, ok := m.items.Get(itemId)
	if ok && i.Wastage.RightSizing.Recommended != nil {
		totalSaving := 0.0
		totalCurrentCost := 0.0
		for _, v := range i.Wastage.VolumeRightSizing {
			totalSaving += v.Current.Cost - v.Recommended.Cost
			totalCurrentCost += v.Current.Cost
		}
		totalSaving += i.Wastage.RightSizing.Current.Cost - i.Wastage.RightSizing.Recommended.Cost
		totalCurrentCost += i.Wastage.RightSizing.Current.Cost
		m.summaryMutex.Lock()
		m.summary[itemId] = EC2InstanceSummary{
			CurrentRuntimeCost: totalCurrentCost,
			Savings:            totalSaving,
		}
		m.summaryMutex.Unlock()

	}
	m.publishResultSummary(m.ResultsSummary())
}
