//go:build unit

package handler

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/stretchr/testify/require"
)

// This guard keeps the embed exchange response from regressing into the
// refresh-token-bearing chatbot SSO response.
func TestAuthHandler_EmbedExchangeDoesNotReturnRefreshToken(t *testing.T) {
	file, err := parser.ParseFile(token.NewFileSet(), "auth_handler.go", nil, parser.ParseComments)
	require.NoError(t, err)
	var source string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "ExchangeEmbedTicket" {
			continue
		}
		ast.Inspect(fn, func(node ast.Node) bool {
			if lit, ok := node.(*ast.BasicLit); ok {
				source += lit.Value
			}
			return true
		})
	}
	require.NotEmpty(t, source)
	require.NotContains(t, source, "refresh_token")
	require.Contains(t, source, "session_token")
}
