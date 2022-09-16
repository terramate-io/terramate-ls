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

package test

import (
	"context"
	"io"
	"net"
	"testing"

	tmls "github.com/mineiros-io/terramate-ls"
	"github.com/mineiros-io/terramate/test/sandbox"
	"go.lsp.dev/jsonrpc2"
)

// Fixture is the default test fixture.
type Fixture struct {
	Sandbox sandbox.S
	Editor  *Editor
}

// Setup a new fixture.
func Setup(t *testing.T, layout ...string) Fixture {
	t.Helper()

	s := sandbox.NoGit(t)
	s.BuildTree(layout)

	// WHY: LSP is bidirectional, the editor calls the server
	// and the server also calls the editor (not only sending responses),
	// It is not a classic request/response protocol so we need both
	// running + connected through a pipe.

	editorRW, serverRW := net.Pipe()

	serverConn := jsonrpc2Conn(serverRW)
	server := tmls.NewServer(serverConn)
	serverConn.Go(context.Background(), server.Handler)

	editorConn := jsonrpc2Conn(editorRW)
	e := NewEditor(t, s, editorConn)
	editorConn.Go(context.Background(), e.Handler)

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
		case req := <-e.Requests:
			{
				t.Fatalf("unhandled editor request: %s %s", req.Method(), req.Params())
			}
		default:
		}
	})

	return Fixture{
		Editor:  e,
		Sandbox: s,
	}
}

func jsonrpc2Conn(rw io.ReadWriteCloser) jsonrpc2.Conn {
	stream := jsonrpc2.NewStream(rw)
	return jsonrpc2.NewConn(stream)
}
