// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import executors from '../../../../../data/executors.json';
import type {Resource} from '../../../../../models/resources';
import ExtensionAuthResolverProperties from '../ExtensionAuthResolverProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, fallback?: unknown) => (typeof fallback === 'string' ? fallback : key),
  }),
}));

const {mockFieldError} = vi.hoisted(() => ({mockFieldError: vi.fn<(resourceId?: string, field?: string) => string>()}));
vi.mock('../../../../../hooks/useResourceFieldError', () => ({
  default: (resourceId?: string, field?: string): string => mockFieldError(resourceId, field),
}));

const createResource = (properties: Record<string, unknown> = {}): Resource =>
  ({
    id: 'resolve-user',
    data: {
      action: {executor: {name: 'ExtensionAuthResolver'}},
      properties,
    },
  }) as unknown as Resource;

describe('ExtensionAuthResolverProperties', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockFieldError.mockReturnValue('');
  });

  it('is offered in the flow builder with an empty source key', () => {
    const entry = executors.find((e) => e.data.action.executor.name === 'ExtensionAuthResolver');

    expect(entry?.data.properties).toEqual({sourceKey: ''});
    expect(entry?.display.showOnResourcePanel).toBe(true);
  });

  it('stores allowAuthenticationWithoutLocalUser as true when ticked', async () => {
    const user = userEvent.setup();
    const resource = createResource({sourceKey: 'extension.pin_check.username'});
    render(<ExtensionAuthResolverProperties resource={resource} onChange={mockOnChange} />);

    const checkbox = screen.getByRole('checkbox', {name: 'Allow authentication without a local user'});
    expect(checkbox).not.toBeChecked();
    await user.click(checkbox);

    expect(mockOnChange).toHaveBeenCalledWith('data.properties.allowAuthenticationWithoutLocalUser', true, resource);
  });

  it('shows a stored allowAuthenticationWithoutLocalUser as ticked and stores false when cleared', async () => {
    const user = userEvent.setup();
    const resource = createResource({
      sourceKey: 'extension.pin_check.username',
      allowAuthenticationWithoutLocalUser: true,
    });
    render(<ExtensionAuthResolverProperties resource={resource} onChange={mockOnChange} />);

    const checkbox = screen.getByRole('checkbox', {name: 'Allow authentication without a local user'});
    expect(checkbox).toBeChecked();
    await user.click(checkbox);

    expect(mockOnChange).toHaveBeenCalledWith('data.properties.allowAuthenticationWithoutLocalUser', false, resource);
  });

  it('stores the trimmed source key on blur', async () => {
    const user = userEvent.setup();
    const resource = createResource({sourceKey: ''});
    render(<ExtensionAuthResolverProperties resource={resource} onChange={mockOnChange} />);

    await user.type(screen.getByLabelText(/Source key/), '  extension.pin_check.verifiedUserId ');
    await user.tab();

    expect(mockOnChange).toHaveBeenCalledWith(
      'data.properties.sourceKey',
      'extension.pin_check.verifiedUserId',
      resource,
    );
  });

  it('shows the save error for the source key in place of the hint', () => {
    mockFieldError.mockImplementation((_id, field) =>
      field === 'data.properties.sourceKey' ? 'Source key is required' : '',
    );
    render(<ExtensionAuthResolverProperties resource={createResource({sourceKey: ''})} onChange={mockOnChange} />);

    expect(screen.getByText('Source key is required')).toBeInTheDocument();
    expect(mockFieldError).toHaveBeenCalledWith('resolve-user', 'data.properties.sourceKey');
  });

  it('offers only attributes users cannot change, storing userID and removing username as the default', async () => {
    const user = userEvent.setup();
    const resource = createResource({sourceKey: 'extension.pin_check.verifiedUserId', matchAttribute: 'userID'});
    render(<ExtensionAuthResolverProperties resource={resource} onChange={mockOnChange} />);

    await user.click(screen.getByRole('combobox'));
    const options = screen.getAllByRole('option').map((option) => option.textContent);
    expect(options).toEqual(['Username', 'User ID']);
    await user.click(screen.getByRole('option', {name: 'Username'}));
    expect(mockOnChange).toHaveBeenLastCalledWith(
      'data.properties',
      {sourceKey: 'extension.pin_check.verifiedUserId'},
      resource,
    );
  });

  it('stores userID when chosen', async () => {
    const user = userEvent.setup();
    const resource = createResource({sourceKey: 'extension.pin_check.verifiedUserId'});
    render(<ExtensionAuthResolverProperties resource={resource} onChange={mockOnChange} />);

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', {name: 'User ID'}));

    expect(mockOnChange).toHaveBeenCalledWith('data.properties.matchAttribute', 'userID', resource);
  });

  it('adds an attribute mapping row and removes the mapping when the last row is removed', async () => {
    const user = userEvent.setup();
    const resource = createResource({
      sourceKey: 'extension.pin_check.username',
      attributeMapping: {email: 'extension.pin_check.email'},
    });
    render(<ExtensionAuthResolverProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.getByDisplayValue('email')).toBeInTheDocument();
    expect(screen.getByDisplayValue('extension.pin_check.email')).toBeInTheDocument();

    await user.click(screen.getByRole('button', {name: 'Add attribute'}));
    expect(mockOnChange).toHaveBeenCalledWith(
      'data.properties.attributeMapping',
      {email: 'extension.pin_check.email', '': ''},
      resource,
    );

    await user.click(screen.getByRole('button', {name: 'Delete'}));
    expect(mockOnChange).toHaveBeenLastCalledWith(
      'data.properties',
      {sourceKey: 'extension.pin_check.username'},
      resource,
    );
  });
});
