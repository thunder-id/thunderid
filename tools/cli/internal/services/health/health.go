// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// Package health polls the ThunderID readiness endpoint and provides browser-launch helpers.
package health

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"time"
)

// DefaultPort is the port ThunderID listens on by default.
const DefaultPort = 8090

// ResolveBaseURL polls until ThunderID responds on https or http, returning the
// confirmed base URL and true. Returns ("", false) if neither scheme responds
// within timeout. Each individual probe is capped to min(2s, remaining budget)
// so the function never overruns its deadline.
func ResolveBaseURL(port int, timeout time.Duration) (string, bool) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		for _, scheme := range []string{"https", "http"} {
			remaining := time.Until(deadline)
			if remaining <= 0 {
				return "", false
			}
			probeTimeout := remaining
			if probeTimeout > 2*time.Second {
				probeTimeout = 2 * time.Second
			}
			base := fmt.Sprintf("%s://localhost:%d", scheme, port)
			if checkReadyIn(base, probeTimeout) {
				return base, true
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return "", false
}

// CheckReady returns true if ThunderID is responding on the readiness endpoint.
func CheckReady(baseURL string) bool {
	return checkReadyIn(baseURL, 2*time.Second)
}

func checkReadyIn(baseURL string, timeout time.Duration) bool {
	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
		},
	}
	resp, err := client.Get(baseURL + "/health/readiness")
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode == http.StatusOK
}
