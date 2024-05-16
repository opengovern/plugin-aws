package processor

import (
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/plugin-aws/plugin/aws"
	"github.com/kaytu-io/plugin-aws/plugin/kaytu"
	"github.com/kaytu-io/plugin-aws/plugin/processor/rds_cluster"
	"github.com/kaytu-io/plugin-aws/plugin/processor/rds_instance"
)

type RDSProcessor struct {
	rdsInstanceProcessor *rds_instance.Processor
	rdsClusterProcessor  *rds_cluster.Processor
}

func NewRDSProcessor(provider *aws.AWS, metricProvider *aws.CloudWatch, identification map[string]string, publishOptimizationItem func(item *golang.OptimizationItem), kaytuAcccessToken string, jobQueue *sdk.JobQueue, configurations *kaytu.Configuration) *RDSProcessor {
	lazyloadCounter := &sdk.LazyLoadCounter{}
	return &RDSProcessor{
		rdsInstanceProcessor: rds_instance.NewProcessor(provider, metricProvider, identification, publishOptimizationItem, kaytuAcccessToken, jobQueue, configurations, lazyloadCounter),
		rdsClusterProcessor:  rds_cluster.NewProcessor(provider, metricProvider, identification, publishOptimizationItem, kaytuAcccessToken, jobQueue, configurations, lazyloadCounter),
	}
}

func (m *RDSProcessor) ReEvaluate(id string, items []*golang.PreferenceItem) {
	if m.rdsInstanceProcessor.HasItem(id) {
		m.rdsInstanceProcessor.ReEvaluate(id, items)
	}
	if m.rdsClusterProcessor.HasItem(id) {
		m.rdsClusterProcessor.ReEvaluate(id, items)
	}
}
