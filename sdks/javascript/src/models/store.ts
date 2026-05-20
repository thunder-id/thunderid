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

import {OIDCEndpoints} from './oidc-endpoints';

/**
 * Interface representing a key-value storage mechanism.
 * Implementations can include various storage backends like browser storage,
 * memory cache, or distributed caches like Redis or Memcached.
 */
export interface Storage {
  /**
   * Retrieves a value by its key.
   *
   * @param key - The key of the value to retrieve
   * @returns A promise that resolves with the stored value
   * @throws Error if the key doesn't exist or retrieval fails
   */
  getData(key: string): Promise<string>;

  /**
   * Removes a value from the store.
   *
   * @param key - The key of the value to remove
   * @returns A promise that resolves when the value is successfully removed
   * @throws Error if removal fails or the key doesn't exist
   */
  removeData(key: string): Promise<void>;

  /**
   * Stores a value with the specified key.
   *
   * @param key - The unique identifier for the stored value
   * @param value - The string value to store
   * @returns A promise that resolves when the value is successfully stored
   * @throws Error if storage operation fails
   */
  setData(key: string, value: string): Promise<void>;
}

/**
 * Represents the possible value types that can be stored in the temporary data storage.
 */
export type TemporaryStoreValue = string | string[] | boolean | number | OIDCEndpoints;

/**
 * Represents a key-value store for temporary data storage.
 */
export type TemporaryStore = Record<string, TemporaryStoreValue>;

/**
 * Enum representing different types of data stores used in the application.
 */
export enum Stores {
  /**
   * Store for configuration data that defines the application's behavior and settings.
   */
  ConfigData = 'config_data',

  /**
   * Store for OpenID Connect provider metadata, including endpoints and configuration.
   */
  OIDCProviderMetaData = 'oidc_provider_meta_data',

  /**
   * Store for persisted data that needs to be retained across sessions and application restarts.
   */
  PersistedData = 'persisted_data',

  /**
   * Store for user session-related data like tokens and authentication state.
   */
  SessionData = 'session_data',

  /**
   * Store for temporary data that needs to persist only for a short duration.
   */
  TemporaryData = 'temporary_data',
}
