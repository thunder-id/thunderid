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

import {Storage} from '@thunderid/javascript';

/**
 * `Storage` implementation backed by an in-memory `Map`. Data does not survive page reloads.
 */
class MemoryStore implements Storage {
  private _data: Map<string, string>;

  public constructor() {
    this._data = new Map();
  }

  public async setData(key: string, value: string): Promise<void> {
    this._data.set(key, value);
  }

  public async getData(key: string): Promise<string> {
    return this._data?.get(key) ?? '{}';
  }

  public async removeData(key: string): Promise<void> {
    this._data.delete(key);
  }
}

export default MemoryStore;
