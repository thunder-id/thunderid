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

import ThunderIDBrowserClient from '../ThunderIDBrowserClient';

/**
 * Creates an HTTP utility for making authenticated requests using a `ThunderIDBrowserClient` instance.
 *
 * @param client - The browser client instance to use for requests.
 * @returns An object with `request` and `requestAll` methods bound to the provided client.
 *
 * @example
 * ```typescript
 * const auth = new ThunderIDBrowserClient();
 * await auth.initialize(config);
 * const httpClient = http(auth);
 * const response = await httpClient.request({ url: '/api/data', method: 'GET' });
 * ```
 */
const http = (
  client: ThunderIDBrowserClient,
): {
  request: typeof ThunderIDBrowserClient.prototype.httpRequest;
  requestAll: typeof ThunderIDBrowserClient.prototype.httpRequestAll;
} => {
  return {
    request: client.httpRequest.bind(client),
    requestAll: client.httpRequestAll.bind(client),
  };
};

export default http;
