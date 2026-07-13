/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

/* eslint-disable @typescript-eslint/no-unsafe-return */
import {render, screen} from '@thunderid/test-utils';
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
    type: 'Employee',
    attributes: {email: 'a@b.com', count: 5, isAdmin: true, tags: ['a', 'b']},
  };

  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetUserTypes.mockReturnValue({
      data: {types: [{id: 'schema-1', name: 'Employee', ouId: 'ou-1'}]},
    });
    mockUseGetUserType.mockReturnValue({
      data: {
        id: 'schema-1',
        name: 'Employee',
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
        name: 'Employee',
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
});
