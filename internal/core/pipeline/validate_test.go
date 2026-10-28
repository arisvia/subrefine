package pipeline_test

import (
	"strings"
	"testing"

	"github.com/arisvia/subrefine/internal/core/pipeline"
	"github.com/arisvia/subrefine/internal/storage"
)

func TestValidateStepsAcceptsRealConfiguration(t *testing.T) {
	steps := []storage.PipelineStep{
		{Type: "filter", Include: `^(HK|JP)`, Exclude: `到期`, Types: []string{"ss", "hy2", "wg", "bogus"}},
		{Type: "rename", Pattern: `^(HK|JP)-`, Repl: "[$1] "},
		{Type: "deduplicate"},
		{Type: "sort", By: "tag", Order: "desc"},
		{Type: "decorate", Fingerprint: "chrome", Detour: "WARP"},
	}

	if err := pipeline.ValidateSteps(steps); err != nil {
		t.Fatalf("ValidateSteps(%v) = %v, want nil", steps, err)
	}
	if err := pipeline.ValidateSteps(nil); err != nil {
		t.Fatalf("empty pipeline = %v, want nil", err)
	}
}

func TestValidateStepsRejectsSilentNoOps(t *testing.T) {
	cases := []struct {
		name    string
		steps   []storage.PipelineStep
		wantMsg string
	}{
		{name: "missing type", steps: []storage.PipelineStep{{}}, wantMsg: "unknown operator"},
		{name: "invented type", steps: []storage.PipelineStep{{Type: "quantum"}}, wantMsg: "unknown operator"},
		{name: "bad rename regex", steps: []storage.PipelineStep{{Type: "rename", Pattern: "(?"}}, wantMsg: "invalid pattern"},
		{name: "bad include regex", steps: []storage.PipelineStep{{Type: "filter", Include: "[a-"}}, wantMsg: "invalid include pattern"},
		{name: "bad exclude regex", steps: []storage.PipelineStep{{Type: "filter", Exclude: "*x"}}, wantMsg: "invalid exclude pattern"},
		// Every token unrecognizable would fail open and deliver all nodes.
		{name: "no known protocol", steps: []storage.PipelineStep{{Type: "filter", Types: []string{"nope"}}}, wantMsg: "no recognizable protocol"},
		{name: "bad sort key", steps: []storage.PipelineStep{{Type: "sort", By: "latency"}}, wantMsg: "unknown sort key"},
		{name: "bad sort order", steps: []storage.PipelineStep{{Type: "sort", By: "tag", Order: "random"}}, wantMsg: "unknown sort order"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := pipeline.ValidateSteps(tc.steps)
			if err == nil {
				t.Fatalf("ValidateSteps = nil, want an error mentioning %q", tc.wantMsg)
			}
			if !strings.Contains(err.Error(), tc.wantMsg) {
				t.Fatalf("error = %q, want it to mention %q", err, tc.wantMsg)
			}
			// The operator must be able to find which step is wrong.
			if !strings.HasPrefix(err.Error(), "pipeline step 1") {
				t.Fatalf("error = %q, want it to name the step", err)
			}
		})
	}
}
