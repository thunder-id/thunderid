/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/**
 * Type definitions for the welcome page release assets data.
 */
export interface ReleaseAssetInput {
  downloadUrl: string;
  name: string;
  sizeLabel: string;
}

/**
 * Type definition for the response of the Wayfinder configuration import API.
 */
export interface ReleaseEntry {
  assets: ReleaseAssetInput[];
  tagName: string;
}

/**
 * Type definition for the welcome page releases data, which includes the latest release and a list of all releases.
 */
export interface ReleasesData {
  latestRelease: ReleaseEntry | null;
  releases: ReleaseEntry[];
}
