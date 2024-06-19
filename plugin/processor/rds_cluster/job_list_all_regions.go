package rds_cluster

import "context"

type ListAllRegionsJob struct {
	processor *Processor
}

func NewListAllRegionsJob(processor *Processor) *ListAllRegionsJob {
	return &ListAllRegionsJob{
		processor: processor,
	}
}

func (j *ListAllRegionsJob) Id() string {
	return "list_all_regions_for_rds_cluster"
}
func (j *ListAllRegionsJob) Description() string {
	return "Listing all available regions (RDS Cluster)"
}
func (j *ListAllRegionsJob) Run(ctx context.Context) error {
	regions, err := j.processor.provider.ListAllRegions()
	if err != nil {
		return err
	}
	for _, region := range regions {
		j.processor.jobQueue.Push(NewListRDSInstancesInRegionJob(j.processor, region))
	}
	return nil
}
