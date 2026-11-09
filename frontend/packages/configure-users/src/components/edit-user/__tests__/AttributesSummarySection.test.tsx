// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-return */
import {render, screen, within} from '@thunderid/test-utils';
import type {User} from '@thunderid/types';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import AttributesSummarySection from '../AttributesSummarySection';

const mockUseGetUserTypes = vi.fn();
const mockUseGetUserType = vi.fn();

vi.mock('@/api/useGetUserTypes', () => ({
  default: () => mockUseGetUserTypes(),
}));

vi.mock('@/api/useGetUserType', () => ({
  default: (id?: string) => mockUseGetUserType(id),
}));

describe('AttributesSummarySection', () => {
  const baseUser: User = {
    id: 'user-1',
    ouId: 'ou-1',
    type: 'employee',
    attributes: {email: 'a@b.com', count: 5, isAdmin: true, tags: ['a', 'b']},
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
        schema: {email: {type: 'string', required: true}, count: {type: 'number'}},
      },
      isLoading: false,
    });
  });

  it('shows a loading spinner while the schema is loading', () => {
    mockUseGetUserType.mockReturnValue({data: undefined, isLoading: true});
    render(<AttributesSummarySection user={baseUser} />);

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('shows the attribute key and formatted value for each attribute', () => {
    render(<AttributesSummarySection user={baseUser} />);

    expect(screen.getByText('email')).toBeInTheDocument();
    expect(screen.getByText('a@b.com')).toBeInTheDocument();
    expect(screen.getByText('count')).toBeInTheDocument();
    expect(screen.getByText('5')).toBeInTheDocument();
    expect(screen.getByText('isAdmin')).toBeInTheDocument();
    expect(screen.getByText('Yes')).toBeInTheDocument();
    expect(screen.getByText('tags')).toBeInTheDocument();
    expect(screen.getByText('a, b')).toBeInTheDocument();
  });

  it('shows attributes in the schema order with the ones not in the schema last', () => {
    mockUseGetUserType.mockReturnValue({
      data: {
        id: 'schema-1',
        handle: 'employee',
        displayName: 'Employee',
        schema: {
          sub: {type: 'string'},
          email: {type: 'string', required: true},
          given_name: {type: 'string'},
          username: {type: 'string', required: true},
        },
      },
      isLoading: false,
    });
    render(
      <AttributesSummarySection
        user={{
          ...baseUser,
          attributes: {nickname: 'bo', sub: 's-1', email: 'a@b.com', given_name: 'Bob', username: 'bob'},
        }}
      />,
    );

    const labels = screen.getAllByText(/^(nickname|sub|email|given_name|username)$/).map((el) => el.textContent);
    expect(labels).toEqual(['username', 'given_name', 'email', 'sub', 'nickname']);
  });

  it('shows an empty state when there are no attribute values', () => {
    render(<AttributesSummarySection user={{...baseUser, attributes: {}}} />);

    expect(screen.getByText('No attributes available')).toBeInTheDocument();
  });

  it('never shows an edit action', () => {
    render(<AttributesSummarySection user={baseUser} />);

    expect(screen.queryByRole('button', {name: 'Edit'})).not.toBeInTheDocument();
  });

  it('shows the resolved display name for attributes when the schema defines one', () => {
    mockUseGetUserType.mockReturnValue({
      data: {
        id: 'schema-1',
        handle: 'employee',
        displayName: 'Employee',
        schema: {email: {type: 'string', displayName: 'Email Address'}},
      },
      isLoading: false,
    });
    render(<AttributesSummarySection user={{...baseUser, attributes: {email: 'a@b.com'}}} />);

    expect(screen.getByText('Email Address')).toBeInTheDocument();
  });

  it('shows "No" for false boolean values', () => {
    render(<AttributesSummarySection user={{...baseUser, attributes: {active: false}}} />);

    expect(screen.getByText('No')).toBeInTheDocument();
  });

  it('displays dash for null attribute values', () => {
    render(<AttributesSummarySection user={{...baseUser, attributes: {middleName: null}}} />);

    expect(screen.getByText('middleName').parentElement).toHaveTextContent('-');
  });

  it('displays dash for undefined attribute values', () => {
    render(<AttributesSummarySection user={{...baseUser, attributes: {nickname: undefined}}} />);

    expect(screen.getByText('nickname').parentElement).toHaveTextContent('-');
  });

  it('displays JSON string for object attributes', () => {
    render(<AttributesSummarySection user={{...baseUser, attributes: {address: {city: 'New York'}}}} />);

    expect(screen.getByText('{"city":"New York"}')).toBeInTheDocument();
  });

  it('displays dash for unknown attribute types', () => {
    render(<AttributesSummarySection user={{...baseUser, attributes: {unknownType: Symbol('test')}}} />);

    expect(screen.getByText('unknownType').parentElement).toHaveTextContent('-');
  });

  describe('contact verification', () => {
    const contactUser: User = {
      id: 'user-1',
      ouId: 'ou-1',
      type: 'employee',
      attributes: {email: 'alex@example.com', mobile: '+94771234567', username: 'alex'},
      verifiedAttributes: {email: {verifiedAt: '2026-10-16T08:30:00Z'}},
    };

    beforeEach(() => {
      mockUseGetUserType.mockReturnValue({
        data: {
          id: 'schema-1',
          handle: 'employee',
          displayName: 'Employee',
          schema: {email: {type: 'email'}, mobile: {type: 'mobile'}, username: {type: 'string'}},
        },
        isLoading: false,
      });
    });

    const entry = (label: string): HTMLElement => screen.getByText(label).parentElement!;

    it('shows each stored contact as Verified or Unverified beside its label', () => {
      render(<AttributesSummarySection user={contactUser} />);

      expect(within(entry('email')).getByText('Verified')).toBeInTheDocument();
      expect(within(entry('mobile')).getByText('Unverified')).toBeInTheDocument();
      expect(within(entry('username')).queryByText(/Verified|Unverified/)).not.toBeInTheDocument();
    });

    it('offers no Verify action', () => {
      render(<AttributesSummarySection user={contactUser} />);

      expect(screen.queryByRole('button', {name: /Verify/})).not.toBeInTheDocument();
    });

    it('shows no status for a read-only user', () => {
      render(<AttributesSummarySection user={{...contactUser, isReadOnly: true}} />);

      expect(screen.getByText('alex@example.com')).toBeInTheDocument();
      expect(screen.queryByText(/Verified|Unverified/)).not.toBeInTheDocument();
    });
  });
});
