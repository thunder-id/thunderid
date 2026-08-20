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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRouterDispatchesRegisteredMethod(t *testing.T) {
	r := NewRouter()
	r.Register("Echo", func(_ context.Context, params json.RawMessage) (json.RawMessage, *Error) {
		return params, nil
	})

	req := Request{JSONRPC: Version, ID: "1", Method: "Echo", Params: json.RawMessage(`"hi"`)}
	resp := r.Dispatch(context.Background(), req)
	assert.Equal(t, "1", resp.ID)
	assert.Nil(t, resp.Error)
	assert.JSONEq(t, `"hi"`, string(resp.Result))
}

func TestRouterUnknownMethodReturnsMethodNotFound(t *testing.T) {
	resp := NewRouter().Dispatch(context.Background(), Request{JSONRPC: Version, ID: "2", Method: "Nope"})
	assert.NotNil(t, resp.Error)
	assert.Equal(t, CodeMethodNotFound, resp.Error.Code)
	assert.Equal(t, "2", resp.ID)
}

func TestRouterPropagatesHandlerError(t *testing.T) {
	r := NewRouter()
	r.Register("Fail", func(_ context.Context, _ json.RawMessage) (json.RawMessage, *Error) {
		return nil, NewError(CodeInvalidParams, "bad")
	})
	resp := r.Dispatch(context.Background(), Request{JSONRPC: Version, ID: "3", Method: "Fail"})
	assert.Equal(t, CodeInvalidParams, resp.Error.Code)
}
