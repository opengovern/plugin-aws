package kaytu

func (v RightsizingEBSVolume) IOPS() int32 {
	val := v.BaselineIOPS
	if v.ProvisionedIOPS != nil {
		val += *v.ProvisionedIOPS
	}
	return val
}

func (v RightsizingEBSVolume) Throughput() float64 {
	val := v.BaselineThroughput
	if v.ProvisionedThroughput != nil {
		val += *v.ProvisionedThroughput
	}
	return val
}
