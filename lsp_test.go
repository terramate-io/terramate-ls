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
	"bytes"
	"context"
	"testing"

	"go.lsp.dev/jsonrpc2"

	tmlsp "github.com/mineiros-io/terramate-lsp"
)

func runServer(t *testing.T) jsonrpc2.Conn {
	t.Helper()

	stream := jsonrpc2.NewStream(&testBuffer{})
	conn := jsonrpc2.NewConn(stream)
	server := tmlsp.NewServer(conn)
	conn.Go(context.Background(), server.Handler)

	t.Cleanup(func() {
		conn.Close()
		<-conn.Done()
	})
	return conn
}

type testBuffer struct {
	bytes.Buffer
}

func (tb *testBuffer) Close() error {
	return nil
}
