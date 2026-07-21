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

/* eslint-disable @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access */
import userEvent from '@testing-library/user-event';
import {render, screen, waitFor} from '@thunderid/test-utils';
import type {NavigateFunction} from 'react-router';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import TrustedIssuerCreateForm from '../TrustedIssuerCreateForm';

const {mockMutate} = vi.hoisted(() => ({mockMutate: vi.fn()}));

vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: vi.fn(),
  };
});

vi.mock('../../api/useCreateTrustedIssuer', () => ({
  default: () => ({mutate: mockMutate, isPending: false}),
}));

const {useNavigate} = await import('react-router');

describe('TrustedIssuerCreateForm', () => {
  let mockNavigate: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockNavigate = vi.fn();
    mockMutate.mockReset();
    vi.mocked(useNavigate).mockReturnValue(mockNavigate as unknown as NavigateFunction);
  });

  it('should render the form with the ID-JAG switch off and token exchange switch on by default', () => {
    render(<TrustedIssuerCreateForm />);

    expect(screen.getByLabelText(/^Name/)).toBeInTheDocument();
    expect(screen.getByLabelText(/^Issuer URI/)).toBeInTheDocument();
    expect(screen.getByLabelText(/^JWKS endpoint/)).toBeInTheDocument();
    expect(screen.getByRole('switch', {name: /id-jag/i})).not.toBeChecked();
    expect(screen.getByRole('switch', {name: /enable token exchange/i})).toBeChecked();
  });

  it('should have no back or cancel affordance of its own', () => {
    render(<TrustedIssuerCreateForm />);

    expect(screen.queryByRole('button', {name: /back/i})).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: /^cancel$/i})).not.toBeInTheDocument();
  });

  it('should disable the submit button until all required fields are valid', async () => {
    const user = userEvent.setup();
    render(<TrustedIssuerCreateForm />);

    const submitButton = screen.getByTestId('trusted-issuer-create-submit');
    expect(submitButton).toBeDisabled();

    await user.type(screen.getByLabelText(/^Name/), 'Acme Okta');
    expect(submitButton).toBeDisabled();

    await user.type(screen.getByLabelText(/^Issuer URI/), 'https://acme.okta.com');
    expect(submitButton).toBeDisabled();

    await user.type(screen.getByLabelText(/^JWKS endpoint/), 'https://acme.okta.com/keys');
    expect(submitButton).toBeEnabled();
  });

  it('should show a validation error when an issuer URI is not https', async () => {
    const user = userEvent.setup();
    render(<TrustedIssuerCreateForm />);

    const issuerField = screen.getByLabelText(/^Issuer URI/);
    await user.type(issuerField, 'http://acme.okta.com');
    await user.tab();

    expect(await screen.findByText('Enter a valid https:// URL.')).toBeInTheDocument();
  });

  it('should show a required error when a field is left blank on blur', async () => {
    const user = userEvent.setup();
    render(<TrustedIssuerCreateForm />);

    const nameField = screen.getByLabelText(/^Name/);
    await user.click(nameField);
    await user.tab();

    expect(await screen.findByText('This field is required.')).toBeInTheDocument();
  });

  it('should submit the form and navigate to the detail page on success', async () => {
    const user = userEvent.setup();
    mockMutate.mockImplementation((_data, opts) => {
      opts.onSuccess({
        id: 'ti-1',
        name: 'Acme Okta',
        issuer: 'https://acme.okta.com',
        jwksEndpoint: 'https://acme.okta.com/keys',
        idJagEnabled: true,
      });
    });

    render(<TrustedIssuerCreateForm />);

    await user.type(screen.getByLabelText(/^Name/), 'Acme Okta');
    await user.type(screen.getByLabelText(/^Issuer URI/), 'https://acme.okta.com');
    await user.type(screen.getByLabelText(/^JWKS endpoint/), 'https://acme.okta.com/keys');
    await user.click(screen.getByTestId('trusted-issuer-create-submit'));

    expect(mockMutate).toHaveBeenCalledWith(
      {
        name: 'Acme Okta',
        issuer: 'https://acme.okta.com',
        jwksEndpoint: 'https://acme.okta.com/keys',
        idJagEnabled: false,
        tokenExchangeEnabled: true,
        trustedTokenAudience: undefined,
      },
      expect.any(Object),
    );

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/trusted-issuers/ti-1');
    });
  });

  it('should show an inline duplicate-name error on 409 conflict without navigating', async () => {
    const user = userEvent.setup();
    mockMutate.mockImplementation((_data, opts) => {
      opts.onError({response: {status: 409}});
    });

    render(<TrustedIssuerCreateForm />);

    await user.type(screen.getByLabelText(/^Name/), 'Acme Okta');
    await user.type(screen.getByLabelText(/^Issuer URI/), 'https://acme.okta.com');
    await user.type(screen.getByLabelText(/^JWKS endpoint/), 'https://acme.okta.com/keys');
    await user.click(screen.getByTestId('trusted-issuer-create-submit'));

    expect(await screen.findByText('A trusted issuer with this name already exists.')).toBeInTheDocument();
    expect(mockNavigate).not.toHaveBeenCalledWith(expect.stringContaining('/trusted-issuers/'));
  });

  it('should turn on ID-JAG when the switch is toggled', async () => {
    const user = userEvent.setup();
    mockMutate.mockImplementation((_data, opts) => {
      opts.onSuccess({
        id: 'ti-2',
        name: 'Beta AD',
        issuer: 'https://beta.example.com',
        jwksEndpoint: 'https://beta.example.com/keys',
        idJagEnabled: true,
      });
    });

    render(<TrustedIssuerCreateForm />);

    await user.click(screen.getByRole('switch', {name: /id-jag/i}));
    await user.type(screen.getByLabelText(/^Name/), 'Beta AD');
    await user.type(screen.getByLabelText(/^Issuer URI/), 'https://beta.example.com');
    await user.type(screen.getByLabelText(/^JWKS endpoint/), 'https://beta.example.com/keys');
    await user.click(screen.getByTestId('trusted-issuer-create-submit'));

    expect(mockMutate).toHaveBeenCalledWith(expect.objectContaining({idJagEnabled: true}), expect.any(Object));
  });

  it('should not render a client id field', () => {
    render(<TrustedIssuerCreateForm />);

    expect(screen.queryByLabelText(/^Client ID/)).not.toBeInTheDocument();
  });
});
