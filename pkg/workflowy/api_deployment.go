package workflowy

import (
	"fmt"

	"github.com/mholzen/workflowy/pkg/cache"
)

type APIDeployment string

const (
	ProductionAPI APIDeployment = "production"
	BetaAPI       APIDeployment = "beta"
)

func ParseAPIDeployment(raw string) (APIDeployment, error) {
	if raw == "" {
		return ProductionAPI, nil
	}

	deployment := APIDeployment(raw)
	if _, err := deployment.BaseURL(); err != nil {
		return "", err
	}
	return deployment, nil
}

func (deployment APIDeployment) BaseURL() (string, error) {
	switch deployment {
	case ProductionAPI:
		return "https://workflowy.com/api/v1", nil
	case BetaAPI:
		return "https://beta.workflowy.com/api/v1", nil
	default:
		return "", invalidAPIDeploymentError(deployment)
	}
}

func (deployment APIDeployment) exportCacheFile() (string, error) {
	switch deployment {
	case ProductionAPI:
		return cache.DefaultCacheFile, nil
	case BetaAPI:
		return ".workflowy/export-cache-beta.json", nil
	default:
		return "", invalidAPIDeploymentError(deployment)
	}
}

func invalidAPIDeploymentError(deployment APIDeployment) error {
	return fmt.Errorf("Cannot select Workflowy API %q: expected %q or %q", deployment, ProductionAPI, BetaAPI)
}
