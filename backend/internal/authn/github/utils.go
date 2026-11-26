// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"

	sysconst "github.com/thunder-id/thunderid/internal/system/constants"
	syshttp "github.com/thunder-id/thunderid/internal/system/http"
	"github.com/thunder-id/thunderid/internal/system/log"
)

const (
	teamsPageSize       = "100"
	teamsPageSizeParam  = "per_page"
	githubResourceEmail = "email"
	githubResourceTeams = "teams"
	linkHeaderName      = "Link"
	linkRelNext         = "next"
)

// githubTeam represents a single team entry returned by the GitHub user teams endpoint.
type githubTeam struct {
	Slug         string `json:"slug"`
	Organization struct {
		Login string `json:"login"`
	} `json:"organization"`
}

// buildUserEmailRequest constructs the HTTP request to fetch user emails from GitHub.
func buildUserEmailRequest(ctx context.Context, userEmailEndpoint string, accessToken string, logger *log.Logger) (
	*http.Request, *tidcommon.ServiceError) {
	return buildGitHubAPIRequest(ctx, userEmailEndpoint, accessToken, githubResourceEmail, logger)
}

// buildUserTeamsRequest constructs the HTTP request to fetch user teams from GitHub.
func buildUserTeamsRequest(ctx context.Context, userTeamsEndpoint string, accessToken string, logger *log.Logger) (
	*http.Request, *tidcommon.ServiceError) {
	parsedURL, err := url.Parse(userTeamsEndpoint)
	if err != nil {
		logger.Error(ctx, "Failed to create user teams request", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	query := parsedURL.Query()
	if !query.Has(teamsPageSizeParam) {
		query.Set(teamsPageSizeParam, teamsPageSize)
		parsedURL.RawQuery = query.Encode()
	}
	return buildGitHubAPIRequest(ctx, parsedURL.String(), accessToken, githubResourceTeams, logger)
}

// buildGitHubAPIRequest constructs an authenticated GET request to a GitHub API endpoint.
func buildGitHubAPIRequest(ctx context.Context, endpoint, accessToken, resourceName string, logger *log.Logger) (
	*http.Request, *tidcommon.ServiceError) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		logger.Error(ctx, "Failed to create user "+resourceName+" request", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	req.Header.Set(sysconst.AuthorizationHeaderName, sysconst.TokenTypeBearer+" "+accessToken)
	req.Header.Set(sysconst.AcceptHeaderName, sysconst.ContentTypeJSON)

	return req, nil
}

// sendUserEmailRequest sends the user email request to GitHub and processes the response.
func sendUserEmailRequest(httpReq *http.Request, httpClient syshttp.HTTPClientInterface, logger *log.Logger) (
	[]map[string]interface{}, *tidcommon.ServiceError) {
	var emails []map[string]interface{}
	if _, svcErr := sendGitHubAPIRequest(
		httpReq, httpClient, githubResourceEmail, &emails, logger); svcErr != nil {
		return nil, svcErr
	}
	return emails, nil
}

// sendUserTeamsRequest sends the user teams request to GitHub and returns the decoded teams and next page URL.
func sendUserTeamsRequest(httpReq *http.Request, httpClient syshttp.HTTPClientInterface, logger *log.Logger) (
	[]githubTeam, string, *tidcommon.ServiceError) {
	var teams []githubTeam
	headers, svcErr := sendGitHubAPIRequest(httpReq, httpClient, githubResourceTeams, &teams, logger)
	if svcErr != nil {
		return nil, "", svcErr
	}
	return teams, parseNextLinkHeader(headers, httpReq.URL), nil
}

// sendGitHubAPIRequest executes a GitHub API request and decodes its JSON response body into out.
func sendGitHubAPIRequest(httpReq *http.Request, httpClient syshttp.HTTPClientInterface,
	resourceName string, out interface{}, logger *log.Logger) (http.Header, *tidcommon.ServiceError) {
	ctx := httpReq.Context()
	resp, err := httpClient.Do(httpReq)
	if err != nil {
		logger.Error(ctx, "User "+resourceName+" request to GitHub failed", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Error(ctx, "Failed to close user "+resourceName+" response body", log.Error(closeErr))
		}
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		logger.Error(ctx, "User "+resourceName+" endpoint returned an error response",
			log.Int("statusCode", resp.StatusCode), log.String("response", string(body)))
		return nil, &tidcommon.InternalServerError
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		logger.Error(ctx, "Failed to decode user "+resourceName+" response", log.Error(err))
		return nil, &tidcommon.InternalServerError
	}

	return resp.Header, nil
}

// parseNextLinkHeader extracts the URL for rel="next" from RFC 8288 Link headers.
func parseNextLinkHeader(headers http.Header, baseURL *url.URL) string {
	for _, headerVal := range headers.Values(linkHeaderName) {
		for _, entry := range strings.Split(headerVal, ",") {
			parts := strings.Split(strings.TrimSpace(entry), ";")
			if len(parts) < 2 {
				continue
			}
			target := strings.TrimSpace(parts[0])
			if !strings.HasPrefix(target, "<") || !strings.HasSuffix(target, ">") {
				continue
			}
			if !hasNextRelParam(parts[1:]) {
				continue
			}
			rawURL := target[1 : len(target)-1]
			parsedNext, err := url.Parse(rawURL)
			if err != nil {
				continue
			}
			if baseURL != nil {
				parsedNext = baseURL.ResolveReference(parsedNext)
			}
			return parsedNext.String()
		}
	}
	return ""
}

// hasNextRelParam reports whether any Link header parameter declares rel="next".
func hasNextRelParam(params []string) bool {
	for _, param := range params {
		k, v, ok := strings.Cut(strings.TrimSpace(param), "=")
		if !ok || !strings.EqualFold(strings.TrimSpace(k), "rel") {
			continue
		}
		relVal := strings.Trim(strings.TrimSpace(v), "\"")
		if slices.Contains(strings.Fields(relVal), linkRelNext) {
			return true
		}
	}
	return false
}
