/**
 * Copyright (c) 2020, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
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

import {AuthClientConfig} from './models/config';
import {OIDCDiscoveryApiResponse} from './models/oidc-discovery';
import {SessionData} from './models/session';
import {Stores, Storage, TemporaryStore, HybridStore, TemporaryStoreValue} from './models/store';
import logger from './utils/logger';

type PartialData<T> = Partial<
  AuthClientConfig<T> | OIDCDiscoveryApiResponse | SessionData | TemporaryStore | HybridStore
>;

export const THUNDERID_SESSION_ACTIVE = 'thunderid-session-active';

class StorageManager<T> {
  protected id: string;

  protected store: Storage;

  public constructor(instanceID: string, store: Storage) {
    this.id = instanceID;
    this.store = store;
  }

  protected async setDataInBulk(key: string, data: PartialData<T>): Promise<void> {
    const existingDataJSON: string = (await this.store.getData(key)) ?? null;
    const existingData: PartialData<T> = existingDataJSON && JSON.parse(existingDataJSON);

    const dataToBeSaved: PartialData<T> = {...existingData, ...data};
    const dataToBeSavedJSON: string = JSON.stringify(dataToBeSaved);

    await this.store.setData(key, dataToBeSavedJSON);
  }

  protected async setValue(
    key: string,
    attribute:
      | keyof AuthClientConfig<T>
      | keyof OIDCDiscoveryApiResponse
      | keyof SessionData
      | keyof TemporaryStore
      | keyof HybridStore,
    value: TemporaryStoreValue,
  ): Promise<void> {
    const existingDataJSON: string = (await this.store.getData(key)) ?? null;
    const existingData: PartialData<T> = existingDataJSON && JSON.parse(existingDataJSON);

    const dataToBeSaved: PartialData<T> = {...existingData, [attribute]: value};
    const dataToBeSavedJSON: string = JSON.stringify(dataToBeSaved);

    await this.store.setData(key, dataToBeSavedJSON);
  }

  protected async removeValue(
    key: string,
    attribute:
      | keyof AuthClientConfig<T>
      | keyof OIDCDiscoveryApiResponse
      | keyof SessionData
      | keyof TemporaryStore
      | keyof HybridStore,
  ): Promise<void> {
    const existingDataJSON: string = (await this.store.getData(key)) ?? null;
    const existingData: PartialData<T> = existingDataJSON && JSON.parse(existingDataJSON);

    const dataToBeSaved: PartialData<T> = {...existingData};

    delete dataToBeSaved[attribute];

    const dataToBeSavedJSON: string = JSON.stringify(dataToBeSaved);

    await this.store.setData(key, dataToBeSavedJSON);
  }

  protected resolveKey(store: Stores | string, userId?: string, instanceId?: string): string {
    if (userId && instanceId) {
      return `${store}-${instanceId}-${userId}`;
    }
    if (userId) {
      return `${store}-${this.id}-${userId}`;
    }
    if (instanceId) {
      return `${store}-${instanceId}`;
    }
    return `${store}-${this.id}`;
  }

  protected static isLocalStorageAvailable(): boolean {
    try {
      const testValue = '__THUNDERID_AUTH_CORE_LOCAL_STORAGE_TEST__';

      localStorage.setItem(testValue, testValue);
      localStorage.removeItem(testValue);

      return true;
    } catch (error) {
      return false;
    }
  }

  public async setConfigData(config: Partial<AuthClientConfig<T>>): Promise<void> {
    await this.setDataInBulk(this.resolveKey(Stores.ConfigData), config);
  }

  public async setOIDCProviderMetaData(oidcProviderMetaData: Partial<OIDCDiscoveryApiResponse>): Promise<void> {
    this.setDataInBulk(this.resolveKey(Stores.OIDCProviderMetaData), oidcProviderMetaData);
  }

  public async setTemporaryData(temporaryData: Partial<TemporaryStore>, userId?: string): Promise<void> {
    await this.setDataInBulk(this.resolveKey(Stores.TemporaryData, userId), temporaryData);
  }

  public async setHybridData(hybridData: Partial<HybridStore>, userId?: string): Promise<void> {
    const resolvedKey = this.resolveKey(Stores.HybridData, userId);

    if (StorageManager.isLocalStorageAvailable()) {
      const existingDataJSON = localStorage.getItem(resolvedKey);
      const existingData: Partial<HybridStore> = existingDataJSON ? JSON.parse(existingDataJSON) : {};
      const dataToBeSaved = {...existingData, ...hybridData};
      localStorage.setItem(resolvedKey, JSON.stringify(dataToBeSaved));
    } else {
      await this.setDataInBulk(resolvedKey, hybridData);
    }
  }

  public async setSessionData(sessionData: Partial<SessionData>, userId?: string): Promise<void> {
    this.setDataInBulk(this.resolveKey(Stores.SessionData, userId), sessionData);
  }

  public async setCustomData<K>(key: string, customData: Partial<K>, userId?: string): Promise<void> {
    this.setDataInBulk(this.resolveKey(key, userId), customData);
  }

  public async getConfigData(userId?: string): Promise<AuthClientConfig<T>> {
    return JSON.parse((await this.store.getData(this.resolveKey(Stores.ConfigData, userId))) ?? null);
  }

  public async loadOpenIDProviderConfiguration(): Promise<OIDCDiscoveryApiResponse> {
    return JSON.parse((await this.store.getData(this.resolveKey(Stores.OIDCProviderMetaData))) ?? null);
  }

  public async getTemporaryData(userId?: string): Promise<TemporaryStore> {
    const data: string = await this.store.getData(this.resolveKey(Stores.TemporaryData, userId));
    if (data) {
      try {
        return JSON.parse(data);
      } catch (error) {
        logger.error(`StorageManager: Failed to parse temporary data for key ${this.resolveKey(Stores.TemporaryData, userId)}`, error);
        return {};
      }
    }
    return {};
  }

  public async getHybridData(userId?: string): Promise<HybridStore> {
    const resolvedKey = this.resolveKey(Stores.HybridData, userId);

    if (StorageManager.isLocalStorageAvailable()) {
      const storeDataJSON = localStorage.getItem(resolvedKey);
      if (storeDataJSON) {
        try {
          return JSON.parse(storeDataJSON);
        } catch (error) {
          logger.error(`StorageManager: Failed to parse hybrid data from local storage for key ${resolvedKey}`, error);
          return {};
        }
      }
      return {};
    }

    const storeDataJSON: string | null = (await this.store.getData(resolvedKey)) ?? null;
    if (storeDataJSON) {
      try {
        return JSON.parse(storeDataJSON);
      } catch (error) {
        logger.error(`StorageManager: Failed to parse hybrid data from store for key ${resolvedKey}`, error);
        return {};
      }
    }
    return {};
  }

  public async getPersistedData(userId?: string): Promise<TemporaryStore> {
    return JSON.parse((await this.store.getData(this.resolveKey(Stores.PersistedData, userId))) ?? null);
  }

  public async setPersistedData(persistedData: Partial<TemporaryStore>, userId?: string): Promise<void> {
    this.setDataInBulk(this.resolveKey(Stores.PersistedData, userId), persistedData);
  }

  public async getSessionData(userId?: string, instanceId?: string): Promise<SessionData> {
    return JSON.parse((await this.store.getData(this.resolveKey(Stores.SessionData, userId, instanceId))) ?? null);
  }

  public async getCustomData<K>(key: string, userId?: string): Promise<K> {
    return JSON.parse((await this.store.getData(this.resolveKey(key, userId))) ?? null);
  }

  public setSessionStatus(status: string): void {
    // Using local storage to store the session status as it is required to be available across tabs.
    if (StorageManager.isLocalStorageAvailable()) {
      localStorage.setItem(`${THUNDERID_SESSION_ACTIVE}`, status);
    }
  }

  public getSessionStatus(): string {
    return StorageManager.isLocalStorageAvailable() ? (localStorage.getItem(`${THUNDERID_SESSION_ACTIVE}`) ?? '') : '';
  }

  public removeSessionStatus(): void {
    if (StorageManager.isLocalStorageAvailable()) {
      localStorage.removeItem(`${THUNDERID_SESSION_ACTIVE}`);
    }
  }

  public async removeConfigData(): Promise<void> {
    await this.store.removeData(this.resolveKey(Stores.ConfigData));
  }

  public async removeOIDCProviderMetaData(): Promise<void> {
    await this.store.removeData(this.resolveKey(Stores.OIDCProviderMetaData));
  }

  public async removeTemporaryData(userId?: string): Promise<void> {
    await this.store.removeData(this.resolveKey(Stores.TemporaryData, userId));
  }

  public async removeHybridData(userId?: string): Promise<void> {
    const resolvedKey = this.resolveKey(Stores.HybridData, userId);

    if (StorageManager.isLocalStorageAvailable()) {
      localStorage.removeItem(resolvedKey);
    } else {
      await this.store.removeData(resolvedKey);
    }
  }

  public async removeSessionData(userId?: string): Promise<void> {
    await this.store.removeData(this.resolveKey(Stores.SessionData, userId));
  }

  public async getConfigDataParameter(key: keyof AuthClientConfig<T>): Promise<TemporaryStoreValue> {
    const data: string = await this.store.getData(this.resolveKey(Stores.ConfigData));

    return data && JSON.parse(data)[key];
  }

  public async getOIDCProviderMetaDataParameter(key: keyof OIDCDiscoveryApiResponse): Promise<TemporaryStoreValue> {
    const data: string = await this.store.getData(this.resolveKey(Stores.OIDCProviderMetaData));

    return data && JSON.parse(data)[key];
  }

  public async getTemporaryDataParameter(key: keyof TemporaryStore, userId?: string): Promise<TemporaryStoreValue> {
    const data: string = await this.store.getData(this.resolveKey(Stores.TemporaryData, userId));
    return data && JSON.parse(data)[key];
  }

  public async getHybridDataParameter(key: keyof HybridStore, userId?: string): Promise<TemporaryStoreValue> {
    const resolvedKey = this.resolveKey(Stores.HybridData, userId);

    if (StorageManager.isLocalStorageAvailable()) {
      const existingDataJSON = localStorage.getItem(resolvedKey);
      const existingData = existingDataJSON ? JSON.parse(existingDataJSON) : {};
      return existingData[key];
    }

    const data: string = await this.store.getData(resolvedKey);
    return data && JSON.parse(data)[key];
  }

  public async getSessionDataParameter(key: keyof SessionData, userId?: string): Promise<TemporaryStoreValue> {
    const data: string = await this.store.getData(this.resolveKey(Stores.SessionData, userId));

    return data && JSON.parse(data)[key];
  }

  public async setConfigDataParameter(key: keyof AuthClientConfig<T>, value: TemporaryStoreValue): Promise<void> {
    await this.setValue(this.resolveKey(Stores.ConfigData), key, value);
  }

  public async setOIDCProviderMetaDataParameter(
    key: keyof OIDCDiscoveryApiResponse,
    value: TemporaryStoreValue,
  ): Promise<void> {
    await this.setValue(this.resolveKey(Stores.OIDCProviderMetaData), key, value);
  }

  public async setTemporaryDataParameter(
    key: keyof TemporaryStore,
    value: TemporaryStoreValue,
    userId?: string,
  ): Promise<void> {
    await this.setValue(this.resolveKey(Stores.TemporaryData, userId), key, value);
  }

  public async setHybridDataParameter(
    key: keyof HybridStore,
    value: TemporaryStoreValue,
    userId?: string,
  ): Promise<void> {
    const resolvedKey = this.resolveKey(Stores.HybridData, userId);

    if (StorageManager.isLocalStorageAvailable()) {
      const existingDataJSON = localStorage.getItem(resolvedKey);
      const existingData = existingDataJSON ? JSON.parse(existingDataJSON) : {};
      const dataToBeSaved = {...existingData, [key]: value};
      localStorage.setItem(resolvedKey, JSON.stringify(dataToBeSaved));
    } else {
      await this.setValue(resolvedKey, key, value);
    }
  }

  public async setSessionDataParameter(
    key: keyof SessionData,
    value: TemporaryStoreValue,
    userId?: string,
  ): Promise<void> {
    await this.setValue(this.resolveKey(Stores.SessionData, userId), key, value);
  }

  public async removeConfigDataParameter(key: keyof AuthClientConfig<T>): Promise<void> {
    await this.removeValue(this.resolveKey(Stores.ConfigData), key);
  }

  public async removeOIDCProviderMetaDataParameter(key: keyof OIDCDiscoveryApiResponse): Promise<void> {
    await this.removeValue(this.resolveKey(Stores.OIDCProviderMetaData), key);
  }

  public async removeTemporaryDataParameter(key: keyof TemporaryStore, userId?: string): Promise<void> {
    await this.removeValue(this.resolveKey(Stores.TemporaryData, userId), key);
  }

  public async removeHybridDataParameter(key: keyof HybridStore, userId?: string): Promise<void> {
    const resolvedKey = this.resolveKey(Stores.HybridData, userId);

    if (StorageManager.isLocalStorageAvailable()) {
      const existingDataJSON = localStorage.getItem(resolvedKey);
      const existingData = existingDataJSON ? JSON.parse(existingDataJSON) : {};
      delete existingData[key];
      localStorage.setItem(resolvedKey, JSON.stringify(existingData));
    } else {
      await this.removeValue(resolvedKey, key);
    }
  }

  public async removeSessionDataParameter(key: keyof SessionData, userId?: string): Promise<void> {
    await this.removeValue(this.resolveKey(Stores.SessionData, userId), key);
  }
}

export default StorageManager;
