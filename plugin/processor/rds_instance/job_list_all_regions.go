package rds_instance

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
		ID:          "list_all_regions_rds_instance",
		Description: "Listing all available regions (RDS Instance)",
		MaxRetry:    0,
	}
}

func (j *ListAllRegionsJob) Run(ctx context.Context) error {
	regions, err := j.processor.provider.ListAllRegions(ctx)
	if err != nil {
		return err
	}
	for _, region := range regions {
		j.processor.jobQueue.Push(NewListRDSInstancesInRegionJob(j.processor, region))
	}
	return nil
}
