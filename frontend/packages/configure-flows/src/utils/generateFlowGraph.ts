// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FlowType, FlowNodeType} from '../models/flows';
import {type CreateFlowRequest, type FlowNode, type FlowPrompt} from '../models/responses';

/**
 * Options for generating a flow graph
 */
export interface FlowGeneratorOptions {
  /** Name of the application this flow is being generated for, used to derive a unique flow name/handle. */
  appName?: string;
  hasCredentialsAuth: boolean;
  hasPasskey: boolean;
  /** Passwordless sign-in via a one-time link emailed to the user. */
  hasMagicLink?: boolean;
  googleIdpId?: string;
  githubIdpId?: string;
  hasSmsOtp: boolean;
  relyingPartyId?: string;
  relyingPartyName?: string;
  /** Require an Email OTP challenge after the first factor succeeds. */
  hasEmailOtpMfa?: boolean;
  /** Require an SMS OTP challenge after the first factor succeeds. */
  hasSmsOtpMfa?: boolean;
  /** SMS provider connection ID used to deliver SMS OTP MFA codes. Required when hasSmsOtpMfa is true. */
  smsOtpSenderId?: string;
}

/** Converts an arbitrary display name into a URL/handle-safe slug (lowercase, hyphenated). */
function slugify(value: string): string {
  return value
    .toLowerCase()
    .trim()
    .replace(/\s+/g, '-')
    .replace(/[^a-z0-9-]/g, '')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '');
}

/** OTP MFA delivery channel. */
type MfaChannel = 'email' | 'sms';

/**
 * Builds a self-contained OTP MFA chain for a single delivery channel: generate the code, send it
 * via the channel's executor, prompt for it, then verify it. Runs after an already-authenticated
 * first factor, so the generate step resolves its recipient from the authenticated user rather
 * than a declared node input (see OTPExecutor.executeGenerate on the backend).
 *
 * @param channel - Delivery channel for the OTP code
 * @param idSuffix - Node ID suffix, empty when this is the only MFA channel, `_email`/`_sms` when
 *   both channels are enabled and each needs its own independent chain (see `mfa_choose_channel`)
 * @param smsOtpSenderId - SMS provider connection ID, required when channel is 'sms'
 * @param onVerified - ID of the node to continue to once the OTP is verified (`authorization_check`)
 * @returns The chain's nodes and the ID of its entry node (where a first factor should route to)
 */
