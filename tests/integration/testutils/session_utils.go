// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// SessionView is the part of a session in the session management API's responses that tests read.
type SessionView struct {
	ID              string `json:"id"`
	Type            string `json:"type"`
	UserID          string `json:"userId"`
	UserDisplayName string `json:"userDisplayName"`
	UserAgent       string `json:"userAgent"`
	CreationIP      string `json:"creationIp"`
	LastActiveIP    string `json:"lastActiveIp"`
	IsCurrent       *bool  `json:"isCurrent"`
	Participants    []struct {
		AppID   string `json:"appId"`
		OUID    string `json:"ouId"`
		AppName string `json:"appName"`
	} `json:"participants"`
}

// SessionListView is a page of sessions from GET /sessions or GET /users/me/sessions.
type SessionListView struct {
	TotalResults int           `json:"totalResults"`
	Count        int           `json:"count"`
	Sessions     []SessionView `json:"sessions"`
	Links        []struct {
		Href string `json:"href"`
		Rel  string `json:"rel"`
	} `json:"links"`
}

// ListedSession returns the session with the ID from a GET for path to the session management API, or nil when
// the page doesn't list it.
func ListedSession(client *http.Client, path, sessionID string) (*SessionView, error) {
	var list SessionListView
	status, raw, err := GetSessions(client, path, &list)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("session listing %s returned %d: %s", path, status, raw)
	}
	for i := range list.Sessions {
		if list.Sessions[i].ID == sessionID {
			return &list.Sessions[i], nil
		}
	}
	return nil, nil
}

// GetSessions sends a GET for path to the session management API with client. It decodes a 200 response
// into out when out is not nil, and returns the status code and the raw body.
func GetSessions(client *http.Client, path string, out interface{}) (int, string, error) {
	resp, err := client.Get(TestServerURL + path)
	if err != nil {
		return 0, "", fmt.Errorf("session API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", fmt.Errorf("failed to read the session API response: %w", err)
	}
	if resp.StatusCode == http.StatusOK && out != nil {
		if err := json.Unmarshal(body, out); err != nil {
			return 0, "", fmt.Errorf("failed to decode the session API response %s: %w", body, err)
		}
	}
	return resp.StatusCode, string(body), nil
}

// DeleteSessions sends a DELETE for path to the session management API with client, and returns the status code
// and the raw body.
func DeleteSessions(client *http.Client, path string) (int, string, error) {
	req, err := http.NewRequest(http.MethodDelete, TestServerURL+path, nil)
	if err != nil {
		return 0, "", fmt.Errorf("failed to build the session API request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, "", fmt.Errorf("session API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", fmt.Errorf("failed to read the session API response: %w", err)
	}
	return resp.StatusCode, string(body), nil
}
