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
	"context"
	"encoding/json"
	"io"
	"net"
	"testing"

	"go.lsp.dev/jsonrpc2"
	lsp "go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/google/go-cmp/cmp"
	"github.com/madlambda/spells/assert"
	tmlsp "github.com/mineiros-io/terramate-lsp"
	"github.com/rs/zerolog"

	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestInitialization(t *testing.T) {
	f := setup(t)

	want := lsp.InitializeResult{
		Capabilities: lsp.ServerCapabilities{
			CompletionProvider: &lsp.CompletionOptions{},
			DefinitionProvider: false,
			HoverProvider:      false,
			TextDocumentSync: map[string]interface{}{
				"change":    float64(1),
				"openClose": true,
				"save":      map[string]interface{}{},
			},
		},
	}
	got := lsp.InitializeResult{}

	_, err := f.editor.call(
		lsp.MethodInitialize,
		lsp.InitializeParams{
			RootURI: uri.File(f.sandbox.RootDir()),
		},
		&got)

	assert.NoError(t, err, "calling %q", lsp.MethodInitialize)

	if diff := cmp.Diff(got, want); diff != "" {
		t.Fatalf("init result differs, got(-) want(+):\n%s", diff)
	}

	gotReq := <-f.editor.requests
	assert.EqualStrings(t, lsp.MethodWindowShowMessage, gotReq.Method())

	gotParams := lsp.ShowMessageParams{}
	assert.NoError(t, json.Unmarshal(gotReq.Params(), &gotParams))
	if lsp.MessageTypeInfo != gotParams.Type {
		t.Fatalf("message type got %v != want %v", gotParams.Type, lsp.MessageTypeInfo)
	}
}

type fixture struct {
	sandbox sandbox.S
	editor  *editor
}

func setup(t *testing.T) fixture {
	t.Helper()

	// WHY: LSP is bidirectional, the editor calls the server
	// and the server also calls the editor (not only sending responses),
	// It is not a classic request/response protocol so we need both
	// running + connected through a pipe.

	editorRW, serverRW := net.Pipe()

	serverConn := jsonrpc2Conn(serverRW)
	s := tmlsp.NewServer(serverConn)
	serverConn.Go(context.Background(), s.Handler)

	editorConn := jsonrpc2Conn(editorRW)
	e := newEditor(editorConn)
	editorConn.Go(context.Background(), e.handler)

	t.Cleanup(func() {
		if err := editorConn.Close(); err != nil {
			t.Errorf("closing editor connection: %v", err)
		}
		if err := serverConn.Close(); err != nil {
			t.Errorf("closing server connection: %v", err)
		}

		<-editorConn.Done()
		<-serverConn.Done()

		// Now that we closed and waited for the editor to stop
		// we can check that no requests were left unhandled by the test
		select {
		case req := <-e.requests:
			{
				t.Fatalf("unhandled editor request: %v", req)
			}
		default:
		}
	})

	return fixture{
		editor:  e,
		sandbox: sandbox.New(t),
	}
}

type editor struct {
	conn     jsonrpc2.Conn
	requests chan jsonrpc2.Request
}

func newEditor(conn jsonrpc2.Conn) *editor {
	return &editor{
		conn:     conn,
		requests: make(chan jsonrpc2.Request),
	}
}

func (e *editor) handler(ctx context.Context, reply jsonrpc2.Replier, r jsonrpc2.Request) error {
	go func() {
		e.requests <- r
	}()
	return nil
}

func (e editor) call(method string, params, result interface{}) (jsonrpc2.ID, error) {
	return e.conn.Call(context.Background(), method, params, result)
}

func jsonrpc2Conn(rw io.ReadWriteCloser) jsonrpc2.Conn {
	stream := jsonrpc2.NewStream(rw)
	return jsonrpc2.NewConn(stream)
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