function buildMfaChannelChain(
  channel: MfaChannel,
  idSuffix: string,
  smsOtpSenderId: string | undefined,
  onVerified: string,
): {nodes: FlowNode[]; entryNodeId: string} {
  const generateId = `mfa_generate_otp${idSuffix}`;
  const sendId = `mfa_send_otp${idSuffix}`;
  const promptId = `mfa_verify_prompt${idSuffix}`;
  const verifyId = `mfa_verify_otp${idSuffix}`;

  const nodes: FlowNode[] = [
    {
      id: generateId,
      type: FlowNodeType.TASK_EXECUTION,
      executor: {name: 'OTPExecutor', mode: 'generate'},
      onSuccess: sendId,
    },
    channel === 'email'
      ? {
          id: sendId,
          type: FlowNodeType.TASK_EXECUTION,
          properties: {emailTemplate: 'OTP'},
          executor: {name: 'EmailExecutor', mode: 'send'},
          onSuccess: promptId,
        }
      : {
          id: sendId,
          type: FlowNodeType.TASK_EXECUTION,
          properties: {senderId: smsOtpSenderId, smsTemplate: 'OTP'},
          executor: {name: 'SMSExecutor'},
          onSuccess: promptId,
        },
    {
      id: promptId,
      type: FlowNodeType.PROMPT,
      meta: {
        components: [
          {
            type: 'TEXT',
            id: `text_mfa_heading${idSuffix}`,
            label: 'Verify your identity',
            variant: 'HEADING_3',
          },
          {
            type: 'TEXT',
            id: `text_mfa_desc${idSuffix}`,
            label: channel === 'email' ? 'Enter the code sent to your email' : 'Enter the code sent to your phone',
            variant: 'BODY',
          },
          {
            type: 'BLOCK',
            id: `block_mfa${idSuffix}`,
            components: [
              {
                id: `otp_input_mfa${idSuffix}`,
                ref: 'otp',
                type: 'OTP_INPUT',
                label: 'One-time code',
                required: true,
              },
              {
                type: 'ACTION',
                id: `action_mfa_verify${idSuffix}`,
                label: 'Verify',
                variant: 'PRIMARY',
                eventType: 'SUBMIT',
              },
              {
                type: 'RESEND',
                id: `action_mfa_resend${idSuffix}`,
                label: 'Resend code',
                eventType: 'SUBMIT',
              },
            ],
          },
        ],
      },
      prompts: [
        {
          inputs: [
            {
              ref: `otp_input_mfa${idSuffix}`,
              identifier: 'otp',
              type: 'OTP_INPUT',
              required: true,
            },
          ],
          action: {
            ref: `action_mfa_verify${idSuffix}`,
            nextNode: verifyId,
          },
        },
        {
          action: {
            ref: `action_mfa_resend${idSuffix}`,
            nextNode: generateId,
          },
        },
      ],
    },
    {
      id: verifyId,
      type: FlowNodeType.TASK_EXECUTION,
      executor: {name: 'OTPExecutor', mode: 'verify'},
      onSuccess: onVerified,
    },
  ];

  return {nodes, entryNodeId: generateId};
}

/**
 * Builds the passwordless "Magic Link" chain: prompt for an email, generate a one-time link and
 * email it, show a "check your email" wait screen, then verify the link once clicked. Mirrors the
 * working "Magic Link Authentication Flow" template (see `flows/data/templates.json`), adapted to
 * route into whichever continuation every other first factor uses (`onVerified` — either
 * `authorization_check` directly, or the MFA subgraph's entry) instead of hardcoding straight to
 * it, so Multi-Factor Login still layers on top of it like any other first factor.
 *
 * @param onVerified - ID of the node to continue to once the magic link is verified
 * @returns The chain's nodes. Its entry node is `magic_link_prompt_email`, where the
 *   "Sign in with Magic Link" button routes to, and where any failure in the chain returns to.
 */
