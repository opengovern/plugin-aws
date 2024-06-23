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
	"strings"
	"sync/atomic"
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
	lazyloadCounter         atomic.Uint32
	observabilityDays       int
	defaultPreferences      []*golang.PreferenceItem

	summary util.ConcurrentMap[string, EC2InstanceSummary]
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
	observabilityDays int,
	defaultPreferences []*golang.PreferenceItem,
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
		observabilityDays:       observabilityDays,
		defaultPreferences:      defaultPreferences,

		lazyloadCounter: atomic.Uint32{},

		summary: util.NewMap[string, EC2InstanceSummary](),
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

func (m *Processor) ExportNonInteractive() *golang.NonInteractiveExport {
	return &golang.NonInteractiveExport{
		Csv: m.exportCsv(),
	}
}

func (m *Processor) exportCsv() []*golang.CSVRow {
	headers := []string{
		"AccountID", "Region / AZ", "Resource Type", "Device ID", "Device Name", "Platform / Runtime Engine",
		"Device Runtime (Hrs)", "Current Cost", "Recommendation Cost", "Net Savings", "Current Spec",
		"Suggested Spec", "Parent Device", "Justification", "Additional Details",
	}
	var rows []*golang.CSVRow
	rows = append(rows, &golang.CSVRow{Row: headers})
	m.summary.Range(func(id string, _ EC2InstanceSummary) bool {
		if _, ok := m.items.Get(id); !ok {
			fmt.Println("Skipping item", id)
			return true
		}
		i, _ := m.items.Get(id)
		var name string
		for _, t := range i.Instance.Tags {
			if t.Key != nil && strings.ToLower(*t.Key) == "name" && t.Value != nil {
				name = *t.Value
			}
		}
		if name == "" {
			name = *i.Instance.InstanceId
		}
		var platform string
		if i.Instance.PlatformDetails != nil {
			platform = *i.Instance.PlatformDetails
		}
		var additionalDetails []string
		var rightSizingCost, saving, recSpec string
		if i.Wastage.RightSizing.Recommended != nil {
			rightSizingCost = utils.FormatPriceFloat(i.Wastage.RightSizing.Recommended.Cost)
			saving = utils.FormatPriceFloat(i.Wastage.RightSizing.Current.Cost - i.Wastage.RightSizing.Recommended.Cost)
			recSpec = i.Wastage.RightSizing.Recommended.InstanceType

			additionalDetails = append(additionalDetails,
				fmt.Sprintf("Instance Size:: Current: %s - Recommended: %s", i.Wastage.RightSizing.Current.InstanceType,
					i.Wastage.RightSizing.Recommended.InstanceType))
			additionalDetails = append(additionalDetails,
				fmt.Sprintf("vCPU:: Current: %d - Avg: %s - Recommended: %d", i.Wastage.RightSizing.Current.VCPU,
					utils.Percentage(i.Wastage.RightSizing.VCPU.Avg), i.Wastage.RightSizing.Recommended.VCPU))
			additionalDetails = append(additionalDetails,
				fmt.Sprintf("Processor(s):: Current: %s - Recommended: %s", i.Wastage.RightSizing.Current.Processor,
					i.Wastage.RightSizing.Recommended.Processor))
			additionalDetails = append(additionalDetails,
				fmt.Sprintf("Architecture:: Current: %s - Recommended: %s", i.Wastage.RightSizing.Current.Architecture,
					i.Wastage.RightSizing.Recommended.Architecture))
			additionalDetails = append(additionalDetails,
				fmt.Sprintf("License Cost:: Current: $%.2f - Recommended: $%.2f", i.Wastage.RightSizing.Current.LicensePrice,
					i.Wastage.RightSizing.Recommended.LicensePrice))
			additionalDetails = append(additionalDetails,
				fmt.Sprintf("Memory:: Current: %.1f GB - Avg: %s - Recommended: %.1f GB", i.Wastage.RightSizing.Current.Memory,
					utils.Percentage(i.Wastage.RightSizing.Memory.Avg), i.Wastage.RightSizing.Recommended.Memory))
			additionalDetails = append(additionalDetails,
				fmt.Sprintf("EBS Bandwidth:: Current: %s - Avg: %s - Recommended: %s", i.Wastage.RightSizing.Current.EBSBandwidth,
					PNetworkThroughputMBps(i.Wastage.RightSizing.EBSBandwidth.Avg), i.Wastage.RightSizing.Recommended.EBSBandwidth))
			additionalDetails = append(additionalDetails,
				fmt.Sprintf("EBS IOPS:: Current: %s - Avg: %s io/s - Recommended: %s", i.Wastage.RightSizing.Current.EBSIops,
					utils.PFloat64ToString(i.Wastage.RightSizing.EBSIops.Avg), i.Wastage.RightSizing.Recommended.EBSIops))
		}
		row := []string{m.identification["account"], i.Region, "EC2 Instance", *i.Instance.InstanceId, name, platform,
			"730 hours", utils.FormatPriceFloat(i.Wastage.RightSizing.Current.Cost), rightSizingCost, saving,
			i.Wastage.RightSizing.Current.InstanceType, recSpec, "None", i.Wastage.RightSizing.Description, strings.Join(additionalDetails, "---")}
		rows = append(rows, &golang.CSVRow{Row: row})
		for _, v := range i.Volumes {
			vs, ok := i.Wastage.VolumeRightSizing[utils.HashString(*v.VolumeId)]
			if !ok {
				continue
			}
			var vName string
			for _, t := range i.Instance.Tags {
				if t.Key != nil && strings.ToLower(*t.Key) == "name" && t.Value != nil {
					vName = *t.Value
				}
			}
			if vName == "" {
				vName = *v.VolumeId
			}

			var ebsAdditionalDetails []string
			var ebsRightSizingCost, ebsSaving, ebsRecSpec string
			if vs.Recommended != nil {
				ebsRightSizingCost = utils.FormatPriceFloat(vs.Recommended.Cost)
				ebsSaving = utils.FormatPriceFloat(vs.Current.Cost - vs.Recommended.Cost)
				ebsRecSpec = fmt.Sprintf("%s/%s/%d IOPS", vs.Recommended.Tier, utils.SizeByteToGB(vs.Recommended.VolumeSize), vs.Recommended.IOPS())

				ebsAdditionalDetails = append(ebsAdditionalDetails,
					fmt.Sprintf("EBS Storage Tier:: Current: %s - Recommended: %s", vs.Current.Tier,
						vs.Recommended.Tier))
				ebsAdditionalDetails = append(ebsAdditionalDetails,
					fmt.Sprintf("Volume Size (GB):: Current: %d - Recommended: %d", *vs.Current.VolumeSize,
						*vs.Recommended.VolumeSize))
				ebsAdditionalDetails = append(ebsAdditionalDetails,
					fmt.Sprintf("IOPS:: Current: %d - Avg: %s - Recommended: %d", vs.Current.IOPS(),
						utils.PFloat64ToString(vs.IOPS.Avg), vs.Recommended.IOPS()))
				ebsAdditionalDetails = append(ebsAdditionalDetails,
					fmt.Sprintf("Baseline IOPS:: Current: %d - Recommended: %d", vs.Current.BaselineIOPS,
						vs.Recommended.BaselineIOPS))
				ebsAdditionalDetails = append(ebsAdditionalDetails,
					fmt.Sprintf("Provisioned IOPS:: Current: %s - Recommended: %s", utils.PInt32ToString(vs.Current.ProvisionedIOPS),
						utils.PInt32ToString(vs.Recommended.ProvisionedIOPS)))
				ebsAdditionalDetails = append(ebsAdditionalDetails,
					fmt.Sprintf("Throughput (MB/s):: Current: %.2f GB - Avg: %s - Recommended: %.2f GB", vs.Current.Throughput(),
						PNetworkThroughputMBps(vs.Throughput.Avg), vs.Recommended.Throughput()))
				ebsAdditionalDetails = append(ebsAdditionalDetails,
					fmt.Sprintf("Baseline Throughput:: Current: %s - Recommended: %s", PNetworkThroughputMBps(&vs.Current.BaselineThroughput),
						PNetworkThroughputMBps(&vs.Recommended.BaselineThroughput)))
				ebsAdditionalDetails = append(ebsAdditionalDetails,
					fmt.Sprintf("Provisioned Throughput:: Current: %s - Recommended: %s", PNetworkThroughputMBps(vs.Current.ProvisionedThroughput),
						PNetworkThroughputMBps(vs.Recommended.ProvisionedThroughput)))
			}

			vRow := []string{m.identification["account"], i.Region, "EBS Volume", *v.VolumeId, vName, "N/A",
				"730 hours", utils.FormatPriceFloat(vs.Current.Cost), ebsRightSizingCost, ebsSaving,
				fmt.Sprintf("%s/%s/%d IOPS", vs.Current.Tier, utils.SizeByteToGB(vs.Current.VolumeSize), vs.Current.IOPS()),
				ebsRecSpec, *i.Instance.InstanceId, i.Wastage.RightSizing.Description, strings.Join(ebsAdditionalDetails, "---")}
			rows = append(rows, &golang.CSVRow{Row: vRow})
		}

		return true
	})
	return rows
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
	m.summary.Range(func(_ string, item EC2InstanceSummary) bool {
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
		for _, v := range i.Wastage.VolumeRightSizing {
			totalSaving += v.Current.Cost - v.Recommended.Cost
			totalCurrentCost += v.Current.Cost
		}
		totalSaving += i.Wastage.RightSizing.Current.Cost - i.Wastage.RightSizing.Recommended.Cost
		totalCurrentCost += i.Wastage.RightSizing.Current.Cost

		m.summary.Set(itemId, EC2InstanceSummary{
			CurrentRuntimeCost: totalCurrentCost,
			Savings:            totalSaving,
		})
	}
	m.publishResultSummary(m.ResultsSummary())
}
