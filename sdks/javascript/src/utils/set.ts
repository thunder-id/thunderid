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
 * Sets the value at path of object. If a portion of path doesn't exist,
 * it's created. Arrays are created for missing index properties while
 * objects are created for all other missing properties.
 * Similar to Lodash's set() function
 *
 * @param object - The object to modify
 * @param path - The path of the property to set
 * @param value - The value to set
 * @returns The object
 */
const set = (object: any, path: string | string[], value: any): any => {
  if (!object || !path) return object;

  const pathArray: string[] = Array.isArray(path) ? path : path.split('.');
  const lastIndex: number = pathArray.length - 1;

  pathArray.reduce((current: any, key: string, index: number) => {
    if (index === lastIndex) {
      current[key] = value;
    } else if (!(key in current) || typeof current[key] !== 'object' || current[key] === null) {
      // Create array if next key is numeric, otherwise create object
      const nextKey: string = pathArray[index + 1];

      current[key] = /^\d+$/.test(nextKey) ? [] : {};
    }
    return current[key];
  }, object);

  return object;
};

export default set;
