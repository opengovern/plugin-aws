package main

import (
	"context"
	"github.com/kaytu-io/kaytu/cmd"
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/opengovern/plugin-aws/plugin"
)

func main() {
	ctx := cmd.AppendSignalHandling(context.Background())
	sdk.New(plugin.NewPlugin(), 4).Execute(ctx)
}
