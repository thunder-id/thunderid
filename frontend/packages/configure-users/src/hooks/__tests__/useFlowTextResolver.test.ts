// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@testing-library/react';
import {useTranslation} from 'react-i18next';
import {describe, it, expect, vi, beforeAll, beforeEach} from 'vitest';
import useFlowTextResolver from '../useFlowTextResolver';

const mockMeta: {value: unknown} = {value: null};

vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal();

  return {
    ...(actual as object),
    useThunderID: () => ({meta: mockMeta.value}),
  };
});

describe('useFlowTextResolver', () => {
  beforeAll(() => {
    // Mirrors what the Console's I18nProvider does with the payload of
    // `GET /i18n/languages/{language}/translations/resolve`: one i18next namespace per
    // server namespace, holding flat dot-joined keys.
    const {result} = renderHook(() => useTranslation());
    result.current.i18n.addResourceBundle(
      'en-US',
      'onboarding',
      {'forms.add_user.title': 'Add User Details'},
      true,
      true,
    );
  });

  beforeEach(() => {
    mockMeta.value = null;
  });

  it('resolves a flow translation literal against the Console i18next bundle', () => {
    const {result} = renderHook(() => useFlowTextResolver());

    expect(result.current('{{ t(onboarding:forms.add_user.title) }}')).toBe('Add User Details');
  });

  it('resolves literals embedded in surrounding text', () => {
    const {result} = renderHook(() => useFlowTextResolver());

    expect(result.current('Step: {{ t(onboarding:forms.add_user.title) }}')).toBe('Step: Add User Details');
  });

  it('does not resolve when the namespace separator is lost', () => {
    const {result} = renderHook(() => useFlowTextResolver());

    // Guards the dot-to-colon mapping: without it i18next looks the dotted key up in the
    // default namespace and misses, which is the #4755 symptom.
    const {result: i18nResult} = renderHook(() => useTranslation());
    expect(i18nResult.current.t('onboarding.forms.add_user.title')).toBe('onboarding.forms.add_user.title');
    expect(result.current('{{ t(onboarding:forms.add_user.title) }}')).not.toBe('onboarding.forms.add_user.title');
  });

  it('returns undefined for empty input', () => {
    const {result} = renderHook(() => useFlowTextResolver());

    expect(result.current(undefined)).toBeUndefined();
    expect(result.current('')).toBeUndefined();
  });

  it('leaves plain text untouched', () => {
    const {result} = renderHook(() => useFlowTextResolver());

    expect(result.current('Add User')).toBe('Add User');
  });

  it('resolves meta literals when flow metadata is available', () => {
    mockMeta.value = {application: {name: 'My App'}};
    const {result} = renderHook(() => useFlowTextResolver());

    expect(result.current('Sign in to {{ meta(application.name) }}')).toBe('Sign in to My App');
  });

  it('leaves meta literals untouched when flow metadata is absent', () => {
    const {result} = renderHook(() => useFlowTextResolver());

    expect(result.current('{{ meta(application.name) }}')).toBe('{{ meta(application.name) }}');
  });
});
