/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

// Package thunder is an HTTP client for the ThunderID management APIs used by the environment
// service: export (read config from a control plane), reveal (read secret values), and import
// (apply config, including deletions, to a data plane).
package thunder

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Credentials is the bearer token the client presents.
//
// There is no client_credentials pair. The only server this client talks to is the control plane it
// runs inside, always while serving a request, so the caller's own token is what it forwards. Data
// planes are reached over the channel they dial out on and need no credential at all.
type Credentials struct {
	Token string
}

// Client talks to one ThunderID control plane.
type Client struct {
	baseURL string
	creds   Credentials
	http    *http.Client
}

// New builds a client for baseURL. When insecure is set, TLS certificate verification is skipped
// (useful for self-signed local development servers).
func New(baseURL string, creds Credentials, insecure bool) *Client {
	transport := &http.Transport{}
	if insecure {
		transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} //nolint:gosec // opt-in for local dev
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		creds:   creds,
		http:    &http.Client{Timeout: 60 * time.Second, Transport: transport},
	}
}

// ExportResult is the parsed response of an export call.
type ExportResult struct {
	Resources string
	EnvFile   string
}

// exportRequest mirrors the fields of the ThunderID export API. Each "*" requests all resources of a
// type; includeDependencies pulls related resources so a bundle is self-consistent.
type exportRequest struct {
	Agents            []string `json:"agents"`
	Applications      []string `json:"applications"`
	Connections       []string `json:"connections"`
	UserTypes         []string `json:"userTypes"`
	OrganizationUnits []string `json:"organizationUnits"`
	Users             []string `json:"users"`
	Groups            []string `json:"groups"`
	ResourceServers   []string `json:"resourceServers"`
	Roles             []string `json:"roles"`
	Flows             []string `json:"flows"`
	Translations      []string `json:"translations"`
	Layouts           []string `json:"layouts"`
	Themes            []string `json:"themes"`
	ServerConfigs     []string `json:"serverConfigs"`

	PresentationDefinitions  []string       `json:"presentationDefinitions"`
	CredentialConfigurations []string       `json:"credentialConfigurations"`
	Options                  *exportOptions `json:"options"`
}

type exportOptions struct {
	IncludeDependencies bool   `json:"includeDependencies"`
	Format              string `json:"format"`
}

type jsonExportResponse struct {
	Resources            string `json:"resources"`
	EnvironmentVariables string `json:"environment_variables"`
}

// Export requests every resource type and returns the combined parameterized YAML and the .env body.
func (c *Client) Export(ctx context.Context) (ExportResult, error) {
	all := []string{"*"}
	req := exportRequest{
		Agents: all, Applications: all, Connections: all, UserTypes: all,
		OrganizationUnits: all, Users: all, Groups: all, ResourceServers: all,
		Roles: all, Flows: all, Translations: all, Layouts: all, Themes: all,
		ServerConfigs: all,

		PresentationDefinitions: all, CredentialConfigurations: all,

		Options: &exportOptions{IncludeDependencies: true, Format: "yaml"},
	}
	var resp jsonExportResponse
	if err := c.do(ctx, http.MethodPost, "/export", req, &resp); err != nil {
		return ExportResult{}, err
	}
	return ExportResult{Resources: resp.Resources, EnvFile: resp.EnvironmentVariables}, nil
}

type secretListEntry struct {
	Key string `json:"key"`
}

type secretListResponse struct {
	Secrets []secretListEntry `json:"secrets"`
}

// SecretKeys lists the placeholder names the control plane holds secrets for, without their values.
// Only the names are needed: an apply sends a ${KEY} placeholder for a secret rather than its value,
// so no secret has to leave the control plane at all. A 403 or 404 (no secret module, or no access)
// is treated as an empty list so capture still works against servers without it.
func (c *Client) SecretKeys(ctx context.Context) ([]string, error) {
	var resp secretListResponse
	err := c.do(ctx, http.MethodGet, "/secrets?limit=1000", nil, &resp)
	if err != nil {
		if isStatus(err, http.StatusForbidden) || isStatus(err, http.StatusNotFound) {
			return nil, nil
		}
		return nil, err
	}
	keys := make([]string, 0, len(resp.Secrets))
	for _, s := range resp.Secrets {
		if s.Key != "" {
			keys = append(keys, s.Key)
		}
	}
	return keys, nil
}

type secretNamesResponse struct {
	Names []string `json:"names"`
}

