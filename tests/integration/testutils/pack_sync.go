// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

// The pack keeps gateway.auto_apply on, so a write on the Control Plane reaches the Data Plane on its
// own, some seconds later: the Control Plane waits out a debounce, captures a version, and applies it,
// while the values the version refers to are pushed to the Data Plane in the background. A test that
// reads the Data Plane right after a write races all of that. PackSyncToDP is the barrier: it
// captures, applies and checks, and reports what stopped it rather than sleeping.

const (
	// gatewayMissingValues is the code an apply answers with while a value it refers to has not
	// reached the Data Plane yet. It passes once the background push does.
	gatewayMissingValues = "GTW-1017"

	packSyncPoll = 500 * time.Millisecond
)

// packVersion is a captured configuration version.
type packVersion struct {
	Hash string `json:"version"`
}

// packApplied is what a gateway holds.
type packApplied struct {
	Applied  string `json:"appliedVersion"`
	Complete bool   `json:"complete"`
}

// packApplyOutcome is the answer of an apply.
type packApplyOutcome struct {
	Recorded bool   `json:"recorded"`
	Complete bool   `json:"complete"`
	Queued   bool   `json:"queued"`
	Code     string `json:"code"`
}

// PackGatewayID returns the ID the Control Plane knows the pack's Data Plane by.
func PackGatewayID(cp *PackPlane) (string, error) {
	var gateways []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	status, raw, err := cp.Do(http.MethodGet, "/gateways", nil, &gateways)
	if err != nil {
		return "", err
	}
	if status != http.StatusOK {
		return "", fmt.Errorf("listing the gateways answered %d: %s", status, raw)
	}
	for _, gateway := range gateways {
		if gateway.Name == PackGatewayName {
			return gateway.ID, nil
		}
	}
	return "", fmt.Errorf("the control plane has no gateway named %q: %s", PackGatewayName, raw)
}

// PackSyncToDP returns once the Data Plane holds everything the Control Plane held when it was
// called, or says why it does not within packSyncTimeout. It captures the Control Plane's
// configuration as a version, applies the latest version to the Data Plane, and checks that the Data
// Plane then holds that version in full. It retries an apply refused for a value that has not arrived
// and one queued behind an apply already running, as the one auto-apply sends.
func PackSyncToDP() error {
	cp := PackCP()
	gatewayID, err := PackGatewayID(cp)
	if err != nil {
		return err
	}

	deadline := time.Now().Add(packSyncTimeout)
	lastProblem := "no attempt was made"
	for time.Now().Before(deadline) {
		var captured packVersion
		status, raw, err := cp.Do(http.MethodPost, "/configuration-versions", map[string]any{}, &captured)
		if err != nil {
			return err
		}
		if status != http.StatusCreated && status != http.StatusOK {
			return fmt.Errorf("capturing a version answered %d: %s", status, raw)
		}

		var outcome packApplyOutcome
		status, raw, err = cp.Do(http.MethodPost, "/gateways/"+gatewayID+"/apply",
			map[string]any{"version": "latest"}, &outcome)
		if err != nil {
			return err
		}
		switch {
		case status == http.StatusOK && outcome.Recorded:
			held, err := packAppliedVersion(cp, gatewayID)
			if err != nil {
				return err
			}
			if held.Applied == captured.Hash && held.Complete {
				return nil
			}
			lastProblem = fmt.Sprintf("the data plane holds version %q (complete: %t), not %q",
				held.Applied, held.Complete, captured.Hash)
		case status == http.StatusAccepted && outcome.Queued:
			lastProblem = "the apply is queued behind one already running"
		case strings.Contains(string(raw), gatewayMissingValues):
			lastProblem = "a value the version refers to has not reached the data plane: " + string(raw)
		default:
			return fmt.Errorf("applying version %q answered %d: %s", captured.Hash, status, raw)
		}
		time.Sleep(packSyncPoll)
	}
	return fmt.Errorf("the data plane did not catch up within %s: %s", packSyncTimeout, lastProblem)
}

// packAppliedVersion reads the version a gateway holds.
func packAppliedVersion(cp *PackPlane, gatewayID string) (packApplied, error) {
	var held packApplied
	status, raw, err := cp.Do(http.MethodGet, "/gateways/"+gatewayID+"/applied-version", nil, &held)
	if err != nil {
		return held, err
	}
	if status != http.StatusOK {
		return held, fmt.Errorf("reading the applied version answered %d: %s", status, raw)
	}
	return held, nil
}
