package processor

import (
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/kaytu/pkg/utils"
	"github.com/kaytu-io/plugin-aws/plugin/aws"
	"github.com/kaytu-io/plugin-aws/plugin/kaytu"
	"github.com/kaytu-io/plugin-aws/plugin/processor/ec2_instance"
	"github.com/kaytu-io/plugin-aws/plugin/processor/rds_cluster"
	"github.com/kaytu-io/plugin-aws/plugin/processor/rds_instance"
	"sync/atomic"
)

type RDSProcessor struct {
	rdsInstanceProcessor *rds_instance.Processor
	rdsClusterProcessor  *rds_cluster.Processor
}

func NewRDSProcessor(provider *aws.AWS, metricProvider *aws.CloudWatch, identification map[string]string, publishOptimizationItem func(item *golang.ChartOptimizationItem), publishResultSummary func(summary *golang.ResultSummary), kaytuAcccessToken string, jobQueue *sdk.JobQueue, configurations *kaytu.Configuration, observabilityDays int, preferences []*golang.PreferenceItem) *RDSProcessor {
	lazyloadCounter := atomic.Uint32{}
	summary := utils.NewConcurrentMap[string, ec2_instance.EC2InstanceSummary]()
	return &RDSProcessor{
		rdsInstanceProcessor: rds_instance.NewProcessor(provider, metricProvider, identification, publishOptimizationItem, publishResultSummary, kaytuAcccessToken, jobQueue, configurations, &lazyloadCounter, observabilityDays, &summary, preferences),
		rdsClusterProcessor:  rds_cluster.NewProcessor(provider, metricProvider, identification, publishOptimizationItem, publishResultSummary, kaytuAcccessToken, jobQueue, configurations, &lazyloadCounter, observabilityDays, &summary, preferences),
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

func (m *RDSProcessor) ExportNonInteractive() *golang.NonInteractiveExport {
	return &golang.NonInteractiveExport{
		Csv: m.exportCsv(),
	}
}

func (m *RDSProcessor) exportCsv() []*golang.CSVRow {
	headers := []string{
		"AccountID", "Region / AZ", "Resource Type", "Device ID", "Device Name", "Platform / Runtime Engine",
		"Device Runtime (Hrs)", "Current Cost", "Recommendation Cost", "Net Savings", "Current Spec",
		"Suggested Spec", "Parent Device", "Justification", "Additional Details",
	}
	var rows []*golang.CSVRow
	rows = append(rows, &golang.CSVRow{Row: headers})
	rows = append(rows, m.rdsInstanceProcessor.ExportCsv()...)
	rows = append(rows, m.rdsClusterProcessor.ExportCsv()...)

	return rows
}
