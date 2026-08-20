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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
	"github.com/thunder-id/thunderid/internal/system/deployment"
	"github.com/thunder-id/thunderid/internal/system/log"
	"github.com/thunder-id/thunderid/internal/system/varname"
)

// LocalCaptureRouter is the in-process environment manager's capture entry point.
// LocalCaptureRouter is the environment manager in this process, which knows which data plane holds
// a credential.
type LocalCaptureRouter interface {
	CaptureSecret(ctx context.Context, deploymentID, name string, body map[string]interface{}) (int, error)
}

// localSecretCapture hands a captured credential to the environment manager running in this process,
// which routes it to the deployment's data planes.
//
// When the environment manager is hosted here, a captured credential has no reason to leave the process:
// forwarding it over HTTP would mean this server authenticating to itself, and a deployment would need a
// second environment manager running just to receive its own secrets.
type localSecretCapture struct {
	router LocalCaptureRouter
	// hashConfig reads the server's configured hashing. It is a field so a test can supply one without
	// a server runtime.
	hashConfig func() (cryptolib.HashConfig, error)
}

// CaptureSecret hands over a credential, hashing it first when nothing needs the original.
func (c *localSecretCapture) CaptureSecret(ctx context.Context, resourceType, resourceName, field,
	value string) {
	if value == "" {
		return
	}
	c.capture(ctx, resourceType, resourceName, field, value, verifiableFields[strings.ToLower(field)])
}

// CaptureReplayableSecret hands over a credential that has to stay readable because the Data Plane
// gives it to a third party.
func (c *localSecretCapture) CaptureReplayableSecret(ctx context.Context, resourceType, resourceName,
	field, value string) {
	if value == "" {
		return
	}
	c.capture(ctx, resourceType, resourceName, field, value, false)
}

// capture builds the same body the HTTP forwarder sends and delivers it in process. Failures are logged
// rather than propagated, so creating a resource does not fail because a data plane is unreachable.
func (c *localSecretCapture) capture(ctx context.Context, resourceType, resourceName, field, value string,
	verifiable bool) {
	key := varname.DeriveVariableName(resourceType, resourceName, field)
	tenant := deployment.Resolve(ctx, "")
	if strings.TrimSpace(tenant) == "" {
		log.GetLogger().Warn(ctx, "No tenant in context, so a captured secret could not be routed",
			log.String("key", key))
		return
	}

	forwarder := &secretForwarder{hashConfig: c.hashConfig}
	body, err := forwarder.buildBody(value, verifiable, fmt.Sprintf("Captured %s for %s", field, resourceName))
	if err != nil {
		log.GetLogger().Warn(ctx, "Failed to prepare a secret for the environment manager",
			log.String("key", key), log.Error(err))
		return
	}
	payload, err := toBodyMap(body)
	if err != nil {
		log.GetLogger().Warn(ctx, "Failed to encode a secret for the environment manager",
			log.String("key", key), log.Error(err))
		return
	}

	// Zero deliveries is not a failure: no environment is registered for the tenant yet, and a promotion
	// creates the credential against the target once one is.
	if _, err := c.router.CaptureSecret(ctx, tenant, key, payload); err != nil {
		log.GetLogger().Warn(ctx, "Failed to store a secret through the environment manager",
			log.String("key", key), log.Error(err))
	}
}

// toBodyMap converts the write payload into the generic form the environment manager passes through, so
// both capture paths send a data plane exactly the same document.
func toBodyMap(body putSecretBody) (map[string]interface{}, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	var out map[string]interface{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}
