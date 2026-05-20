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

/**
 * Gets the value at path of object. If the resolved value is undefined,
 * the defaultValue is returned in its place.
 * Similar to Lodash's get() function
 *
 * @param object - The object to query
 * @param path - The path of the property to get
 * @param defaultValue - The value returned for undefined resolved values
 * @returns The resolved value
 */
const get = (object: any, path: string | string[], defaultValue?: any): any => {
  if (!object || !path) return defaultValue;

  const pathArray: string[] = Array.isArray(path) ? path : path.split('.');

  const result: any = pathArray.reduce((current: any, key: string) => current?.[key], object);

  return result !== undefined ? result : defaultValue;
};

export default get;
