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

package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"syscall"

	tmlsp "github.com/mineiros-io/terramate-lsp"
	"go.lsp.dev/jsonrpc2"
)

var (
	mode    = flag.String("mode", "stdio", "communication mode (stdio|tcp|websocket)")
	version = flag.Bool("version", false, "print version and exit")
)

func main() {
	flag.Parse()

	if *version {
		fmt.Println(tmlsp.Version())
		os.Exit(0)
	}

	// TODO(i4k): implement other modes.
	if *mode != "stdio" {
		fmt.Println("We only support stdio mode")
		os.Exit(0)
	}

	runServer(&readWriter{os.Stdin, os.Stdout})
}

func runServer(conn io.ReadWriteCloser) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM)
	defer stop()

	log.Printf("Starting Terramate Language Server in %s mode ...", *mode)

	rpcConn := jsonrpc2.NewConn(jsonrpc2.NewStream(conn))
	server := tmlsp.NewServer(rpcConn)

	rpcConn.Go(ctx, server.Handler)
	<-rpcConn.Done()
}

type readWriter struct {
	io.Reader
	io.Writer
}

func (s *readWriter) Close() error { return nil }
