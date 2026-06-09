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

import {describe, it, expect} from 'vitest';
import {FlowNodeType} from '../../models/flows';
import generateFlowGraph from '../generateFlowGraph';

interface Component {
  id: string;
  type?: string;
  label?: string;
  src?: string;
  components?: Component[];
}

function getPromptComponents(result: ReturnType<typeof generateFlowGraph>): Component[] {
  const promptNode = result.nodes.find((n) => n.type === FlowNodeType.PROMPT);
  return (promptNode?.meta?.components as Component[]) ?? [];
}

describe('generateFlowGraph', () => {
  it('should generate a Basic Auth flow', () => {
    const request = generateFlowGraph({
      hasCredentialsAuth: true,
      hasPasskey: false,
      hasSmsOtp: false,
    });

    expect(request.handle).toBe('generated-basic-flow');
    expect(request.nodes).toHaveLength(5); // START, PROMPT, BASIC_EXEC, AUTH_ASSERT, END

    const components = getPromptComponents(request);
    expect(components.find((c) => c.id === 'block_basic')).toBeDefined();
    expect(components.find((c) => c.id === 'self_sign_up_link')).toBeDefined();
  });

  it('should generate a Passkey flow', () => {
    const request = generateFlowGraph({
      hasCredentialsAuth: false,
      hasPasskey: true,
      hasSmsOtp: false,
    });

    expect(request.handle).toBe('generated-passkey-flow');

    const components = getPromptComponents(request);
    expect(components.find((c) => c.id === 'block_passkey')).toBeDefined();
    expect(components.find((c) => c.id === 'block_basic')).toBeUndefined();
    expect(components.find((c) => c.id === 'self_sign_up_link')).toBeDefined();

    // Executors
    const executors = request.nodes.filter((n) => n.type === FlowNodeType.TASK_EXECUTION);
    const passkeyExecutors = executors.filter((n) => n.executor?.name === 'PasskeyAuthExecutor');
    expect(passkeyExecutors).toHaveLength(2); // Challenge and Verify
  });

  it('should generate a Combined flow (Basic + Passkey + Google)', () => {
    const request = generateFlowGraph({
      hasCredentialsAuth: true,
      hasPasskey: true,
      googleIdpId: 'google-p-id',
      hasSmsOtp: false,
    });

    expect(request.handle).toBe('generated-basic-google-passkey-flow');

    const components = getPromptComponents(request);
    expect(components.find((c) => c.id === 'block_basic')).toBeDefined();
    expect(components.find((c) => c.id === 'block_passkey')).toBeDefined();
    expect(components.find((c) => c.id === 'block_social')).toBeDefined();
    expect(components.find((c) => c.id === 'self_sign_up_link')).toBeDefined();

    // Executors
    const executors = request.nodes.filter((n) => n.type === FlowNodeType.TASK_EXECUTION);
    expect(executors.find((n) => n.executor?.name === 'CredentialsAuthExecutor')).toBeDefined();
    expect(executors.find((n) => n.executor?.name === 'PasskeyAuthExecutor')).toBeDefined();
    expect(executors.find((n) => n.executor?.name === 'GoogleOIDCAuthExecutor')).toBeDefined();
    expect(executors.find((n) => n.executor?.name === 'ProvisioningExecutor')).toBeDefined();
  });

  it('should generate a Combined flow (Basic + Github)', () => {
    const request = generateFlowGraph({
      hasCredentialsAuth: true,
      hasPasskey: false,
      githubIdpId: 'github-id',
      hasSmsOtp: false,
    });

    expect(request.handle).toBe('generated-basic-github-flow');

    const components = getPromptComponents(request);
    expect(components.find((c) => c.id === 'self_sign_up_link')).toBeDefined();

    // Executors
    const executors = request.nodes.filter((n) => n.type === FlowNodeType.TASK_EXECUTION);
    expect(executors.find((n) => n.executor?.name === 'CredentialsAuthExecutor')).toBeDefined();
    expect(executors.find((n) => n.executor?.name === 'GithubOAuthExecutor')).toBeDefined();
    expect(executors.find((n) => n.executor?.name === 'ProvisioningExecutor')).toBeDefined();
  });

  it('should use provided relying party options for Passkey flow', () => {
    const request = generateFlowGraph({
      hasCredentialsAuth: false,
      hasPasskey: true,
      hasSmsOtp: false,
      relyingPartyId: 'my-app.com',
      relyingPartyName: 'My App',
    });

    const challengeNode = request.nodes.find((n) => n.id === 'passkey_challenge');
    expect(challengeNode).toBeDefined();
    expect(challengeNode?.properties?.relyingPartyId).toBe('my-app.com');
    expect(challengeNode?.properties?.relyingPartyName).toBe('My App');

    const components = getPromptComponents(request);
    expect(components.find((c) => c.id === 'self_sign_up_link')).toBeDefined();
  });

  it('should include a Self Sign Up Link as a top-level meta component for basic and passkey flows', () => {
    const cases: Parameters<typeof generateFlowGraph>[0][] = [
      {hasCredentialsAuth: true, hasPasskey: false, hasSmsOtp: false},
      {hasCredentialsAuth: false, hasPasskey: true, hasSmsOtp: false},
      {hasCredentialsAuth: true, hasPasskey: false, googleIdpId: 'google-id', hasSmsOtp: false},
      {hasCredentialsAuth: true, hasPasskey: false, githubIdpId: 'github-id', hasSmsOtp: false},
    ];

    for (const options of cases) {
      const request = generateFlowGraph(options);
      const components = getPromptComponents(request);
      const signUpLink = components.find((c) => c.id === 'self_sign_up_link');

      expect(signUpLink).toBeDefined();
      expect(signUpLink?.type).toBe('RICH_TEXT');
      expect(signUpLink?.label).toContain('{{meta(application.sign_up_url)}}');
    }
  });

  it('should not include a Self Sign Up Link for social-only flows', () => {
    const request = generateFlowGraph({
      hasCredentialsAuth: false,
      hasPasskey: false,
      googleIdpId: 'google-id',
      hasSmsOtp: false,
    });

    const components = getPromptComponents(request);
    expect(components.find((c) => c.id === 'self_sign_up_link')).toBeUndefined();
  });

  it('should place the Self Sign Up Link as the last top-level meta component', () => {
    const request = generateFlowGraph({hasCredentialsAuth: true, hasPasskey: false, hasSmsOtp: false});
    const components = getPromptComponents(request);

    expect(components.length).toBeGreaterThan(0);
    expect(components[components.length - 1].id).toBe('self_sign_up_link');
  });

  it('should place the Self Sign Up Link after all auth blocks (basic + passkey + social)', () => {
    const request = generateFlowGraph({
      hasCredentialsAuth: true,
      hasPasskey: true,
      googleIdpId: 'google-id',
      hasSmsOtp: false,
    });
    const components = getPromptComponents(request);

    const signUpLinkIndex = components.findIndex((c) => c.id === 'self_sign_up_link');
    const basicBlockIndex = components.findIndex((c) => c.id === 'block_basic');
    const passkeyBlockIndex = components.findIndex((c) => c.id === 'block_passkey');
    const socialBlockIndex = components.findIndex((c) => c.id === 'block_social');

    expect(signUpLinkIndex).toBeGreaterThan(basicBlockIndex);
    expect(signUpLinkIndex).toBeGreaterThan(passkeyBlockIndex);
    expect(signUpLinkIndex).toBeGreaterThan(socialBlockIndex);
  });

  it('should include the application logo as the first meta component', () => {
    const request = generateFlowGraph({hasCredentialsAuth: true, hasPasskey: false, hasSmsOtp: false});
    const components = getPromptComponents(request);

    expect(components[0].id).toBe('image');
    expect(components[0].type).toBe('IMAGE');
    expect(components[0].src).toContain('application.logoUrl');
  });
});
