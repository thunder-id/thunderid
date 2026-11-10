// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {getSampleTemplates} from '../sample-templates';
import {CONTEXT_TOKEN_KEYS} from '../template-tokens';

const ctxKeys = (text: string): string[] => [...text.matchAll(/\{\{ctx\(([A-Za-z0-9_.-]+)\)\}\}/g)].map((m) => m[1]);

describe('attribute verification samples', () => {
  // The verification flows send the link as `inviteLink`, so the samples must render it.
  it.each(['email-self-email-verification', 'email-admin-initiated-email-verification'])(
    '%s is an email sample that renders the verification link',
    (id) => {
      const sample = getSampleTemplates('email').find((candidate) => candidate.id === id);

      expect(sample).toBeDefined();
      expect(sample?.subject).toBe('Verify your email address');
      expect(ctxKeys(sample?.body ?? '')).toEqual(['inviteLink', 'inviteLink', 'inviteLink']);
      expect(ctxKeys(sample?.body ?? '').every((key) => CONTEXT_TOKEN_KEYS.has(key))).toBe(true);
    },
  );
});
