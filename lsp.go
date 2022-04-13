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
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	ctx context.Context,
	reply jsonrpc2.Replier,
	req jsonrpc2.Request,
	log zerolog.Logger,
) error

type handlers map[string]handler

// NewServer creates a new language server.
func NewServer(conn jsonrpc2.Conn) *Server {
	return ServerWithLogger(conn, log.Logger)
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
		lsp.MethodInitialize:            s.handleInitialize,
		lsp.MethodTextDocumentDidOpen:   s.handleDocumentOpen,
		lsp.MethodTextDocumentDidChange: s.handleDocumentChange,
		lsp.MethodTextDocumentDidSave:   s.handleDocumentSaved,
	}
}

// Handler handles the client requests.
func (s *Server) Handler(ctx context.Context, reply jsonrpc2.Replier, r jsonrpc2.Request) error {
	logger := s.log.With().
		Str("action", "server.Handler()").
		Str("workspace", s.workspace).
		Str("method", r.Method()).
		Logger()

	logger.Debug().
		RawJSON("params", r.Params()).
		Msg("handling request.")

	if handler, ok := s.handlers[r.Method()]; ok {
		return handler(ctx, reply, r, logger)
	}

	logger.Trace().Msg("not implemented")
	return jsonrpc2.ErrMethodNotFound
}

func (s *Server) handleInitialize(
	ctx context.Context,
	reply jsonrpc2.Replier,
	r jsonrpc2.Request,
	log zerolog.Logger,
) error {
	type initParams struct {
		ProcessID int    `json:"processId,omitempty"`
		RootURI   string `json:"rootUri,omitempty"`
	}

	var params initParams
	if err := json.Unmarshal(r.Params(), &params); err != nil {
		log.Error().Err(err).Msg("failed to unmarshal params")
		// TODO(i4k): we should check if it's a json.UnmarshallTypeErr or
		// json.UnmarshalFieldError to return jsonrpc2.ErrInvalidParams and
		// json.ErrParse otherwise.
		return jsonrpc2.ErrInvalidParams
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
		// WHY(i4k): in stdio mode it's impossible to have network issues.
		// TODO(i4k): improve this for the networked server.
		log.Fatal().Err(err).Msg("failed to reply")
	}

	log.Info().Msgf("client connected using workspace %q", s.workspace)

	err = s.conn.Notify(ctx, lsp.MethodWindowShowMessage, lsp.ShowMessageParams{
		Message: "connected to terramate-lsp",
		Type:    lsp.MessageTypeInfo,
	})

	if err != nil {
		log.Fatal().Err(err).Msg("failed to notify client")
	}
	return nil
}

func (s *Server) handleDocumentOpen(
	ctx context.Context,
	reply jsonrpc2.Replier,
	r jsonrpc2.Request,
	log zerolog.Logger,
) error {
	var params lsp.DidOpenTextDocumentParams
	if err := json.Unmarshal(r.Params(), &params); err != nil {
		log.Error().Err(err).Msg("failed to unmarshal params")
		return jsonrpc2.ErrParse
	}

	fname := params.TextDocument.URI.Filename()
	content := params.TextDocument.Text

	err := checkFile(fname, content)
	return s.sendErrorDiagnostics(ctx, params.TextDocument.URI, err)
}

func (s *Server) handleDocumentChange(
	ctx context.Context,
	reply jsonrpc2.Replier,
	r jsonrpc2.Request,
	log zerolog.Logger,
) error {
	var params lsp.DidChangeTextDocumentParams
	if err := json.Unmarshal(r.Params(), &params); err != nil {
		log.Error().Err(err).Msg("failed to unmarshal params")
		return err
	}

	if len(params.ContentChanges) != 1 {
		err := fmt.Errorf("expected content changes = 1, got = %d", len(params.ContentChanges))
		log.Error().Err(err).Send()
		return err
	}

	content := params.ContentChanges[0].Text
	fname := params.TextDocument.URI.Filename()

	err := checkFile(fname, content)
	return s.sendErrorDiagnostics(ctx, params.TextDocument.URI, err)
}

