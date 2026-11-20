// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/thunder-id/thunderid/internal/system/constants"
	"github.com/thunder-id/thunderid/internal/variablestore"
)

// gatewayStoreClientInterface reads and writes a gateway's variables and secrets, presenting its key.
// A call the gateway refuses fails with a *StoreRefusal.
type gatewayStoreClientInterface interface {
	ListVariables(ctx context.Context, gw *Gateway, key string,
		query StoreListQuery) (*variablestore.VariableListResponse, error)
	GetVariable(ctx context.Context, gw *Gateway, key, name string) (*variablestore.Variable, error)
	CreateVariable(ctx context.Context, gw *Gateway, key string,
		req variablestore.VariableRequest) (*variablestore.Variable, error)
	// SetVariable creates or replaces a variable, and reports whether it created it.
	SetVariable(ctx context.Context, gw *Gateway, key, name string,
		req variablestore.VariableUpdateRequest) (*variablestore.Variable, bool, error)
	DeleteVariable(ctx context.Context, gw *Gateway, key, name string) error

	ListSecrets(ctx context.Context, gw *Gateway, key string,
		query StoreListQuery) (*variablestore.SecretListResponse, error)
	GetSecret(ctx context.Context, gw *Gateway, key, name string) (*variablestore.Secret, error)
	CreateSecret(ctx context.Context, gw *Gateway, key string,
		req variablestore.SecretRequest) (*variablestore.Secret, error)
	// SetSecret creates or replaces a secret, and reports whether it created it.
	SetSecret(ctx context.Context, gw *Gateway, key, name string,
		req variablestore.SecretUpdateRequest) (*variablestore.Secret, bool, error)
	DeleteSecret(ctx context.Context, gw *Gateway, key, name string) error
}

// StoreListQuery narrows a listing of a gateway's variables or secrets. Each field is given to the
// gateway as it came, so the gateway validates it as it does for its own callers.
type StoreListQuery struct {
	Limit  string
	Offset string
	// Names is a comma-separated list of the names to list.
	Names  string
	Filter string
}

func (q StoreListQuery) values() url.Values {
	values := url.Values{}
	for name, value := range map[string]string{
		"limit": q.Limit, "offset": q.Offset, "names": q.Names, "filter": q.Filter,
	} {
		if value != "" {
			values.Set(name, value)
		}
	}
	return values
}

// StoreRefusal is a gateway refusing a call to its store: its status and its error, as it gave them.
type StoreRefusal struct {
	Status int
	Body   json.RawMessage
}

func (r *StoreRefusal) Error() string {
	return fmt.Sprintf("the gateway refused the call to its store: status %d", r.Status)
}

// Store collections a gateway serves.
const (
	collectionVariables = "variables"
	collectionSecrets   = "secrets"
)

// maxNamesPerLookup matches the most names a gateway's store answers for in one call.
const maxNamesPerLookup = 100

// maxNamesQueryBytes bounds the names= parameter of one call to a gateway's store, as it is sent: each
// name escaped and a %2C between two. Many proxies refuse a request line past 8 KB with a 414 or a
// 400, and a name may run to 255 characters, so a hundred of them would not fit; a batch stays well
// short of that, leaving room for the rest of the request line.
const maxNamesQueryBytes = 6 * 1024

// nameBatches splits names into the batches a call to the store takes: at most maxNamesPerLookup
// names each, whose names= parameter fits in maxNamesQueryBytes. A name too long to fit on its own,
// which a valid name never is, goes alone.
func nameBatches(names []string) [][]string {
	var batches [][]string
	batch := make([]string, 0, min(len(names), maxNamesPerLookup))
	size := 0
	for _, name := range names {
		cost := len(url.QueryEscape(name))
		if len(batch) > 0 {
			cost += len("%2C")
		}
		if len(batch) == maxNamesPerLookup || (len(batch) > 0 && size+cost > maxNamesQueryBytes) {
			batches = append(batches, batch)
			batch, size, cost = make([]string, 0, maxNamesPerLookup), 0, len(url.QueryEscape(name))
		}
		batch = append(batch, name)
		size += cost
	}
	if len(batch) > 0 {
		batches = append(batches, batch)
	}
	return batches
}

