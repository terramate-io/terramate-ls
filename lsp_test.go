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
	s := sandbox.New(t)
	server := runServer(t)
	got := lsp.InitializeResult{}
	_, err := server.Call(
		lsp.MethodInitialize,
		lsp.InitializeParams{
			RootURI: uri.File(s.RootDir()),
		},
		&got)
	assert.NoError(t, err, "calling %q", lsp.MethodInitialize)
}

func runServer(t *testing.T) server {
	t.Helper()

	reader, writer := net.Pipe()
	stream := jsonrpc2.NewStream(&testBuffer{reader, writer})
	conn := jsonrpc2.NewConn(stream)
	s := tmlsp.NewServer(conn)
	conn.Go(context.Background(), s.Handler)

	t.Cleanup(func() {
		conn.Close()
		<-conn.Done()
	})
	return server{conn: conn}
}

type server struct {
	conn jsonrpc2.Conn
}

func (s server) Call(method string, params, result interface{}) (jsonrpc2.ID, error) {
	return s.conn.Call(context.Background(), method, params, result)
}

type testBuffer struct {
	io.Reader
	io.Writer
}

func (tb *testBuffer) Close() error {
	return nil
}

func init() {
	zerolog.SetGlobalLevel(zerolog.Disabled)
}
