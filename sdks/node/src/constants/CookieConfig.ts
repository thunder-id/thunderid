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

import {VendorConstants} from '@thunderid/javascript';

class CookieConfig {
  static readonly SESSION_COOKIE_NAME: string = `__${VendorConstants.VENDOR_PREFIX}__session`;

  static readonly TEMP_SESSION_COOKIE_NAME: string = `__${VendorConstants.VENDOR_PREFIX}__temp.session`;

  static readonly DEFAULT_MAX_AGE: number = 3600;

  static readonly DEFAULT_HTTP_ONLY: boolean = true;

  static readonly DEFAULT_SAME_SITE: 'lax' | 'strict' | 'none' = 'lax';

  static readonly DEFAULT_SECURE: boolean = true;

  // eslint-disable-next-line @typescript-eslint/no-empty-function
  private constructor() {}
}

export default CookieConfig;