function buildMagicLinkChain(onVerified: string): FlowNode[] {
  const promptId = 'magic_link_prompt_email';
  const generateId = 'magic_link_generate';
  const sendId = 'magic_link_send_email';
  const sentPromptId = 'magic_link_prompt_sent';
  const verifyId = 'magic_link_verify';
  const failureTarget = promptId;

  const nodes: FlowNode[] = [
    {
      id: promptId,
      type: FlowNodeType.PROMPT,
      meta: {
        components: [
          {
            category: 'DISPLAY',
            type: 'IMAGE',
            id: 'image_magic_link',
            src: '{{meta(application.logoUrl)}}',
            alt: '{{t(signin:images.app_logo.alt)}}',
            height: '60',
            width: '',
            resourceType: 'ELEMENT',
          },
          {
            type: 'TEXT',
            id: 'text_magic_link_heading',
            label: 'Sign in with Magic Link',
            variant: 'HEADING_1',
          },
          {
            type: 'BLOCK',
            id: 'block_magic_link_email',
            components: [
              {
                id: 'input_magic_link_email',
                ref: 'email',
                type: 'EMAIL_INPUT',
                label: 'Email',
                required: true,
                placeholder: 'Enter your email',
              },
              {
                type: 'ACTION',
                id: 'action_magic_link_submit',
                label: 'Send Magic Link',
                variant: 'PRIMARY',
                eventType: 'SUBMIT',
              },
            ],
          },
        ],
      },
      prompts: [
        {
          inputs: [
            {
              ref: 'input_magic_link_email',
              identifier: 'email',
              type: 'EMAIL_INPUT',
              required: true,
            },
          ],
          action: {
            ref: 'action_magic_link_submit',
            nextNode: generateId,
          },
        },
      ],
    },
    {
      id: generateId,
      type: FlowNodeType.TASK_EXECUTION,
      executor: {
        name: 'MagicLinkExecutor',
        mode: 'generate',
        inputs: [
          {
            ref: 'input_magic_link_email',
            identifier: 'email',
            type: 'EMAIL_INPUT',
            required: true,
          },
        ],
      },
      onSuccess: sendId,
      onIncomplete: failureTarget,
      onFailure: failureTarget,
    },
    {
      id: sendId,
      type: FlowNodeType.TASK_EXECUTION,
      properties: {emailTemplate: 'MAGIC_LINK'},
      executor: {
        name: 'EmailExecutor',
        mode: 'send',
        inputs: [{identifier: 'email', type: 'EMAIL_INPUT', required: true}],
      },
      onSuccess: sentPromptId,
      onIncomplete: failureTarget,
      onFailure: failureTarget,
    },
    {
      id: sentPromptId,
      type: FlowNodeType.PROMPT,
      meta: {
        components: [
          {
            type: 'TEXT',
            id: 'text_magic_link_sent_heading',
            label: 'Check your email',
            variant: 'HEADING_1',
          },
          {
            type: 'TEXT',
            id: 'text_magic_link_sent_desc',
            label: "We've sent a sign-in link to your email address.",
            variant: 'BODY',
          },
        ],
      },
      next: verifyId,
    },
    {
      id: verifyId,
      type: FlowNodeType.TASK_EXECUTION,
      executor: {name: 'MagicLinkExecutor', mode: 'verify'},
      onSuccess: onVerified,
      onFailure: failureTarget,
    },
  ];

  return nodes;
}

/** Horizontal gap between graph layers (stage reached from `start`), in canvas pixels. */
const LAYOUT_COLUMN_WIDTH = 500;
/** Vertical gap between nodes that land in the same layer, in canvas pixels. */
const LAYOUT_ROW_HEIGHT = 250;
const LAYOUT_BASE_X = 62;
const LAYOUT_BASE_Y = 300;

/** Default canvas size per node type, used when a node has no more specific size of its own. */
const LAYOUT_NODE_SIZE: Partial<Record<FlowNodeType, {width: number; height: number}>> = {
  [FlowNodeType.START]: {width: 101, height: 34},
  [FlowNodeType.END]: {width: 85, height: 34},
  [FlowNodeType.PROMPT]: {width: 350, height: 600},
  [FlowNodeType.TASK_EXECUTION]: {width: 200, height: 113},
};

/** IDs of the nodes a given node can transition to, across every edge shape a FlowNode supports. */
function getOutgoingNodeIds(node: FlowNode): string[] {
  const ids: string[] = [];
  if (node.onSuccess) ids.push(node.onSuccess);
  if (node.onFailure) ids.push(node.onFailure);
  if (node.onIncomplete) ids.push(node.onIncomplete);
  if (node.next) ids.push(node.next);
  if (node.condition?.onSkip) ids.push(node.condition.onSkip);
  node.prompts?.forEach((prompt) => {
    if (prompt.action?.nextNode) ids.push(prompt.action.nextNode);
  });
  return ids;
}

/**
 * Assigns each node a `layout` (position and size) so the flow doesn't open with every node
 * stacked on the canvas origin. Nodes are laid out in columns by their BFS distance from `start`
 * (so parallel first-factor chains land side by side) and stacked vertically within a column when
 * more than one node shares the same distance. Cycles (e.g. an OTP "resend" looping back to the
 * generate step) are safe: BFS only assigns a column the first time a node is reached, so a back
 * edge just doesn't move it.
 */
