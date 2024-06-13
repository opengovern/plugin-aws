package processor

import "github.com/kaytu-io/kaytu/pkg/plugin/proto/src/golang"

type Processor interface {
	ReEvaluate(id string, items []*golang.PreferenceItem)
	ExportNonInteractive() *golang.NonInteractiveExport
}
