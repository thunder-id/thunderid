// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-return */
import {render, screen, userEvent, waitFor} from '@thunderid/test-utils';
import type {User} from '@thunderid/types';
import {isEqualIgnoringEmpty} from '@thunderid/utils';
import {useCallback, useState, type JSX} from 'react';
import {Controller, type Control} from 'react-hook-form';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import EditUserAttributes from '../EditUserAttributes';

const mockUseGetUserTypes = vi.fn();
const mockUseGetUserType = vi.fn();

vi.mock('@/api/useGetUserTypes', () => ({
  default: () => mockUseGetUserTypes(),
}));

vi.mock('@/api/useGetUserType', () => ({
  default: (id?: string) => mockUseGetUserType(id),
}));

vi.mock('@/utils/renderSchemaField', () => ({
  default: (fieldName: string, _fieldDef: unknown, control: Control<Record<string, unknown>>) => (
    <div key={fieldName} data-testid={`field-${fieldName}`}>
      <Controller
        name={fieldName}
        control={control}
        render={({field}) => (
          <input
            aria-label={`${fieldName}-input`}
            value={typeof field.value === 'string' ? field.value : ''}
            onChange={(e) => field.onChange(e.target.value)}
          />
        )}
      />
    </div>
  ),
}));

// Mirrors useMutation: a fresh object every render, so a callback memoized on it churns.
const freshMutationResult = (): {isError: boolean; reset: () => void} => ({
  isError: false,
  reset: () => undefined,
});

