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

import ThunderIDRuntimeError from '../errors/ThunderIDRuntimeError';
import {
  EmbeddedSignInFlowAuthenticatorExtendedParamType,
  EmbeddedSignInFlowAuthenticatorParamType,
} from '../models/embedded-signin-flow';
import {FieldType} from '../models/field';

const resolveFieldType = (field: any): FieldType => {
  if (field.type === EmbeddedSignInFlowAuthenticatorParamType.String) {
    // Check if there's a `param` property and if it matches a known type.
    if (field.param === EmbeddedSignInFlowAuthenticatorExtendedParamType.Otp) {
      return FieldType.Otp;
    }
    if (field?.confidential) {
      return FieldType.Password;
    }

    return FieldType.Text;
  }

  throw new ThunderIDRuntimeError(
    `Field type is not supported: ${field.type}`,
    'resolveFieldType-Invalid-001',
    'javascript',
    'The provided field type is not supported. Please check the field configuration.',
  );
};

export default resolveFieldType;