func (s *Server) handleDocumentSaved(
	ctx context.Context,
	reply jsonrpc2.Replier,
	r jsonrpc2.Request,
	log zerolog.Logger,
) error {
	var params lsp.DidSaveTextDocumentParams
	if err := json.Unmarshal(r.Params(), &params); err != nil {
		log.Error().Err(err).Msg("failed to unmarshal params")
		return jsonrpc2.ErrParse
	}

	fname := params.TextDocument.URI.Filename()
	data, err := os.ReadFile(fname)
	if err != nil {
		log.Error().Err(err).Msg("reading saved file.")
		return nil
	}

	err = checkFile(fname, string(data))
	return s.sendErrorDiagnostics(ctx, params.TextDocument.URI, err)
}

func (s *Server) sendErrorDiagnostics(ctx context.Context, currentFile lsp.URI, err error) error {
	if err == nil {
		// this is required to clear the `problems panel` for the active file
		// if it had errors before.
		s.sendDiagnostics(ctx, currentFile, []lsp.Diagnostic{})
		return nil
	}

	e, ok := err.(*errors.Error)
	if !ok {
		log.Debug().Err(err).Msg("unknown error ignored because it doesn't provide file range")
		return nil
	}

	log.Debug().Str("error", e.Detailed()).Msg("sending diagnostics")

	fileRange := lsp.Range{}
	fileRange.Start.Line = uint32(e.FileRange.Start.Line) - 1
	fileRange.Start.Character = uint32(e.FileRange.Start.Column) - 1
	fileRange.End.Line = uint32(e.FileRange.End.Line) - 1
	fileRange.End.Character = uint32(e.FileRange.End.Column) - 1

	// TODO(i4k): improve terramate to support multiple errors.
	diags := []lsp.Diagnostic{
		{
			Message:  e.Error(),
			Range:    fileRange,
			Severity: lsp.DiagnosticSeverityError,
			Source:   "terramate",
		},
	}

	filePath := lsp.URI(uri.File(filepath.ToSlash(e.FileRange.Filename)))
	s.sendDiagnostics(ctx, filePath, diags)

	if filePath != currentFile {
		s.sendDiagnostics(ctx, currentFile, []lsp.Diagnostic{})
	}
	return nil
}

func (s *Server) sendDiagnostics(ctx context.Context, uri lsp.URI, diags []lsp.Diagnostic) {
	err := s.conn.Notify(ctx, lsp.MethodTextDocumentPublishDiagnostics, lsp.PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: diags,
	})

	if err != nil {
		log.Fatal().Err(err).Msg("failed to send diagnostics to the client.")
	}
}

// checkFile checks if the given changed file has any errors.
// It parses all files in the directory but the provided one is added manually
// because it can be unsaved.
func checkFile(fname string, content string) error {
	dir := filepath.Dir(fname)
	parser := hcl.NewTerramateParser(dir)
	err := parser.AddFile(fname, []byte(content))
	if err != nil {
		log.Error().Err(err).Send()
		return err
	}

	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		log.Error().Msgf("adding directory to terramate parser: %s", err)
		return err
	}

	log.Trace().Msg("looking for Terramate files")

	for _, dirEntry := range dirEntries {
		logger := log.With().
			Str("entryName", dirEntry.Name()).
			Logger()

		if dirEntry.IsDir() {
			logger.Trace().Msg("ignoring dir")
			continue
		}

		filename := dirEntry.Name()
		if strings.HasSuffix(filename, ".tm") || strings.HasSuffix(filename, ".tm.hcl") {
			path := filepath.Join(dir, filename)

			if path == fname {
				// file already added
				continue
			}

			contents, err := os.ReadFile(path)
			if err != nil {
				log.Error().Err(err).Send()
				return err
			}

			err = parser.AddFile(path, contents)
			if err != nil {
				log.Error().Err(err).Send()
				return err
			}
		}
	}
	_, err = parser.Parse()
	return err
}
