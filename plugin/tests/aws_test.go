package tests

import (
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	rdstype "github.com/aws/aws-sdk-go-v2/service/rds/types"
	"github.com/stretchr/testify/suite"
	"testing"
)

type AWSTestSuite struct {
	suite.Suite
}

func TestAWS(t *testing.T) {
	suite.Run(t, &AWSTestSuite{})
}

func (ts *AWSTestSuite) SetupSuite() {

	return
}

type AWSAPI interface {
	Identify() (map[string]string, error)
	ListAllRegions() ([]string, error)
	ListInstances(region string) ([]types.Instance, error)
	ListAttachedVolumes(region string, instance types.Instance) ([]types.Volume, error)
	ListRDSInstance(region string) ([]rdstype.DBInstance, error)
}

type MockAWS struct{}

func (m *MockAWS) Identify() (map[string]string, error) {
	return nil, nil
}

func (m *MockAWS) ListAllRegions() ([]string, error) {
	return nil, nil
}

func (m *MockAWS) ListInstances(region string) ([]types.Instance, error) {
	return nil, nil
}

func (m *MockAWS) ListAttachedVolumes(region string, instance types.Instance) ([]types.Volume, error) {
	return nil, nil
}

func (m *MockAWS) ListRDSInstance(region string) ([]rdstype.DBInstance, error) {
	return nil, nil
}
