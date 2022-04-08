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

package tmlsp

import (
	"context"
	"encoding/json"
	"path/filepath"

	"github.com/mineiros-io/terramate/errors"
	"github.com/mineiros-io/terramate/hcl"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"go.lsp.dev/jsonrpc2"
	lsp "go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// Server is the Language Server.
type Server struct {
	conn      jsonrpc2.Conn
	workspace string
	handlers  handlers

	log zerolog.Logger
}

// handler is a jsonrpc2.Handler with a custom logger.
type handler = func(
	l zerolog.Logger,
	ctx context.Context,
	reply jsonrpc2.Replier,
	req jsonrpc2.Request,
) error

type handlers map[string]handler

// NewServer creates a new language server.
func NewServer(conn jsonrpc2.Conn) *Server {
	s := &Server{
		conn: conn,
		log:  log.Logger, // by default uses global Logger
	}
	s.buildHandlers()
	return s
}

// ServerWithLogger creates a new language server with a custom logger.
func ServerWithLogger(conn jsonrpc2.Conn, l zerolog.Logger) *Server {
	s := &Server{
		conn: conn,
		log:  l,
	}
	s.buildHandlers()
	return s
}

func (s *Server) buildHandlers() {
	s.handlers = map[string]handler{
		lsp.MethodInitialize:          s.handleInitialize,
		lsp.MethodTextDocumentDidSave: s.handleDocumentSaved,
	}
}

// Handler handles the client requests.
func (s *Server) Handler(ctx context.Context, reply jsonrpc2.Replier, r jsonrpc2.Request) error {
	logger := s.log.With().
		Str("action", "server.Handler()").
		Str("method", r.Method()).
		Logger()

	logger.Debug().
		RawJSON("params", r.Params()).
		Msg("handling request.")

	if handler, ok := s.handlers[r.Method()]; ok {
		return handler(logger, ctx, reply, r)
	}

	logger.Trace().Msg("not implemented")
	return nil
}

func (s *Server) handleInitialize(
	log zerolog.Logger,
	ctx context.Context,
	reply jsonrpc2.Replier,
	r jsonrpc2.Request,
) error {
	type initParams struct {
		ProcessID int    `json:"processId,omitempty"`
		RootURI   string `json:"rootUri,omitempty"`
	}

	var params initParams
	if err := json.Unmarshal(r.Params(), &params); err != nil {
		log.Fatal().Err(err).Msg("failed to unmarshal params")
	}

	s.workspace = string(uri.New(params.RootURI).Filename())
	err := reply(ctx, lsp.InitializeResult{
		Capabilities: lsp.ServerCapabilities{
			CompletionProvider: &lsp.CompletionOptions{},

			// if we support `goto` definition.
			DefinitionProvider: false,

			// If we support `hover` info.
			HoverProvider: false,

			TextDocumentSync: lsp.TextDocumentSyncOptions{
				// Send all file content on every change (can be optimized later).
				Change: lsp.TextDocumentSyncKindFull,

				// if we want to be notified about open/close of Terramate files.
				OpenClose: true,
				Save: &lsp.SaveOptions{
					// If we want the file content on save,
					IncludeText: false,
				},
			},
		},
	}, nil)

	if err != nil {
		log.Fatal().Err(err).Msg("failed to reply")
	}

	log.Info().Msgf("client connected using workspace %q", s.workspace)

	err = s.conn.Notify(ctx, lsp.MethodWindowShowMessage, lsp.ShowMessageParams{
		Message: "connected to terramate-lsp",
		Type:    lsp.MessageTypeInfo,
	})

	if err != nil {
		log.Err(err).Msg("failed to notify client")
	}

	return nil
}

func (s *Server) handleDocumentSaved(
	log zerolog.Logger,
	ctx context.Context,
	reply jsonrpc2.Replier,
	r jsonrpc2.Request,
) error {
	type SaveParams struct {
		TextDocument struct {
			URI string `json:"uri"`
		} `json:"textDocument"`

		Text string `json:"text"`
	}

	var params SaveParams
	if err := json.Unmarshal(r.Params(), &params); err != nil {
		log.Err(err).Msg("failed to unmarshal params")
	}

	diags := []lsp.Diagnostic{}
	_, err := hcl.ParseDir(filepath.Dir(uri.New(params.TextDocument.URI).Filename()))
	if err != nil {
		e, ok := err.(*errors.Error)
		fileRange := lsp.Range{}

		if ok {
			fileRange.Start.Line = uint32(e.FileRange.Start.Line)
			fileRange.Start.Character = uint32(e.FileRange.Start.Byte)
			fileRange.End.Line = uint32(e.FileRange.End.Line)
			fileRange.End.Character = uint32(e.FileRange.End.Byte)
		}
		diags = append(diags, lsp.Diagnostic{
			Message: err.Error(),
			Range:   fileRange,
		})
	}

	err = s.conn.Notify(ctx, lsp.MethodTextDocumentPublishDiagnostics, lsp.PublishDiagnosticsParams{
		URI:         uri.URI(params.TextDocument.URI),
		Diagnostics: diags,
	})

	if err != nil {
		log.Fatal().Err(err).Msg("failed to send diagnostics to the client.")
	}

	return nil
}