describe('EditUserAttributes', () => {
  const mockOnFieldChange = vi.fn();
  const baseUser: User = {
    id: 'user-1',
    ouId: 'ou-1',
    type: 'employee',
    attributes: {email: 'a@b.com', count: 5},
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetUserTypes.mockReturnValue({
      data: {types: [{id: 'schema-1', handle: 'employee', displayName: 'Employee', ouId: 'ou-1'}]},
    });
    mockUseGetUserType.mockReturnValue({
      data: {
        id: 'schema-1',
        handle: 'employee',
        displayName: 'Employee',
        schema: {
          email: {type: 'string', required: true},
          count: {type: 'number'},
          // Credentials should be filtered out of the editable schema fields.
          password: {type: 'string', credential: true},
        },
      },
      isLoading: false,
    });
  });

  it('shows a loading spinner while the schema is loading', () => {
    mockUseGetUserType.mockReturnValue({data: undefined, isLoading: true});
    render(<EditUserAttributes user={baseUser} editedUser={{}} onFieldChange={mockOnFieldChange} />);

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('shows the edit form directly, with a field per editable schema entry', () => {
    render(<EditUserAttributes user={baseUser} editedUser={{}} onFieldChange={mockOnFieldChange} />);

    expect(screen.getByTestId('field-email')).toBeInTheDocument();
    expect(screen.getByTestId('field-count')).toBeInTheDocument();
    expect(screen.queryByTestId('field-password')).not.toBeInTheDocument();
  });

  it('has no local Save/Cancel controls — the page-level Save bar is the only save path', () => {
    render(<EditUserAttributes user={baseUser} editedUser={{}} onFieldChange={mockOnFieldChange} />);

    expect(screen.queryByRole('button', {name: /save/i})).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: /cancel/i})).not.toBeInTheDocument();
  });

  it('shows a message when the schema has no editable fields', () => {
    mockUseGetUserType.mockReturnValue({
      data: {
        id: 'schema-1',
        handle: 'employee',
        displayName: 'Employee',
        schema: {password: {type: 'string', credential: true}},
      },
      isLoading: false,
    });
    render(<EditUserAttributes user={baseUser} editedUser={{}} onFieldChange={mockOnFieldChange} />);

    expect(screen.getByText('No schema available for editing')).toBeInTheDocument();
  });

  it('stages the original attributes on mount without reporting a spurious change', async () => {
    render(<EditUserAttributes user={baseUser} editedUser={{}} onFieldChange={mockOnFieldChange} />);

    // The page treats staged values equal to the original (via isEqualIgnoringEmpty) as "no
    // change", so mounting must not stage anything that differs from user.attributes.
    await waitFor(() => {
      expect(mockOnFieldChange).toHaveBeenCalledWith('attributes', baseUser.attributes);
    });
    expect(mockOnFieldChange.mock.calls.every(([, value]) => isEqualIgnoringEmpty(value, baseUser.attributes))).toBe(
      true,
    );
  });

  it('stages a bounded number of times when the parent recreates onFieldChange on every render', async () => {
    // The loop that wedged the React root and broke navigation: staging re-rendered the page,
    // which produced a new onFieldChange, which refired this effect. Capped so a regression
    // fails instead of hanging the test browser.
    const CALL_CAP = 25;
    const staged = vi.fn();
    const identities = new Set<unknown>();

    function Harness(): JSX.Element {
      const [editedUser, setEditedUser] = useState<Partial<User>>({});
      const mutation = freshMutationResult();
      const onFieldChange = useCallback(
        (field: keyof User, value: unknown) => {
          if (mutation.isError) mutation.reset();
          staged(field, value);
          if (staged.mock.calls.length > CALL_CAP) return;
          setEditedUser((prev) => ({...prev, [field]: value}));
        },
        [mutation],
      );

      identities.add(onFieldChange);
      return <EditUserAttributes user={baseUser} editedUser={editedUser} onFieldChange={onFieldChange} />;
    }

    render(<Harness />);

    await screen.findByTestId('field-email');
    await waitFor(() => {
      expect(staged).toHaveBeenCalled();
    });

    // Guards the harness: without churn this test would pass vacuously.
    expect(identities.size).toBeGreaterThan(1);
    expect(staged.mock.calls.length).toBeLessThanOrEqual(3);
  });

  it('stages the merged attributes into the shared editedUser state as the user types', async () => {
    const user = userEvent.setup();
    render(<EditUserAttributes user={baseUser} editedUser={{}} onFieldChange={mockOnFieldChange} />);

    const input = screen.getByLabelText('email-input');
    await user.clear(input);
    await user.type(input, 'new@b.com');

    await waitFor(() => {
      expect(mockOnFieldChange).toHaveBeenLastCalledWith(
        'attributes',
        expect.objectContaining({email: 'new@b.com', count: 5}),
      );
    });
  });

  it('restores the original attributes when the user manually reverts an edit', async () => {
    const user = userEvent.setup();
    render(<EditUserAttributes user={baseUser} editedUser={{}} onFieldChange={mockOnFieldChange} />);

    const input = screen.getByLabelText('email-input');
    await user.clear(input);
    await user.type(input, 'new@b.com');
    await waitFor(() => {
      expect(mockOnFieldChange).toHaveBeenLastCalledWith('attributes', expect.objectContaining({email: 'new@b.com'}));
    });

    await user.clear(input);
    await user.type(input, 'a@b.com');
    await waitFor(() => {
      expect(mockOnFieldChange).toHaveBeenLastCalledWith('attributes', baseUser.attributes);
    });
  });

  it('uses editedUser.attributes over user.attributes as the starting values', () => {
    render(
      <EditUserAttributes
        user={baseUser}
        editedUser={{attributes: {email: 'pending@b.com', count: 5}}}
        onFieldChange={mockOnFieldChange}
      />,
    );

    expect(screen.getByLabelText('email-input')).toHaveValue('pending@b.com');
  });

  describe('read-only users', () => {
    it('shows the read-only attribute summary instead of the edit form', () => {
      render(
        <EditUserAttributes user={{...baseUser, isReadOnly: true}} editedUser={{}} onFieldChange={mockOnFieldChange} />,
      );

      expect(screen.getByText('email')).toBeInTheDocument();
      expect(screen.getByText('a@b.com')).toBeInTheDocument();
      expect(screen.queryByTestId('field-email')).not.toBeInTheDocument();
    });
  });
});
