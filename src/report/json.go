package report

import (
	"encoding/json"
	"io"

	"github.com/solguardlabs/compassdtl/src/scenario"
)

func WriteScenarioResult(writer io.Writer, result scenario.Result) error {
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}
