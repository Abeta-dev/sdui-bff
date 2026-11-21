// Package sduibff embeds the repository-root llms.txt and llms-full.txt
// AI-agent discovery documents as a single source of truth shared between
// GitHub raw links and live HTTP endpoints (/llms.txt and /llms-full.txt).
package sduibff

import _ "embed"

// LLMsTxt contains the embedded llmstxt.org index from repository root llms.txt.
//
//go:embed llms.txt
var LLMsTxt []byte

// LLMsFullTxt contains the embedded comprehensive LLM reference from repository root llms-full.txt.
//
//go:embed llms-full.txt
var LLMsFullTxt []byte
