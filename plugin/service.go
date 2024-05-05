package plugin

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	awsConfig "github.com/kaytu-io/plugin-aws/plugin/aws"
	"github.com/kaytu-io/plugin-aws/plugin/preferences"
	processor2 "github.com/kaytu-io/plugin-aws/plugin/processor"
	"github.com/kaytu-io/plugin-aws/plugin/version"
)

type AWSPlugin struct {
	cfg       aws.Config
	stream    golang.Plugin_RegisterClient
	processor processor2.Processor
}

func NewPlugin() (*AWSPlugin, error) {
	return &AWSPlugin{}, nil
}

func (p *AWSPlugin) GetConfig() golang.RegisterConfig {
	return golang.RegisterConfig{
		Name:     "aws",
		Version:  version.VERSION,
		Provider: "aws",
		Commands: []*golang.Command{
			{
				Name:        "ec2-instance",
				Description: "Optimize your AWS EC2 Instances",
				Flags: []*golang.Flag{
					{
						Name:        "profile",
						Default:     "",
						Description: "AWS profile for authentication",
						Required:    false,
					},
				},
				DefaultPreferences: preferences.DefaultEC2Preferences,
			},
			{
				Name:        "rds-instance",
				Description: "Optimize your AWS RDS Instances",
				Flags: []*golang.Flag{
					{
						Name:        "profile",
						Default:     "",
						Description: "AWS profile for authentication",
						Required:    false,
					},
				},
				DefaultPreferences: preferences.DefaultRDSPreferences,
			},
		},
	}
}

func (p *AWSPlugin) SetStream(stream golang.Plugin_RegisterClient) {
	p.stream = stream
}

func (p *AWSPlugin) StartProcess(command string, flags map[string]string, kaytuAccessToken string) error {
	profile := flags["profile"]
	cfg, err := awsConfig.GetConfig(context.Background(), "", "", "", "", &profile, nil)
	if err != nil {
		return err
	}

	awsPrv, err := awsConfig.NewAWS(cfg)
	if err != nil {
		return err
	}

	cloudWatch, err := awsConfig.NewCloudWatch(cfg)
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

	if command == "ec2-instance" {
		p.processor = processor2.NewEC2InstanceProcessor(
			awsPrv,
			cloudWatch,
			identification,
			publishJobResult,
			publishError,
			publishOptimizationItem,
			kaytuAccessToken,
		)
	} else if command == "rds-instance" {
		p.processor = processor2.NewRDSInstanceProcessor(
			awsPrv,
			cloudWatch,
			identification,
			publishJobResult,
			publishError,
			publishOptimizationItem,
			kaytuAccessToken,
		)
	} else {
		return fmt.Errorf("invalid command: %s", command)
	}

	return nil
}

func (p *AWSPlugin) ReEvaluate(evaluate *golang.ReEvaluate) {
	p.processor.ReEvaluate(evaluate.Id, evaluate.Preferences)
}
