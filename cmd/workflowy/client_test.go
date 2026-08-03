package main

import (
	"context"
	"testing"

	genericclient "github.com/mholzen/workflowy/pkg/client"
	"github.com/mholzen/workflowy/pkg/workflowy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v3"
)

type getCommandClient struct {
	MockClient
}

func (client *getCommandClient) ListChildrenRecursiveWithDepth(context.Context, string, int) (*workflowy.ListChildrenResponse, error) {
	return &workflowy.ListChildrenResponse{Items: []*workflowy.Item{}}, nil
}

func TestClientProviderSelectsAPI(t *testing.T) {
	t.Setenv("WORKFLOWY_API_KEY", "test-key")

	tests := []struct {
		name string
		args []string
		want workflowy.APIDeployment
	}{
		{name: "production default", args: []string{"test"}, want: workflowy.ProductionAPI},
		{name: "beta override", args: []string{"test", "--api=beta"}, want: workflowy.BetaAPI},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var selected workflowy.APIDeployment
			factory := func(deployment workflowy.APIDeployment, _ ...genericclient.Option) (workflowy.Client, error) {
				selected = deployment
				return &MockClient{}, nil
			}

			command := &cli.Command{
				Flags: []cli.Flag{getAPIFlag(), getAPIKeyFlag()},
				Action: newClientProvider(factory, false)(func(context.Context, *cli.Command, workflowy.Client) error {
					return nil
				}),
			}

			require.NoError(t, command.Run(context.Background(), test.args))
			assert.Equal(t, test.want, selected)
		})
	}
}

func TestClientProviderRejectsInvalidAPIBeforeCredentials(t *testing.T) {
	t.Setenv("WORKFLOWY_API_KEY", "")

	factoryCalls := 0
	factory := func(workflowy.APIDeployment, ...genericclient.Option) (workflowy.Client, error) {
		factoryCalls++
		return &MockClient{}, nil
	}
	command := &cli.Command{
		Flags: []cli.Flag{getAPIFlag(), getAPIKeyFlag()},
		Action: newClientProvider(factory, true)(func(context.Context, *cli.Command, workflowy.Client) error {
			return nil
		}),
	}

	err := command.Run(context.Background(), []string{
		"test",
		"--api=staging",
		"--api-key-file=" + t.TempDir() + "/missing-api-key",
	})
	require.EqualError(t, err, `Cannot select Workflowy API "staging": expected "production" or "beta"`)
	assert.Zero(t, factoryCalls)
}

func TestGetCommandUsesSelectedAPI(t *testing.T) {
	t.Setenv("WORKFLOWY_API_KEY", "test-key")

	var selected workflowy.APIDeployment
	fakeClient := &getCommandClient{}
	factory := func(deployment workflowy.APIDeployment, _ ...genericclient.Option) (workflowy.Client, error) {
		selected = deployment
		return fakeClient, nil
	}

	command := getGetCommandWithClientProvider(newClientProvider(factory, false))
	root := &cli.Command{
		Name: "workflowy",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "format", Value: "json"},
			getReadRootIdFlag(),
			getWriteRootIdFlag(),
		},
		Commands: []*cli.Command{command},
	}
	err := root.Run(context.Background(), []string{"workflowy", "get", "--api=beta", "--method=get", "--depth=0"})

	require.NoError(t, err)
	assert.Equal(t, workflowy.BetaAPI, selected)
}
