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

import {describe, it, expect} from 'vitest';
import ThunderIDRuntimeError from '../../errors/ThunderIDRuntimeError';
import {
  EmbeddedSignInFlowAuthenticatorParamType,
  EmbeddedSignInFlowAuthenticatorExtendedParamType,
} from '../../models/embedded-signin-flow';
import {FieldType} from '../../models/field';
import resolveFieldType from '../resolveFieldType';

describe('resolveFieldType', () => {
  it('should return FieldType.Text for STRING fields without param/confidential', () => {
    const field: {type: EmbeddedSignInFlowAuthenticatorParamType} = {
      type: EmbeddedSignInFlowAuthenticatorParamType.String,
    };
    expect(resolveFieldType(field)).toBe(FieldType.Text);
  });

  it('should return FieldType.Otp when STRING field has param = OTP (wins over confidential)', () => {
    const field: {
      confidential: boolean;
      param: EmbeddedSignInFlowAuthenticatorExtendedParamType;
      type: EmbeddedSignInFlowAuthenticatorParamType;
    } = {
      confidential: true,
      param: EmbeddedSignInFlowAuthenticatorExtendedParamType.Otp,
      type: EmbeddedSignInFlowAuthenticatorParamType.String,
    };
    expect(resolveFieldType(field)).toBe(FieldType.Otp);
  });

  it('should return FieldType.Password for STRING fields with confidential=true (and non-OTP param)', () => {
    const field: {confidential: boolean; param: string; type: EmbeddedSignInFlowAuthenticatorParamType} = {
      confidential: true,
      param: 'username',
      type: EmbeddedSignInFlowAuthenticatorParamType.String,
    };
    expect(resolveFieldType(field)).toBe(FieldType.Password);
  });

  it('should return FieldType.Text for STRING fields with non-OTP param and confidential=false', () => {
    const field: {confidential: boolean; param: string; type: EmbeddedSignInFlowAuthenticatorParamType} = {
      confidential: false,
      param: 'username',
      type: EmbeddedSignInFlowAuthenticatorParamType.String,
    };
    expect(resolveFieldType(field)).toBe(FieldType.Text);
  });

  it('should throw ThunderIDRuntimeError for non-STRING types', () => {
    const field: {type: string} = {type: 'number'};
    expect(() => resolveFieldType(field as any)).toThrow(ThunderIDRuntimeError);
    expect(() => resolveFieldType(field as any)).toThrow('Field type is not supported');
  });

  it('should throw a TypeError when field is undefined', () => {
    expect(() => resolveFieldType(undefined as any)).toThrow(TypeError);
  });
});
