// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {FlowNodeType} from '../../models/flows';
import generateFlowGraph from '../generateFlowGraph';

interface Component {
  id: string;
  type?: string;
  label?: string;
  ref?: string;
  src?: string;
  variant?: string;
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

    expect(request.handle).toMatch(/^generated-sign-in-flow-[a-z0-9]{6}$/);
    // START, PROMPT, BASIC_EXEC, AUTHORIZATION_CHECK, AUTH_ASSERT, END
    expect(request.nodes).toHaveLength(6);

    const components = getPromptComponents(request);
    expect(components.find((c) => c.id === 'block_basic')).toBeDefined();
  });

  describe('authorization', () => {
    it('should insert exactly one AuthorizationExecutor, wired between authentication and the assertion', () => {
      const request = generateFlowGraph({hasCredentialsAuth: true, hasPasskey: false, hasSmsOtp: false});

      const authzNodes = request.nodes.filter((n) => n.executor?.name === 'AuthorizationExecutor');
      expect(authzNodes).toHaveLength(1);
      expect(authzNodes[0].id).toBe('authorization_check');
      expect(authzNodes[0].type).toBe(FlowNodeType.TASK_EXECUTION);
      expect(authzNodes[0].onSuccess).toBe('auth_assert');
      expect(request.nodes.find((n) => n.id === 'auth_assert')?.onSuccess).toBe('end');
    });

    it('should only ever reach the assertion through the authorization check', () => {
      const request = generateFlowGraph({
        hasCredentialsAuth: true,
        hasPasskey: true,
        hasMagicLink: true,
        hasSmsOtp: true,
        googleIdpId: 'google-p-id',
        githubIdpId: 'github-p-id',
        hasEmailOtpMfa: true,
        hasSmsOtpMfa: true,
      });

      const intoAssert = request.nodes.filter((n) => n.onSuccess === 'auth_assert').map((n) => n.id);
      expect(intoAssert).toEqual(['authorization_check']);
    });

    // hasSmsOtp is absent here on purpose: it generates no first-factor nodes (it only feeds the
    // "passkey is the sole method" heuristic), so SMS OTP is only reachable as MFA. See the MFA tests.
    it.each([
      ['credentials', {hasCredentialsAuth: true}, 'credentials_auth'],
      ['passkey', {hasPasskey: true}, 'passkey_verify'],
      ['magic link', {hasMagicLink: true}, 'magic_link_verify'],
      ['Google', {googleIdpId: 'google-p-id'}, 'provisioning'],
      ['GitHub', {githubIdpId: 'github-p-id'}, 'provisioning'],
    ])('should route a completed %s sign-in into the authorization check', (_label, options, terminalNodeId) => {
      const request = generateFlowGraph({hasCredentialsAuth: false, hasPasskey: false, hasSmsOtp: false, ...options});

      expect(request.nodes.find((n) => n.id === terminalNodeId)?.onSuccess).toBe('authorization_check');
    });

    it('should route both social provisioning outcomes into the authorization check', () => {
      const request = generateFlowGraph({
        hasCredentialsAuth: false,
        hasPasskey: false,
        hasSmsOtp: false,
        googleIdpId: 'google-p-id',
      });

      // Newly provisioned users take onSuccess; users that already exist locally take onSkip.
      const provisioning = request.nodes.find((n) => n.id === 'provisioning');
      expect(provisioning?.onSuccess).toBe('authorization_check');
      expect(provisioning?.condition?.onSkip).toBe('authorization_check');
    });
  });

  it('should generate a Passkey flow', () => {
    const request = generateFlowGraph({
      hasCredentialsAuth: false,
      hasPasskey: true,
      hasSmsOtp: false,
    });

    expect(request.handle).toMatch(/^generated-sign-in-flow-[a-z0-9]{6}$/);

    const components = getPromptComponents(request);
    expect(components.find((c) => c.id === 'block_passkey')).toBeDefined();
    expect(components.find((c) => c.id === 'block_basic')).toBeUndefined();

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

    expect(request.handle).toMatch(/^generated-sign-in-flow-[a-z0-9]{6}$/);

    const components = getPromptComponents(request);
    expect(components.find((c) => c.id === 'block_basic')).toBeDefined();
    expect(components.find((c) => c.id === 'block_passkey')).toBeDefined();
    expect(components.find((c) => c.id === 'block_social')).toBeDefined();

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

    expect(request.handle).toMatch(/^generated-sign-in-flow-[a-z0-9]{6}$/);

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
  });

  it('should never include a Self Sign Up Link, since action_signup has no node to route to', () => {
    const cases: Parameters<typeof generateFlowGraph>[0][] = [
      {hasCredentialsAuth: true, hasPasskey: false, hasSmsOtp: false},
      {hasCredentialsAuth: false, hasPasskey: true, hasSmsOtp: false},
      {hasCredentialsAuth: false, hasPasskey: false, hasMagicLink: true, hasSmsOtp: false},
      {hasCredentialsAuth: true, hasPasskey: true, googleIdpId: 'google-id', hasSmsOtp: false},
    ];

    for (const options of cases) {
      const request = generateFlowGraph(options);
      const components = getPromptComponents(request);
      expect(components.find((c) => c.id === 'self_sign_up_link')).toBeUndefined();
    }
  });

  it('should include the application logo as the first meta component', () => {
    const request = generateFlowGraph({hasCredentialsAuth: true, hasPasskey: false, hasSmsOtp: false});
    const components = getPromptComponents(request);

    expect(components[0].id).toBe('image');
    expect(components[0].type).toBe('IMAGE');
    expect(components[0].src).toContain('application.logoUrl');
  });

  describe('MFA subgraph', () => {
    it('should route the first factor into the Email OTP chain instead of the authorization check', () => {
      const request = generateFlowGraph({
        hasCredentialsAuth: true,
        hasPasskey: false,
        hasSmsOtp: false,
        hasEmailOtpMfa: true,
      });

      const credentialsAuth = request.nodes.find((n) => n.id === 'credentials_auth');
      expect(credentialsAuth?.onSuccess).toBe('mfa_generate_otp');

      const generateNode = request.nodes.find((n) => n.id === 'mfa_generate_otp');
      expect(generateNode?.executor).toEqual({name: 'OTPExecutor', mode: 'generate'});
      expect(generateNode?.onSuccess).toBe('mfa_send_otp');

      const sendNode = request.nodes.find((n) => n.id === 'mfa_send_otp');
      expect(sendNode?.executor).toEqual({name: 'EmailExecutor', mode: 'send'});
      expect(sendNode?.properties?.emailTemplate).toBe('OTP');
      expect(sendNode?.onSuccess).toBe('mfa_verify_prompt');

      const promptNode = request.nodes.find((n) => n.id === 'mfa_verify_prompt');
      expect(promptNode?.type).toBe(FlowNodeType.PROMPT);
      const verifyAction = promptNode?.prompts?.find((p) => p.action?.nextNode === 'mfa_verify_otp');
      expect(verifyAction).toBeDefined();
      const resendAction = promptNode?.prompts?.find((p) => p.action?.nextNode === 'mfa_generate_otp');
      expect(resendAction).toBeDefined();

      const verifyNode = request.nodes.find((n) => n.id === 'mfa_verify_otp');
      expect(verifyNode?.executor).toEqual({name: 'OTPExecutor', mode: 'verify'});
      expect(verifyNode?.onSuccess).toBe('authorization_check');
    });

    it('should route the SMS OTP chain through SMSExecutor with the configured senderId', () => {
      const request = generateFlowGraph({
        hasCredentialsAuth: true,
        hasPasskey: false,
        hasSmsOtp: false,
        hasSmsOtpMfa: true,
        smsOtpSenderId: 'sender-123',
      });

      expect(request.nodes.find((n) => n.id === 'credentials_auth')?.onSuccess).toBe('mfa_generate_otp');

      const sendNode = request.nodes.find((n) => n.id === 'mfa_send_otp');
      expect(sendNode?.executor?.name).toBe('SMSExecutor');
      expect(sendNode?.properties?.senderId).toBe('sender-123');
      expect(sendNode?.properties?.smsTemplate).toBe('OTP');
    });

    it('should branch into an independent channel-choice chain when both Email and SMS OTP MFA are enabled', () => {
      const request = generateFlowGraph({
        hasCredentialsAuth: true,
        hasPasskey: false,
        hasSmsOtp: false,
        hasEmailOtpMfa: true,
        hasSmsOtpMfa: true,
        smsOtpSenderId: 'sender-123',
      });

      expect(request.nodes.find((n) => n.id === 'credentials_auth')?.onSuccess).toBe('mfa_choose_channel');

      const chooseNode = request.nodes.find((n) => n.id === 'mfa_choose_channel');
      expect(chooseNode?.prompts?.find((p) => p.action?.nextNode === 'mfa_generate_otp_email')).toBeDefined();
      expect(chooseNode?.prompts?.find((p) => p.action?.nextNode === 'mfa_generate_otp_sms')).toBeDefined();

      // Each channel's chain is fully independent, including its own resend loop.
      const emailPrompt = request.nodes.find((n) => n.id === 'mfa_verify_prompt_email');
      expect(emailPrompt?.prompts?.find((p) => p.action?.nextNode === 'mfa_generate_otp_email')).toBeDefined();
      const smsPrompt = request.nodes.find((n) => n.id === 'mfa_verify_prompt_sms');
      expect(smsPrompt?.prompts?.find((p) => p.action?.nextNode === 'mfa_generate_otp_sms')).toBeDefined();

      const smsSendNode = request.nodes.find((n) => n.id === 'mfa_send_otp_sms');
      expect(smsSendNode?.properties?.senderId).toBe('sender-123');
      expect(smsSendNode?.properties?.smsTemplate).toBe('OTP');

      expect(request.nodes.find((n) => n.id === 'mfa_verify_otp_email')?.onSuccess).toBe('authorization_check');
      expect(request.nodes.find((n) => n.id === 'mfa_verify_otp_sms')?.onSuccess).toBe('authorization_check');
    });

    it('should leave the first factor routed straight to the authorization check when MFA is not enabled', () => {
      const request = generateFlowGraph({hasCredentialsAuth: true, hasPasskey: false, hasSmsOtp: false});

      expect(request.nodes.find((n) => n.id === 'credentials_auth')?.onSuccess).toBe('authorization_check');
      expect(request.nodes.find((n) => n.id === 'mfa_generate_otp')).toBeUndefined();
    });

    it('should route incomplete credential submissions back to the sign-in prompt', () => {
      const request = generateFlowGraph({hasCredentialsAuth: true, hasPasskey: false, hasSmsOtp: false});

      const credentialsAuth = request.nodes.find((n) => n.id === 'credentials_auth');
      const promptNode = request.nodes.find((n) => n.type === FlowNodeType.PROMPT);
      expect(credentialsAuth?.onIncomplete).toBe(promptNode?.id);
    });

    it('should route passkey and social provisioning success into MFA too', () => {
      const request = generateFlowGraph({
        hasCredentialsAuth: false,
        hasPasskey: true,
        googleIdpId: 'google-id',
        hasSmsOtp: false,
        hasEmailOtpMfa: true,
      });

      expect(request.nodes.find((n) => n.id === 'passkey_verify')?.onSuccess).toBe('mfa_generate_otp');
      const provisioning = request.nodes.find((n) => n.id === 'provisioning');
      expect(provisioning?.onSuccess).toBe('mfa_generate_otp');
      expect(provisioning?.condition?.onSkip).toBe('mfa_generate_otp');
    });
  });

  describe('Magic Link', () => {
    it('should generate a Magic Link only flow', () => {
      const request = generateFlowGraph({
        hasCredentialsAuth: false,
        hasPasskey: false,
        hasMagicLink: true,
        hasSmsOtp: false,
      });

      expect(request.handle).toMatch(/^generated-sign-in-flow-[a-z0-9]{6}$/);

      const components = getPromptComponents(request);
      expect(components.find((c) => c.id === 'block_magic_link')).toBeDefined();
    });

    it('should route the "Sign in with Magic Link" button to its own email-entry screen when there is no password to pair it with', () => {
      const request = generateFlowGraph({
        hasCredentialsAuth: false,
        hasPasskey: true,
        hasMagicLink: true,
        hasSmsOtp: false,
      });

      const mainPrompt = request.nodes.find((n) => n.id === 'choose_auth_method');
      const magicLinkAction = mainPrompt?.prompts?.find((p) => p.action?.ref === 'action_magic_link');
      expect(magicLinkAction?.action?.nextNode).toBe('magic_link_prompt_email');
    });

    it('should build the full generate -> send -> wait -> verify chain', () => {
      const request = generateFlowGraph({
        hasCredentialsAuth: false,
        hasPasskey: false,
        hasMagicLink: true,
        hasSmsOtp: false,
      });

      const emailPrompt = request.nodes.find((n) => n.id === 'magic_link_prompt_email');
      expect(emailPrompt?.type).toBe(FlowNodeType.PROMPT);
      const submitAction = emailPrompt?.prompts?.find((p) => p.action?.ref === 'action_magic_link_submit');
      expect(submitAction?.action?.nextNode).toBe('magic_link_generate');

      const generateNode = request.nodes.find((n) => n.id === 'magic_link_generate');
      expect(generateNode?.executor?.name).toBe('MagicLinkExecutor');
      expect(generateNode?.executor?.mode).toBe('generate');
      expect(generateNode?.onSuccess).toBe('magic_link_send_email');
      expect(generateNode?.onFailure).toBe('magic_link_prompt_email');

      const sendNode = request.nodes.find((n) => n.id === 'magic_link_send_email');
      expect(sendNode?.executor?.name).toBe('EmailExecutor');
      expect(sendNode?.properties?.emailTemplate).toBe('MAGIC_LINK');
      expect(sendNode?.onSuccess).toBe('magic_link_prompt_sent');

      const sentPrompt = request.nodes.find((n) => n.id === 'magic_link_prompt_sent');
      expect(sentPrompt?.type).toBe(FlowNodeType.PROMPT);
      expect(sentPrompt?.prompts).toBeUndefined();
      expect(sentPrompt?.next).toBe('magic_link_verify');

      const verifyNode = request.nodes.find((n) => n.id === 'magic_link_verify');
      expect(verifyNode?.executor).toEqual({name: 'MagicLinkExecutor', mode: 'verify'});
    });

    it('should route a verified magic link straight to the authorization check when MFA is not enabled', () => {
      const request = generateFlowGraph({
        hasCredentialsAuth: false,
        hasPasskey: false,
        hasMagicLink: true,
        hasSmsOtp: false,
      });

      expect(request.nodes.find((n) => n.id === 'magic_link_verify')?.onSuccess).toBe('authorization_check');
    });

    it('should route a verified magic link into the MFA subgraph when MFA is enabled', () => {
      const request = generateFlowGraph({
        hasCredentialsAuth: false,
        hasPasskey: false,
        hasMagicLink: true,
        hasSmsOtp: false,
        hasEmailOtpMfa: true,
      });

      expect(request.nodes.find((n) => n.id === 'magic_link_verify')?.onSuccess).toBe('mfa_generate_otp');
    });

    it('should produce unique node ids with every option enabled simultaneously', () => {
      const request = generateFlowGraph({
        hasCredentialsAuth: true,
        hasPasskey: true,
        hasMagicLink: true,
        googleIdpId: 'google-id',
        githubIdpId: 'github-id',
        hasSmsOtp: true,
        hasEmailOtpMfa: true,
        hasSmsOtpMfa: true,
        smsOtpSenderId: 'sender-123',
      });

      const ids = request.nodes.map((n) => n.id);
      expect(new Set(ids).size).toBe(ids.length);
    });

    describe('alongside Basic auth', () => {
      it('should offer Magic Link as an alternative button next to the password form', () => {
        const request = generateFlowGraph({
          hasCredentialsAuth: true,
          hasPasskey: false,
          hasMagicLink: true,
          hasSmsOtp: false,
        });

        const magicLinkBlock = getPromptComponents(request).find((c) => c.id === 'block_magic_link');
        expect(magicLinkBlock).toBeDefined();
        expect(magicLinkBlock?.components?.[0]?.variant).toBe('SECONDARY');

        const mainPrompt = request.nodes.find((n) => n.id === 'choose_auth_method');
        const magicLinkAction = mainPrompt?.prompts?.find((p) => p.action?.ref === 'action_magic_link');
        expect(magicLinkAction?.action?.nextNode).toBe('magic_link_prompt_email');
      });

      it('should route credentials success straight to the authorization check, without an email-collecting step', () => {
        const request = generateFlowGraph({
          hasCredentialsAuth: true,
          hasPasskey: false,
          hasMagicLink: true,
          hasSmsOtp: false,
        });

        expect(request.nodes.find((n) => n.id === 'credentials_auth')?.onSuccess).toBe('authorization_check');
        expect(request.nodes.find((n) => n.id === 'collect_email')).toBeUndefined();
      });

      it('should route every chain failure back to its own email prompt', () => {
        const request = generateFlowGraph({
          hasCredentialsAuth: true,
          hasPasskey: false,
          hasMagicLink: true,
          hasSmsOtp: false,
        });

        const generateNode = request.nodes.find((n) => n.id === 'magic_link_generate');
        expect(generateNode?.onIncomplete).toBe('magic_link_prompt_email');
        expect(generateNode?.onFailure).toBe('magic_link_prompt_email');

        const sendNode = request.nodes.find((n) => n.id === 'magic_link_send_email');
        expect(sendNode?.onIncomplete).toBe('magic_link_prompt_email');
        expect(sendNode?.onFailure).toBe('magic_link_prompt_email');

        const verifyNode = request.nodes.find((n) => n.id === 'magic_link_verify');
        expect(verifyNode?.onFailure).toBe('magic_link_prompt_email');
        expect(verifyNode?.onSuccess).toBe('authorization_check');
      });

      it('should still layer OTP MFA on top when enabled, after the magic link verifies', () => {
        const request = generateFlowGraph({
          hasCredentialsAuth: true,
          hasPasskey: false,
          hasMagicLink: true,
          hasSmsOtp: false,
          hasEmailOtpMfa: true,
        });

        expect(request.nodes.find((n) => n.id === 'magic_link_verify')?.onSuccess).toBe('mfa_generate_otp');
      });

      it('should leave Passkey unaffected as its own alternative first factor', () => {
        const request = generateFlowGraph({
          hasCredentialsAuth: true,
          hasPasskey: true,
          hasMagicLink: true,
          hasSmsOtp: false,
        });

        expect(request.nodes.find((n) => n.id === 'passkey_verify')?.onSuccess).toBe('authorization_check');
        expect(request.nodes.find((n) => n.id === 'credentials_auth')?.onSuccess).toBe('authorization_check');
      });
    });

    describe('button emphasis', () => {
      const primaryActionIds = (request: ReturnType<typeof generateFlowGraph>): string[] =>
        getPromptComponents(request)
          .flatMap((c) => (c.type === 'BLOCK' ? (c.components ?? []) : [c]))
          .filter((c) => c.type === 'ACTION' && c.variant === 'PRIMARY')
          .map((c) => c.id);

      it('should make the password form the only primary action when credentials are enabled', () => {
        const request = generateFlowGraph({
          hasCredentialsAuth: true,
          hasPasskey: true,
          hasMagicLink: true,
          hasSmsOtp: false,
        });

        expect(primaryActionIds(request)).toEqual(['action_basic']);
      });

      it('should promote Passkey to the only primary action when credentials are disabled', () => {
        const request = generateFlowGraph({
          hasCredentialsAuth: false,
          hasPasskey: true,
          hasMagicLink: true,
          hasSmsOtp: false,
        });

        expect(primaryActionIds(request)).toEqual(['action_passkey']);
      });

      it('should promote Magic Link to the only primary action when it is the only method', () => {
        const request = generateFlowGraph({
          hasCredentialsAuth: false,
          hasPasskey: false,
          hasMagicLink: true,
          hasSmsOtp: false,
        });

        expect(primaryActionIds(request)).toEqual(['action_magic_link']);
      });
    });
  });

  describe('layout', () => {
    it('gives every node a position and size, so nothing opens stacked at the canvas origin', () => {
      const request = generateFlowGraph({
        hasCredentialsAuth: true,
        hasPasskey: true,
        hasMagicLink: true,
        googleIdpId: 'google-id',
        githubIdpId: 'github-id',
        hasSmsOtp: true,
        hasEmailOtpMfa: true,
        hasSmsOtpMfa: true,
        smsOtpSenderId: 'sender-123',
      });

      request.nodes.forEach((node) => {
        expect(node.layout?.position).toBeDefined();
        expect(node.layout?.size).toBeDefined();
      });

      const positions = request.nodes.map((node) => `${node.layout?.position.x},${node.layout?.position.y}`);
      expect(new Set(positions).size).toBe(positions.length);
    });

    it('places parallel first-factor chains (Basic vs Passkey) in different columns', () => {
      const request = generateFlowGraph({hasCredentialsAuth: true, hasPasskey: true, hasSmsOtp: false});

      const credentialsAuth = request.nodes.find((n) => n.id === 'credentials_auth');
      const passkeyChallenge = request.nodes.find((n) => n.id === 'passkey_challenge');

      expect(credentialsAuth?.layout?.position.x).toBe(passkeyChallenge?.layout?.position.x);
      expect(credentialsAuth?.layout?.position.y).not.toBe(passkeyChallenge?.layout?.position.y);
    });

    it('keeps a stable layout even when a chain loops back on itself (OTP resend)', () => {
      const request = generateFlowGraph({
        hasCredentialsAuth: true,
        hasPasskey: false,
        hasSmsOtp: false,
        hasEmailOtpMfa: true,
      });

      request.nodes.forEach((node) => {
        expect(node.layout?.position.x).toBeGreaterThanOrEqual(0);
        expect(node.layout?.position.y).toBeGreaterThanOrEqual(0);
      });
    });
  });

  describe('generated name and handle', () => {
    it('incorporates the application name into the flow name and handle', () => {
      const request = generateFlowGraph({
        appName: 'My App',
        hasCredentialsAuth: true,
        hasPasskey: false,
        hasSmsOtp: false,
      });

      expect(request.name).toBe('My App Sign-in Flow');
      expect(request.handle).toMatch(/^my-app-sign-in-flow-[a-z0-9]{6}$/);
    });

    it('falls back to a generic name/handle when no application name is given', () => {
      const request = generateFlowGraph({
        hasCredentialsAuth: true,
        hasPasskey: false,
        hasSmsOtp: false,
      });

      expect(request.name).toBe('Generated Sign-in Flow');
      expect(request.handle).toMatch(/^generated-sign-in-flow-[a-z0-9]{6}$/);
    });

    it('produces a different handle on every call, even with identical options', () => {
      const options = {appName: 'My App', hasCredentialsAuth: true, hasPasskey: false, hasSmsOtp: false};

      const first = generateFlowGraph(options);
      const second = generateFlowGraph(options);

      expect(first.handle).not.toBe(second.handle);
    });
  });

  describe('graph integrity across every toggle combination', () => {
    const flatten = (components: Component[]): Component[] =>
      components.flatMap((c) => [c, ...flatten(c.components ?? [])]);

    // Mirrors every option the wizard can pass, so a new combination cannot silently skip the checks.
    const allCombinations = (): Parameters<typeof generateFlowGraph>[0][] => {
      const combos: Parameters<typeof generateFlowGraph>[0][] = [];
      [false, true].forEach((hasCredentialsAuth) =>
        [false, true].forEach((hasPasskey) =>
          [false, true].forEach((hasMagicLink) =>
            [false, true].forEach((hasSmsOtp) =>
              [false, true].forEach((hasEmailOtpMfa) =>
                [undefined, 'idp-1'].forEach((socialIdpId) => {
                  combos.push({
                    githubIdpId: socialIdpId,
                    googleIdpId: socialIdpId,
                    hasCredentialsAuth,
                    hasEmailOtpMfa,
                    hasMagicLink,
                    hasPasskey,
                    hasSmsOtp,
                  });
                }),
              ),
            ),
          ),
        ),
      );
      return combos;
    };

    // The React SDK names each submitted field after the meta component's `ref`, rewriting it only
    // when the ref matches an entry in the prompt's `inputs[].ref -> identifier` map
    // (flowTransformer.ts createInputRefMapping). A component whose id an input points at must
    // therefore carry that input's identifier as its ref, or the field is submitted under a name
    // the executor never reads.
    it('names every input component ref after the identifier its executor reads', () => {
      const mismatched: string[] = [];

      allCombinations().forEach((options) => {
        const request = generateFlowGraph(options);

        request.nodes
          .filter((node) => node.type === FlowNodeType.PROMPT)
          .forEach((node) => {
            const components = flatten((node.meta?.components as Component[]) ?? []);

            (node.prompts ?? [])
              .flatMap((prompt) => prompt.inputs ?? [])
              .forEach((input) => {
                const component = components.find((c) => c.id === input.ref);
                if (component && component.ref !== input.identifier) {
                  mismatched.push(`${node.id}/${component.id} ref=${component.ref} identifier=${input.identifier}`);
                }
              });
          });
      });

      expect(mismatched).toEqual([]);
    });

    it('points every edge at a node that exists', () => {
      const dangling: string[] = [];

      allCombinations().forEach((options) => {
        const request = generateFlowGraph(options);
        const ids = new Set(request.nodes.map((node) => node.id));

        request.nodes.forEach((node) => {
          const targets: [string, string | undefined][] = [
            ['onSuccess', node.onSuccess],
            ['onFailure', node.onFailure],
            ['onIncomplete', node.onIncomplete],
            ['next', node.next],
            ['condition.onSkip', node.condition?.onSkip],
            ...(node.prompts ?? []).map((prompt): [string, string | undefined] => [
              'action.nextNode',
              prompt.action?.nextNode,
            ]),
          ];

          targets.forEach(([key, target]) => {
            if (target && !ids.has(target)) {
              dangling.push(`${node.id}.${key} -> ${target}`);
            }
          });
        });
      });

      expect(dangling).toEqual([]);
    });

    it('keeps the Magic Link chain looping back to its own email prompt on failure', () => {
      const request = generateFlowGraph({
        hasCredentialsAuth: true,
        hasMagicLink: true,
        hasPasskey: false,
        hasSmsOtp: false,
      });

      ['magic_link_generate', 'magic_link_send_email', 'magic_link_verify'].forEach((id) => {
        const node = request.nodes.find((n) => n.id === id);
        expect(node?.onFailure).toBe('magic_link_prompt_email');
      });
      expect(request.nodes.find((n) => n.id === 'magic_link_prompt_email')).toBeDefined();
    });
  });
});