function assignLayout(nodes: FlowNode[]): void {
  const byId = new Map(nodes.map((node) => [node.id, node]));
  const column = new Map<string, number>([['start', 0]]);
  const queue: string[] = ['start'];

  while (queue.length > 0) {
    const currentId = queue.shift()!;
    const current = byId.get(currentId);
    if (!current) continue;
    const currentColumn = column.get(currentId)!;

    getOutgoingNodeIds(current).forEach((nextId) => {
      if (!column.has(nextId)) {
        column.set(nextId, currentColumn + 1);
        queue.push(nextId);
      }
    });
  }

  const rowCountByColumn = new Map<number, number>();

  nodes.forEach((node) => {
    const col = column.get(node.id) ?? 0;
    const row = rowCountByColumn.get(col) ?? 0;
    rowCountByColumn.set(col, row + 1);

    node.layout = {
      size: LAYOUT_NODE_SIZE[node.type] ?? {width: 200, height: 113},
      position: {
        x: LAYOUT_BASE_X + col * LAYOUT_COLUMN_WIDTH,
        y: LAYOUT_BASE_Y + row * LAYOUT_ROW_HEIGHT,
      },
    };
  });
}

/**
 * Generates a CreateFlowRequest with a complete flow graph based on selected authentication methods.
 * Supports any combination of Basic, Passkey, Magic Link, Google, GitHub, and SMS.
 *
 * @param options - Selected authentication methods and IDs
 * @returns Complete flow creation request payload
 */
