// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Edge, Node} from '@xyflow/react';
import {describe, it, expect} from 'vitest';
import resolveUpstreamOtpLength, {DEFAULT_OTP_LENGTH} from '../resolveUpstreamOtpLength';

const generateNode = (id: string, otpLength?: unknown): Node => ({
  id,
  position: {x: 0, y: 0},
  data: {
    action: {executor: {name: 'OTPExecutor', mode: 'generate'}},
    ...(otpLength === undefined ? {} : {properties: {otpLength}}),
  },
});

const plainNode = (id: string, executorName?: string, mode?: string): Node => ({
  id,
  position: {x: 0, y: 0},
  data: executorName ? {action: {executor: {name: executorName, mode}}} : {},
});

const edge = (source: string, target: string): Edge => ({id: `${source}-${target}`, source, target});

describe('resolveUpstreamOtpLength', () => {
  it('reads the length from the generate node feeding the step', () => {
    const nodes = [generateNode('generate', 8), plainNode('prompt')];
    const edges = [edge('generate', 'prompt')];

    expect(resolveUpstreamOtpLength('prompt', nodes, edges)).toBe(8);
  });

  it('walks through intermediate nodes to reach the generate node', () => {
    const nodes = [generateNode('generate', 4), plainNode('send', 'SMSExecutor'), plainNode('prompt')];
    const edges = [edge('generate', 'send'), edge('send', 'prompt')];

    expect(resolveUpstreamOtpLength('prompt', nodes, edges)).toBe(4);
  });

  it('picks the nearest generate node when a flow has several OTP legs', () => {
    const nodes = [
      generateNode('generate_email', 4),
      plainNode('prompt_email'),
      generateNode('generate_sms', 8),
      plainNode('prompt_sms'),
    ];
    const edges = [
      edge('generate_email', 'prompt_email'),
      edge('prompt_email', 'generate_sms'),
      edge('generate_sms', 'prompt_sms'),
    ];

    expect(resolveUpstreamOtpLength('prompt_sms', nodes, edges)).toBe(8);
    expect(resolveUpstreamOtpLength('prompt_email', nodes, edges)).toBe(4);
  });

  it('falls back to the default when the generate node has no configured length', () => {
    const nodes = [generateNode('generate'), plainNode('prompt')];
    const edges = [edge('generate', 'prompt')];

    expect(resolveUpstreamOtpLength('prompt', nodes, edges)).toBe(DEFAULT_OTP_LENGTH);
  });

  it.each([[3], [11], ['abc'], [0], [6.5]])('falls back to the default for an unusable length %s', (otpLength) => {
    const nodes = [generateNode('generate', otpLength), plainNode('prompt')];
    const edges = [edge('generate', 'prompt')];

    expect(resolveUpstreamOtpLength('prompt', nodes, edges)).toBe(DEFAULT_OTP_LENGTH);
  });

  it('falls back to the default when the step has no upstream generate node', () => {
    const nodes = [plainNode('start'), plainNode('prompt')];
    const edges = [edge('start', 'prompt')];

    expect(resolveUpstreamOtpLength('prompt', nodes, edges)).toBe(DEFAULT_OTP_LENGTH);
  });

  it('falls back to the default for an unwired step', () => {
    expect(resolveUpstreamOtpLength('prompt', [plainNode('prompt')], [])).toBe(DEFAULT_OTP_LENGTH);
  });

  it('ignores a verify mode OTP node', () => {
    const nodes = [plainNode('verify', 'OTPExecutor', 'verify'), plainNode('prompt')];
    const edges = [edge('verify', 'prompt')];

    expect(resolveUpstreamOtpLength('prompt', nodes, edges)).toBe(DEFAULT_OTP_LENGTH);
  });

  it('terminates on a cyclic graph', () => {
    const nodes = [plainNode('a'), plainNode('b')];
    const edges = [edge('a', 'b'), edge('b', 'a')];

    expect(resolveUpstreamOtpLength('a', nodes, edges)).toBe(DEFAULT_OTP_LENGTH);
  });
});
