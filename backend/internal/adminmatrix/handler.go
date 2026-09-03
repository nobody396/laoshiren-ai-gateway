// Package adminmatrix serves the generated model/client evidence matrix.
package adminmatrix

import (
	_ "embed"
	"net/http"

	"github.com/gin-gonic/gin"
)

// matrixJSON is generated from the canonical model and client contracts.
//
//go:embed model_client_matrix.json
var matrixJSON []byte

// Get returns the immutable administrator-only matrix payload.
func Get(c *gin.Context) {
	c.Header("Cache-Control", "private, no-store")
	c.Data(http.StatusOK, "application/json; charset=utf-8", matrixJSON)
}
