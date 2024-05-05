package main

import (
	"github.com/kaytu-io/kaytu/pkg/plugin/sdk"
	"github.com/kaytu-io/plugin-aws/plugin"
)

func main() {
	plg, err := plugin.NewPlugin()
	if err != nil {
		panic(err)
	}

	sdk.New(plg).Execute()
}
