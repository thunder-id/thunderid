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

import {Platform} from '@thunderid/browser';
import {FC} from 'react';
import BaseRecoveryV1, {BaseRecoveryProps as BaseRecoveryV1Props} from './v1/BaseRecovery';
import BaseRecoveryV2, {BaseRecoveryProps as BaseRecoveryV2Props} from './v2/BaseRecovery';
import useThunderID from '../../../../contexts/ThunderID/useThunderID';

export type BaseRecoveryProps = BaseRecoveryV1Props | BaseRecoveryV2Props;

const BaseRecovery: FC<BaseRecoveryProps> = (props: BaseRecoveryProps) => {
  const {platform} = useThunderID();

  if (platform === Platform.ThunderID) {
    return <BaseRecoveryV2 {...(props as BaseRecoveryV2Props)} />;
  }

  return <BaseRecoveryV1 {...(props as BaseRecoveryV1Props)} />;
};

export default BaseRecovery;
