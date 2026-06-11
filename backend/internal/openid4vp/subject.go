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

package openid4vp

import (
	"encoding/base64"
	"fmt"
	"sort"
	"strings"

	"github.com/thunder-id/thunderid/internal/system/cryptolib"
)

// defaultSubjectDeriver returns a subjectDeriver that prefers the credential's
// "sub" and otherwise hashes the issuer plus sorted "name=value" pairs.
func defaultSubjectDeriver(claims []string) subjectDeriver {
	sorted := append([]string(nil), claims...)
	sort.Strings(sorted)
	return func(vp *VerifiedPresentation) string {
		if vp == nil {
			return ""
		}
		if vp.Subject != "" {
			return vp.Subject
		}
		parts := []string{vp.Issuer}
		for _, field := range sorted {
			if val, ok := vp.Claims[field]; ok {
				parts = append(parts, fmt.Sprintf("%s=%v", field, val))
			}
		}
		sum, err := cryptolib.Hash([]byte(strings.Join(parts, "|")), cryptolib.GenericSHA256)
		if err != nil {
			return ""
		}
		return base64.RawURLEncoding.EncodeToString(sum)
	}
}
