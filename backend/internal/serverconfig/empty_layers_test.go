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

package serverconfig

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/cors"
)

// TestGetConfig_EmptyLayersSerializeToArrays wires the real CORS handler with an empty store and asserts
// that unset layers serialize as [] (not null), consistent with the merged layer.
func TestGetConfig_EmptyLayersSerializeToArrays(t *testing.T) {
	store := newServerConfigStoreInterfaceMock(t)
	store.EXPECT().GetServerConfig(mock.Anything, ConfigNameCORS).Return(storeLayers{}, nil)

	svc := newServerConfigService(store,
		map[ConfigName]ServerConfigHandlerInterface{ConfigNameCORS: cors.OriginHandler{}})

	layers, svcErr := svc.GetConfig(context.Background(), ConfigNameCORS)
	require.Nil(t, svcErr)

	out, err := json.Marshal(layers)
	require.NoError(t, err)
	assert.JSONEq(t,
		`{"readOnly":{"allowedOrigins":[]},"writable":{"allowedOrigins":[]},"merged":{"allowedOrigins":[]}}`,
		string(out))
}
