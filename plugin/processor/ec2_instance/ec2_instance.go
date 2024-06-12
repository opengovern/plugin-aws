package ec2_instance

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/kaytu/pkg/utils"
	aws2 "github.com/kaytu-io/plugin-aws/plugin/aws"
	kaytu2 "github.com/kaytu-io/plugin-aws/plugin/kaytu"
	util "github.com/kaytu-io/plugin-aws/utils"
)

type Processor struct {
	provider                *aws2.AWS
	metricProvider          *aws2.CloudWatch
	identification          map[string]string
	items                   util.ConcurrentMap[string, EC2InstanceItem]
	publishOptimizationItem func(item *golang.ChartOptimizationItem)
	kaytuAcccessToken       string
	jobQueue                *sdk.JobQueue
	configuration           *kaytu2.Configuration
	lazyloadCounter         *sdk.SafeCounter
	observabilityDays       int
}

func NewProcessor(
	prv *aws2.AWS,
	metric *aws2.CloudWatch,
	identification map[string]string,
	publishOptimizationItem func(item *golang.ChartOptimizationItem),
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
		kaytuAcccessToken:       kaytuAcccessToken,
		jobQueue:                jobQueue,
		configuration:           configurations,
		lazyloadCounter:         lazyloadCounter,
		observabilityDays:       observabilityDays,
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
