/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

import {
  EmbeddedSignUpFlowRequestV2,
  EmbeddedSignUpFlowResponseV2,
  EmbeddedSignUpFlowTypeV2,
  EmbeddedFlowType,
} from '@thunderid/browser';
import {FC, ReactElement, ReactNode} from 'react';
// eslint-disable-next-line import/no-named-as-default
import BaseSignUp, {BaseSignUpProps, BaseSignUpRenderProps} from './BaseSignUp';
import useThunderID from '../../../../../contexts/ThunderID/useThunderID';

/**
 * Render props function parameters (re-exported from BaseSignUp for convenience)
 */
export type SignUpRenderProps = BaseSignUpRenderProps;

/**
 * Props for the SignUp component.
 */
export type SignUpProps = BaseSignUpProps & {
  /**
   * Render props function for custom UI
   */
  children?: (props: SignUpRenderProps) => ReactNode;
};

/**
 * A styled SignUp component for ThunderIDV2 (AKA Thunder) platform that provides embedded sign-up flow with pre-built styling.
 * This component handles the API calls for sign-up and delegates UI logic to BaseSignUp.
 */
const SignUp: FC<SignUpProps> = ({
  className,
  size = 'medium',
  afterSignUpUrl,
  onError,
  onComplete,
  shouldRedirectAfterSignUp = true,
  children,
  ...rest
}: SignUpProps): ReactElement => {
  const {signUp, isInitialized, applicationId, scopes} = useThunderID();

  /**
   * Initialize the sign-up flow.
   */
  const handleInitialize = async (payload?: EmbeddedSignUpFlowRequestV2): Promise<EmbeddedSignUpFlowResponseV2> => {
    const urlParams: URLSearchParams = new URL(window.location.href).searchParams;
    const executionIdFromUrl: string = urlParams.get('executionId') || '';
    const applicationIdFromUrl: string = urlParams.get('applicationId') ?? '';

    const effectiveApplicationId: any = applicationId ?? applicationIdFromUrl;

    const challengeToken: string | undefined = (payload as any)?.challengeToken;

    let initialPayload: EmbeddedSignUpFlowRequestV2 | any;
    if (executionIdFromUrl) {
      initialPayload = {
        executionId: executionIdFromUrl,
        ...(challengeToken ? {challengeToken} : {}),
      };
    } else if (!payload || !('flowType' in payload)) {
      initialPayload = {
        ...(payload || {}),
        flowType: EmbeddedFlowType.Registration,
        ...(effectiveApplicationId && {applicationId: effectiveApplicationId}),
        ...(scopes && {scopes}),
      };
    } else {
      initialPayload = payload;
    }

    return (await signUp(initialPayload)) as EmbeddedSignUpFlowResponseV2;
  };

  /**
   * Handle sign-up steps.
   */
  const handleOnSubmit = async (payload: EmbeddedSignUpFlowRequestV2): Promise<EmbeddedSignUpFlowResponseV2> =>
    (await signUp(payload)) as EmbeddedSignUpFlowResponseV2;

  /**
   * Handle successful sign-up and redirect.
   */
  const handleComplete = (response: EmbeddedSignUpFlowResponseV2): any => {
    onComplete?.(response);

    if (!shouldRedirectAfterSignUp) {
      return;
    }

    const redirectURL: string | undefined = (response?.data as Record<string, unknown>)?.['redirectURL'] as
      | string
      | undefined;

    if (
      response?.type === EmbeddedSignUpFlowTypeV2.Redirection &&
      redirectURL &&
      !redirectURL.includes('oauth') && // Not a social provider redirect
      !redirectURL.includes('auth') // Not an auth provider redirect
    ) {
      window.location.href = redirectURL;
      return;
    }

    const oauthRedirectUrl: any = (response as any)?.redirectUrl;
    if (oauthRedirectUrl) {
      window.location.href = oauthRedirectUrl;
      return;
    }

    // For non-redirection responses (regular sign-up completion), handle redirect if configured.
    // Skip when assertion is present — the SDK stored the session and the caller handled navigation.
    if (response?.type !== EmbeddedSignUpFlowTypeV2.Redirection && afterSignUpUrl && !(response as any)?.assertion) {
      window.location.href = afterSignUpUrl;
    }
  };

  return (
    <BaseSignUp
      afterSignUpUrl={afterSignUpUrl}
      onInitialize={handleInitialize}
      onSubmit={handleOnSubmit}
      onError={onError}
      onComplete={handleComplete}
      className={className}
      size={size}
      isInitialized={isInitialized}
      children={children}
      showTitle={true}
      showSubtitle={true}
      {...rest}
    />
  );
};

export default SignUp;