func (c *gatewayClient) ListVariables(ctx context.Context, gw *Gateway, key string,
	query StoreListQuery) (*variablestore.VariableListResponse, error) {
	var listing variablestore.VariableListResponse
	if _, err := c.callStore(ctx, gw, key, http.MethodGet, "/"+collectionVariables, query.values(), nil,
		&listing); err != nil {
		return nil, err
	}
	return &listing, nil
}

func (c *gatewayClient) GetVariable(ctx context.Context, gw *Gateway, key,
	name string) (*variablestore.Variable, error) {
	var variable variablestore.Variable
	if _, err := c.callStore(ctx, gw, key, http.MethodGet, itemPath(collectionVariables, name), nil, nil,
		&variable); err != nil {
		return nil, err
	}
	return &variable, nil
}

func (c *gatewayClient) CreateVariable(ctx context.Context, gw *Gateway, key string,
	req variablestore.VariableRequest) (*variablestore.Variable, error) {
	var variable variablestore.Variable
	if _, err := c.callStore(ctx, gw, key, http.MethodPost, "/"+collectionVariables, nil, req,
		&variable); err != nil {
		return nil, err
	}
	return &variable, nil
}

func (c *gatewayClient) SetVariable(ctx context.Context, gw *Gateway, key, name string,
	req variablestore.VariableUpdateRequest) (*variablestore.Variable, bool, error) {
	var variable variablestore.Variable
	status, err := c.callStore(ctx, gw, key, http.MethodPut, itemPath(collectionVariables, name), nil, req,
		&variable)
	if err != nil {
		return nil, false, err
	}
	return &variable, status == http.StatusCreated, nil
}

func (c *gatewayClient) DeleteVariable(ctx context.Context, gw *Gateway, key, name string) error {
	_, err := c.callStore(ctx, gw, key, http.MethodDelete, itemPath(collectionVariables, name), nil, nil, nil)
	return err
}

// DeleteVariables removes the named variables, a batch nameBatches makes, in one call, and
// returns those the gateway removed. A positive unchangedFor asks the gateway to remove only those
// last set longer ago than it, which the gateway checks on its own clock in the statement that removes
// them. A gateway too old to remove many at once refuses with a 404 or a 405.
func (c *gatewayClient) DeleteVariables(ctx context.Context, gw *Gateway, key string,
	names []string, unchangedFor time.Duration) ([]string, error) {
	return c.deleteMany(ctx, gw, key, collectionVariables, names, unchangedFor)
}

func (c *gatewayClient) ListSecrets(ctx context.Context, gw *Gateway, key string,
	query StoreListQuery) (*variablestore.SecretListResponse, error) {
	var listing variablestore.SecretListResponse
	if _, err := c.callStore(ctx, gw, key, http.MethodGet, "/"+collectionSecrets, query.values(), nil,
		&listing); err != nil {
		return nil, err
	}
	return &listing, nil
}

func (c *gatewayClient) GetSecret(ctx context.Context, gw *Gateway, key,
	name string) (*variablestore.Secret, error) {
	var secret variablestore.Secret
	if _, err := c.callStore(ctx, gw, key, http.MethodGet, itemPath(collectionSecrets, name), nil, nil,
		&secret); err != nil {
		return nil, err
	}
	return &secret, nil
}

func (c *gatewayClient) CreateSecret(ctx context.Context, gw *Gateway, key string,
	req variablestore.SecretRequest) (*variablestore.Secret, error) {
	var secret variablestore.Secret
	if _, err := c.callStore(ctx, gw, key, http.MethodPost, "/"+collectionSecrets, nil, req,
		&secret); err != nil {
		return nil, err
	}
	return &secret, nil
}

func (c *gatewayClient) SetSecret(ctx context.Context, gw *Gateway, key, name string,
	req variablestore.SecretUpdateRequest) (*variablestore.Secret, bool, error) {
	var secret variablestore.Secret
	status, err := c.callStore(ctx, gw, key, http.MethodPut, itemPath(collectionSecrets, name), nil, req,
		&secret)
	if err != nil {
		return nil, false, err
	}
	return &secret, status == http.StatusCreated, nil
}

func (c *gatewayClient) DeleteSecret(ctx context.Context, gw *Gateway, key, name string) error {
	_, err := c.callStore(ctx, gw, key, http.MethodDelete, itemPath(collectionSecrets, name), nil, nil, nil)
	return err
}

