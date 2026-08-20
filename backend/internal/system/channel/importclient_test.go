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

package channel

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/thunder-id/thunderid/internal/system/importer"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

func TestImportClientPushesToItsDataPlane(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runner := &fakeImportRunner{
		resp: &importer.ImportResponse{Summary: &importer.ImportSummary{TotalDocuments: 1, Imported: 1}},
	}
	s := wireEndToEnd(t, ctx, runner)

	client := NewImportClient(s, "dp-1")
	assert.Equal(t, "dp-1", client.DataPlaneID())

	resp, err := client.Import(ctx, &importer.ImportRequest{Content: "resource_type: application"})
	require.NoError(t, err)
	require.NotNil(t, resp.Summary)
	assert.Equal(t, 1, resp.Summary.Imported)
	assert.Equal(t, "resource_type: application", runner.lastContent)
}

func TestImportClientPropagatesServiceError(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	runner := &fakeImportRunner{svcErr: &tidcommon.ServiceError{
		Type:  tidcommon.ClientErrorType,
		Code:  "IMP-1001",
		Error: tidcommon.I18nMessage{DefaultValue: "invalid import request"},
	}}
	s := wireEndToEnd(t, ctx, runner)

	_, err := NewImportClient(s, "dp-1").Import(ctx, &importer.ImportRequest{Content: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid import request")
}

func TestImportClientFailsFastWhenDataPlaneOffline(t *testing.T) {
	s := InitializeServer(http.NewServeMux(), ServerConfig{Enabled: true, Path: "/cp/connect", AuthToken: "tok"}, nil)
	defer s.Close()

	_, err := NewImportClient(s, "dp-absent").Import(context.Background(), &importer.ImportRequest{Content: "x"})
	assert.ErrorIs(t, err, ErrDataPlaneNotConnected)
}

func TestImportClientSatisfiesDataPlaneImporter(t *testing.T) {
	var _ DataPlaneImporter = NewImportClient(&Server{}, "dp-1")
}
