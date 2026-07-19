// Package github discovers organization repositories using the GitHub REST API.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
)

const (
	defaultBaseURL  = "https://api.github.com"
	maxResponseSize = 4 << 20
	maxErrorSize    = 8 << 10
)

// Repository is the subset of GitHub repository metadata used by asc.
type Repository struct {
	Name          string `json:"name"`
	Archived      bool   `json:"archived"`
	Fork          bool   `json:"fork"`
	CloneURL      string `json:"clone_url"`
	SSHURL        string `json:"ssh_url"`
	DefaultBranch string `json:"default_branch"`
	Private       bool   `json:"private"`
}

// Client calls the GitHub REST API.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	Token      string
	Version    string
}

// NewClient returns a GitHub client with bounded request timeouts.
func NewClient(token, version string) *Client {
	return &Client{
		BaseURL:    defaultBaseURL,
		HTTPClient: &http.Client{Timeout: 15 * time.Second},
		Token:      token,
		Version:    version,
	}
}

// ListOrganizationRepositories returns all repositories visible to the caller.
func (c *Client) ListOrganizationRepositories(ctx context.Context, organization string) ([]Repository, error) {
	baseURL := strings.TrimRight(c.BaseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	var repositories []Repository
	for page := 1; ; page++ {
		endpoint := fmt.Sprintf("%s/orgs/%s/repos?type=all&per_page=100&page=%d", baseURL, url.PathEscape(organization), page)
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return nil, fmt.Errorf("create GitHub request: %w", err)
		}
		request.Header.Set("Accept", "application/vnd.github+json")
		request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
		version := c.Version
		if version == "" {
			version = "dev"
		}
		request.Header.Set("User-Agent", "asc-devtools/"+version)
		if c.Token != "" {
			request.Header.Set("Authorization", "Bearer "+c.Token)
		}
		response, err := httpClient.Do(request)
		if err != nil {
			return nil, fmt.Errorf("GitHub API request failed: %w", err)
		}
		pageRepositories, hasNext, err := decodeResponse(response)
		if err != nil {
			return nil, err
		}
		repositories = append(repositories, pageRepositories...)
		if !hasNext && len(pageRepositories) < 100 {
			break
		}
		if len(pageRepositories) == 0 {
			break
		}
	}
	return repositories, nil
}

func decodeResponse(response *http.Response) ([]Repository, bool, error) {
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, maxErrorSize))
		return nil, false, apiError(response)
	}
	limited := io.LimitReader(response.Body, maxResponseSize+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, false, fmt.Errorf("read GitHub API response: %w", err)
	}
	if len(body) > maxResponseSize {
		return nil, false, errors.New("GitHub API response exceeded size limit")
	}
	var repositories []Repository
	if err := json.Unmarshal(body, &repositories); err != nil {
		return nil, false, fmt.Errorf("decode GitHub API response: %w", err)
	}
	return repositories, strings.Contains(response.Header.Get("Link"), `rel="next"`), nil
}

func apiError(response *http.Response) error {
	switch response.StatusCode {
	case http.StatusUnauthorized:
		return fmt.Errorf("GitHub API authentication failed (401): configure ASC_GITHUB_TOKEN")
	case http.StatusForbidden:
		if response.Header.Get("X-RateLimit-Remaining") == "0" {
			return fmt.Errorf("GitHub API rate limit exceeded (403): authenticate or wait for reset")
		}
		return fmt.Errorf("GitHub API access forbidden (403): verify organization permissions")
	case http.StatusNotFound:
		return fmt.Errorf("GitHub organization or endpoint not found (404): verify organization and token access")
	default:
		return fmt.Errorf("GitHub API returned %d: %s", response.StatusCode, http.StatusText(response.StatusCode))
	}
}

// FilterManaged filters active repositories according to asc configuration.
func FilterManaged(repositories []Repository, prefix string, includeDotGitHub bool) []Repository {
	filtered := make([]Repository, 0, len(repositories))
	for _, repository := range repositories {
		if repository.Archived {
			continue
		}
		if strings.HasPrefix(repository.Name, prefix) || (includeDotGitHub && repository.Name == ".github") {
			filtered = append(filtered, repository)
		}
	}
	sort.Slice(filtered, func(i, j int) bool { return filtered[i].Name < filtered[j].Name })
	return filtered
}