// DeleteSecrets removes the named secrets in one call, as DeleteVariables does variables.
func (c *gatewayClient) DeleteSecrets(ctx context.Context, gw *Gateway, key string,
	names []string, unchangedFor time.Duration) ([]string, error) {
	return c.deleteMany(ctx, gw, key, collectionSecrets, names, unchangedFor)
}

func (c *gatewayClient) deleteMany(ctx context.Context, gw *Gateway, key, collection string,
	names []string, unchangedFor time.Duration) ([]string, error) {
	var answer variablestore.DeleteResponse
	query := url.Values{"names": []string{strings.Join(names, ",")}}
	if unchangedFor > 0 {
		// Whole seconds, rounded up, so a value is never removed younger than asked.
		seconds := (unchangedFor + time.Second - 1) / time.Second
		query.Set("unchangedFor", strconv.FormatInt(int64(seconds), 10))
	}
	if _, err := c.callStore(ctx, gw, key, http.MethodDelete, "/"+collection, query, nil, &answer); err != nil {
		return nil, err
	}
	return answer.Deleted, nil
}

// HeldValues asks the gateway's store for the named variables and secrets, in batches it answers
// for, and returns those it holds with when each was last set.
func (c *gatewayClient) HeldValues(ctx context.Context, gw *Gateway, key string,
	variables, secrets []string) (*HeldValues, error) {
	held := &HeldValues{Variables: map[string]time.Time{}, Secrets: map[string]time.Time{}}
	for _, batch := range nameBatches(variables) {
		listing, err := c.ListVariables(ctx, gw, key, lookupOf(batch))
		if err != nil {
			return nil, err
		}
		for _, variable := range listing.Variables {
			held.Variables[variable.Name] = asTime(variable.UpdatedAt)
		}
	}
	for _, batch := range nameBatches(secrets) {
		listing, err := c.ListSecrets(ctx, gw, key, lookupOf(batch))
		if err != nil {
			return nil, err
		}
		for _, secret := range listing.Secrets {
			held.Secrets[secret.Name] = asTime(secret.UpdatedAt)
		}
	}
	return held, nil
}

func lookupOf(names []string) StoreListQuery {
	return StoreListQuery{Names: strings.Join(names, ","), Limit: fmt.Sprint(maxNamesPerLookup)}
}

func unheld(names []string, held map[string]time.Time) []string {
	var out []string
	for _, name := range names {
		if _, ok := held[name]; !ok {
			out = append(out, name)
		}
	}
	return out
}

// itemPath is the store path of one value. The name is escaped, so it cannot reach past the store to
// another of the gateway's paths.
func itemPath(collection, name string) string {
	return "/" + collection + "/" + url.PathEscape(name)
}

// callStore makes one call to the gateway's store with its key, and reads a successful answer into
// out. It returns the status the gateway answered with, and a *StoreRefusal when it refused the call.
func (c *gatewayClient) callStore(ctx context.Context, gw *Gateway, key, method, path string,
	query url.Values, in, out any) (int, error) {
	target := gw.BaseURL + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	var body io.Reader
	if in != nil {
		encoded, err := json.Marshal(in)
		if err != nil {
			return 0, fmt.Errorf("failed to encode the store request: %w", err)
		}
		body = bytes.NewReader(encoded)
	}
	httpReq, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return 0, fmt.Errorf("failed to build the store request: %w", err)
	}
	if in != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}
	httpReq.Header.Set(constants.APIKeyHeaderName, key)

	client, err := httpClientFor(gw, storeTimeout)
	if err != nil {
		return 0, err
	}
	defer releaseConnections(client)
	resp, err := client.Do(httpReq)
	if err != nil {
		return 0, fmt.Errorf("failed to reach the gateway: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	answer, err := io.ReadAll(io.LimitReader(resp.Body, maxImportAnswer))
	if err != nil {
		return 0, fmt.Errorf("failed to read the gateway's answer: %w", err)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return resp.StatusCode, &StoreRefusal{Status: resp.StatusCode, Body: answer}
	}
	if out != nil {
		if err := json.Unmarshal(answer, out); err != nil {
			return resp.StatusCode, fmt.Errorf("failed to read the gateway's answer: %w", err)
		}
	}
	return resp.StatusCode, nil
}
