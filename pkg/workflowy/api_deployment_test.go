package workflowy

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseAPIDeployment(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    APIDeployment
		wantErr string
	}{
		{name: "empty defaults to production", raw: "", want: ProductionAPI},
		{name: "production", raw: "production", want: ProductionAPI},
		{name: "beta", raw: "beta", want: BetaAPI},
		{name: "invalid", raw: "prod", wantErr: `Cannot select Workflowy API "prod": expected "production" or "beta"`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := ParseAPIDeployment(test.raw)
			if test.wantErr != "" {
				require.EqualError(t, err, test.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.want, got)
		})
	}
}

func TestAPIDeploymentMappings(t *testing.T) {
	productionURL, err := ProductionAPI.BaseURL()
	require.NoError(t, err)
	assert.Equal(t, "https://workflowy.com/api/v1", productionURL)

	betaURL, err := BetaAPI.BaseURL()
	require.NoError(t, err)
	assert.Equal(t, "https://beta.workflowy.com/api/v1", betaURL)

	productionCacheFile, err := ProductionAPI.exportCacheFile()
	require.NoError(t, err)
	assert.Equal(t, ".workflowy/export-cache.json", productionCacheFile)

	betaCacheFile, err := BetaAPI.exportCacheFile()
	require.NoError(t, err)
	assert.Equal(t, ".workflowy/export-cache-beta.json", betaCacheFile)
	assert.NotEqual(t, productionCacheFile, betaCacheFile)
}

func TestAPIDeploymentMappingsRejectInvalidValue(t *testing.T) {
	invalid := APIDeployment("staging")

	_, err := invalid.BaseURL()
	assert.EqualError(t, err, `Cannot select Workflowy API "staging": expected "production" or "beta"`)

	_, err = invalid.exportCacheFile()
	assert.EqualError(t, err, `Cannot select Workflowy API "staging": expected "production" or "beta"`)
}
