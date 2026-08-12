// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import UserAccessSection from '../UserAccessSection';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string) => fallback ?? key,
  }),
}));

describe('UserAccessSection', () => {
  const mockOnUserTypesChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  const mockUserTypes = [
    {id: '1', name: 'Internal', ouId: 'INTERNAL', allowSelfRegistration: true},
    {id: '2', name: 'External', ouId: 'EXTERNAL', allowSelfRegistration: false},
  ];

  it('renders nothing when fewer than two user types exist', () => {
    const singleUserType = [{id: '1', name: 'Internal', ouId: 'INTERNAL', allowSelfRegistration: true}];

    const {container} = render(
      <UserAccessSection userTypes={singleUserType} selectedUserTypes={[]} onUserTypesChange={mockOnUserTypesChange} />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it('renders the master checkbox title when two or more user types exist', () => {
    render(
      <UserAccessSection
        userTypes={mockUserTypes}
        selectedUserTypes={['Internal', 'External']}
        onUserTypesChange={mockOnUserTypesChange}
      />,
    );

    expect(screen.getByText('Allow all user types to sign up for this application')).toBeInTheDocument();
  });

  it('checks the master checkbox when every user type is selected', () => {
    render(
      <UserAccessSection
        userTypes={mockUserTypes}
        selectedUserTypes={['Internal', 'External']}
        onUserTypesChange={mockOnUserTypesChange}
      />,
    );

    expect(screen.getAllByRole('checkbox')[0]).toBeChecked();
  });

  it('marks the master checkbox indeterminate when only some user types are selected', () => {
    render(
      <UserAccessSection
        userTypes={mockUserTypes}
        selectedUserTypes={['Internal']}
        onUserTypesChange={mockOnUserTypesChange}
      />,
    );

    const masterCheckboxInput = screen.getAllByRole('checkbox')[0] as HTMLInputElement;
    expect(masterCheckboxInput).toHaveAttribute('data-indeterminate', 'true');
  });

  it('selects every user type when the master checkbox is checked', async () => {
    const user = userEvent.setup();
    render(
      <UserAccessSection userTypes={mockUserTypes} selectedUserTypes={[]} onUserTypesChange={mockOnUserTypesChange} />,
    );

    await user.click(screen.getAllByRole('checkbox')[0]);

    expect(mockOnUserTypesChange).toHaveBeenCalledWith(['Internal', 'External']);
  });

  it('clears the selection when the fully-checked master checkbox is unchecked', async () => {
    const user = userEvent.setup();
    render(
      <UserAccessSection
        userTypes={mockUserTypes}
        selectedUserTypes={['Internal', 'External']}
        onUserTypesChange={mockOnUserTypesChange}
      />,
    );

    await user.click(screen.getAllByRole('checkbox')[0]);

    expect(mockOnUserTypesChange).toHaveBeenCalledWith([]);
  });

  it('shows an error message when no user type is selected', () => {
    render(
      <UserAccessSection userTypes={mockUserTypes} selectedUserTypes={[]} onUserTypesChange={mockOnUserTypesChange} />,
    );

    expect(screen.getByText('Please select at least one user type')).toBeInTheDocument();
  });

  it('does not show an error message once a user type is selected', () => {
    render(
      <UserAccessSection
        userTypes={mockUserTypes}
        selectedUserTypes={['Internal']}
        onUserTypesChange={mockOnUserTypesChange}
      />,
    );

    expect(screen.queryByText('Please select at least one user type')).not.toBeInTheDocument();
  });

  it('toggles an individual user type checkbox', async () => {
    const user = userEvent.setup();
    render(
      <UserAccessSection
        userTypes={mockUserTypes}
        selectedUserTypes={['Internal']}
        onUserTypesChange={mockOnUserTypesChange}
      />,
    );

    await user.click(screen.getByRole('button', {name: 'Expand'}));
    const externalCheckbox = screen.getByRole('checkbox', {name: 'External'});
    await user.click(externalCheckbox);

    expect(mockOnUserTypesChange).toHaveBeenCalledWith(['Internal', 'External']);
  });

  it('removes an individual user type when its checkbox is unchecked', async () => {
    const user = userEvent.setup();
    render(
      <UserAccessSection
        userTypes={mockUserTypes}
        selectedUserTypes={['Internal', 'External']}
        onUserTypesChange={mockOnUserTypesChange}
      />,
    );

    await user.click(screen.getByRole('button', {name: 'Expand'}));
    const internalCheckbox = screen.getByRole('checkbox', {name: 'Internal'});
    await user.click(internalCheckbox);

    expect(mockOnUserTypesChange).toHaveBeenCalledWith(['External']);
  });

  it('expands and collapses via the chevron button', async () => {
    const user = userEvent.setup();
    render(
      <UserAccessSection
        userTypes={mockUserTypes}
        selectedUserTypes={['Internal']}
        onUserTypesChange={mockOnUserTypesChange}
      />,
    );

    const toggle = screen.getByRole('button', {name: 'Expand'});
    await user.click(toggle);

    expect(screen.getByRole('button', {name: 'Collapse'})).toBeInTheDocument();
  });

  describe('with more than 5 user types', () => {
    const manyUserTypes = [
      {id: '1', name: 'Type1', ouId: 'TYPE1', allowSelfRegistration: true},
      {id: '2', name: 'Type2', ouId: 'TYPE2', allowSelfRegistration: false},
      {id: '3', name: 'Type3', ouId: 'TYPE3', allowSelfRegistration: true},
      {id: '4', name: 'Type4', ouId: 'TYPE4', allowSelfRegistration: false},
      {id: '5', name: 'Type5', ouId: 'TYPE5', allowSelfRegistration: true},
      {id: '6', name: 'Type6', ouId: 'TYPE6', allowSelfRegistration: false},
    ];

    it('renders an autocomplete instead of individual checkboxes', async () => {
      const user = userEvent.setup();
      render(
        <UserAccessSection
          userTypes={manyUserTypes}
          selectedUserTypes={['Type1']}
          onUserTypesChange={mockOnUserTypesChange}
        />,
      );

      await user.click(screen.getByRole('button', {name: 'Expand'}));

      expect(screen.getByRole('combobox')).toBeInTheDocument();
      expect(screen.queryByRole('checkbox', {name: 'Type2'})).not.toBeInTheDocument();
    });

    it('allows selecting user types from the autocomplete', async () => {
      const user = userEvent.setup();
      render(
        <UserAccessSection
          userTypes={manyUserTypes}
          selectedUserTypes={['Type1']}
          onUserTypesChange={mockOnUserTypesChange}
        />,
      );

      await user.click(screen.getByRole('button', {name: 'Expand'}));
      const autocomplete = screen.getByRole('combobox');
      await user.click(autocomplete);

      const type2Option = await screen.findByText('Type2');
      await user.click(type2Option);

      expect(mockOnUserTypesChange).toHaveBeenCalledWith(['Type1', 'Type2']);
    });
  });

  describe('with exactly 5 user types', () => {
    const fiveUserTypes = [
      {id: '1', name: 'Type1', ouId: 'TYPE1', allowSelfRegistration: true},
      {id: '2', name: 'Type2', ouId: 'TYPE2', allowSelfRegistration: false},
      {id: '3', name: 'Type3', ouId: 'TYPE3', allowSelfRegistration: true},
      {id: '4', name: 'Type4', ouId: 'TYPE4', allowSelfRegistration: false},
      {id: '5', name: 'Type5', ouId: 'TYPE5', allowSelfRegistration: true},
    ];

    it('still renders individual checkboxes rather than an autocomplete', async () => {
      const user = userEvent.setup();
      render(
        <UserAccessSection
          userTypes={fiveUserTypes}
          selectedUserTypes={['Type1']}
          onUserTypesChange={mockOnUserTypesChange}
        />,
      );

      await user.click(screen.getByRole('button', {name: 'Expand'}));

      expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
      expect(screen.getByRole('checkbox', {name: 'Type5'})).toBeInTheDocument();
    });
  });
});
