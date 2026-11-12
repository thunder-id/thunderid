// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-return */
import {render, screen, userEvent, waitFor, within} from '@thunderid/test-utils';
import type {User} from '@thunderid/types';
import {isEqualIgnoringEmpty} from '@thunderid/utils';
import {useCallback, useState, type JSX, type ReactNode} from 'react';
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

vi.mock('@/api/useGetAttributeVerificationFlow', () => ({
  default: () => ({data: 'default-admin-attribute-verification-flow'}),
}));

vi.mock('@/utils/renderSchemaField', () => ({
  default: (
    fieldName: string,
    _fieldDef: unknown,
    control: Control<Record<string, unknown>>,
    _errors: unknown,
    _resolveDisplayName: unknown,
    labelSuffix?: ReactNode,
    endAdornment?: ReactNode,
  ) => (
    <div key={fieldName} data-testid={`field-${fieldName}`}>
      {labelSuffix}
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
      {endAdornment}
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

  describe('contact verification', () => {
    const VERIFIED_AT = '2026-10-16T08:30:00Z';
    const contactUser: User = {
      id: 'user-1',
      ouId: 'ou-1',
      type: 'employee',
      attributes: {
        email: 'alex@example.com',
        workEmail: 'alex@work.example.com',
        mobile: '+94771234567',
        username: 'alex',
      },
      verifiedAttributes: {email: {verifiedAt: VERIFIED_AT}},
    };

    beforeEach(() => {
      mockUseGetUserType.mockReturnValue({
        data: {
          id: 'schema-1',
          handle: 'employee',
          displayName: 'Employee',
          schema: {
            email: {type: 'email'},
            workEmail: {type: 'email', displayName: 'Work Email'},
            mobile: {type: 'mobile'},
            username: {type: 'string'},
          },
        },
        isLoading: false,
      });
    });

    const field = (name: string): HTMLElement => screen.getByTestId(`field-${name}`);

    it('shows each stored contact as Verified or Unverified and no status on other attributes', () => {
      render(<EditUserAttributes user={contactUser} editedUser={{}} onFieldChange={mockOnFieldChange} />);

      expect(within(field('email')).getByText('Verified')).toBeInTheDocument();
      expect(within(field('workEmail')).getByText('Unverified')).toBeInTheDocument();
      expect(within(field('mobile')).getByText('Unverified')).toBeInTheDocument();
      expect(within(field('username')).queryByText(/Verified|Unverified/)).not.toBeInTheDocument();
    });

    it('offers a verification link beside the unverified email only', () => {
      render(<EditUserAttributes user={contactUser} editedUser={{}} onFieldChange={mockOnFieldChange} />);

      expect(
        within(field('workEmail')).getByRole('button', {name: 'Send verification link for Work Email'}),
      ).toBeInTheDocument();
      expect(screen.getAllByRole('button', {name: /^Send verification link/})).toHaveLength(1);
    });

    it('disables the verification link for a suspended user', () => {
      render(
        <EditUserAttributes
          user={{...contactUser, status: {value: 'SUSPENDED'}}}
          editedUser={{}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(
        within(field('workEmail')).getByRole('button', {name: 'Send verification link for Work Email'}),
      ).toBeDisabled();
    });

    it('hides the verification link while the email holds an edited value', async () => {
      const user = userEvent.setup();
      render(<EditUserAttributes user={contactUser} editedUser={{}} onFieldChange={mockOnFieldChange} />);

      await user.type(screen.getByLabelText('workEmail-input'), 'x');

      expect(screen.queryByRole('button', {name: /^Send verification link/})).not.toBeInTheDocument();
      expect(within(field('workEmail')).getByText('Save to verify')).toBeInTheDocument();
    });

    it('asks to save an edited verified email before verifying it', async () => {
      const user = userEvent.setup();
      render(<EditUserAttributes user={contactUser} editedUser={{}} onFieldChange={mockOnFieldChange} />);

      expect(within(field('email')).queryByText('Save to verify')).not.toBeInTheDocument();
      await user.type(screen.getByLabelText('email-input'), 'x');

      expect(within(field('email')).getByText('Save to verify')).toBeInTheDocument();
    });

    it('shows no save hint for a cleared email, an edited mobile or a suspended user', async () => {
      const user = userEvent.setup();
      const {unmount} = render(
        <EditUserAttributes user={contactUser} editedUser={{}} onFieldChange={mockOnFieldChange} />,
      );

      await user.clear(screen.getByLabelText('workEmail-input'));
      await user.type(screen.getByLabelText('mobile-input'), '1');

      expect(screen.queryByText('Save to verify')).not.toBeInTheDocument();
      unmount();

      render(
        <EditUserAttributes
          user={{...contactUser, status: {value: 'SUSPENDED'}}}
          editedUser={{}}
          onFieldChange={mockOnFieldChange}
        />,
      );
      await user.type(screen.getByLabelText('workEmail-input'), 'x');

      expect(screen.queryByText('Save to verify')).not.toBeInTheDocument();
    });

    it('hides the status while a contact holds an edited value, keeps the input, and restores it on revert', async () => {
      const user = userEvent.setup();
      render(<EditUserAttributes user={contactUser} editedUser={{}} onFieldChange={mockOnFieldChange} />);

      const input = screen.getByLabelText('workEmail-input');
      await user.clear(input);
      await user.type(input, 'new@example.com');

      expect(input).toHaveValue('new@example.com');
      expect(within(field('workEmail')).queryByText('Unverified')).not.toBeInTheDocument();
      expect(within(field('mobile')).getByText('Unverified')).toBeInTheDocument();

      await user.clear(input);
      await user.type(input, 'alex@work.example.com');

      expect(within(field('workEmail')).getByText('Unverified')).toBeInTheDocument();
    });

    it('hides the status of a verified contact once its value is edited', async () => {
      const user = userEvent.setup();
      render(<EditUserAttributes user={contactUser} editedUser={{}} onFieldChange={mockOnFieldChange} />);

      await user.type(screen.getByLabelText('email-input'), 'x');

      expect(within(field('email')).queryByText('Verified')).not.toBeInTheDocument();
    });

    it('shows no status for a contact without a stored value', () => {
      render(
        <EditUserAttributes
          user={{...contactUser, attributes: {username: 'alex', email: ''}}}
          editedUser={{}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.queryByText(/Verified|Unverified/)).not.toBeInTheDocument();
    });

    it('shows no verification for a read-only user', () => {
      render(
        <EditUserAttributes
          user={{...contactUser, isReadOnly: true}}
          editedUser={{}}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByText('alex@work.example.com')).toBeInTheDocument();
      expect(screen.queryByText(/Verified|Unverified/)).not.toBeInTheDocument();
    });
  });
});
