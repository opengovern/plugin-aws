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
	"strings"
	"sync/atomic"
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
	lazyloadCounter         *atomic.Uint32
	observabilityDays       int

	summary *util.ConcurrentMap[string, ec2_instance.EC2InstanceSummary]
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
	lazyloadCounter *atomic.Uint32,
	observabilityDays int,
	summary *util.ConcurrentMap[string, ec2_instance.EC2InstanceSummary],
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

func (m *Processor) ExportNonInteractive() *golang.NonInteractiveExport {
	return nil
}

func (m *Processor) ExportCsv() []*golang.CSVRow {
	var rows []*golang.CSVRow

	m.summary.Range(func(id string, _ ec2_instance.EC2InstanceSummary) bool {
		if _, ok := m.items.Get(id); !ok {
			fmt.Println("Skipping item", id)
			return true
		}
		i, _ := m.items.Get(id)
		var platform string
		if i.Instance.Engine != nil {
			platform = *i.Instance.Engine
		}
		var computeAdditionalDetails []string
		var computeRightSizingCost, computeSaving, computeRecSpec string
		if i.Wastage.RightSizing.Recommended != nil {
			computeRightSizingCost = utils.FormatPriceFloat(i.Wastage.RightSizing.Recommended.ComputeCost)
			computeSaving = utils.FormatPriceFloat(i.Wastage.RightSizing.Current.ComputeCost - i.Wastage.RightSizing.Recommended.ComputeCost)
			computeRecSpec = i.Wastage.RightSizing.Recommended.InstanceType

			computeAdditionalDetails = append(computeAdditionalDetails,
				fmt.Sprintf("Instance Size:: Current: %s - Recommended: %s", i.Wastage.RightSizing.Current.InstanceType,
					i.Wastage.RightSizing.Recommended.InstanceType))
			computeAdditionalDetails = append(computeAdditionalDetails,
				fmt.Sprintf("Engine:: Current: %s - Recommended: %s", i.Wastage.RightSizing.Current.Engine,
					i.Wastage.RightSizing.Recommended.Engine))
			computeAdditionalDetails = append(computeAdditionalDetails,
				fmt.Sprintf("Engine Version:: Current: %s - Recommended: %s", i.Wastage.RightSizing.Current.EngineVersion,
					i.Wastage.RightSizing.Recommended.EngineVersion))
			computeAdditionalDetails = append(computeAdditionalDetails,
				fmt.Sprintf("Cluster Type:: Current: %s - Recommended: %s", i.Wastage.RightSizing.Current.ClusterType,
					i.Wastage.RightSizing.Recommended.ClusterType))
			computeAdditionalDetails = append(computeAdditionalDetails,
				fmt.Sprintf("vCPU:: Current: %d - Avg: %s - Recommended: %d", i.Wastage.RightSizing.Current.VCPU,
					utils.Percentage(i.Wastage.RightSizing.VCPU.Avg), i.Wastage.RightSizing.Recommended.VCPU))
			computeAdditionalDetails = append(computeAdditionalDetails,
				fmt.Sprintf("Processor(s):: Current: %s - Recommended: %s", i.Wastage.RightSizing.Current.Processor,
					i.Wastage.RightSizing.Recommended.Processor))
			computeAdditionalDetails = append(computeAdditionalDetails,
				fmt.Sprintf("Architecture:: Current: %s - Recommended: %s", i.Wastage.RightSizing.Current.Architecture,
					i.Wastage.RightSizing.Recommended.Architecture))
			computeAdditionalDetails = append(computeAdditionalDetails,
				fmt.Sprintf("Memory:: Current: %d GB - Avg: %s - Recommended: %d GB", i.Wastage.RightSizing.Current.MemoryGb,
					utils.MemoryUsagePercentageByFreeSpace(i.Wastage.RightSizing.FreeMemoryBytes.Avg, float64(i.Wastage.RightSizing.Current.MemoryGb)),
					i.Wastage.RightSizing.Recommended.MemoryGb))
		}
		computeRow := []string{m.identification["account"], i.Region, "RDS Instance Compute", fmt.Sprintf("%s-compute", *i.Instance.DBInstanceIdentifier),
			*i.Instance.DBInstanceIdentifier, platform, "730 hours", utils.FormatPriceFloat(i.Wastage.RightSizing.Current.ComputeCost),
			computeRightSizingCost, computeSaving, i.Wastage.RightSizing.Current.InstanceType, computeRecSpec, *i.Instance.DBInstanceIdentifier,
			i.Wastage.RightSizing.Description, strings.Join(computeAdditionalDetails, "---")}
		rows = append(rows, &golang.CSVRow{Row: computeRow})

		var storageAdditionalDetails []string
		var storageRightSizingCost, storageSaving, storageRecSpec string
		if i.Wastage.RightSizing.Recommended != nil {
			storageRightSizingCost = utils.FormatPriceFloat(i.Wastage.RightSizing.Recommended.StorageCost)
			storageSaving = utils.FormatPriceFloat(i.Wastage.RightSizing.Current.StorageCost - i.Wastage.RightSizing.Recommended.StorageCost)
			storageRecSpec = fmt.Sprintf("%s/%s/%s IOPS", *i.Wastage.RightSizing.Recommended.StorageType,
				utils.SizeByteToGB(i.Wastage.RightSizing.Recommended.StorageSize), utils.PInt32ToString(i.Wastage.RightSizing.Recommended.StorageIops))

			storageAdditionalDetails = append(storageAdditionalDetails,
				fmt.Sprintf("Type:: Current: %s - Recommended: %s", utils.PString(i.Wastage.RightSizing.Current.StorageType),
					utils.PString(i.Wastage.RightSizing.Recommended.StorageType)))
			storageAdditionalDetails = append(storageAdditionalDetails,
				fmt.Sprintf("Size:: Current: %s - Avg : %s - Recommended: %s", utils.SizeByteToGB(i.Wastage.RightSizing.Current.StorageSize),
					utils.StorageUsagePercentageByFreeSpace(i.Wastage.RightSizing.FreeStorageBytes.Avg, i.Wastage.RightSizing.Current.StorageSize),
					utils.SizeByteToGB(i.Wastage.RightSizing.Current.StorageSize)))
			storageAdditionalDetails = append(storageAdditionalDetails,
				fmt.Sprintf("IOPS:: Current: %s - Avg: %s - Recommended: %s", utils.PInt32ToString(i.Wastage.RightSizing.Current.StorageIops),
					fmt.Sprintf("%s io/s", utils.PFloat64ToString(i.Wastage.RightSizing.StorageIops.Avg)),
					utils.PInt32ToString(i.Wastage.RightSizing.Recommended.StorageIops)))
			storageAdditionalDetails = append(storageAdditionalDetails,
				fmt.Sprintf("Throughput:: Current: %s - Avg: %s - Recommended: %s", utils.PStorageThroughputMbps(i.Wastage.RightSizing.Current.StorageThroughput),
					utils.PStorageThroughputMbps(i.Wastage.RightSizing.StorageThroughput.Avg), utils.PStorageThroughputMbps(i.Wastage.RightSizing.Recommended.StorageThroughput)))
		}
		storageRow := []string{m.identification["account"], i.Region, "RDS Instance Storage", fmt.Sprintf("%s-storage", *i.Instance.DBInstanceIdentifier),
			*i.Instance.DBInstanceIdentifier, "N/A", "730 hours", utils.FormatPriceFloat(i.Wastage.RightSizing.Current.StorageCost),
			storageRightSizingCost, storageSaving, fmt.Sprintf("%s/%s/%s IOPS", *i.Wastage.RightSizing.Current.StorageType,
				utils.SizeByteToGB(i.Wastage.RightSizing.Current.StorageSize), utils.PInt32ToString(i.Wastage.RightSizing.Current.StorageIops)), storageRecSpec, *i.Instance.DBInstanceIdentifier,
			i.Wastage.RightSizing.Description, strings.Join(storageAdditionalDetails, "---")}
		rows = append(rows, &golang.CSVRow{Row: storageRow})

		return true
	})
	return rows
}

func (m *Processor) HasItem(id string) bool {
	_, ok := m.items.Get(id)
	return ok
}

func (m *Processor) ResultsSummary() *golang.ResultSummary {
	summary := &golang.ResultSummary{}
	var totalCost, savings float64

	m.summary.Range(func(_ string, item ec2_instance.EC2InstanceSummary) bool {
		totalCost += item.CurrentRuntimeCost
		savings += item.Savings
		return true
	})

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

		m.summary.Set(itemId, ec2_instance.EC2InstanceSummary{
			CurrentRuntimeCost: totalCurrentCost,
			Savings:            totalSaving,
		})
	}
	m.publishResultSummary(m.ResultsSummary())
}
