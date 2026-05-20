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

export interface HttpResponse<T = any> {
  config: HttpRequestConfig;
  data: T;
  headers: Record<string, string>;
  status: number;
  statusText: string;
}

export interface HttpError extends Error {
  code?: string;
  config?: HttpRequestConfig;
  response?: {
    data?: any;
    headers?: Record<string, string>;
    status: number;
    statusText?: string;
  };
}

export interface HttpRequestConfig extends Omit<RequestInit, 'body' | 'headers' | 'method'> {
  attachToken?: boolean;
  data?: any;
  headers?: Record<string, string>;
  method?: string;
  params?: Record<string, any>;
  shouldAttachIDPAccessToken?: boolean;
  shouldEncodeToFormData?: boolean;
  startTimeInMs?: number;
  url?: string;
}
