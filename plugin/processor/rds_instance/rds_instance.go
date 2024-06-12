package rds_instance

import (
	"fmt"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/kaytu/pkg/style"
	"github.com/kaytu-io/kaytu/pkg/utils"
	"github.com/kaytu-io/plugin-aws/plugin/aws"
	"github.com/kaytu-io/plugin-aws/plugin/kaytu"
	"github.com/kaytu-io/plugin-aws/plugin/processor/ec2_instance"
	util "github.com/kaytu-io/plugin-aws/utils"
	"sync"
)

type Processor struct {
	provider                *aws.AWS
	metricProvider          *aws.CloudWatch
	identification          map[string]string
	items                   util.ConcurrentMap[string, RDSInstanceItem]
	publishOptimizationItem func(item *golang.ChartOptimizationItem)
	publishResultSummary    func(summary *golang.ResultSummary)
	kaytuAcccessToken       string
	jobQueue                *sdk.JobQueue
	configuration           *kaytu.Configuration
	lazyloadCounter         *sdk.SafeCounter
	observabilityDays       int

	summary      map[string]ec2_instance.EC2InstanceSummary
	summaryMutex sync.RWMutex
}

func NewProcessor(
	provider *aws.AWS,
	metricProvider *aws.CloudWatch,
	identification map[string]string,
	publishOptimizationItem func(item *golang.ChartOptimizationItem),
	publishResultSummary func(summary *golang.ResultSummary),
	kaytuAcccessToken string,
	jobQueue *sdk.JobQueue,
	configurations *kaytu.Configuration,
	lazyloadCounter *sdk.SafeCounter,
	observabilityDays int,
	summary map[string]ec2_instance.EC2InstanceSummary,
	summaryMutex sync.RWMutex,
) *Processor {
	r := &Processor{
		provider:                provider,
		metricProvider:          metricProvider,
		identification:          identification,
		items:                   util.NewMap[string, RDSInstanceItem](),
		publishOptimizationItem: publishOptimizationItem,
		publishResultSummary:    publishResultSummary,
		kaytuAcccessToken:       kaytuAcccessToken,
		jobQueue:                jobQueue,
		configuration:           configurations,
		lazyloadCounter:         lazyloadCounter,
		observabilityDays:       observabilityDays,
		summary:                 summary,
		summaryMutex:            summaryMutex,
	}

	jobQueue.Push(NewListAllRegionsJob(r))
	return r
}

func (m *Processor) ReEvaluate(id string, items []*golang.PreferenceItem) {
	v, _ := m.items.Get(id)
	v.Preferences = items
	m.items.Set(id, v)
	m.jobQueue.Push(NewOptimizeRDSJob(m, v))
}

func (m *Processor) HasItem(id string) bool {
	_, ok := m.items.Get(id)
	return ok
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

		totalSaving += i.Wastage.RightSizing.Current.ComputeCost - i.Wastage.RightSizing.Recommended.ComputeCost
		totalCurrentCost += i.Wastage.RightSizing.Current.ComputeCost
		totalSaving += i.Wastage.RightSizing.Current.StorageCost - i.Wastage.RightSizing.Recommended.StorageCost
		totalCurrentCost += i.Wastage.RightSizing.Current.StorageCost
		m.summaryMutex.Lock()
		m.summary[itemId] = ec2_instance.EC2InstanceSummary{
			CurrentRuntimeCost: totalCurrentCost,
			Savings:            totalSaving,
		}
		m.summaryMutex.Unlock()

	}
	m.publishResultSummary(m.ResultsSummary())
}
