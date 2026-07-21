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

import {FlowType, FlowNodeType} from '../models/flows';
import {type CreateFlowRequest, type FlowNode, type FlowPrompt} from '../models/responses';

/**
 * Options for generating a flow graph
 */
export interface FlowGeneratorOptions {
  hasCredentialsAuth: boolean;
  hasPasskey: boolean;
  googleIdpId?: string;
  githubIdpId?: string;
  hasSmsOtp: boolean;
  relyingPartyId?: string;
  relyingPartyName?: string;
}

/**
 * Generates a CreateFlowRequest with a complete flow graph based on selected authentication methods.
 * Supports any combination of Basic, Passkey, Google, GitHub, and SMS.
 *
 * @param options - Selected authentication methods and IDs
 * @returns Complete flow creation request payload
 */
export default function generateFlowGraph(options: FlowGeneratorOptions): CreateFlowRequest {
  const {hasCredentialsAuth, hasPasskey, googleIdpId, githubIdpId, hasSmsOtp, relyingPartyId, relyingPartyName} =
    options;

  // 1. Generate Flow Handle and Name
  const parts: string[] = [];
  if (hasCredentialsAuth) parts.push('basic');
  if (hasPasskey) parts.push('passkey');
  if (googleIdpId) parts.push('google');
  if (githubIdpId) parts.push('github');
  if (hasSmsOtp) parts.push('sms');

  // Sort parts to ensure deterministic handle generation
  // But keep "basic" first for readability if present
  const sortedParts = parts.filter((p) => p !== 'basic').sort();
  if (hasCredentialsAuth) sortedParts.unshift('basic');

  const handle = `generated-${sortedParts.join('-')}-flow`;
  const name = `Generated ${sortedParts.map((p) => p.charAt(0).toUpperCase() + p.slice(1)).join(' + ')} Flow`;

  // 2. Build Nodes
  const nodes: FlowNode[] = [];

  // START Node
  nodes.push({
    id: 'start',
    type: FlowNodeType.START,
    onSuccess: 'choose_auth_method',
  });

  // PROMPT Node (choose_auth_method)
  const promptNodeId = 'choose_auth_method';
  const promptPrompts: FlowPrompt[] = [];
  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const metaComponents: any[] = [];

  // Self Sign Up Link component — shared across auth blocks. The anchor is sentinel-marked
  // with `data-action-ref="action_signup"`, and the component carries a matching `action`
  // wiring so the SDK's RICH_TEXT renderer dispatches the action instead of navigating.
  const selfSignUpLink = {
    category: 'DISPLAY',
    type: 'RICH_TEXT',
    id: 'self_sign_up_link',
    action: {
      ref: 'action_signup',
      eventType: 'SUBMIT',
    },
    label:
      '<p class="rich-text-paragraph"><span class="rich-text-pre-wrap">Don\'t have an account? </span>' +
      '<a href="{{meta(application.sign_up_url)}}" data-action-ref="action_signup" target="_blank" ' +
      'rel="noopener noreferrer" class="rich-text-link">' +
      '<span class="rich-text-pre-wrap">Sign up</span></a></p>',
  };

  // Application Logo
  metaComponents.push({
    category: 'DISPLAY',
    type: 'IMAGE',
    id: 'image',
    src: '{{meta(application.logoUrl)}}',
    alt: '{{t(signin:images.app_logo.alt)}}',
    height: '60',
    width: '',
    resourceType: 'ELEMENT',
  });

  // Header
  metaComponents.push({
    type: 'TEXT',
    id: 'text_header',
    label: 'Sign In',
    variant: 'HEADING_1',
  });

  // Passkey Description (only if passkey is the only method)
  if (hasPasskey && !hasCredentialsAuth && !googleIdpId && !githubIdpId && !hasSmsOtp) {
    metaComponents.push({
      type: 'TEXT',
      id: 'text_passkey_desc',
      label: 'Use your passkey to securely sign in to your account without a password.',
      variant: 'BODY',
    });
  }

  // Credentials Auth Block
  if (hasCredentialsAuth) {
    metaComponents.push({
      type: 'BLOCK',
      id: 'block_basic',
      components: [
        {
          id: 'input_username',
          ref: 'username',
          type: 'TEXT_INPUT',
          label: 'Username',
          required: true,
          placeholder: 'Enter your Username',
        },
        {
          id: 'input_password',
          ref: 'password',
          type: 'PASSWORD_INPUT',
          label: 'Password',
          required: true,
          placeholder: 'Enter your Password',
        },
        {
          type: 'ACTION',
          id: 'action_basic',
          label: 'Sign In',
          variant: 'PRIMARY',
          eventType: 'SUBMIT',
        },
      ],
    });

    promptPrompts.push({
      inputs: [
        {
          ref: 'input_username',
          identifier: 'username',
          type: 'TEXT_INPUT',
          required: true,
        },
        {
          ref: 'input_password',
          identifier: 'password',
          type: 'PASSWORD_INPUT',
          required: true,
        },
      ],
      action: {
        ref: 'action_basic',
        nextNode: 'credentials_auth',
      },
    });
  }

  // Passkey Block
  if (hasPasskey) {
    const passkeyButtonLabel = 'Sign in with Passkey';

    metaComponents.push({
      type: 'BLOCK',
      id: 'block_passkey',
      components: [
        {
          type: 'ACTION',
          id: 'action_passkey',
          label: passkeyButtonLabel,
          variant: hasCredentialsAuth ? 'SECONDARY' : 'PRIMARY',
          eventType: 'SUBMIT',
        },
      ],
    });

    promptPrompts.push({
      action: {
        ref: 'action_passkey',
        nextNode: 'passkey_challenge',
      },
    });
  }

  // Social Block
  if (googleIdpId || githubIdpId) {
    // eslint-disable-next-line @typescript-eslint/no-explicit-any
    const socialComponents: any[] = [];

    if (googleIdpId) {
      socialComponents.push({
        type: 'ACTION',
        id: 'action_google',
        label: 'Sign in with Google',
        variant: 'OUTLINED',
        eventType: 'TRIGGER',
        image: 'assets/images/icons/google.svg',
      });

      promptPrompts.push({
        action: {
          ref: 'action_google',
          nextNode: 'google_auth',
        },
      });
    }

    if (githubIdpId) {
      socialComponents.push({
        type: 'ACTION',
        id: 'action_github',
        label: 'Sign in with GitHub',
        variant: 'OUTLINED',
        eventType: 'TRIGGER',
        image: 'assets/images/icons/github.svg',
      });

      promptPrompts.push({
        action: {
          ref: 'action_github',
          nextNode: 'github_auth',
        },
      });
    }

    metaComponents.push({
      type: 'BLOCK',
      id: 'block_social',
      components: socialComponents,
    });
  }

  // Self Sign Up Link — always at the bottom, after all auth options
  if (hasCredentialsAuth || hasPasskey) {
    metaComponents.push(selfSignUpLink);
  }

  // Connect PROMPT node
  nodes.push({
    id: promptNodeId,
    type: FlowNodeType.PROMPT,
    meta: {
      components: metaComponents,
    },
    prompts: promptPrompts,
  });

  // 3. Executor Nodes

  // Credentials Auth Executor
  if (hasCredentialsAuth) {
    nodes.push({
      id: 'credentials_auth',
      type: FlowNodeType.TASK_EXECUTION,
      executor: {
        name: 'CredentialsAuthExecutor',
      },
      onSuccess: 'auth_assert',
    });
  }

  // Passkey Executor
  if (hasPasskey) {
    // Challenge Node
    nodes.push({
      id: 'passkey_challenge',
      type: FlowNodeType.TASK_EXECUTION,
      properties: {
        relyingPartyId,
        relyingPartyName,
      },
      executor: {
        name: 'PasskeyAuthExecutor',
        mode: 'challenge',
      },
      onSuccess: 'passkey_verify',
    });

    // Verify Node
    nodes.push({
      id: 'passkey_verify',
      type: FlowNodeType.TASK_EXECUTION,
      executor: {
        name: 'PasskeyAuthExecutor',
        mode: 'verify',
      },
      onSuccess: 'auth_assert',
    });
  }

  // Google Executor
  if (googleIdpId) {
    nodes.push({
      id: 'google_auth',
      type: FlowNodeType.TASK_EXECUTION,
      properties: {
        idpId: googleIdpId,
        allowAuthenticationWithoutLocalUser: true,
      },
      executor: {
        name: 'GoogleOIDCAuthExecutor',
        inputs: [
          {
            ref: 'input_google_code',
            type: 'TEXT_INPUT',
            identifier: 'code',
            required: true,
          },
        ],
      },
      onSuccess: 'provisioning',
    });
  }

  // GitHub Executor
  if (githubIdpId) {
    nodes.push({
      id: 'github_auth',
      type: FlowNodeType.TASK_EXECUTION,
      properties: {
        idpId: githubIdpId,
        allowAuthenticationWithoutLocalUser: true,
      },
      executor: {
        name: 'GithubOAuthExecutor',
        inputs: [
          {
            ref: 'input_github_code',
            type: 'TEXT_INPUT',
            identifier: 'code',
            required: true,
          },
        ],
      },
      onSuccess: 'provisioning',
    });
  }

  // Provisioning Executor (For Social Auth)
  if (googleIdpId || githubIdpId) {
    nodes.push({
      id: 'provisioning',
      type: FlowNodeType.TASK_EXECUTION,
      condition: {
        key: '{{ctx(userEligibleForProvisioning)}}',
        value: 'true',
        onSkip: 'auth_assert',
      },
      executor: {
        name: 'ProvisioningExecutor',
      },
      onSuccess: 'auth_assert',
    });
  }

  // Auth Assert Executor (Common completion step)
  nodes.push({
    id: 'auth_assert',
    type: FlowNodeType.TASK_EXECUTION,
    executor: {
      name: 'AuthAssertExecutor',
    },
    onSuccess: 'end',
  });

  // END Node
  nodes.push({
    id: 'end',
    type: FlowNodeType.END,
  });

  return {
    name,
    handle,
    flowType: FlowType.AUTHENTICATION,
    nodes,
  };
}
