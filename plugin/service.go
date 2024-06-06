package plugin

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	awsConfig "github.com/kaytu-io/plugin-aws/plugin/aws"
	"github.com/kaytu-io/plugin-aws/plugin/kaytu"
	"github.com/kaytu-io/plugin-aws/plugin/preferences"
	processor2 "github.com/kaytu-io/plugin-aws/plugin/processor"
	"github.com/kaytu-io/plugin-aws/plugin/processor/ec2_instance"
	"github.com/kaytu-io/plugin-aws/plugin/version"
	"math"
	"strconv"
	"strings"
)

type AWSPlugin struct {
	cfg       aws.Config
	stream    golang.Plugin_RegisterClient
	processor processor2.Processor
}

func NewPlugin() *AWSPlugin {
	return &AWSPlugin{}
}

func (p *AWSPlugin) GetConfig() golang.RegisterConfig {
	return golang.RegisterConfig{
		Name:     "kaytu-io/plugin-aws",
		Version:  version.VERSION,
		Provider: "aws",
		Commands: []*golang.Command{
			{
				Name:        "ec2-instance",
				Description: "Get optimization suggestions for your AWS EC2 Instances",
				Flags: []*golang.Flag{
					{
						Name:        "profile",
						Default:     "",
						Description: "AWS profile for authentication",
						Required:    false,
					},
					{
						Name:        "observabilityDays",
						Default:     "1",
						Description: "Observability Days",
						Required:    false,
					},
				},
				DefaultPreferences: preferences.DefaultEC2Preferences,
				LoginRequired:      true,
			},
			{
				Name:        "rds-instance",
				Description: "Get optimization suggestions for your AWS RDS Instances",
				Flags: []*golang.Flag{
					{
						Name:        "profile",
						Default:     "",
						Description: "AWS profile for authentication",
						Required:    false,
					},
					{
						Name:        "observabilityDays",
						Default:     "1",
						Description: "Observability Days",
						Required:    false,
					},
				},
				DefaultPreferences: preferences.DefaultRDSPreferences,
				LoginRequired:      true,
			},
		},
	}
}

func (p *AWSPlugin) SetStream(stream golang.Plugin_RegisterClient) {
	p.stream = stream
}

func (p *AWSPlugin) StartProcess(command string, flags map[string]string, kaytuAccessToken string, jobQueue *sdk.JobQueue) error {
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

	configurations, err := kaytu.ConfigurationRequest()
	if err != nil {
		return err
	}

	for key, value := range flags {
		if key == "output" && value != "" && value != "interactive" {
			configurations.EC2LazyLoad = math.MaxInt
			configurations.RDSLazyLoad = math.MaxInt
		}
	}

	publishOptimizationItem := func(item *golang.OptimizationItem) {
		p.stream.Send(&golang.PluginMessage{
			PluginMessage: &golang.PluginMessage_Oi{
				Oi: item,
			},
		})
	}

	publishResultsReady := func(b bool) {
		p.stream.Send(&golang.PluginMessage{
			PluginMessage: &golang.PluginMessage_Ready{
				Ready: &golang.ResultsReady{
					Ready: b,
				},
			},
		})
	}
	publishResultsReady(false)

	observabilityDays := 1
	if flags["observabilityDays"] != "" {
		days, _ := strconv.ParseInt(strings.TrimSpace(flags["observabilityDays"]), 10, 64)
		if days > 0 {
			observabilityDays = int(days)
		}
	}
	if command == "ec2-instance" {
		p.processor = ec2_instance.NewProcessor(
			awsPrv,
			cloudWatch,
			identification,
			publishOptimizationItem,
			kaytuAccessToken,
			jobQueue,
			configurations,
			&sdk.SafeCounter{},
			observabilityDays,
		)
	} else if command == "rds-instance" {
		p.processor = processor2.NewRDSProcessor(
			awsPrv,
			cloudWatch,
			identification,
			publishOptimizationItem,
			kaytuAccessToken,
			jobQueue,
			configurations,
			observabilityDays,
		)
	} else {
		return fmt.Errorf("invalid command: %s", command)
	}
	jobQueue.SetOnFinish(func() {
		publishResultsReady(true)
	})

	return nil
}

func (p *AWSPlugin) ReEvaluate(evaluate *golang.ReEvaluate) {
	p.processor.ReEvaluate(evaluate.Id, evaluate.Preferences)
}
