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

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
