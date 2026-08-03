package mcp

import (
	"testing"

	genericclient "github.com/mholzen/workflowy/pkg/client"
	"github.com/mholzen/workflowy/pkg/workflowy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildToolBuilderValidatesAPIBeforeCredentials(t *testing.T) {
	credentialCalls := 0
	factoryCalls := 0

	_, err := buildToolBuilder(
		Config{API: "staging"},
		func(string, string) (genericclient.Option, error) {
			credentialCalls++
			return nil, nil
		},
		func(workflowy.APIDeployment, ...genericclient.Option) (workflowy.Client, error) {
			factoryCalls++
			return &recordingClient{}, nil
		},
	)

	require.EqualError(t, err, `Cannot select Workflowy API "staging": expected "production" or "beta"`)
	assert.Zero(t, credentialCalls)
	assert.Zero(t, factoryCalls)
}

func TestBuildToolBuilderConstructsBothAPIsFromOneCredential(t *testing.T) {
	credentialCalls := 0
	optionApplications := 0
	constructed := make([]workflowy.APIDeployment, 0, 2)

	builder, err := buildToolBuilder(
		Config{API: "beta"},
		func(string, string) (genericclient.Option, error) {
			credentialCalls++
			return func(*genericclient.Client) {
				optionApplications++
			}, nil
		},
		func(deployment workflowy.APIDeployment, options ...genericclient.Option) (workflowy.Client, error) {
			constructed = append(constructed, deployment)
			genericclient.New("https://example.invalid", options...)
			return &recordingClient{}, nil
		},
	)

	require.NoError(t, err)
	assert.Equal(t, workflowy.BetaAPI, builder.defaultDeployment)
	assert.Equal(t, 1, credentialCalls)
	assert.Equal(t, 2, optionApplications)
	assert.ElementsMatch(t, []workflowy.APIDeployment{workflowy.ProductionAPI, workflowy.BetaAPI}, constructed)
}
