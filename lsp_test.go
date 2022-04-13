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
