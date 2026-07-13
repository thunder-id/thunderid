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

package dpop

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	oauthconfig "github.com/thunder-id/thunderid/internal/oauth/config"
	engineconfig "github.com/thunder-id/thunderid/pkg/thunderidengine/config"
	"github.com/thunder-id/thunderid/tests/mocks/oauth/oauth2/jtimock"
)

// TestInitialize verifies that Initialize builds a verifier wired to the given JTI store
// and carrying the policy settings from the OAuth config's DPoP section.
func TestInitialize(t *testing.T) {
	jtiStore := jtimock.NewJTIStoreInterfaceMock(t)
	cfg := oauthconfig.Config{
		OAuth: engineconfig.OAuthConfig{
			DPoP: engineconfig.DPoPConfig{
				IatWindow:    120,
				Leeway:       5,
				AllowedAlgs:  []string{"ES256", "PS256"},
				MaxJTILength: 100,
			},
		},
	}

	v := Initialize(cfg, jtiStore)
	require.NotNil(t, v)

	impl, ok := v.(*verifier)
	require.True(t, ok)

	assert.Same(t, jtiStore, impl.jtiStore)
	assert.Equal(t, 120*time.Second, impl.iatWindow)
	assert.Equal(t, 5*time.Second, impl.leeway)
	assert.Equal(t, 100, impl.maxJTILength)
	assert.Equal(t, map[string]struct{}{"ES256": {}, "PS256": {}}, impl.allowedAlgs)
}
