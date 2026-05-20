/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
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

import PKCEConstants from '../../constants/PKCEConstants';
import {TemporaryStore} from '../../models/store';
import generatePkceStorageKey from '../generatePkceStorageKey';

describe('generatePkceStorageKey', (): void => {
  it('should generate PKCE key with index 0 for empty temporary data', (): void => {
    const tempData: TemporaryStore = {};
    const expectedKey = `${PKCEConstants.Storage.StorageKeys.CODE_VERIFIER}${PKCEConstants.Storage.StorageKeys.SEPARATOR}0`;

    expect(generatePkceStorageKey(tempData)).toBe(expectedKey);
  });

  it('should generate PKCE key with incremented index for existing PKCE keys', (): void => {
    const tempData: TemporaryStore = {
      [`${PKCEConstants.Storage.StorageKeys.CODE_VERIFIER}${PKCEConstants.Storage.StorageKeys.SEPARATOR}1`]: 'value1',
      [`${PKCEConstants.Storage.StorageKeys.CODE_VERIFIER}${PKCEConstants.Storage.StorageKeys.SEPARATOR}2`]: 'value2',
    };
    const expectedKey = `${PKCEConstants.Storage.StorageKeys.CODE_VERIFIER}${PKCEConstants.Storage.StorageKeys.SEPARATOR}3`;

    expect(generatePkceStorageKey(tempData)).toBe(expectedKey);
  });

  it('should handle non-sequential PKCE keys', (): void => {
    const tempData: TemporaryStore = {
      [`${PKCEConstants.Storage.StorageKeys.CODE_VERIFIER}${PKCEConstants.Storage.StorageKeys.SEPARATOR}1`]: 'value1',
      [`${PKCEConstants.Storage.StorageKeys.CODE_VERIFIER}${PKCEConstants.Storage.StorageKeys.SEPARATOR}5`]: 'value5',
    };
    const expectedKey = `${PKCEConstants.Storage.StorageKeys.CODE_VERIFIER}${PKCEConstants.Storage.StorageKeys.SEPARATOR}6`;

    expect(generatePkceStorageKey(tempData)).toBe(expectedKey);
  });

  it('should ignore non-PKCE keys in temporary data', (): void => {
    const tempData: TemporaryStore = {
      [`${PKCEConstants.Storage.StorageKeys.CODE_VERIFIER}${PKCEConstants.Storage.StorageKeys.SEPARATOR}1`]: 'value1',
      'other-key': 'other-value',
    };
    const expectedKey = `${PKCEConstants.Storage.StorageKeys.CODE_VERIFIER}${PKCEConstants.Storage.StorageKeys.SEPARATOR}2`;

    expect(generatePkceStorageKey(tempData)).toBe(expectedKey);
  });
});
