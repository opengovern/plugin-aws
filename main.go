package main

import (
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/plugin-aws/plugin"
)

func main() {
	sdk.New(plugin.NewPlugin()).Execute()
}
