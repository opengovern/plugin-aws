package ec2_instance

import (
	"context"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
)

type ListAllRegionsJob struct {
	processor *Processor
}

func NewListAllRegionsJob(processor *Processor) *ListAllRegionsJob {
	return &ListAllRegionsJob{
		processor: processor,
	}
}

func (j *ListAllRegionsJob) Properties() sdk.JobProperties {
	return sdk.JobProperties{
		ID:          "list_all_regions_for_ec2_instance",
		Description: "Listing all available regions (EC2 Instance)",
		MaxRetry:    0,
	}
}

func (j *ListAllRegionsJob) Run(ctx context.Context) error {
	regions, err := j.processor.provider.ListAllRegions(ctx)
	if err != nil {
		return err
	}
	for _, region := range regions {
		j.processor.jobQueue.Push(NewListEC2InstancesInRegionJob(j.processor, region))
	}
	return nil
}
