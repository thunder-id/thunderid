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

import userEvent from '@testing-library/user-event';
import {render, screen} from '@thunderid/test-utils';
import type {NavigateFunction, Params} from 'react-router';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {TrustedIssuer} from '../../models/trusted-issuer';
import TrustedIssuerDetailPage from '../TrustedIssuerDetailPage';

const {mockMutate, mockRefetch} = vi.hoisted(() => ({mockMutate: vi.fn(), mockRefetch: vi.fn()}));

const TRUSTED_ISSUER: TrustedIssuer = {
  id: 'ti-1',
  name: 'Acme Okta',
  issuer: 'https://acme.okta.com',
  jwksEndpoint: 'https://acme.okta.com/keys',
  idJagEnabled: true,
};

vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: vi.fn(),
    useParams: vi.fn(),
  };
});

vi.mock('../../api/useTrustedIssuer', () => ({
  default: vi.fn(),
}));

vi.mock('../../api/useUpdateTrustedIssuer', () => ({
  default: () => ({mutate: mockMutate, isPending: false}),
}));

vi.mock('../../components/TrustedIssuerDeleteDialog', () => ({
  default: function StubTrustedIssuerDeleteDialog({open, onSuccess}: {open: boolean; onSuccess?: () => void}) {
    return open ? (
      <div data-testid="stub-delete-dialog">
        <button type="button" onClick={onSuccess}>
          Simulate delete success
        </button>
      </div>
    ) : null;
  },
}));

const {useNavigate, useParams} = await import('react-router');
const {default: useTrustedIssuer} = await import('../../api/useTrustedIssuer');

describe('TrustedIssuerDetailPage', () => {
  let mockNavigate: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockNavigate = vi.fn();
    mockMutate.mockReset();
    mockRefetch.mockReset();
    vi.mocked(useNavigate).mockReturnValue(mockNavigate as unknown as NavigateFunction);
    vi.mocked(useParams).mockReturnValue({id: 'ti-1'} as unknown as Params);
    vi.mocked(useTrustedIssuer).mockReturnValue({
      data: TRUSTED_ISSUER,
      isLoading: false,
      isError: false,
      refetch: mockRefetch,
    } as unknown as ReturnType<typeof useTrustedIssuer>);
  });

  it('should not render a client id field', () => {
    render(<TrustedIssuerDetailPage />);

    expect(screen.queryByLabelText(/^Client ID/)).not.toBeInTheDocument();
  });

  it('should render the ID-JAG card title and enabled note when assertions are accepted', () => {
    render(<TrustedIssuerDetailPage />);

    expect(
      screen.getByRole('heading', {name: 'Identity Assertion JWT Authorization Grant (ID-JAG)'}),
    ).toBeInTheDocument();
    expect(
      screen.getByText('Identity assertions from this issuer are accepted via the ID-JAG protocol.'),
    ).toBeInTheDocument();
  });

  it('should not render capability chips next to the page title', () => {
    render(<TrustedIssuerDetailPage />);

    expect(screen.queryByText('Token exchange')).not.toBeInTheDocument();
    expect(screen.queryByText('Inactive')).not.toBeInTheDocument();
    expect(screen.queryByText('ID-JAG')).not.toBeInTheDocument();
  });

  it('should hide the ID-JAG enabled note when assertions are not accepted', () => {
    vi.mocked(useTrustedIssuer).mockReturnValue({
      data: {...TRUSTED_ISSUER, idJagEnabled: false},
      isLoading: false,
      isError: false,
      refetch: mockRefetch,
    } as unknown as ReturnType<typeof useTrustedIssuer>);

    render(<TrustedIssuerDetailPage />);

    expect(
      screen.queryByText('Identity assertions from this issuer are accepted via the ID-JAG protocol.'),
    ).not.toBeInTheDocument();
  });

  it('should show the unsaved changes bar and save the updated name', async () => {
    const user = userEvent.setup();
    render(<TrustedIssuerDetailPage />);

    expect(screen.queryByTestId('save-bar')).not.toBeInTheDocument();

    const nameField = screen.getByLabelText(/^Name/);
    await user.clear(nameField);
    await user.type(nameField, 'Updated Acme Okta');

    expect(await screen.findByText('You have unsaved changes')).toBeInTheDocument();

    await user.click(screen.getByText('Save changes'));

    expect(mockMutate).toHaveBeenCalledWith(expect.objectContaining({name: 'Updated Acme Okta'}), expect.any(Object));
  });

  it('should navigate back to connections when Back is clicked', async () => {
    const user = userEvent.setup();
    render(<TrustedIssuerDetailPage />);

    await user.click(screen.getByRole('button', {name: /back to connections/i}));

    expect(mockNavigate).toHaveBeenCalledWith('/connections');
  });

  it('should navigate to connections when the trusted issuer is deleted', async () => {
    const user = userEvent.setup();
    render(<TrustedIssuerDetailPage />);

    await user.click(screen.getByTestId('trusted-issuer-delete-button'));
    await user.click(screen.getByRole('button', {name: /simulate delete success/i}));

    expect(mockNavigate).toHaveBeenCalledWith('/connections');
  });
});
