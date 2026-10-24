// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it, vi} from 'vitest';
import {FlowExecutionFailure} from '../../flow/administrationFlow';
import getErrorMessage from '../getErrorMessage';

describe('getErrorMessage', () => {
  const makeApiError = (code: string): Error =>
    Object.assign(new Error('request failed'), {
      response: {data: {code}},
    });

  it('should return the specific error message when the code resolves to a translation', () => {
    const t = vi.fn((key: string) =>
      key === 'errors.APP-1020' ? 'An application with this name already exists.' : '',
    );

    expect(getErrorMessage(makeApiError('APP-1020'), t, 'create.error')).toBe(
      'An application with this name already exists.',
    );
  });

  it('should fall back to the fallback key when no specific translation exists', () => {
    const t = vi.fn((key: string) => (key === 'create.error' ? 'Failed to create application.' : ''));

    expect(getErrorMessage(makeApiError('APP-9999'), t, 'create.error')).toBe('Failed to create application.');
  });

  it('should fall back to the fallback key when the error has no response code', () => {
    const t = vi.fn((key: string) => (key === 'create.error' ? 'Failed to create application.' : ''));

    expect(getErrorMessage(new Error('network error'), t, 'create.error')).toBe('Failed to create application.');
  });

  it('should call the fallback key without options when no fallbackDefaultValue is given', () => {
    const t = vi.fn().mockReturnValue('Failed to create application.');

    getErrorMessage(new Error('network error'), t, 'create.error');

    expect(t).toHaveBeenLastCalledWith('create.error');
  });

  it('should pass fallbackDefaultValue as the translation default when the key is missing', () => {
    const t = vi.fn((key: string, options?: {defaultValue: string}) =>
      key === 'errors.APP-9999' ? '' : (options?.defaultValue ?? ''),
    );

    expect(
      getErrorMessage(makeApiError('APP-9999'), t, 'create.error', 'Failed to create application. Please try again.'),
    ).toBe('Failed to create application. Please try again.');
    expect(t).toHaveBeenLastCalledWith('create.error', {
      defaultValue: 'Failed to create application. Please try again.',
    });
  });

  it('should prefer the specific translation over fallbackDefaultValue', () => {
    const t = vi.fn((key: string) =>
      key === 'errors.APP-1020' ? 'An application with this name already exists.' : '',
    );

    expect(
      getErrorMessage(makeApiError('APP-1020'), t, 'create.error', 'Failed to create application. Please try again.'),
    ).toBe('An application with this name already exists.');
  });

  it('should fall back to the shared common catalog when the feature namespace has no entry', () => {
    const t = vi.fn((key: string) =>
      key === 'common:errors.SSE-4030' ? 'You do not have permission to perform this action.' : '',
    );

    expect(getErrorMessage(makeApiError('SSE-4030'), t, 'update.error')).toBe(
      'You do not have permission to perform this action.',
    );
  });

  it('should prefer the feature namespace over the shared common catalog', () => {
    const t = vi.fn((key: string) => {
      if (key === 'errors.SSE-4030') return 'Feature-specific permission message.';
      if (key === 'common:errors.SSE-4030') return 'Generic permission message.';
      return '';
    });

    expect(getErrorMessage(makeApiError('SSE-4030'), t, 'update.error')).toBe('Feature-specific permission message.');
  });

  it('should fall back to the fallback key when neither namespace resolves the code', () => {
    const t = vi.fn((key: string) => (key === 'update.error' ? 'Failed to update. Please try again.' : ''));

    expect(getErrorMessage(makeApiError('XYZ-0000'), t, 'update.error')).toBe('Failed to update. Please try again.');
  });

  it('should not read a step code off an axios transport error', () => {
    const t = vi.fn((key: string, options?: Record<string, unknown>) =>
      key === 'errors.ERR_NETWORK' ? 'Unrelated message.' : ((options?.['defaultValue'] as string | undefined) ?? ''),
    );
    const error = Object.assign(new Error('Network Error'), {code: 'ERR_NETWORK', isAxiosError: true});

    expect(getErrorMessage(error, t, 'update.error', 'Failed to update.')).toBe('Failed to update.');
  });

  // A flow refusal arrives as a failed step inside a 200 response, so its code is on the thrown error
  // rather than on response.data.
  describe('for a refused administration flow step', () => {
    it('should resolve the step code to its localized message', () => {
      const t = vi.fn((key: string) =>
        key === 'common:errors.FET-1100' ? 'This role cannot be deleted in its current state.' : '',
      );
      const error = new FlowExecutionFailure(
        {code: 'FET-1100', message: {defaultValue: 'Role deletion not allowed'}},
        'The role deletion flow did not complete',
      );

      expect(getErrorMessage(error, t, 'delete.error', 'Failed to delete role.')).toBe(
        'This role cannot be deleted in its current state.',
      );
    });

    it('should prefer the feature namespace for a step code', () => {
      const t = vi.fn((key: string) => (key === 'errors.ROL-1020' ? 'The assignee no longer holds this role.' : ''));
      const error = new FlowExecutionFailure({code: 'ROL-1020'}, 'The role assignment removal flow did not complete');

      expect(getErrorMessage(error, t, 'assignments.remove.error')).toBe('The assignee no longer holds this role.');
    });

    it('should substitute the params the step carries on its message', () => {
      const t = vi.fn((key: string, options?: {max?: string}) =>
        key === 'common:errors.GRP-1017' ? `Nested more than ${options?.max} levels deep.` : '',
      );
      const error = new FlowExecutionFailure(
        {code: 'GRP-1017', message: {params: {max: '8'}}},
        'The group deletion flow did not complete',
      );

      expect(getErrorMessage(error, t, 'delete.error')).toBe('Nested more than 8 levels deep.');
    });

    it('should substitute the params the step carries on its description', () => {
      const t = vi.fn((key: string, options?: {max?: string}) =>
        key === 'common:errors.GRP-1017' ? `Nested more than ${options?.max} levels deep.` : '',
      );
      const error = new FlowExecutionFailure(
        {code: 'GRP-1017', description: {params: {max: '8'}}},
        'The group deletion flow did not complete',
      );

      expect(getErrorMessage(error, t, 'delete.error')).toBe('Nested more than 8 levels deep.');
    });

    it('should fall back to the fallback key when the step code is unknown', () => {
      const t = vi.fn((key: string, options?: Record<string, unknown>) =>
        key === 'delete.error' ? ((options?.['defaultValue'] as string | undefined) ?? '') : '',
      );
      const error = new FlowExecutionFailure({code: 'FET-9999'}, 'The role deletion flow did not complete');

      expect(getErrorMessage(error, t, 'delete.error', 'Failed to delete role.')).toBe('Failed to delete role.');
    });
  });
});
