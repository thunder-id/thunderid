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
import {FC, PropsWithChildren, ReactElement, useCallback} from 'react';
import BaseRecovery, {BaseRecoveryProps} from './BaseRecovery';
import useThunderID from '../../../../../contexts/ThunderID/useThunderID';

export type RecoveryProps = PropsWithChildren<
  BaseRecoveryProps & {
    /**
     * URL query parameter name that carries the recovery token when the user lands via a recovery link.
     * When set and both `executionId` and this param are present in the URL, the component resumes the
     * existing flow instead of starting a new one.
     *
     * @example
     * // For a link like /recovery?executionId=xxx&recoveryToken=yyy
     * <Recovery tokenUrlParam="recoveryToken" />
     */
    tokenUrlParam?: string;
  }
>;

/**
 * Recovery component for ThunderIDV2 that provides an embedded account/password recovery flow.
 *
 * @example
 * ```tsx
 * // Default UI
 * <Recovery
 *   afterRecoveryUrl="/sign-in"
 *   onComplete={(response) => console.log('Recovery complete', response)}
 *   onError={(error) => console.error('Recovery failed', error)}
 * />
 *
 * // Custom UI with render props
 * <Recovery>
 *   {({ values, fieldErrors, handleInputChange, handleSubmit, isLoading, components }) => (
 *     <form onSubmit={(e) => { e.preventDefault(); handleSubmit(components[0], values); }}>
 *       ...
 *     </form>
 *   )}
 * </Recovery>
 * ```
 */
const Recovery: FC<RecoveryProps> = ({
  className,
  size = 'medium',
  afterRecoveryUrl,
  onError,
  onComplete,
  tokenUrlParam,
  children,
  ...rest
}: RecoveryProps): ReactElement => {
  const {recover, isInitialized, applicationId} = useThunderID();

  const handleInitialize: (payload?: EmbeddedFlowExecuteRequestPayload) => Promise<EmbeddedFlowExecuteResponse> =
    useCallback(
      async (payload?: EmbeddedFlowExecuteRequestPayload): Promise<EmbeddedFlowExecuteResponse> => {
        const urlParams: URLSearchParams = new URL(window.location.href).searchParams;
        const applicationIdFromUrl: string | null = urlParams.get('applicationId');
        const effectiveApplicationId: string | null = applicationId ?? applicationIdFromUrl;

        if (tokenUrlParam) {
          const executionId: string | null = urlParams.get('executionId');
          const tokenValue: string | null = urlParams.get(tokenUrlParam);

          if (executionId && tokenValue) {
            const resumePayload: any = {
              executionId,
              inputs: {[tokenUrlParam]: tokenValue},
              verbose: true,
            };
            return (await recover(resumePayload)) as EmbeddedFlowExecuteResponse;
          }
        }

        const initialPayload: any = payload || {
          flowType: EmbeddedFlowType.Recovery,
          ...(effectiveApplicationId && {applicationId: effectiveApplicationId}),
        };

        return (await recover(initialPayload)) as EmbeddedFlowExecuteResponse;
      },
      [applicationId, tokenUrlParam, recover],
    );

  const handleOnSubmit: (payload: EmbeddedFlowExecuteRequestPayload) => Promise<EmbeddedFlowExecuteResponse> =
    useCallback(
      async (payload: EmbeddedFlowExecuteRequestPayload): Promise<EmbeddedFlowExecuteResponse> =>
        (await recover(payload)) as EmbeddedFlowExecuteResponse,
      [recover],
    );

  const handleComplete: (response: EmbeddedFlowExecuteResponse) => void = useCallback(
    (response: EmbeddedFlowExecuteResponse): void => {
      onComplete?.(response);

      if (afterRecoveryUrl) {
        window.location.href = afterRecoveryUrl;
      }
    },
    [onComplete, afterRecoveryUrl],
  );

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
      showTitle={true}
      showSubtitle={true}
      children={children}
      {...rest}
    />
  );
};

export default Recovery;
