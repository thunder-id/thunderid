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

import {ref} from 'vue';

export const navigateTo = async (...args: unknown[]): Promise<void> => {
  if (args.length) {
    // noop
  }
};

export const useState = <T>(key: string, init?: () => T): {value: T} => {
  const defaultValue: T = init ? init() : (undefined as unknown as T);
  return ref<T>(defaultValue) as {value: T};
};

export const defineNuxtRouteMiddleware = (fn: Function): Function => fn;

export const useRuntimeConfig = (): Record<string, unknown> => ({});

export const useNuxtApp = (): Record<string, unknown> => ({});
