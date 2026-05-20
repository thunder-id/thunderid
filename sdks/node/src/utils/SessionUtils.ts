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

import {SessionData} from '@thunderid/javascript';
import {validate as uuidValidate, version as uuidVersion, v4 as uuidv4} from 'uuid';

const UUID_VERSION = 4;

/**
 * Utility class for session validation and UUID management.
 */
class SessionUtils {
  private constructor() {}

  /**
   * Generates a new UUID v4 string.
   *
   * @returns A new UUID string.
   */
  public static createUUID(): string {
    return uuidv4();
  }

  /**
   * Returns `true` if the given string is a valid UUID v4.
   *
   * @param uuid - The UUID string to validate.
   */
  public static validateUUID(uuid: string): Promise<boolean> {
    if (uuidValidate(uuid) && uuidVersion(uuid) === UUID_VERSION) {
      return Promise.resolve(true);
    }
    return Promise.resolve(false);
  }

  /**
   * Returns `true` if the session token is still within its validity window.
   *
   * @param sessionData - The session data to check.
   */
  public static validateSession(sessionData: SessionData): Promise<boolean> {
    const currentTime: number = Date.now();
    const expiryTimeStamp: number = sessionData.created_at + parseInt(sessionData.expires_in, 10) * 60 * 1000;
    return Promise.resolve(currentTime < expiryTimeStamp);
  }
}

export default SessionUtils;
