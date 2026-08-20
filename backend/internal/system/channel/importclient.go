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

	"github.com/thunder-id/thunderid/internal/system/importer"
)

// DataPlaneImporter pushes import requests to one Data Plane. Control Plane code depends on this
// rather than on the channel server, so a caller needs neither the Data Plane id nor the transport,
// and can be given a test double instead.
type DataPlaneImporter interface {
	// Import applies req on the Data Plane and returns its outcome. It returns
	// ErrDataPlaneNotConnected when that Data Plane has no live connection, so a caller can tell a
	// disconnected Data Plane apart from a failed import.
	Import(ctx context.Context, req *importer.ImportRequest) (*importer.ImportResponse, error)
}

// ImportClient is the DataPlaneImporter backed by the channel server, bound to a single Data Plane.
// It is exported so a Control Plane service can hold one as a field.
type ImportClient struct {
	server *Server
	dpID   string
}

// NewImportClient binds an import client to one Data Plane id. The id is resolved on each call
// rather than at construction, so a client stays usable across that Data Plane's reconnects.
func NewImportClient(server *Server, dpID string) *ImportClient {
	return &ImportClient{server: server, dpID: dpID}
}

// DataPlaneID reports the Data Plane this client targets.
func (c *ImportClient) DataPlaneID() string {
	return c.dpID
}

// Import pushes req to this client's Data Plane over the channel.
//
// The call is at-most-once from the Control Plane's point of view: if the connection drops while the
// Data Plane is applying the import, this returns an error even though the import may have been
// applied. There is no idempotency key, so retrying after an ambiguous failure can apply the same
// import twice.
func (c *ImportClient) Import(
	ctx context.Context, req *importer.ImportRequest,
) (*importer.ImportResponse, error) {
	return c.server.CallImport(ctx, c.dpID, req)
}
