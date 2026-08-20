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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRequestRoundTrips(t *testing.T) {
	req := Request{JSONRPC: Version, ID: "abc", Method: "Import.Run", Params: json.RawMessage(`{"content":"x"}`)}
	raw, err := json.Marshal(req)
	assert.NoError(t, err)

	var got Request
	assert.NoError(t, json.Unmarshal(raw, &got))
	assert.Equal(t, "2.0", got.JSONRPC)
	assert.Equal(t, "abc", got.ID)
	assert.Equal(t, "Import.Run", got.Method)
	assert.JSONEq(t, `{"content":"x"}`, string(got.Params))
}

func TestResponseOmitsEmptyFields(t *testing.T) {
	raw, err := json.Marshal(Response{JSONRPC: Version, ID: "1", Result: json.RawMessage(`{"ok":true}`)})
	assert.NoError(t, err)
	assert.NotContains(t, string(raw), "error")

	raw, err = json.Marshal(Response{JSONRPC: Version, ID: "1", Error: NewError(CodeMethodNotFound, "nope")})
	assert.NoError(t, err)
	assert.NotContains(t, string(raw), "result")
}

func TestErrorImplementsError(t *testing.T) {
	var err error = NewError(CodeInvalidParams, "bad params")
	assert.Contains(t, err.Error(), "-32602")
	assert.Contains(t, err.Error(), "bad params")
}
