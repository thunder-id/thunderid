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

/**
 * Attempts to extract a human-readable message from a structured API error response body.
 *
 * The backend returns errors in the following form:
 * {"code":"...","message":{"key":"...","defaultValue":"..."},"description":{"key":"...","defaultValue":"..."}}
 *
 * Returns `description.defaultValue` if present, then `message.defaultValue`, and falls back
 * to the raw `errorText` when the response is not a recognised structured error.
 */
const parseApiErrorMessage = (errorText: string): string => {
  try {
    const parsed: Record<string, unknown> = JSON.parse(errorText) as Record<string, unknown>;
    const description: {defaultValue?: string} | undefined = parsed['description'] as
      | {defaultValue?: string}
      | undefined;
    const message: {defaultValue?: string} | undefined = parsed['message'] as {defaultValue?: string} | undefined;
    if (description?.defaultValue) return description.defaultValue;
    if (message?.defaultValue) return message.defaultValue;
  } catch {
    // not JSON — fall through to raw text
  }
  return errorText;
};

export default parseApiErrorMessage;
