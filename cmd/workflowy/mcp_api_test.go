package main

import (
	"context"
	"testing"

	"github.com/mholzen/workflowy/pkg/mcp"
	"github.com/mholzen/workflowy/pkg/workflowy"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMCPCommandPassesAPISelection(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "production default", args: []string{"mcp"}, want: string(workflowy.ProductionAPI)},
		{name: "beta override", args: []string{"mcp", "--api=beta"}, want: string(workflowy.BetaAPI)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var config mcp.Config
			command := getMcpCommandWithRunner(func(_ context.Context, received mcp.Config) error {
				config = received
				return nil
			})

			require.NoError(t, command.Run(context.Background(), test.args))
			assert.Equal(t, test.want, config.API)
		})
	}
}
