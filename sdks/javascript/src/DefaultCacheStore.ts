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

export class DefaultCacheStore implements Storage {
  private cache: Map<string, string>;

  constructor() {
    this.cache = new Map<string, string>();
  }

  public get length(): number {
    return this.cache.size;
  }

  public getItem(key: string): string | null {
    return this.cache.get(key) ?? null;
  }

  public setItem(key: string, value: string): void {
    this.cache.set(key, value);
  }

  public removeItem(key: string): void {
    this.cache.delete(key);
  }

  public clear(): void {
    this.cache.clear();
  }

  public key(index: number): string | null {
    const keys: string[] = Array.from(this.cache.keys());
    return keys[index] ?? null;
  }

  public async setData(key: string, value: string): Promise<void> {
    this.cache.set(key, value);
  }

  public async getData(key: string): Promise<string> {
    return this.cache.get(key) ?? '{}';
  }

  public async removeData(key: string): Promise<void> {
    this.cache.delete(key);
  }
}
