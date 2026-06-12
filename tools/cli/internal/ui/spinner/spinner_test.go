/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
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

package spinner_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/tools/cli/internal/ui/spinner"
)

func TestDefaultWidth_Positive(t *testing.T) {
	assert.Greater(t, spinner.DefaultWidth, 0)
}

func TestRender_ReturnsNonEmptyString(t *testing.T) {
	for _, pct := range []int{0, 25, 50, 75, 100} {
		result := spinner.Render(pct)
		assert.NotEmpty(t, result, "Render(%d) returned empty string", pct)
	}
}
