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

import {describe, expect, it} from 'vitest';
import PKCEConstants from '../../constants/PKCEConstants';
import extractPkceStorageKeyFromState from '../extractPkceStorageKeyFromState';

describe('extractPkceStorageKeyFromState', (): void => {
  it('should extract PKCE key from state parameter', (): void => {
    const state = 'request_1';
    const expectedKey = `${PKCEConstants.Storage.StorageKeys.CODE_VERIFIER}${PKCEConstants.Storage.StorageKeys.SEPARATOR}1`;

    expect(extractPkceStorageKeyFromState(state)).toBe(expectedKey);
  });

  it('should handle state with prefix', (): void => {
    const state = 'myState_request_2';
    const expectedKey = `${PKCEConstants.Storage.StorageKeys.CODE_VERIFIER}${PKCEConstants.Storage.StorageKeys.SEPARATOR}2`;

    expect(extractPkceStorageKeyFromState(state)).toBe(expectedKey);
  });

  it('should extract index from complex state string', (): void => {
    const state = 'custom_state_with_request_3';
    const expectedKey = `${PKCEConstants.Storage.StorageKeys.CODE_VERIFIER}${PKCEConstants.Storage.StorageKeys.SEPARATOR}3`;

    expect(extractPkceStorageKeyFromState(state)).toBe(expectedKey);
  });

  it('should return ...NaN when "request_" is missing', () => {
    const key: string = extractPkceStorageKeyFromState('state_without_marker');
    expect(key).toBe(
      `${PKCEConstants.Storage.StorageKeys.CODE_VERIFIER}${PKCEConstants.Storage.StorageKeys.SEPARATOR}NaN`,
    );
  });

  it('should return ...NaN for empty state', () => {
    const key: string = extractPkceStorageKeyFromState('');
    expect(key.endsWith('NaN')).toBe(true);
  });

  it('should parse until non-digit characters after "request_"', () => {
    const key: string = extractPkceStorageKeyFromState('request_abc');
    expect(key.endsWith('NaN')).toBe(true);
  });

  it('should handle extra suffix after the number', () => {
    const key: string = extractPkceStorageKeyFromState('request_42_extra');
    expect(key.endsWith('42')).toBe(true);
  });

  it('should use the first "request_" occurrence if multiple exist', () => {
    const key: string = extractPkceStorageKeyFromState('foo_request_7_bar_request_9');
    expect(key.endsWith('7')).toBe(true);
  });
});
