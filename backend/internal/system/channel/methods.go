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
	"encoding/json"

	"github.com/thunder-id/thunderid/internal/system/importer"
	"github.com/thunder-id/thunderid/internal/system/security"
	tidcommon "github.com/thunder-id/thunderid/pkg/thunderidengine/common"
)

// Channel RPC method names.
const (
	MethodImportRun = "Import.Run"
	MethodAgentPing = "Agent.Ping"
)

// ImportRunner is the subset of the importer service the channel invokes on the Data Plane. It is
// satisfied by importer.ImportServiceInterface.
type ImportRunner interface {
	ImportResources(
		ctx context.Context, request *importer.ImportRequest,
	) (*importer.ImportResponse, *tidcommon.ServiceError)
}

// pingResult is the Agent.Ping reply payload.
type pingResult struct {
	DataPlaneID string `json:"dataPlaneId"`
}

// RegisterDataPlaneMethods registers the Data Plane's inbound RPC handlers on the router.
func RegisterDataPlaneMethods(router *Router, runner ImportRunner, dpID string) {
	router.Register(MethodImportRun, func(ctx context.Context, params json.RawMessage) (json.RawMessage, *Error) {
		var req importer.ImportRequest
		if err := json.Unmarshal(params, &req); err != nil {
			return nil, NewError(CodeInvalidParams, "invalid import params: "+err.Error())
		}
		// An applied configuration is seeded, not requested by anyone, so it runs privileged, as the
		// bootstrap of the same resources does. Without it a deployment cannot accept its own first
		// apply: the bundle carries the user, group and role that would authorize importing it, so
		// nothing on a deployment that has never been applied to grants the permission to write them.
		//
		// The claim being acted on is the handshake, not the call. This router is reachable only over
		// the connection the data plane dialed out and authenticated with its own token, and is not
		// mounted on any HTTP route, so nothing a user sends can arrive here. Imports over the
		// management API keep the authorization they have; only what a control plane applies is
		// privileged.
		ctx = security.WithRuntimeContext(ctx)
		resp, svcErr := runner.ImportResources(ctx, &req)
		if svcErr != nil {
			return nil, serviceErrorToRPC(svcErr)
		}
		raw, err := json.Marshal(resp)
		if err != nil {
			return nil, NewError(CodeInternalError, err.Error())
		}
		return raw, nil
	})

	router.Register(MethodAgentPing, func(_ context.Context, _ json.RawMessage) (json.RawMessage, *Error) {
		raw, err := json.Marshal(pingResult{DataPlaneID: dpID})
		if err != nil {
			return nil, NewError(CodeInternalError, err.Error())
		}
		return raw, nil
	})
}

// serviceErrorToRPC maps a ThunderID ServiceError to a JSON-RPC error, preserving the human message
// and carrying the service error code in Data.
func serviceErrorToRPC(svcErr *tidcommon.ServiceError) *Error {
	code := CodeInternalError
	if svcErr.Type == tidcommon.ClientErrorType {
		code = CodeInvalidParams
	}
	data, _ := json.Marshal(map[string]string{"serviceCode": svcErr.Code})
	return &Error{Code: code, Message: svcErr.Error.DefaultValue, Data: data}
}

// CallImport pushes an import request to the given Data Plane and decodes its response.
func (s *Server) CallImport(
	ctx context.Context, dpID string, req *importer.ImportRequest,
) (*importer.ImportResponse, error) {
	raw, err := s.CallMethod(ctx, dpID, MethodImportRun, req)
	if err != nil {
		return nil, err
	}
	var resp importer.ImportResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Ping actively round-trips the Agent.Ping RPC to confirm the Data Plane's handler loop is alive.
func (s *Server) Ping(ctx context.Context, dpID string) error {
	_, err := s.CallMethod(ctx, dpID, MethodAgentPing, nil)
	return err
}
