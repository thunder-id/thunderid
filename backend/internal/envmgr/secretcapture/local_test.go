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
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

package secretcapture

import (
	"context"
	"errors"
	"testing"

	"github.com/thunder-id/thunderid/internal/system/deployment"
)

// recordingRouter stands in for the in-process environment manager.
type recordingRouter struct {
	tenant string
	name   string
	body   map[string]interface{}
	calls  int
	err    error
}

func (r *recordingRouter) CaptureSecret(_ context.Context, deploymentID, name string,
	body map[string]interface{}) (int, error) {
	r.calls++
	r.tenant, r.name, r.body = deploymentID, name, body
	return 1, r.err
}

func TestLocalCaptureRoutesToTheTenantUnderThePlaceholderKey(t *testing.T) {
	router := &recordingRouter{}
	capturer := &localSecretCapture{router: router, hashConfig: testHashConfig}

	ctx := deployment.WithID(context.Background(), "tenant-a")
	capturer.CaptureSecret(ctx, "application", "App A", "clientSecret", "s3cret")

	if router.tenant != "tenant-a" {
		t.Fatalf("expected the credential to be routed to its own tenant, got %q", router.tenant)
	}
	// The key has to match the placeholder the export emits, or the import cannot resolve it.
	if router.name != "APPLICATION_APP_A_CLIENT_SECRET" {
		t.Fatalf("unexpected key %q", router.name)
	}
	// An application's client secret is only ever compared, so nothing needs the original.
	if router.body["kind"] != "hash" {
		t.Fatalf("expected a hash, got %v", router.body["kind"])
	}
	if router.body["value"] == "s3cret" {
		t.Fatal("the plaintext credential must never be handed over")
	}
}

func TestLocalCaptureKeepsAReplayedCredentialReadable(t *testing.T) {
	router := &recordingRouter{}
	capturer := &localSecretCapture{router: router, hashConfig: testHashConfig}

	ctx := deployment.WithID(context.Background(), "tenant-a")
	capturer.CaptureReplayableSecret(ctx, "connection", "Twilio", "authToken", "provider-issued")

	// Hashing this would leave the connection unable to authenticate to Twilio.
	if router.body["kind"] != "value" || router.body["value"] != "provider-issued" {
		t.Fatalf("expected the value to be kept as is, got %v", router.body)
	}
}

func TestLocalCaptureRefusesToRouteWithoutATenant(t *testing.T) {
	router := &recordingRouter{}
	capturer := &localSecretCapture{router: router, hashConfig: testHashConfig}

	// Without a tenant the credential would land on some other deployment's data plane.
	capturer.CaptureSecret(context.Background(), "application", "App A", "clientSecret", "s3cret")

	if router.calls != 0 {
		t.Fatal("a credential with no tenant must not be routed")
	}
}

func TestLocalCaptureSurvivesAFailingEnvironmentManager(t *testing.T) {
	router := &recordingRouter{err: errors.New("data plane unreachable")}
	capturer := &localSecretCapture{router: router, hashConfig: testHashConfig}

	// Creating a resource must not fail because a data plane is briefly unavailable.
	ctx := deployment.WithID(context.Background(), "tenant-a")
	capturer.CaptureSecret(ctx, "application", "App A", "clientSecret", "s3cret")

	if router.calls != 1 {
		t.Fatalf("expected the delivery to be attempted, got %d calls", router.calls)
	}
}

func TestLocalCaptureIgnoresAnEmptyValue(t *testing.T) {
	router := &recordingRouter{}
	capturer := &localSecretCapture{router: router, hashConfig: testHashConfig}

	ctx := deployment.WithID(context.Background(), "tenant-a")
	capturer.CaptureSecret(ctx, "application", "App A", "clientSecret", "")

	if router.calls != 0 {
		t.Fatal("there is nothing to store for an empty credential")
	}
}
