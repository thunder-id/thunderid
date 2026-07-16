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

package utils_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/tools/cli/internal/utils"
)

func TestMeetsMinNodeVersion(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{utils.MinNodeVersion, true},
		{"22.23.2", true},
		{"22.24.0", true},
		{"23.0.0", true},
		{"22.23.0", false},
		{"22.22.9", false},
		{"20.11.0", false},
		{"9.0.0", false},
	}
	for _, tt := range tests {
		assert.Equal(t, tt.want, utils.MeetsMinNodeVersion(tt.version), "version %q", tt.version)
	}
}

func TestNodeUpgradeHint_MentionsNvmAndDownloadURL(t *testing.T) {
	hint := utils.NodeUpgradeHint()
	assert.Contains(t, hint, "nvm install "+utils.MinNodeVersion)
	assert.Contains(t, hint, "https://nodejs.org/en/download")
}
