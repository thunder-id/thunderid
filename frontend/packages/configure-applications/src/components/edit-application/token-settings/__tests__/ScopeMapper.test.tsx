// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, expect, it, vi, beforeEach} from 'vitest';
import ScopeMapper from '../ScopeMapper';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({t: (key: string, fallback?: string) => fallback ?? key}),
}));

vi.mock('../../../../constants/token-constants', () => ({
  default: {
    DEFAULT_TOKEN_ATTRIBUTES: ['sub', 'iss', 'aud', 'exp', 'iat'],
    ADDITIONAL_USER_ATTRIBUTES: [],
  },
}));

const defaultProps = {
  scopeClaims: {openid: [], profile: []},
  userAttributes: ['email', 'username', 'given_name'],
  isLoadingUserAttributes: false,
  onScopeClaimsChange: vi.fn(),
};

describe('ScopeMapper', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('prompts to add a scope when the mapping is empty', () => {
    render(
      <ScopeMapper
        scopeClaims={{}}
        userAttributes={[]}
        isLoadingUserAttributes={false}
        onScopeClaimsChange={vi.fn()}
      />,
    );

    expect(screen.getByText('Add a scope to start mapping attributes.')).toBeInTheDocument();
  });

  it('renders the mapped scopes in the left panel', () => {
    render(<ScopeMapper {...defaultProps} />);

    expect(screen.getByText('openid')).toBeInTheDocument();
    expect(screen.getByText('profile')).toBeInTheDocument();
  });

  it('clicking a scope in the left panel selects it and shows its mapping panel', async () => {
    const user = userEvent.setup();
    render(<ScopeMapper {...defaultProps} />);

    const profileScope = screen.getByRole('button', {name: /profile/i});
    await user.click(profileScope);

    expect(screen.getByText('Mapped Attributes')).toBeInTheDocument();
    expect(screen.getByText('Available Attributes')).toBeInTheDocument();
  });

  it('shows "Mapped Attributes" and "Available Attributes" labels when a scope is selected', () => {
    render(<ScopeMapper {...defaultProps} />);

    // First scope is auto-selected on mount
    expect(screen.getByText('Mapped Attributes')).toBeInTheDocument();
    expect(screen.getByText('Available Attributes')).toBeInTheDocument();
  });

  it('clicking an available attribute chip calls onScopeClaimsChange with it added to the selected scope', async () => {
    const onScopeClaimsChange = vi.fn();
    const user = userEvent.setup();

    render(<ScopeMapper {...defaultProps} scopeClaims={{openid: []}} onScopeClaimsChange={onScopeClaimsChange} />);

    // email is an available attribute (not in DEFAULT_TOKEN_ATTRIBUTES mock)
    const availableSection = screen.getByText('Available Attributes').closest('div')!.parentElement!;
    await user.click(within(availableSection).getByText('email'));

    expect(onScopeClaimsChange).toHaveBeenCalledWith({openid: ['email']});
  });

  it('clicking delete on a mapped attribute calls onScopeClaimsChange with it removed', async () => {
    const onScopeClaimsChange = vi.fn();
    const user = userEvent.setup();

    render(
      <ScopeMapper {...defaultProps} scopeClaims={{openid: ['email']}} onScopeClaimsChange={onScopeClaimsChange} />,
    );

    const mappedSection = screen.getByText('Mapped Attributes').closest('div')!.parentElement!;
    const emailChipRoot = within(mappedSection).getByText('email').closest('.MuiChip-root')!;
    const deleteIcon = emailChipRoot.querySelector('.MuiChip-deleteIcon')!;

    await user.click(deleteIcon);

    expect(onScopeClaimsChange).toHaveBeenCalledWith({openid: []});
  });

  it('shows loading message when isLoadingUserAttributes is true', () => {
    render(<ScopeMapper {...defaultProps} isLoadingUserAttributes />);

    expect(screen.getByText('Loading available attributes...')).toBeInTheDocument();
  });

  it('auto-selects the first scope on mount', () => {
    render(<ScopeMapper {...defaultProps} />);

    // Mapped Attributes panel visible means a scope was auto-selected
    expect(screen.getByText('Mapped Attributes')).toBeInTheDocument();
  });

  it('shows "all mapped" message when all available attributes are already mapped to the selected scope', () => {
    render(
      <ScopeMapper
        {...defaultProps}
        scopeClaims={{openid: ['email', 'given_name', 'username']}}
        userAttributes={['email', 'given_name', 'username']}
      />,
    );

    expect(screen.getByText('All available attributes are already mapped to this scope')).toBeInTheDocument();
  });

  it('adds a custom scope from the input', async () => {
    const onScopeClaimsChange = vi.fn();
    const user = userEvent.setup();

    render(<ScopeMapper {...defaultProps} scopeClaims={{openid: []}} onScopeClaimsChange={onScopeClaimsChange} />);

    await user.type(screen.getByPlaceholderText('e.g. custom:read'), 'custom:read');
    await user.click(screen.getByRole('button', {name: 'Add'}));

    expect(onScopeClaimsChange).toHaveBeenCalledWith({'custom:read': [], openid: []});
  });

  it('rejects a custom scope that is already mapped', async () => {
    const onScopeClaimsChange = vi.fn();
    const user = userEvent.setup();

    render(<ScopeMapper {...defaultProps} scopeClaims={{openid: []}} onScopeClaimsChange={onScopeClaimsChange} />);

    await user.type(screen.getByPlaceholderText('e.g. custom:read'), 'openid');
    await user.click(screen.getByRole('button', {name: 'Add'}));

    expect(screen.getByText('This scope is already added')).toBeInTheDocument();
    expect(onScopeClaimsChange).not.toHaveBeenCalled();
  });

  it('removing a scope deletes its entry from the mapping', async () => {
    const onScopeClaimsChange = vi.fn();
    const user = userEvent.setup();

    render(
      <ScopeMapper
        {...defaultProps}
        scopeClaims={{openid: ['email'], profile: []}}
        onScopeClaimsChange={onScopeClaimsChange}
      />,
    );

    const removeButtons = screen.getAllByRole('button', {name: 'Remove scope'});
    await user.click(removeButtons[1]);

    expect(onScopeClaimsChange).toHaveBeenCalledWith({openid: ['email']});
  });
});
