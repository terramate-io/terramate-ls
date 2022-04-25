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

package tmlsp_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/madlambda/spells/assert"
	"github.com/mineiros-io/terramate-lsp/test"
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
	f.Editor.Open("stack/terramate.tm.hcl")
	r := <-f.Editor.Requests
	assert.EqualStrings(t, "textDocument/publishDiagnostics", r.Method(),
		"unexpected notification request")

	var params lsp.PublishDiagnosticsParams
	assert.NoError(t, json.Unmarshal(r.Params(), &params), "unmarshaling params")
	assert.EqualInts(t, 0, len(params.Diagnostics))
	assert.EqualStrings(t, filepath.Join(stack.Path(), "terramate.tm.hcl"),
		params.URI.Filename())
}

func TestDocumentChange(t *testing.T) {
	type change struct {
		file string
		text string
	}
	type testcase struct {
		name   string
		layout []string
		change change
		want   []lsp.PublishDiagnosticsParams
	}

	for _, tc := range []testcase{
		{
			name: "empty workspace and empty file change",
			change: change{
				file: "terramate.tm",
				text: "",
			},
			want: []lsp.PublishDiagnosticsParams{
				{
					URI:         "terramate.tm",
					Diagnostics: []lsp.Diagnostic{},
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
			want: []lsp.PublishDiagnosticsParams{
				{
					URI: "bug.tm",
					Diagnostics: []lsp.Diagnostic{
						{
							Severity: lsp.DiagnosticSeverityError,
							Source:   "terramate",
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
					Diagnostics: []lsp.Diagnostic{},
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
				gotReq := <-f.Editor.Requests
				assert.EqualStrings(t, lsp.MethodTextDocumentPublishDiagnostics,
					gotReq.Method())

				var gotParams lsp.PublishDiagnosticsParams
				assert.NoError(t, json.Unmarshal(gotReq.Params(), &gotParams))
				assertDiagnostics(t, gotParams, want)
			}
		})
	}
}

func assertDiagnostics(t *testing.T, got, want lsp.PublishDiagnosticsParams) {
	assert.StrContains(t, got.URI.Filename(), want.URI.Filename())
	assert.EqualInts(t, int(want.Version), int(got.Version), "version mismatch")
	assert.EqualInts(t, len(want.Diagnostics), len(got.Diagnostics),
		"number of diagnostics mismatch")

	for i := 0; i < len(want.Diagnostics); i++ {
		dw := want.Diagnostics[i]
		dg := got.Diagnostics[i]

		assert.StrContains(t, dg.Message, dw.Message)
		assert.EqualStrings(t, dw.Source, dg.Source)
		if dw.Range != dg.Range {
			t.Fatalf("want[%v] is not got[%v]", dw, dg)
		}
	}
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
