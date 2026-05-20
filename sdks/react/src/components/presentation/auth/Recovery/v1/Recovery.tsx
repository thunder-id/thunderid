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

import {EmbeddedFlowExecuteRequestPayload, EmbeddedFlowExecuteResponse, EmbeddedFlowType} from '@thunderid/browser';
import {FC, PropsWithChildren, ReactElement} from 'react';
import BaseRecovery, {BaseRecoveryProps} from './BaseRecovery';
import useThunderID from '../../../../../contexts/ThunderID/useThunderID';

export type RecoveryProps = PropsWithChildren<BaseRecoveryProps>;

/**
 * Recovery component for ThunderID V1 that provides an embedded account/password recovery flow.
 */
const Recovery: FC<RecoveryProps> = ({
  className,
  size = 'medium',
  afterRecoveryUrl,
  onError,
  onComplete,
  children,
  ...rest
}: RecoveryProps): ReactElement => {
  const {recover, isInitialized} = useThunderID();

  const handleInitialize = async (
    payload?: EmbeddedFlowExecuteRequestPayload,
  ): Promise<EmbeddedFlowExecuteResponse> => {
    const initialPayload: any = payload || {flowType: EmbeddedFlowType.Recovery};
    return (await recover(initialPayload)) as EmbeddedFlowExecuteResponse;
  };

  const handleOnSubmit = async (payload: EmbeddedFlowExecuteRequestPayload): Promise<EmbeddedFlowExecuteResponse> =>
    (await recover(payload)) as EmbeddedFlowExecuteResponse;

  const handleComplete = (response: EmbeddedFlowExecuteResponse): void => {
    onComplete?.(response);

    if (afterRecoveryUrl) {
      window.location.href = afterRecoveryUrl;
    }
  };

  return (
    <BaseRecovery
      afterRecoveryUrl={afterRecoveryUrl}
      onInitialize={handleInitialize}
      onSubmit={handleOnSubmit}
      onError={onError}
      onComplete={handleComplete}
      className={className}
      size={size}
      isInitialized={isInitialized}
      showLogo={true}
      showTitle={false}
      showSubtitle={false}
      children={children}
      {...rest}
    />
  );
};

export default Recovery;
