package rds_cluster

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
		ID:          "list_all_regions_for_rds_cluster",
		Description: "Listing all available regions (RDS Cluster)",
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
