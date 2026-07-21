/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

export interface AppConfig {
  baseUrl: string;
  notificationSenderId?: string;
  directAuthSecret?: string;
}

let config: AppConfig | null = null;

export async function loadConfig(): Promise<AppConfig> {
  if (config) {
    return config;
  }

  const response = await fetch("/config.json");
  if (!response.ok) {
    throw new Error("Failed to load config");
  }

  config = await response.json();
  return config!;
}

export function getConfig(): AppConfig {
  if (!config) {
    throw new Error("Config not loaded. Call loadConfig() first.");
  }
  return config;
}

// getDirectAuthHeaders returns the header carrying the Direct Auth Secret required by the direct
// authentication APIs (/auth/**). The server is secure by default, so these endpoints reject
// requests without a matching Direct-Auth-Secret header.
export function getDirectAuthHeaders(): Record<string, string> {
  const { directAuthSecret } = getConfig();
  return directAuthSecret ? { "Direct-Auth-Secret": directAuthSecret } : {};
}
