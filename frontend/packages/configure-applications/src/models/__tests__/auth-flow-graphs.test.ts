// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {AUTH_FLOW_GRAPHS, REGISTRATION_FLOW_GRAPHS} from '../auth-flow-graphs';

describe('AUTH_FLOW_GRAPHS', () => {
  it('should have BASIC flow graph', () => {
    expect(AUTH_FLOW_GRAPHS.BASIC).toBe('auth_flow_config_basic');
  });

  it('should have GOOGLE flow graph', () => {
    expect(AUTH_FLOW_GRAPHS.GOOGLE).toBe('auth_flow_config_google');
  });

  it('should have GITHUB flow graph', () => {
    expect(AUTH_FLOW_GRAPHS.GITHUB).toBe('auth_flow_config_github');
  });

  it('should have BASIC_GOOGLE flow graph', () => {
    expect(AUTH_FLOW_GRAPHS.BASIC_GOOGLE).toBe('auth_flow_config_basic_google');
  });

  it('should have BASIC_GOOGLE_GITHUB flow graph', () => {
    expect(AUTH_FLOW_GRAPHS.BASIC_GOOGLE_GITHUB).toBe('auth_flow_config_basic_google_github');
  });

  it('should have BASIC_GOOGLE_GITHUB_SMS flow graph', () => {
    expect(AUTH_FLOW_GRAPHS.BASIC_GOOGLE_GITHUB_SMS).toBe('auth_flow_config_basic_google_github_sms');
  });

  it('should have BASIC_WITH_PROMPT flow graph', () => {
    expect(AUTH_FLOW_GRAPHS.BASIC_WITH_PROMPT).toBe('auth_flow_config_basic_with_prompt');
  });

  it('should have SMS flow graph', () => {
    expect(AUTH_FLOW_GRAPHS.SMS).toBe('auth_flow_config_sms');
  });

  it('should have SMS_WITH_USERNAME flow graph', () => {
    expect(AUTH_FLOW_GRAPHS.SMS_WITH_USERNAME).toBe('auth_flow_config_sms_with_username');
  });

  it('should have all expected properties', () => {
    const expectedKeys = [
      'BASIC',
      'GOOGLE',
      'GITHUB',
      'BASIC_GOOGLE',
      'BASIC_GOOGLE_GITHUB',
      'BASIC_GOOGLE_GITHUB_SMS',
      'BASIC_WITH_PROMPT',
      'SMS',
      'SMS_WITH_USERNAME',
    ];

    expect(Object.keys(AUTH_FLOW_GRAPHS)).toEqual(expectedKeys);
  });
});

describe('REGISTRATION_FLOW_GRAPHS', () => {
  it('should have BASIC registration flow graph', () => {
    expect(REGISTRATION_FLOW_GRAPHS.BASIC).toBe('registration_flow_config_basic');
  });

  it('should have BASIC_GOOGLE_GITHUB registration flow graph', () => {
    expect(REGISTRATION_FLOW_GRAPHS.BASIC_GOOGLE_GITHUB).toBe('registration_flow_config_basic_google_github');
  });

  it('should have BASIC_GOOGLE_GITHUB_SMS registration flow graph', () => {
    expect(REGISTRATION_FLOW_GRAPHS.BASIC_GOOGLE_GITHUB_SMS).toBe('registration_flow_config_basic_google_github_sms');
  });

  it('should have SMS registration flow graph', () => {
    expect(REGISTRATION_FLOW_GRAPHS.SMS).toBe('registration_flow_config_sms');
  });

  it('should have all expected properties', () => {
    const expectedKeys = ['BASIC', 'BASIC_GOOGLE_GITHUB', 'BASIC_GOOGLE_GITHUB_SMS', 'SMS'];

    expect(Object.keys(REGISTRATION_FLOW_GRAPHS)).toEqual(expectedKeys);
  });
});
