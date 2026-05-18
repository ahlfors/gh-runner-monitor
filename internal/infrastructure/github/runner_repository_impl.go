package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/VeyronSakai/gh-runner-monitor/internal/domain/entity"
	domainrepo "github.com/VeyronSakai/gh-runner-monitor/internal/domain/repository"
	"github.com/cli/go-gh/v2/pkg/api"
)

// RunnerRepositoryImpl implements the RunnerRepository interface using GitHub API
type RunnerRepositoryImpl struct {
	restClient *api.RESTClient
}

// NewRunnerRepository creates a new instance of RunnerRepositoryImpl
func NewRunnerRepository() (domainrepo.RunnerRepository, error) {
	restClient, err := api.DefaultRESTClient()
	if err != nil {
		return nil, fmt.Errorf("failed to create REST client: %w\nPlease run 'gh auth login' to authenticate with GitHub", err)
	}

	return &RunnerRepositoryImpl{
		restClient: restClient,
	}, nil
}

// FetchRunners retrieves all runners for a repository or organization
func (r *RunnerRepositoryImpl) FetchRunners(ctx context.Context, owner, repo, org string) ([]*entity.Runner, error) {
	path := r.getRunnersPath(owner, repo, org)
	runners, err := r.requestGetAllRunners(path)
	if err != nil {
		return nil, err
	}

	result := make([]*entity.Runner, 0, len(runners))
	for _, runner := range runners {
		status := entity.StatusOffline
		if runner.Status == "online" {
			if runner.Busy {
				status = entity.StatusActive
			} else {
				status = entity.StatusIdle
			}
		}

		labels := make([]string, 0, len(runner.Labels))
		for _, l := range runner.Labels {
			labels = append(labels, l.Name)
		}

		result = append(result, &entity.Runner{
			ID:        runner.ID,
			Name:      runner.Name,
			Status:    status,
			Labels:    labels,
			OS:        runner.OS,
			UpdatedAt: time.Now(),
		})
	}
	return result, nil
}

// getRunnersPath constructs the API path for fetching runners
func (r *RunnerRepositoryImpl) getRunnersPath(owner, repo, org string) string {
	if org != "" {
		return fmt.Sprintf("orgs/%s/actions/runners", org)
	}
	return fmt.Sprintf("repos/%s/%s/actions/runners", owner, repo)
}

// requestGetAllRunners fetches ALL runners from GitHub API with pagination
func (r *RunnerRepositoryImpl) requestGetAllRunners(path string) ([]runnerResponse, error) {
	const perPage = 100
	var allRunners []runnerResponse
	page := 1

	for {
		pagedPath := fmt.Sprintf("%s?per_page=%d&page=%d", path, perPage, page)

		resp, err := r.restClient.Request(http.MethodGet, pagedPath, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to request runners (page %d): %w", page, err)
		}

		var runnersResp runnersResponse
		if err := json.NewDecoder(resp.Body).Decode(&runnersResp); err != nil {
			_ = resp.Body.Close()
			return nil, fmt.Errorf("failed to decode runners response (page %d): %w", page, err)
		}
		_ = resp.Body.Close()

		allRunners = append(allRunners, runnersResp.Runners...)

		if len(allRunners) >= runnersResp.TotalCount {
			break
		}

		if len(runnersResp.Runners) < perPage {
			break
		}

		page++
	}

	return allRunners, nil
}