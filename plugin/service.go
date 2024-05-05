package plugin

import (
	"context"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	awsConfig "github.com/kaytu-io/plugin-aws/plugin/aws"
)

var VERSION string

type AWSPlugin struct {
	cfg       aws.Config
	stream    golang.Plugin_RegisterClient
	processor *EC2InstanceProcessor
}

func NewPlugin() (*AWSPlugin, error) {
	return &AWSPlugin{}, nil
}

func (p *AWSPlugin) GetConfig() golang.RegisterConfig {
	return golang.RegisterConfig{
		Name:     "aws",
		Version:  VERSION,
		Provider: "aws",
		Commands: []*golang.Command{
			{
				Name:        "ec2-instance",
				Description: "Optimize your AWS EC2 Instances",
				Flags: []*golang.Flag{
					{
						Name:        "profile",
						Default:     "",
						Description: "AWS Profile",
						Required:    true,
					},
				},
			},
		},
	}
}

func (p *AWSPlugin) SetStream(stream golang.Plugin_RegisterClient) {
	p.stream = stream
}

func (p *AWSPlugin) StartProcess(flags map[string]string) error {
	profile := flags["profile"]
	cfg, err := awsConfig.GetConfig(context.Background(), "", "", "", "", &profile, nil)
	if err != nil {
		return err
	}

	awsPrv, err := NewAWS(cfg)
	if err != nil {
		return err
	}

	cloudWatch, err := NewCloudWatch(cfg)
	if err != nil {
		return err
	}

	identification, err := awsPrv.Identify()
	if err != nil {
		return err
	}

	publishJobResult := func(result *golang.JobResult) *golang.JobResult {
		p.stream.Send(&golang.PluginMessage{
			PluginMessage: &golang.PluginMessage_Job{
				Job: result,
			},
		})
		return result
	}

	publishError := func(err error) {
		p.stream.Send(&golang.PluginMessage{
			PluginMessage: &golang.PluginMessage_Err{
				Err: &golang.Error{
					Error: err.Error(),
				},
			},
		})
	}

	publishOptimizationItem := func(item *golang.OptimizationItem) {
		p.stream.Send(&golang.PluginMessage{
			PluginMessage: &golang.PluginMessage_Oi{
				Oi: item,
			},
		})
	}

	p.processor = NewEC2InstanceProcessor(
		awsPrv,
		cloudWatch,
		identification,
		publishJobResult,
		publishError,
		publishOptimizationItem,
	)

	return nil
}

func (p *AWSPlugin) ReEvaluate(evaluate *golang.ReEvaluate) {
	p.processor.ReEvaluate(evaluate.Id, evaluate.Preferences)
}
