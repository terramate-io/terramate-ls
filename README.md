# Terramate Language Server

![CI Status](https://github.com/mineiros-io/terramate-ls/actions/workflows/ci.yml/badge.svg)
[![Go Report Card](https://goreportcard.com/badge/github.com/mineiros-io/terramate-ls)](https://goreportcard.com/report/github.com/mineiros-io/terramate-ls)
[![Join Slack](https://img.shields.io/badge/slack-@mineiros--community-f32752.svg?logo=slack)](https://mineiros.io/slack)

The Terramate Language Server provides features to any [LSP](https://microsoft.github.io/language-server-protocol/)-compatible code editor.

## Getting Started

### Installing

#### Using Go

To install using Go just run:

```sh
go install github.com/mineiros-io/terramate-ls/cmd/terramate-ls@<version>
```

Where `<version>` is any terramate-ls [version tag](https://github.com/mineiros-io/terramate-ls/tags),
or you can just install the **latest** release:

```sh
go install github.com/mineiros-io/terramate-ls/cmd/terramate-ls@latest
```

#### Using Release Binaries

To install `terramate-ls` using a released binary, find the
[appropriate package](https://github.com/mineiros-io/terramate-ls/releases) for
your system and download it.

After downloading the language server, unzip the package. The 
language server runs as a single binary named `terramate-ls`. 
Any other files in the package can be safely removed and the language server
will still work.

Finally, make sure that the `terramate-ls` binary is available on your PATH.
This process will differ depending on your operating system.

### Setup in the code editor

At the moment only [vscode](https://code.visualstudio.com/) is officially
supported by the [vscode-terramate](https://github.com/mineiros-io/vscode-terramate)
extension.

The only `terramate-ls` specific setup required is making sure it is installed in a
directory in the editor's `PATH` environment variable.
