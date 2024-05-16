package rds_instance

import (
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/plugin-aws/plugin/aws"
	"github.com/kaytu-io/plugin-aws/plugin/kaytu"
)

type Processor struct {
	provider                *aws.AWS
	metricProvider          *aws.CloudWatch
	identification          map[string]string
	items                   map[string]RDSInstanceItem
	publishOptimizationItem func(item *golang.OptimizationItem)
	kaytuAcccessToken       string
	jobQueue                *sdk.JobQueue
	configuration           *kaytu.Configuration
	lazyloadCounter         *sdk.LazyLoadCounter
}

func NewProcessor(
	provider *aws.AWS,
	metricProvider *aws.CloudWatch,
	identification map[string]string,
	publishOptimizationItem func(item *golang.OptimizationItem),
	kaytuAcccessToken string,
	jobQueue *sdk.JobQueue,
	configurations *kaytu.Configuration,
	lazyloadCounter *sdk.LazyLoadCounter,
) *Processor {
	r := &Processor{
		provider:                provider,
		metricProvider:          metricProvider,
		identification:          identification,
		items:                   map[string]RDSInstanceItem{},
		publishOptimizationItem: publishOptimizationItem,
		kaytuAcccessToken:       kaytuAcccessToken,
		jobQueue:                jobQueue,
		configuration:           configurations,
		lazyloadCounter:         lazyloadCounter,
	}

	jobQueue.Push(NewListAllRegionsJob(r))
	return r
}

func (m *Processor) ReEvaluate(id string, items []*golang.PreferenceItem) {
	v := m.items[id]
	v.Preferences = items
	m.items[id] = v
	m.jobQueue.Push(NewOptimizeRDSJob(m, m.items[id]))
}

func (m *Processor) HasItem(id string) bool {
	_, ok := m.items[id]
	return ok
}
