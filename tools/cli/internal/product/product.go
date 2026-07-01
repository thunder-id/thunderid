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

// Package product contains product-level constants shared across the CLI.
package product

// Product identity constants.
const (
	Name = "ThunderID"
	Slug = "thunderid"
)

// Distribution URLs.
const (
	ReleasesURL      = "https://thunderid.dev/data/releases.json"
	GitHubAPI        = "https://api.github.com/repos/thunder-id/thunderid/releases/latest"
	GitHubArchiveURL = "https://codeload.github.com/thunder-id/thunderid/zip/refs/heads/main"
)

// Brand colors.
const (
	ColorDeepNavy     = "#05213F" // primary brand — logo text and dark backgrounds
	ColorElectricBlue = "#3688FF" // accent — icon highlight, links, call-to-action
	ColorWhite        = "#FFFFFF" // light backgrounds and inverted text
)
