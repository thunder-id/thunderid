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

import {
  User,
  HttpResponse,
  FetchHttpClient,
  HttpRequestConfig,
  getScim2Me as baseGetScim2Me,
  GetScim2MeConfig as BaseGetScim2MeConfig,
} from '@thunderid/browser';

export interface GetScim2MeConfig extends Omit<BaseGetScim2MeConfig, 'fetcher'> {
  fetcher?: (url: string, config: RequestInit) => Promise<Response>;
  instanceId?: number;
}

const getScim2Me = async ({fetcher, instanceId = 0, ...requestConfig}: GetScim2MeConfig): Promise<User> => {
  const defaultFetcher = async (url: string, config: RequestInit): Promise<Response> => {
    const httpClient: FetchHttpClient = FetchHttpClient.getInstance(instanceId);
    
    const response: HttpResponse<any> = await httpClient.request({
      headers: config.headers as Record<string, string>,
      method: config.method || 'GET',
      url,
    } as HttpRequestConfig);

    return {
      json: () => Promise.resolve(response.data),
      ok: response.status >= 200 && response.status < 300,
      status: response.status,
      statusText: response.statusText || '',
      text: () => Promise.resolve(typeof response.data === 'string' ? response.data : JSON.stringify(response.data)),
    } as Response;
  };

  return baseGetScim2Me({
    ...requestConfig,
    fetcher: fetcher || defaultFetcher,
  });
};

export default getScim2Me;