export default function generateFlowGraph(options: FlowGeneratorOptions): CreateFlowRequest {
  const {
    appName,
    hasCredentialsAuth,
    hasPasskey,
    hasMagicLink = false,
    googleIdpId,
    githubIdpId,
    hasSmsOtp,
    relyingPartyId,
    relyingPartyName,
    hasEmailOtpMfa = false,
    hasSmsOtpMfa = false,
    smsOtpSenderId,
  } = options;

  // 1. Generate Flow Name and Handle. The name is app name + flow type (e.g. "React Quickstart
  // Sign-in Flow"), matching the flow type this generator always produces (FlowType.AUTHENTICATION).
  // The handle adds a random suffix so generating a flow more than once (retrying after a failure,
  // or creating a second application with the same name) never collides with an existing flow's
  // handle for this flow type — the backend rejects duplicate handles per flow type (FLM-1013).
  const randomSuffix = Math.random().toString(36).slice(2, 8);
  const name = appName ? `${appName} Sign-in Flow` : 'Generated Sign-in Flow';
  const handle = `${slugify(name)}-${randomSuffix}`;

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
  if (hasPasskey && !hasCredentialsAuth && !hasMagicLink && !googleIdpId && !githubIdpId && !hasSmsOtp) {
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

  // Magic Link Block — an alternative first-factor button, routing to its own dedicated
  // email-entry screen (magic_link_prompt_email), not straight to an executor like the other
  // buttons here, since it needs its own multi-step chain (see buildMagicLinkChain).
  if (hasMagicLink) {
    metaComponents.push({
      type: 'BLOCK',
      id: 'block_magic_link',
      components: [
        {
          type: 'ACTION',
          id: 'action_magic_link',
          label: 'Sign in with Magic Link',
          variant: hasCredentialsAuth || hasPasskey ? 'SECONDARY' : 'PRIMARY',
          eventType: 'SUBMIT',
        },
      ],
    });

    promptPrompts.push({
      action: {
        ref: 'action_magic_link',
        nextNode: 'magic_link_prompt_email',
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

  // Connect PROMPT node
  nodes.push({
    id: promptNodeId,
    type: FlowNodeType.PROMPT,
    meta: {
      components: metaComponents,
    },
    prompts: promptPrompts,
  });

  // 3. MFA Subgraph — every first factor below routes its success here instead of straight to
  // authorization_check when MFA is enabled. See buildMfaChannelChain for why no phone/email input
  // is collected: the recipient resolves from the already-authenticated user.
  const mfaNodes: FlowNode[] = [];
  let postAuthNodeId = 'authorization_check';

  if (hasEmailOtpMfa && hasSmsOtpMfa) {
    const emailChain = buildMfaChannelChain('email', '_email', undefined, 'authorization_check');
    const smsChain = buildMfaChannelChain('sms', '_sms', smsOtpSenderId, 'authorization_check');

    mfaNodes.push(
      {
        id: 'mfa_choose_channel',
        type: FlowNodeType.PROMPT,
        meta: {
          components: [
            {type: 'TEXT', id: 'text_mfa_choose_heading', label: 'Verify your identity', variant: 'HEADING_3'},
            {
              type: 'BLOCK',
              id: 'block_mfa_choose',
              components: [
                {
                  type: 'ACTION',
                  id: 'action_mfa_choose_email',
                  label: 'Email me a code',
                  variant: 'PRIMARY',
                  eventType: 'TRIGGER',
                },
                {
                  type: 'ACTION',
                  id: 'action_mfa_choose_sms',
                  label: 'Text me a code',
                  variant: 'SECONDARY',
                  eventType: 'TRIGGER',
                },
              ],
            },
          ],
        },
        prompts: [
          {action: {ref: 'action_mfa_choose_email', nextNode: emailChain.entryNodeId}},
          {action: {ref: 'action_mfa_choose_sms', nextNode: smsChain.entryNodeId}},
        ],
      },
      ...emailChain.nodes,
      ...smsChain.nodes,
    );
    postAuthNodeId = 'mfa_choose_channel';
  } else if (hasEmailOtpMfa) {
    const emailChain = buildMfaChannelChain('email', '', undefined, 'authorization_check');
    mfaNodes.push(...emailChain.nodes);
    postAuthNodeId = emailChain.entryNodeId;
  } else if (hasSmsOtpMfa) {
    const smsChain = buildMfaChannelChain('sms', '', smsOtpSenderId, 'authorization_check');
    mfaNodes.push(...smsChain.nodes);
    postAuthNodeId = smsChain.entryNodeId;
  }

  // 4. Executor Nodes

  // Credentials Auth Executor
  if (hasCredentialsAuth) {
    nodes.push({
      id: 'credentials_auth',
      type: FlowNodeType.TASK_EXECUTION,
      executor: {
        name: 'CredentialsAuthExecutor',
      },
      onSuccess: postAuthNodeId,
      onIncomplete: promptNodeId,
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
      onSuccess: postAuthNodeId,
    });
  }

  // Magic Link chain — built only now that postAuthNodeId is finalized, so verifying the link
  // routes into the same MFA-or-assert continuation every other first factor uses.
  if (hasMagicLink) {
    nodes.push(...buildMagicLinkChain(postAuthNodeId));
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
        onSkip: postAuthNodeId,
      },
      executor: {
        name: 'ProvisioningExecutor',
      },
      onSuccess: postAuthNodeId,
    });
  }

  // MFA Subgraph nodes, built above — appended once every first-factor executor has been wired
  // to route into them.
  nodes.push(...mfaNodes);

  // Authorization Executor — evaluates the OAuth request's requested permission scopes against the
  // authenticated user's roles and groups, recording the permitted subset as authorized_permissions.
  // AuthAssertExecutor only ever asserts permissions that passed this evaluation, so every completed
  // authentication path must route through here for resource scopes to reach the issued token.
  nodes.push({
    id: 'authorization_check',
    type: FlowNodeType.TASK_EXECUTION,
    executor: {
      name: 'AuthorizationExecutor',
    },
    onSuccess: 'auth_assert',
  });

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

  // 5. Position every node on the canvas so the flow doesn't open with everything stacked at (0,0).
  assignLayout(nodes);

  return {
    name,
    handle,
    flowType: FlowType.AUTHENTICATION,
    nodes,
  };
}
