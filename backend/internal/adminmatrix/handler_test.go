//go:build unit

package adminmatrix

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEmbeddedMatrixContainsCanonicalCartesianDimensions(t *testing.T) {
	var payload struct {
		Counts struct {
			Models        int `json:"models"`
			Clients       int `json:"clients"`
			Intersections int `json:"intersections"`
		} `json:"counts"`
		Contracts    []json.RawMessage `json:"contracts"`
		ClientMatrix struct {
			Clients []json.RawMessage `json:"clients"`
		} `json:"client_matrix"`
	}
	require.NoError(t, json.Unmarshal(matrixJSON, &payload))
	require.Equal(t, 36, payload.Counts.Models)
	require.Equal(t, 14, payload.Counts.Clients)
	require.Equal(t, 36*14, payload.Counts.Intersections)
	require.Len(t, payload.Contracts, 36)
	require.Len(t, payload.ClientMatrix.Clients, 14)
}
