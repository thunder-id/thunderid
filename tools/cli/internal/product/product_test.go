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

package product_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/thunder-id/thunderid/tools/cli/internal/product"
)

func TestConstants_NonEmpty(t *testing.T) {
	assert.NotEmpty(t, product.Name)
	assert.NotEmpty(t, product.Slug)
	assert.NotEmpty(t, product.ReleasesURL)
	assert.NotEmpty(t, product.GitHubAPI)
}

func TestBrandColors_HexFormat(t *testing.T) {
	for _, color := range []string{product.ColorDeepNavy, product.ColorElectricBlue, product.ColorWhite} {
		assert.True(t, strings.HasPrefix(color, "#"), "color %q should start with #", color)
		assert.Equal(t, 7, len(color), "color %q should be 7 chars (#RRGGBB)", color)
	}
}

func TestReleasesURL_HTTPS(t *testing.T) {
	assert.True(t, strings.HasPrefix(product.ReleasesURL, "https://"))
	assert.True(t, strings.HasPrefix(product.GitHubAPI, "https://"))
}
