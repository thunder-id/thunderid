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

import {Platform} from '@thunderid/browser';
import {FC} from 'react';
import BaseSignInV1, {BaseSignInProps as BaseSignInV1Props} from './v1/BaseSignIn';
import BaseSignInV2, {BaseSignInProps as BaseSignInV2Props} from './v2/BaseSignIn';
import useThunderID from '../../../../contexts/ThunderID/useThunderID';

/**
 * Props for the BaseSignIn component.
 * Extends BaseSignInV1Props & BaseSignInV2Props for full compatibility with both React BaseSignIn components.
 */
export type BaseSignInProps = BaseSignInV1Props | BaseSignInV2Props;

const BaseSignIn: FC<BaseSignInProps> = (props: BaseSignInProps) => {
  const {platform} = useThunderID();

  if (platform === Platform.ThunderID) {
    return <BaseSignInV2 {...(props as BaseSignInV2Props)} />;
  }

  return <BaseSignInV1 {...(props as BaseSignInV1Props)} />;
};

export default BaseSignIn;
