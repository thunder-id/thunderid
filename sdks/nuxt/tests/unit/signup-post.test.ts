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

/* eslint-disable @typescript-eslint/typedef, sort-keys, @typescript-eslint/explicit-function-return-type */

import {readBody} from 'h3';
import {describe, it, expect, vi, beforeEach} from 'vitest';

// ── Imports (after mocks) ─────────────────────────────────────────────────────

import signupHandler from '../../src/runtime/server/routes/auth/session/signup.post';

// ── vi.hoisted ────────────────────────────────────────────────────────────────
const mockClientInstance = vi.hoisted(() => ({
  signUp: vi.fn<() => Promise<any>>().mockResolvedValue({flowStatus: 'INCOMPLETE'}),
}));

// ── Module mocks ──────────────────────────────────────────────────────────────

vi.mock('h3', () => ({
  defineEventHandler: (fn: Function) => fn,
  readBody: vi.fn(),
  createError: vi.fn((opts: any) => Object.assign(new Error(opts.statusMessage), opts)),
}));

vi.mock('#imports', () => ({
  useRuntimeConfig: vi.fn(() => ({
    thunderid: {sessionSecret: 'test-secret'},
    public: {
      thunderid: {afterSignInUrl: '/welcome'},
    },
  })),
}));

vi.mock('../../src/runtime/server/ThunderIDNuxtClient', () => ({
  default: {
    getInstance: () => mockClientInstance,
  },
}));

vi.mock('@thunderid/node', () => ({
  EmbeddedFlowStatus: {Complete: 'COMPLETE'},
}));

// ── Helpers ───────────────────────────────────────────────────────────────────

const mockEvent = {};

// ── Tests ─────────────────────────────────────────────────────────────────────

describe('POST /api/auth/signup', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockClientInstance.signUp.mockResolvedValue({flowStatus: 'INCOMPLETE'});
  });

  it('returns empty signUpUrl when no payload provided', async () => {
    vi.mocked(readBody).mockResolvedValue({});

    const result = await (signupHandler as any)(mockEvent);

    expect(result).toEqual({data: {signUpUrl: ''}, success: true});
  });

  it('returns step data when flow is incomplete', async () => {
    const stepPayload = {
      flowType: 'SIGNUP',
      flowId: 'signup-flow-abc',
      selectedAuthenticator: {authenticatorId: 'EmailOTPAuthenticator'},
      flowInputs: [{name: 'email', value: 'newuser@example.com'}],
    };
    const incompleteResponse = {
      flowStatus: 'INCOMPLETE',
      flowId: 'signup-flow-abc',
      nextStep: {authenticators: []},
    };
    mockClientInstance.signUp.mockResolvedValueOnce(incompleteResponse);
    vi.mocked(readBody).mockResolvedValue({payload: stepPayload});

    const result = await (signupHandler as any)(mockEvent);

    expect(result).toEqual({data: incompleteResponse, success: true});
  });

  it('returns afterSignUpUrl when flow is complete', async () => {
    const lastStepPayload = {
      flowType: 'SIGNUP',
      flowId: 'signup-flow-abc',
      selectedAuthenticator: {authenticatorId: 'EmailOTPAuthenticator'},
      flowInputs: [{name: 'otp', value: '123456'}],
    };
    mockClientInstance.signUp.mockResolvedValueOnce({flowStatus: 'COMPLETE'});
    vi.mocked(readBody).mockResolvedValue({payload: lastStepPayload});

    const result = await (signupHandler as any)(mockEvent);

    expect(result).toEqual({data: {afterSignUpUrl: '/welcome'}, success: true});
  });

  it('throws 502 when signUp execution fails', async () => {
    vi.mocked(readBody).mockResolvedValue({
      payload: {flowType: 'SIGNUP', flowId: 'f', selectedAuthenticator: {}, flowInputs: []},
    });
    mockClientInstance.signUp.mockRejectedValueOnce(new Error('upstream error'));

    await expect((signupHandler as any)(mockEvent)).rejects.toMatchObject({
      statusCode: 502,
    });
  });
});
