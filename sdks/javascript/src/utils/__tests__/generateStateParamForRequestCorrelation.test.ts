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
import generateStateParamForRequestCorrelation from '../generateStateParamForRequestCorrelation';

describe('generateStateParamForRequestCorrelation', (): void => {
  it('should generate state parameter with custom state', (): void => {
    const pkceKey = `${PKCEConstants.Storage.StorageKeys.CODE_VERIFIER}${PKCEConstants.Storage.StorageKeys.SEPARATOR}1`;
    const customState = 'myState';

    expect(generateStateParamForRequestCorrelation(pkceKey, customState)).toBe('myState_request_1');
  });

  it('should generate state parameter without custom state', (): void => {
    const pkceKey = `${PKCEConstants.Storage.StorageKeys.CODE_VERIFIER}${PKCEConstants.Storage.StorageKeys.SEPARATOR}2`;

    expect(generateStateParamForRequestCorrelation(pkceKey)).toBe('request_2');
  });

  it('should handle different index values', (): void => {
    const pkceKey = `${PKCEConstants.Storage.StorageKeys.CODE_VERIFIER}${PKCEConstants.Storage.StorageKeys.SEPARATOR}999`;

    expect(generateStateParamForRequestCorrelation(pkceKey)).toBe('request_999');
  });

  it('should combine custom state with request index correctly', (): void => {
    const pkceKey = `${PKCEConstants.Storage.StorageKeys.CODE_VERIFIER}${PKCEConstants.Storage.StorageKeys.SEPARATOR}5`;
    const customState = 'complex_state_123';

    expect(generateStateParamForRequestCorrelation(pkceKey, customState)).toBe('complex_state_123_request_5');
  });

  it('should use the last separator segment as index if prefix contains the separator', () => {
    const sep: string = PKCEConstants.Storage.StorageKeys.SEPARATOR;
    const base = `${PKCEConstants.Storage.StorageKeys.CODE_VERIFIER}`;
    const pkceKey = `${base}${sep}12`;
    expect(generateStateParamForRequestCorrelation(pkceKey)).toBe('request_12');
  });

  it('should handle non-numeric indices', () => {
    const pkceKey = `${PKCEConstants.Storage.StorageKeys.CODE_VERIFIER}`;
    expect(generateStateParamForRequestCorrelation(pkceKey)).toBe('request_NaN');
  });

  it('should handle zero-padded indices consistently', () => {
    const pkceKey = `${PKCEConstants.Storage.StorageKeys.CODE_VERIFIER}${PKCEConstants.Storage.StorageKeys.SEPARATOR}007`;
    expect(generateStateParamForRequestCorrelation(pkceKey)).toBe('request_7');
  });
});
