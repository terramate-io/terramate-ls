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
	"log"

	"go.lsp.dev/jsonrpc2"
	lsp "go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

// InitializeParams
type InitializeParams struct {
	ProcessID int    `json:"processId,omitempty"`
	RootURI   string `json:"rootUri,omitempty"`
}

// Deliver ...
func Handler(conn jsonrpc2.Conn) jsonrpc2.Handler {
	var workspace string

	return func(ctx context.Context, reply jsonrpc2.Replier, r jsonrpc2.Request) error {
		log.Printf("got request %s with params: %s", r.Method(), r.Params())
		switch r.Method() {
		default:
			log.Printf("%s is not implemented", r.Method())
		case lsp.MethodInitialize:
			// Get params.
			var params InitializeParams
			if err := json.Unmarshal(r.Params(), &params); err != nil {
				log.Fatal(err)
			}

			workspace = string(uri.New(params.RootURI).Filename())

			// Send back the response.
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
				log.Fatal(err)
				panic(err)
			}

			log.Printf("client connected using workspace %q", workspace)

			conn.Notify(ctx, lsp.MethodWindowShowMessage, lsp.ShowMessageParams{
				Message: "connected to terramate-lsp",
				Type:    lsp.MessageTypeInfo,
			})

			return nil

		}

		return nil
	}
}