/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
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

// ResolveBaseURL polls until Thunder responds on https or http, returning the
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

// CheckReady returns true if Thunder is responding on the readiness endpoint.
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
