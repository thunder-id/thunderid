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

import {useConfig} from '@thunderid/contexts';
import {AuthCardLayout, FlowComponentRenderer, useDesign} from '@thunderid/design';
import {useLogger} from '@thunderid/logger/react';
import {normalizeFlowResponse, useThunderID, type EmbeddedFlowComponent} from '@thunderid/react';
import {Alert, Box, CircularProgress} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useEffect, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useSearchParams} from 'react-router';

/**
 * The subset of the /flow/execute response this box reads: the status (to know when the flow is done)
 * and the challenge token to echo on the next interactive submit.
 */
interface FlowExecuteResponse {
  flowStatus?: string;
  challengeToken?: string;
}

/**
 * Renders the sign-out flow the /oauth2/logout endpoint initiates.
 *
 * The endpoint runs the flow up to its first interactive step and redirects the browser here with an
 * `executionId` and a `logoutId`. This box resumes that execution against the generic /flow/execute
 * endpoint until it completes (a flow with no interactive step completes on the first call; a flow with
 * a confirmation step renders its components first). On completion it calls the sign-out completion
 * endpoint (/oauth2/logout/callback) with the `logoutId`; the server runs any protocol-level actions and
 * returns the validated post-logout redirect URI for the browser to land on. Keeping the redirect in the
 * OAuth layer (not the flow) leaves the flow engine protocol-agnostic.
 *
 * `credentials: 'include'` ensures the per-flow SSO cookie is sent and the clearing Set-Cookie the flow
 * emits on completion is applied by the browser.
 */
export default function SignOutBox(): JSX.Element {
  const [searchParams] = useSearchParams();
  const executionId = searchParams.get('executionId') ?? '';
  const logoutId = searchParams.get('logoutId') ?? '';
  const {getServerUrl} = useConfig();
  const {resolveFlowTemplateLiterals} = useThunderID();
  const {t} = useTranslation();
  const logger = useLogger('SignOutBox');
  const {isDesignEnabled} = useDesign();

  const baseUrl = getServerUrl() ?? (import.meta.env.VITE_THUNDER_BASE_URL as string);

  const [components, setComponents] = useState<EmbeddedFlowComponent[]>([]);
  const [values, setValues] = useState<Record<string, string>>({});
  const [challengeToken, setChallengeToken] = useState<string>('');
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [flowError, setFlowError] = useState<string | null>(null);

  // Completes the sign-out with the OAuth layer once the flow finishes: the server runs any
  // protocol-level actions and returns the validated post-logout redirect URI to land on.
  const completeSignOut = async (): Promise<void> => {
    const response = await fetch(`${baseUrl}/oauth2/logout/callback`, {
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      credentials: 'include',
      body: JSON.stringify({logoutId}),
    });
    if (!response.ok) {
      throw new Error(`logout callback failed: ${response.status}`);
    }
    const result = (await response.json()) as {redirect_uri?: string};
    if (result.redirect_uri) {
      window.location.href = result.redirect_uri;
    }
    // No redirect_uri: the RP requested no landing; sign-out is complete.
  };

  // Executes one /flow/execute call: completes with the OAuth layer on COMPLETE, else renders next step.
  const run = async (payload: Record<string, unknown>): Promise<void> => {
    setIsLoading(true);
    setFlowError(null);
    try {
      const response = await fetch(`${baseUrl}/flow/execute`, {
        method: 'POST',
        headers: {'Content-Type': 'application/json', Accept: 'application/json'},
        credentials: 'include',
        body: JSON.stringify({...payload, verbose: true}),
      });
      if (!response.ok) {
        throw new Error(`flow execute failed: ${response.status}`);
      }
      const res = (await response.json()) as FlowExecuteResponse;

      if (res.flowStatus === 'COMPLETE') {
        await completeSignOut();
        return;
      }

      // Each interactive step mints a fresh challenge token that the next submit must echo back.
      setChallengeToken(res.challengeToken ?? '');
      const {components: next} = normalizeFlowResponse(res, t, {throwOnError: false});
      setComponents(next);
    } catch (error) {
      logger.error('Sign-out flow error:', error instanceof Error ? error : undefined);
      setFlowError(t('signout:errors.failed.description', 'Something went wrong. Please try again.'));
    } finally {
      setIsLoading(false);
    }
  };

  // Resume the execution the sign-out endpoint initiated.
  useEffect(() => {
    void run({executionId});
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [executionId]);

  return (
    <AuthCardLayout
      variant="RecoveryBox"
      logo={{
        src: {
          light: `${import.meta.env.BASE_URL}/assets/images/logo.svg`,
          dark: `${import.meta.env.BASE_URL}/assets/images/logo-inverted.svg`,
        },
        alt: {light: '', dark: ''},
      }}
      showLogo={!isDesignEnabled}
      logoDisplay={!isDesignEnabled ? {xs: 'flex', md: 'none'} : {display: 'none'}}
    >
      {flowError && (
        <Alert severity="error" sx={{mb: 2}}>
          {flowError}
        </Alert>
      )}
      {isLoading && components.length === 0 ? (
        <Box sx={{display: 'flex', justifyContent: 'center', p: 3}}>
          <CircularProgress />
        </Box>
      ) : (
        components.length > 0 && (
          <Box sx={{display: 'flex', flexDirection: 'column', gap: 2}}>
            {components.map((component, index) => (
              <FlowComponentRenderer
                key={component.id ?? index}
                component={component}
                index={index}
                values={values}
                isLoading={isLoading}
                resolve={resolveFlowTemplateLiterals}
                onInputChange={(id: string, value: string) => setValues((prev) => ({...prev, [id]: value}))}
                onSubmit={(action: {id?: string}, inputs: Record<string, string>) => {
                  void run({executionId, challengeToken, action: action.id ?? component.id, inputs});
                }}
              />
            ))}
          </Box>
        )
      )}
    </AuthCardLayout>
  );
}
