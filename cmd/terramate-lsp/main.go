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

	"os"
	"os/signal"
	"syscall"
	"time"

	tmlsp "github.com/mineiros-io/terramate-lsp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.lsp.dev/jsonrpc2"
)

const (
	defaultLogLevel = "info"
	defaultLogFmt   = "console"
)

var (
	modeFlag     = flag.String("mode", "stdio", "communication mode (stdio)")
	versionFlag  = flag.Bool("version", false, "print version and exit")
	logLevelFlag = flag.String(
		"log-level", defaultLogLevel,
		"Log level to use: 'trace', 'debug', 'info', 'warn', 'error', or 'fatal'",
	)
	logFmtFlag = flag.String(
		"log-fmt", defaultLogFmt,
		"Log format to use: 'console', 'text', or 'json'.",
	)
)

func main() {
	flag.Parse()

	if *versionFlag {
		fmt.Println(tmlsp.Version())
		os.Exit(0)
	}

	// TODO(i4k): implement other modes.
	if *modeFlag != "stdio" {
		fmt.Println("terramate-lsp only supports stdio mode")
		os.Exit(1)
	}

	configureLogging(*logLevelFlag, *logFmtFlag, os.Stderr)
	runServer(&readWriter{os.Stdin, os.Stdout})
}

func runServer(conn io.ReadWriteCloser) {
	logger := log.With().
		Str("action", "main.runServer()").
		Logger()

	logger.Trace().Msg("Creating context for OS signals.")

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill, syscall.SIGTERM)
	defer stop()

	logger.Info().
		Str("mode", *modeFlag).
		Msg("Starting Terramate Language Server")

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

func configureLogging(logLevel string, logFmt string, output io.Writer) {
	zloglevel, err := zerolog.ParseLevel(logLevel)

	if err != nil {
		zloglevel = zerolog.FatalLevel
	}

	zerolog.SetGlobalLevel(zloglevel)

	if logFmt == "json" {
		zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
		log.Logger = log.Output(output)
	} else if logFmt == "text" { // no color
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: output, NoColor: true, TimeFormat: time.RFC3339})
	} else { // default: console mode using color
		log.Logger = log.Output(zerolog.ConsoleWriter{Out: output, NoColor: false, TimeFormat: time.RFC3339})
	}
}