// SecretNames lists the names a secret service holds, without any value. It answers whether a
// credential is present, which is all an apply readiness check needs to know.
func (c *Client) SecretNames(ctx context.Context) ([]string, error) {
	var resp secretNamesResponse
	if err := c.do(ctx, http.MethodGet, "/secrets/names", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Names, nil
}

// PutSecret writes one secret to a secret provider. The body carries the provider's own write shape
// (kind, value, algorithm, parameters), which is passed through untouched so this client does not
// constrain what a provider can store.
func (c *Client) PutSecret(ctx context.Context, name string, body map[string]interface{}) error {
	return c.do(ctx, http.MethodPut, "/secrets/"+name, body, nil)
}

// GetSecret reads one secret from a secret provider. found is false when the provider does not hold
// the name, which is not an error: it is the answer the caller asked for.
func (c *Client) GetSecret(ctx context.Context, name string) (kind, value string, found bool, err error) {
	var resp struct {
		Kind  string `json:"kind"`
		Value string `json:"value"`
	}
	if err := c.do(ctx, http.MethodGet, "/secrets/"+name, nil, &resp); err != nil {
		if isStatus(err, http.StatusNotFound) {
			return "", "", false, nil
		}
		return "", "", false, err
	}
	return resp.Kind, resp.Value, true, nil
}

type environmentVariablesResponse struct {
	Variables map[string]string `json:"variables"`
}

// EnvironmentVariables returns the non-secret variable values configured for one environment, such as
// redirect URLs. These are authoritative for an apply: they are what the operator set for that
// environment, so they override whatever the export happened to emit. A 403 or 404 is treated as an
// empty map so capture still works against servers without the module.
func (c *Client) EnvironmentVariables(ctx context.Context, envID string) (map[string]string, error) {
	var resp environmentVariablesResponse
	err := c.do(ctx, http.MethodGet, "/environments/"+url.PathEscape(envID)+"/variables/resolve", nil, &resp)
	if err != nil {
		if isStatus(err, http.StatusForbidden) || isStatus(err, http.StatusNotFound) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	if resp.Variables == nil {
		return map[string]string{}, nil
	}
	return resp.Variables, nil
}

// ImportOptions controls import behavior. Upsert and ContinueOnError default to true server-side.
type ImportOptions struct {
	Upsert          *bool  `json:"upsert,omitempty"`
	ContinueOnError *bool  `json:"continueOnError,omitempty"`
	Target          string `json:"target,omitempty"`
	// MarkManaged declares that everything in this payload comes from the control plane, which makes
	// it read only on the deployment receiving it. An import without it writes resources that
	// deployment owns and can edit, which is how the same API stays usable for local work.
	MarkManaged bool `json:"markManaged,omitempty"`
}

// ResourceDeletion asks the import API to delete a resource by id. Category scopes a user_type
// deletion and is ignored for other resource types.
type ResourceDeletion struct {
	ResourceType string `json:"resourceType"`
	ID           string `json:"id"`
	Category     string `json:"category,omitempty"`
}

// ImportRequest is the payload for POST /import. Deletions is the environment service's extension for
// pruning resources that a diff shows were removed.
type ImportRequest struct {
	Content   string                 `json:"content,omitempty"`
	Variables map[string]interface{} `json:"variables,omitempty"`
	DryRun    bool                   `json:"dryRun,omitempty"`
	Options   *ImportOptions         `json:"options,omitempty"`
	Deletions []ResourceDeletion     `json:"deletions,omitempty"`
}

// ImportItemOutcome is one resource's result from an import.
type ImportItemOutcome struct {
	ResourceType string `json:"resourceType"`
	ResourceID   string `json:"resourceId"`
	ResourceName string `json:"resourceName"`
	Operation    string `json:"operation"`
	Status       string `json:"status"`
	Code         string `json:"code,omitempty"`
	Message      string `json:"message,omitempty"`
}

// ImportSummary is the aggregate result of an import.
type ImportSummary struct {
	TotalDocuments int    `json:"totalDocuments"`
	Imported       int    `json:"imported"`
	Failed         int    `json:"failed"`
	ImportedAt     string `json:"importedAt"`
}

// ImportResponse is the full response of POST /import.
type ImportResponse struct {
	Summary *ImportSummary      `json:"summary"`
	Results []ImportItemOutcome `json:"results"`
}

// Import applies a bundle (and any deletions) to the server.
func (c *Client) Import(ctx context.Context, req ImportRequest) (*ImportResponse, error) {
	var resp ImportResponse
	if err := c.do(ctx, http.MethodPost, "/import", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// HTTPError carries a non-2xx response.
type HTTPError struct {
	StatusCode int
	Body       string
	URL        string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("thunder request %s returned %d: %s", e.URL, e.StatusCode, e.Body)
}

func isStatus(err error, code int) bool {
	var he *HTTPError
	ok := errors.As(err, &he)
	return ok && he.StatusCode == code
}

func (c *Client) do(ctx context.Context, method, path string, body, out interface{}) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to encode request body: %w", err)
		}
		reader = bytes.NewReader(raw)
	}
	url := c.baseURL + path
	httpReq, err := http.NewRequestWithContext(ctx, method, url, reader)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	if body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	httpReq.Header.Set("Accept", "application/json")
	if c.creds.Token != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.creds.Token)
	}

	resp, err := c.http.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request to %s failed: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response from %s: %w", url, err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &HTTPError{StatusCode: resp.StatusCode, Body: string(raw), URL: url}
	}
	if out != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, out); err != nil {
			return fmt.Errorf("failed to decode response from %s: %w", url, err)
		}
	}
	return nil
}
