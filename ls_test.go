// Copyright 2022 Mineiros GmbH
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package tmls_test

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate-ls/test"
	stackpkg "github.com/mineiros-io/terramate/stack"
	"github.com/rs/zerolog"
	lsp "go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

func TestInitialization(t *testing.T) {
	f := test.Setup(t)
	f.Editor.CheckInitialize()
}

func TestDocumentOpen(t *testing.T) {
	f := test.Setup(t)

	stack := f.Sandbox.CreateStack("stack")
	f.Editor.CheckInitialize()
	f.Editor.Open(fmt.Sprintf("stack/%s", stackpkg.DefaultFilename))
	r := <-f.Editor.Requests
	assert.EqualStrings(t, "textDocument/publishDiagnostics", r.Method(),
		"unexpected notification request")

	var params lsp.PublishDiagnosticsParams
	assert.NoError(t, json.Unmarshal(r.Params(), &params), "unmarshaling params")
	assert.EqualInts(t, 0, len(params.Diagnostics))
	assert.EqualStrings(t, filepath.Join(stack.Path(), stackpkg.DefaultFilename),
		params.URI.Filename())
}

func TestDocumentChange(t *testing.T) {
	type change struct {
		file string
		text string
	}
	type WantDiag struct {
		Range    lsp.Range
		Message  string
		Severity lsp.DiagnosticSeverity
	}
	type WantDiagParams struct {
		URI         lsp.URI
		Diagnostics []WantDiag
	}
	type testcase struct {
		name   string
		layout []string
		change change
		want   []WantDiagParams
	}

	for _, tc := range []testcase{
		{
			name: "empty workspace and empty file change",
			change: change{
				file: "terramate.tm",
				text: "",
			},
			want: []WantDiagParams{
				{
					URI:         "terramate.tm",
					Diagnostics: []WantDiag{},
				},
			},
		},
		{
			name: "workspace ok and empty file",
			layout: []string{
				"f:stack.tm:stack {}",
				"f:globals.tm:globals {}",
				"f:config.tm:terramate {}",
			},
			change: change{
				file: "empty.tm",
				text: "",
			},
			want: []WantDiagParams{
				{
					URI:         "config.tm",
					Diagnostics: []WantDiag{},
				},
				{
					URI:         "empty.tm",
					Diagnostics: []WantDiag{},
				},
				{
					URI:         "globals.tm",
					Diagnostics: []WantDiag{},
				},
				{
					URI:         "stack.tm",
					Diagnostics: []WantDiag{},
				},
			},
		},
		{
			name: "workspace with issues and empty file change",
			layout: []string{
				"f:bug.tm:bug",
			},
			change: change{
				file: "terramate.tm",
				text: "",
			},
			want: []WantDiagParams{
				{
					URI: "bug.tm",
					Diagnostics: []WantDiag{
						{
							Message:  "HCL syntax error",
							Severity: lsp.DiagnosticSeverityError,
							Range: lsp.Range{
								Start: lsp.Position{},
								End: lsp.Position{
									Character: 3,
								},
							},
						},
					},
				},
				{
					URI:         "terramate.tm",
					Diagnostics: []WantDiag{},
				},
			},
		},
		{
			name: "workspace with issues and file with issues",
			layout: []string{
				"f:bug.tm:bug",
			},
			change: change{
				file: "terramate.tm",
				text: "bug2",
			},
			want: []WantDiagParams{
				{
					URI: "bug.tm",
					Diagnostics: []WantDiag{
						{
							Message:  "HCL syntax error",
							Severity: lsp.DiagnosticSeverityError,
							Range: lsp.Range{
								Start: lsp.Position{},
								End: lsp.Position{
									Character: 3,
								},
							},
						},
					},
				},
				{
					URI: "terramate.tm",
					Diagnostics: []WantDiag{
						{
							Message:  "HCL syntax error",
							Severity: lsp.DiagnosticSeverityError,
							Range: lsp.Range{
								Start: lsp.Position{},
								End: lsp.Position{
									Character: 4,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "workspace with issues and file ok",
			layout: []string{
				"f:bug1.tm:bug1",
				"f:bug2.tm:terramate {test=1}",
			},
			change: change{
				file: "terramate.tm",
				text: "stack {}",
			},
			want: []WantDiagParams{
				{
					URI: "bug1.tm",
					Diagnostics: []WantDiag{
						{
							Message:  "HCL syntax error",
							Severity: lsp.DiagnosticSeverityError,
							Range: lsp.Range{
								Start: lsp.Position{},
								End: lsp.Position{
									Character: 4,
								},
							},
						},
					},
				},
				{
					URI: "bug2.tm",
					Diagnostics: []WantDiag{
						{
							Message:  "terramate schema error",
							Severity: lsp.DiagnosticSeverityError,
							Range: lsp.Range{
								Start: lsp.Position{
									Character: 11,
								},
								End: lsp.Position{
									Character: 15,
								},
							},
						},
					},
				},
				{
					URI:         "terramate.tm",
					Diagnostics: []WantDiag{},
				},
			},
		},
		{
			name: "multiple errors in the same file",
			change: change{
				file: "terramate.tm",
				text: `
terramate {
    a = 1
	config {
		b = 1
	}
	invalid {

	}
}
stack {
	n = "a"
}
`,
			},
			want: []WantDiagParams{
				{
					URI: "terramate.tm",
					Diagnostics: []WantDiag{
						{
							Message:  "terramate schema error",
							Severity: lsp.DiagnosticSeverityError,
							Range: lsp.Range{
								Start: lsp.Position{
									Line:      2,
									Character: 4,
								},
								End: lsp.Position{
									Line:      2,
									Character: 5,
								},
							},
						},
						{
							Message:  "terramate schema error",
							Severity: lsp.DiagnosticSeverityError,
							Range: lsp.Range{
								Start: lsp.Position{
									Line:      6,
									Character: 1,
								},
								End: lsp.Position{
									Line:      6,
									Character: 10,
								},
							},
						},
						{
							Message:  "terramate schema error",
							Severity: lsp.DiagnosticSeverityError,
							Range: lsp.Range{
								Start: lsp.Position{
									Line:      4,
									Character: 2,
								},
								End: lsp.Position{
									Line:      4,
									Character: 3,
								},
							},
						},
						{
							Message:  "terramate schema error",
							Severity: lsp.DiagnosticSeverityError,
							Range: lsp.Range{
								Start: lsp.Position{
									Line:      11,
									Character: 1,
								},
								End: lsp.Position{
									Line:      11,
									Character: 2,
								},
							},
						},
					},
				},
			},
		},
		{
			name: "multiple errors in the workspace",
			change: change{
				file: "terramate.tm",
				text: "",
			},
			layout: []string{
				`f:bug1.tm:terramate {
					a = 1
					b = 2
				}	
					`,
			},
			want: []WantDiagParams{
				{
					URI: "bug1.tm",
					Diagnostics: []WantDiag{
						{
							Message:  "terramate schema error",
							Severity: lsp.DiagnosticSeverityError,
							Range: lsp.Range{
								Start: lsp.Position{
									Line:      1,
									Character: 5,
								},
								End: lsp.Position{
									Line:      1,
									Character: 6,
								},
							},
						},
						{
							Message:  "terramate schema error",
							Severity: lsp.DiagnosticSeverityError,
							Range: lsp.Range{
								Start: lsp.Position{
									Line:      2,
									Character: 5,
								},
								End: lsp.Position{
									Line:      2,
									Character: 6,
								},
							},
						},
					},
				},
				{
					URI:         "terramate.tm",
					Diagnostics: []WantDiag{},
				},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := test.Setup(t, tc.layout...)
			f.Editor.CheckInitialize()

			f.Editor.Change(tc.change.file, tc.change.text)
			for i := 0; i < len(tc.want); i++ {
				want := tc.want[i]

				// fix the wanted path as it depends on the sandbox root.
				want.URI = uri.File(filepath.Join(f.Sandbox.RootDir(), string(want.URI)))
				select {
				case gotReq := <-f.Editor.Requests:
					assert.EqualStrings(t, lsp.MethodTextDocumentPublishDiagnostics,
						gotReq.Method())

					var gotParams lsp.PublishDiagnosticsParams
					assert.NoError(t, json.Unmarshal(gotReq.Params(), &gotParams))
					assert.EqualInts(t,
						len(gotParams.Diagnostics), len(want.Diagnostics),
						"number of diagnostics mismatch")

					assert.Partial(t, gotParams, want, "diagnostic mismatch")
				case <-time.After(10 * time.Millisecond):
					t.Fatal("expected more requests")
				}
			}
		})
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
