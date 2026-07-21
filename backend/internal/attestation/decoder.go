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

package attestation

import (
	"context"
	"fmt"

	"cloud.google.com/go/auth/credentials"
	"google.golang.org/api/option"
	playintegrity "google.golang.org/api/playintegrity/v1"
)

// integrityTokenDecoder decodes a Play Integrity token into its plaintext payload by calling
// Google's Play Integrity API. It is an internal seam so the API call can be mocked in tests.
type integrityTokenDecoder interface {
	Decode(ctx context.Context, credentialsJSON, packageName, token string) (
		*playintegrity.TokenPayloadExternal, error)
}

// googlePlayIntegrityDecoder decodes tokens by calling the Google Play Integrity API using the
// application's service account credentials.
type googlePlayIntegrityDecoder struct{}

// newGooglePlayIntegrityDecoder creates a token decoder backed by the Google Play Integrity API.
func newGooglePlayIntegrityDecoder() integrityTokenDecoder {
	return &googlePlayIntegrityDecoder{}
}

// Decode calls the Play Integrity decodeIntegrityToken endpoint for the given package.
func (d *googlePlayIntegrityDecoder) Decode(ctx context.Context, credentialsJSON, packageName, token string) (
	*playintegrity.TokenPayloadExternal, error) {
	creds, err := credentials.DetectDefault(&credentials.DetectOptions{
		CredentialsJSON: []byte(credentialsJSON),
		Scopes:          []string{playintegrity.PlayintegrityScope},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to parse play integrity credentials: %w", err)
	}

	svc, err := playintegrity.NewService(ctx, option.WithAuthCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("failed to create play integrity client: %w", err)
	}

	resp, err := svc.V1.DecodeIntegrityToken(packageName,
		&playintegrity.DecodeIntegrityTokenRequest{IntegrityToken: token}).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("play integrity decode request failed: %w", err)
	}
	return resp.TokenPayloadExternal, nil
}
