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

import {HttpError, HttpRequestConfig, HttpResponse} from './models/http';

/**
 * Abstract base class for HTTP clients. Owns all handler/callback state and
 * the request lifecycle (pre-processing, transport, post-processing).
 *
 * Extend this class and implement `transport()` to plug in a custom HTTP transport.
 *
 * @example
 * ```ts
 * class MyHttpClient extends HttpClient {
 *   protected async transport<T>(config: HttpRequestConfig): Promise<HttpResponse<T>> {
 *     // custom fetch logic
 *   }
 * }
 * ```
 */
export abstract class HttpClient {
  private static readonly DEFAULT_HANDLER_DISABLE_TIMEOUT: number = 1000;

  private requestStartCallback: (request: HttpRequestConfig) => void = () => null;

  private requestSuccessCallback: (response: HttpResponse) => void = () => null;

  private requestErrorCallback: (error: HttpError) => void = () => null;

  private requestFinishCallback: () => void = () => null;

  constructor(
    private isHandlerEnabled = true,
    private attachToken: (request: HttpRequestConfig) => Promise<void> = (): Promise<void> => Promise.resolve(),
  ) {}

  /**
   * Implemented by subclasses. Performs the actual HTTP call with no handler
   * logic applied — that is handled by `request()`.
   */
  protected abstract transport<T = any>(config: HttpRequestConfig): Promise<HttpResponse<T>>;

  /**
   * Public HTTP request entry point. Applies pre/post processing around `transport()`.
   */
  async request<T = any>(config: HttpRequestConfig): Promise<HttpResponse<T>> {
    const processedConfig: HttpRequestConfig = await this.requestHandler(config);
    try {
      const response: HttpResponse<T> = await this.transport<T>(processedConfig);
      return this.successHandler(response);
    } catch (error: any) {
      this.errorHandler(error as HttpError);
      throw error;
    }
  }

  enableHandler(): void {
    this.isHandlerEnabled = true;
  }

  disableHandler(): void {
    this.isHandlerEnabled = false;
  }

  disableHandlerWithTimeout(timeout: number = HttpClient.DEFAULT_HANDLER_DISABLE_TIMEOUT): void {
    this.isHandlerEnabled = false;
    setTimeout(() => {
      this.isHandlerEnabled = true;
    }, timeout);
  }

  setHttpRequestStartCallback(cb: (req: HttpRequestConfig) => void): void {
    this.requestStartCallback = cb;
  }

  setHttpRequestSuccessCallback(cb: (res: HttpResponse) => void): void {
    this.requestSuccessCallback = cb;
  }

  setHttpRequestErrorCallback(cb: (err: HttpError) => void): void {
    this.requestErrorCallback = cb;
  }

  setHttpRequestFinishCallback(cb: () => void): void {
    this.requestFinishCallback = cb;
  }

  all<T>(values: (T | Promise<T>)[]): Promise<T[]> {
    return Promise.all(values);
  }

  spread<T, R>(callback: (...args: T[]) => R): (array: T[]) => R {
    return (array: T[]) => callback(...array);
  }

  protected async requestHandler(config: HttpRequestConfig): Promise<HttpRequestConfig> {
    await this.attachToken(config);

    if (config.shouldEncodeToFormData && config.data) {
      const formData: FormData = new FormData();
      Object.keys(config.data).forEach((key: string) => formData.append(key, config.data[key]));

      config.data = formData;
    }

    config.startTimeInMs = Date.now();

    if (this.isHandlerEnabled && typeof this.requestStartCallback === 'function') {
      this.requestStartCallback(config);
    }

    return config;
  }

  protected successHandler(response: HttpResponse): HttpResponse {
    if (this.isHandlerEnabled) {
      if (typeof this.requestSuccessCallback === 'function') {
        this.requestSuccessCallback(response);
      }
      if (typeof this.requestFinishCallback === 'function') {
        this.requestFinishCallback();
      }
    }
    return response;
  }

  protected errorHandler(error: HttpError): void {
    if (this.isHandlerEnabled) {
      if (typeof this.requestErrorCallback === 'function') {
        this.requestErrorCallback(error);
      }
      if (typeof this.requestFinishCallback === 'function') {
        this.requestFinishCallback();
      }
    }
  }
}
