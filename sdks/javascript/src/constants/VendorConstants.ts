/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
 * Constants for vendor-specific configurations.
 * By default, the vendor is inferred as ThunderID.
 *
 * @example
 * ```typescript
 *  // Using the vendor prefix in a URL
 * const apiUrl = `${VendorConstants.VENDOR_PREFIX}/api/v1/resource`;
 * ```
 */
const VendorConstants: {
  VENDOR_PREFIX: string;
} = {
  /**
   * The prefix used for vendor-specific API endpoints, CSS classes, or other identifiers.
   */
  VENDOR_PREFIX: 'thunderid',
} as const;

export default VendorConstants;
