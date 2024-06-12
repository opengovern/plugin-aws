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
		OverviewChart: &golang.ChartDefinition{
			Columns: []*golang.ChartColumnItem{
				{
					Id:    "resource_id",
					Name:  "Resource ID",
					Width: 23,
				},
				{
					Id:    "resource_name",
					Name:  "Resource Name",
					Width: 23,
				},
				{
					Id:    "resource_type",
					Name:  "Resource Type",
					Width: 15,
				},
				{
					Id:    "region",
					Name:  "Region",
					Width: 15,
				},
				{
					Id:    "platform",
					Name:  "Platform",
					Width: 15,
				},
				{
					Id:    "total_saving",
					Name:  "Total Saving (Monthly)",
					Width: 40,
				},
				{
					Id:    "x_kaytu_right_arrow",
					Name:  "",
					Width: 1,
				},
			},
		},
		DevicesChart: &golang.ChartDefinition{
			Columns: []*golang.ChartColumnItem{
				{
					Id:    "resource_id",
					Name:  "Resource ID",
					Width: 23,
				},
				{
					Id:    "resource_name",
					Name:  "Resource Name",
					Width: 23,
				},
				{
					Id:    "resource_type",
					Name:  "ResourceType",
					Width: 15,
				},
				{
					Id:    "runtime",
					Name:  "Runtime",
					Width: 10,
				},
				{
					Id:    "current_cost",
					Name:  "Current Cost",
					Width: 20,
				},
				{
					Id:    "right_sized_cost",
					Name:  "Right sized Cost",
					Width: 20,
				},
				{
					Id:    "savings",
					Name:  "Savings",
					Width: 20,
				},
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

	publishOptimizationItem := func(item *golang.ChartOptimizationItem) {
		p.stream.Send(&golang.PluginMessage{
			PluginMessage: &golang.PluginMessage_Coi{
				Coi: item,
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

	publishResultSummary := func(summary *golang.ResultSummary) {
		p.stream.Send(&golang.PluginMessage{
			PluginMessage: &golang.PluginMessage_Summary{
				Summary: summary,
			},
		})
	}

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
			publishResultSummary,
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
