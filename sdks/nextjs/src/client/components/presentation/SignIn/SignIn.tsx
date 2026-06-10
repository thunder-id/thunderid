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

'use client';

import {
  ThunderIDRuntimeError,
  EmbeddedFlowExecuteRequestConfig,
  EmbeddedFlowType,
  EmbeddedSignInFlowResponse,
} from '@thunderid/node';
import {BaseSignIn, BaseSignInProps} from '@thunderid/react';
import {FC, useEffect, useRef, useState} from 'react';
import useThunderID from '../../../contexts/ThunderID/useThunderID';

/**
 * Props for the SignIn component.
 * Extends BaseSignInProps for full compatibility with the React BaseSignIn component
 */
export type SignInProps = Pick<BaseSignInProps, 'className' | 'onSuccess' | 'onError' | 'variant' | 'size'>;

/**
 * A SignIn component for Next.js that provides native authentication flow.
 * Initializes the embedded sign-in flow on mount and delegates UI rendering to BaseSignIn.
 */
const SignIn: FC<SignInProps> = ({size = 'medium', variant = 'outlined', ...rest}: SignInProps) => {
  const {signIn, applicationId, scopes} = useThunderID();
  const [components, setComponents] = useState<any[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [flowError, setFlowError] = useState<Error | null>(null);
  const initAttemptedRef = useRef(false);

  useEffect(() => {
    if (initAttemptedRef.current || !signIn) return;
    initAttemptedRef.current = true;

    (async (): Promise<void> => {
      try {
        const response: EmbeddedSignInFlowResponse | undefined = await signIn({
          flowType: EmbeddedFlowType.Authentication,
          ...(applicationId && {applicationId}),
          ...(scopes && {scopes}),
        });

        const flowComponents: any[] = response?.data?.meta?.components ?? [];
        setComponents(flowComponents);
      } catch (err) {
        setFlowError(err instanceof Error ? err : new Error(String(err)));
      } finally {
        setIsLoading(false);
      }
    })();
  }, [signIn]);

  const handleOnSubmit = async (
    payload: any,
    request?: EmbeddedFlowExecuteRequestConfig,
  ): Promise<any> => {
    if (!signIn) {
      throw new ThunderIDRuntimeError(
        '`signIn` function is not available.',
        'SignIn-handleOnSubmit-RuntimeError-001',
        'nextjs',
      );
    }

    return signIn(payload, request);
  };

  return (
    <BaseSignIn
      components={components}
      error={flowError}
      isLoading={isLoading}
      onSubmit={handleOnSubmit as any}
      size={size}
      variant={variant}
      {...rest}
    />
  );
};

SignIn.displayName = 'SignIn';

export default SignIn;
