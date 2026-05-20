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

import {StorageManager, TokenConstants} from '@thunderid/javascript';

/**
 * Helper that manages automatic access-token refresh scheduling via `setTimeout`.
 *
 * @typeParam T - Browser client config type.
 */
class SPAHelper<T> {
  private _storageManager: StorageManager<T>;
  private _isTokenRefreshLoading = false;

  /**
   * @param storageManager - The storage manager instance used to read config and session data.
   */
  public constructor(storageManager: StorageManager<T>) {
    this._storageManager = storageManager;
  }

  /**
   * Schedules an automatic access-token refresh if `periodicTokenRefresh` is enabled in config.
   * No-op if the feature is disabled or there is no refresh token.
   *
   * @param refreshAccessToken - Async callback that performs the refresh.
   */
  public async refreshAccessTokenAutomatically(refreshAccessToken: () => Promise<any>): Promise<void> {
    const config = await this._storageManager.getConfigData();
    const shouldRefreshAutomatically: boolean = (config as any)?.periodicTokenRefresh ?? false;

    if (!shouldRefreshAutomatically) {
      return;
    }

    const sessionData = await this._storageManager.getSessionData();

    if (sessionData?.refresh_token) {
      if (sessionData.created_at == null || sessionData.expires_in == null) {
        return;
      }

      const TOKEN_REFRESH_BUFFER_MS = 10_000;
      const expiryTime = Number(sessionData.expires_in) * 1000;
      const absoluteExpiryTime: number = sessionData.created_at + expiryTime;
      const timeUntilRefresh = absoluteExpiryTime - Date.now() - TOKEN_REFRESH_BUFFER_MS;

      if (timeUntilRefresh <= 0) {
        if (this._isTokenRefreshLoading) return;

        this._isTokenRefreshLoading = true;
        try {
          await refreshAccessToken();
        } finally {
          this._isTokenRefreshLoading = false;
        }
        return;
      }

      const timer = setTimeout(async () => {
        if (this._isTokenRefreshLoading) return;

        this._isTokenRefreshLoading = true;
        try {
          await refreshAccessToken();
        } finally {
          this._isTokenRefreshLoading = false;
        }
      }, timeUntilRefresh);

      await this._storageManager.setTemporaryDataParameter(
        TokenConstants.Storage.StorageKeys.REFRESH_TOKEN_TIMER,
        JSON.stringify(timer),
      );
    }
  }

  /**
   * Returns the current refresh timer ID from storage, or `-1` if none is set.
   */
  public async getRefreshTimeoutTimer(): Promise<number> {
    const raw = await this._storageManager.getTemporaryDataParameter(
      TokenConstants.Storage.StorageKeys.REFRESH_TOKEN_TIMER,
    );

    if (raw) {
      return JSON.parse(raw as string);
    }

    return -1;
  }

  /**
   * Clears the automatic-refresh timer.
   *
   * @param timer - Timer ID to clear. If omitted, the stored timer ID is used.
   */
  public async clearRefreshTokenTimeout(timer?: number): Promise<void> {
    if (timer) {
      clearTimeout(timer);

      return;
    }

    const refreshTimer: number = await this.getRefreshTimeoutTimer();

    if (refreshTimer !== -1) {
      clearTimeout(refreshTimer);
    }
  }
}

export default SPAHelper;
