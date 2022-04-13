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
	"fmt"
	"io"
	"net"
	"testing"

	"go.lsp.dev/jsonrpc2"
	lsp "go.lsp.dev/protocol"
	"go.lsp.dev/uri"

	"github.com/madlambda/spells/assert"
	tmlsp "github.com/mineiros-io/terramate-lsp"
	"github.com/rs/zerolog"

	"github.com/mineiros-io/terramate/test/sandbox"
)

func TestInitialization(t *testing.T) {
	f := setup(t)
	got := lsp.InitializeResult{}
	_, err := f.server.Call(
		lsp.MethodInitialize,
		lsp.InitializeParams{
			RootURI: uri.File(f.sandbox.RootDir()),
		},
		&got)
	assert.NoError(t, err, "calling %q", lsp.MethodInitialize)
}

type fixture struct {
	sandbox sandbox.S
	server  *server
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

	e := &editor{}
	editorConn := jsonrpc2Conn(editorRW)
	editorConn.Go(context.Background(), e.Handler)

	t.Cleanup(func() {
		editorConn.Close()
		serverConn.Close()

		<-editorConn.Done()
		<-serverConn.Done()
	})

	return fixture{
		sandbox: sandbox.New(t),
		editor:  e,
		server:  &server{conn: serverConn},
	}
}

type editor struct {
}

func (e *editor) Handler(ctx context.Context, reply jsonrpc2.Replier, r jsonrpc2.Request) error {
	fmt.Println("request method", r.Method())
	fmt.Println("request params", string(r.Params()))
	return jsonrpc2.ErrMethodNotFound
}

type server struct {
	conn jsonrpc2.Conn
}

func (s server) Call(method string, params, result interface{}) (jsonrpc2.ID, error) {
	return s.conn.Call(context.Background(), method, params, result)
}

func jsonrpc2Conn(rw io.ReadWriteCloser) jsonrpc2.Conn {
	stream := jsonrpc2.NewStream(rw)
	return jsonrpc2.NewConn(stream)
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
