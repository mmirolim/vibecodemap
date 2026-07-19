package projectdsl

import _ "embed"

// The CLI embeds the complete machine schema and human grammar so an AI agent
// or reviewer can inspect the accepted language without locating a checkout.

//go:embed assets/project.schema.json
var schemaDocument []byte

//go:embed assets/project.grammar.md
var grammarDocument []byte

func Schema() []byte {
	return append([]byte(nil), schemaDocument...)
}

func Grammar() []byte {
	return append([]byte(nil), grammarDocument...)
}
