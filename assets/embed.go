// Copyright (c) 2026 Arnaud Guiovanna <https://www.aguiovanna.fr>
// SPDX-License-Identifier: MIT

// Package assets embeds static files served by the MCP server, currently
// the cockpit HTML resource (ui://cockpit) consumed by Claude Desktop's
// MCP App iframe.
package assets

import "embed"

//go:embed app.html
var FS embed.FS
