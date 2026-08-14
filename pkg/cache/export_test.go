package cache

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExportCacheIsolation(t *testing.T) {
	temporaryDirectory := t.TempDir()
	productionPath := filepath.Join(temporaryDirectory, "production", "export.json")
	betaPath := filepath.Join(temporaryDirectory, "beta", "export.json")

	require.NoError(t, WriteExportCache(productionPath, map[string]string{"deployment": "production"}))
	require.NoError(t, WriteExportCache(betaPath, map[string]string{"deployment": "beta"}))

	productionCache, err := ReadExportCache(productionPath)
	require.NoError(t, err)
	require.NotNil(t, productionCache)

	betaCache, err := ReadExportCache(betaPath)
	require.NoError(t, err)
	require.NotNil(t, betaCache)

	var productionPayload map[string]string
	require.NoError(t, json.Unmarshal(productionCache.Data, &productionPayload))
	assert.Equal(t, map[string]string{"deployment": "production"}, productionPayload)

	var betaPayload map[string]string
	require.NoError(t, json.Unmarshal(betaCache.Data, &betaPayload))
	assert.Equal(t, map[string]string{"deployment": "beta"}, betaPayload)
}
