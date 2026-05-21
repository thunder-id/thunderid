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

package executor

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestShouldRegisterExecutor(t *testing.T) {
	require.True(t, shouldRegisterExecutor(ExecutorNameBasicAuth, nil))
	require.True(t, shouldRegisterExecutor(ExecutorNameBasicAuth, []string{ExecutorNameBasicAuth}))
	require.False(t, shouldRegisterExecutor(ExecutorNameBasicAuth, []string{ExecutorNameOAuth}))
}

func TestDefaultExecutorNamesIncludesBuiltins(t *testing.T) {
	names := DefaultExecutorNames()
	require.Contains(t, names, ExecutorNameBasicAuth)
	require.Contains(t, names, ExecutorNameAuthorization)
	require.Contains(t, names, ExecutorNameOAuth)
}
