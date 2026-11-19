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

  it('offers only username and user ID, storing userID and removing username as the default', async () => {
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

  it('stores userID when chosen, dropping the provisioning options the server refuses with it', async () => {
    const user = userEvent.setup();
    const resource = createResource({
      sourceKey: 'extension.pin_check.verifiedUserId',
      allowAuthenticationWithoutLocalUser: true,
      attributeMapping: {email: 'extension.pin_check.email'},
    });
    render(<ExtensionAuthResolverProperties resource={resource} onChange={mockOnChange} />);

    await user.click(screen.getByRole('combobox'));
    await user.click(screen.getByRole('option', {name: 'User ID'}));

    expect(mockOnChange).toHaveBeenCalledWith(
      'data.properties',
      {sourceKey: 'extension.pin_check.verifiedUserId', matchAttribute: 'userID'},
      resource,
    );
  });

  it('hides the provisioning options when matching by user ID', () => {
    const resource = createResource({sourceKey: 'extension.pin_check.verifiedUserId', matchAttribute: 'userID'});
    render(<ExtensionAuthResolverProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.queryByText('Allow authentication without a local user')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Add attribute'})).not.toBeInTheDocument();
  });

  it('follows the stored mapping when it changes from outside', () => {
    const resource = createResource({
      sourceKey: 'extension.pin_check.username',
      attributeMapping: {email: 'extension.pin_check.email'},
    });
    const {rerender} = render(<ExtensionAuthResolverProperties resource={resource} onChange={mockOnChange} />);

    const undone = createResource({
      sourceKey: 'extension.pin_check.username',
      attributeMapping: {given_name: 'extension.pin_check.givenName'},
    });
    rerender(<ExtensionAuthResolverProperties resource={undone} onChange={mockOnChange} />);

    expect(screen.queryByDisplayValue('email')).not.toBeInTheDocument();
    expect(screen.getByDisplayValue('given_name')).toBeInTheDocument();
  });

  it('shows a new attribute row without storing it until it is complete', async () => {
    const user = userEvent.setup();
    const resource = createResource({
      sourceKey: 'extension.pin_check.username',
      attributeMapping: {email: 'extension.pin_check.email'},
    });
    render(<ExtensionAuthResolverProperties resource={resource} onChange={mockOnChange} />);

    expect(screen.getByDisplayValue('email')).toBeInTheDocument();
    expect(screen.getByDisplayValue('extension.pin_check.email')).toBeInTheDocument();

    await user.click(screen.getByRole('button', {name: 'Add attribute'}));
    await user.click(screen.getByRole('button', {name: 'Add attribute'}));

    expect(screen.getAllByRole('button', {name: 'Delete'})).toHaveLength(3);
    expect(mockOnChange).toHaveBeenLastCalledWith(
      'data.properties.attributeMapping',
      {email: 'extension.pin_check.email'},
      resource,
    );
  });

  it('stores a new attribute once both its key and its source key are given', async () => {
    const user = userEvent.setup();
    const resource = createResource({sourceKey: 'extension.pin_check.username'});
    render(<ExtensionAuthResolverProperties resource={resource} onChange={mockOnChange} />);

    await user.click(screen.getByRole('button', {name: 'Add attribute'}));
    await user.type(screen.getByPlaceholderText('email'), 'given_name');
    await user.tab();
    await user.type(screen.getByPlaceholderText('extension.pin_check.email'), 'extension.pin_check.givenName');
    await user.tab();

    expect(mockOnChange).toHaveBeenLastCalledWith(
      'data.properties.attributeMapping',
      {given_name: 'extension.pin_check.givenName'},
      resource,
    );
  });

  it('keeps the first row when two rows name the same attribute', async () => {
    const user = userEvent.setup();
    const resource = createResource({
      sourceKey: 'extension.pin_check.username',
      attributeMapping: {email: 'extension.pin_check.email'},
    });
    render(<ExtensionAuthResolverProperties resource={resource} onChange={mockOnChange} />);

    await user.click(screen.getByRole('button', {name: 'Add attribute'}));
    const keyFields = screen.getAllByPlaceholderText('email');
    const valueFields = screen.getAllByPlaceholderText('extension.pin_check.email');
    await user.type(keyFields[1], 'email');
    await user.tab();
    await user.type(valueFields[1], 'extension.pin_check.other');
    await user.tab();

    expect(screen.getAllByRole('button', {name: 'Delete'})).toHaveLength(2);
    expect(
      screen.getByText('Each user attribute can be mapped once. Only the first row for an attribute is kept.'),
    ).toBeInTheDocument();
    expect(mockOnChange).toHaveBeenLastCalledWith(
      'data.properties.attributeMapping',
      {email: 'extension.pin_check.email'},
      resource,
    );
  });

  it('removes the mapping when the last row is removed', async () => {
    const user = userEvent.setup();
    const resource = createResource({
      sourceKey: 'extension.pin_check.username',
      attributeMapping: {email: 'extension.pin_check.email'},
    });
    render(<ExtensionAuthResolverProperties resource={resource} onChange={mockOnChange} />);

    await user.click(screen.getByRole('button', {name: 'Delete'}));

    expect(mockOnChange).toHaveBeenLastCalledWith(
      'data.properties',
      {sourceKey: 'extension.pin_check.username'},
      resource,
    );
  });
});
