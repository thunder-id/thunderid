// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@thunderid/test-utils';
import {useEffect} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import ConnectionConfigureWizardPage from '../ConnectionConfigureWizardPage';

const mutateMock = vi.fn();
const resetMock = vi.fn();
const navigateMock = vi.fn();
const showToastMock = vi.fn();
const mockParams = {type: 'google'};
const mutationState = {isPending: false, isError: false};

vi.mock('react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router')>()),
  useNavigate: () => navigateMock,
  useParams: () => mockParams,
}));
vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useConfig: () => ({getGateCallbackUrl: () => 'https://id.acme.io/gate/callback'}),
  useToast: () => ({showToast: showToastMock}),
}));
vi.mock('../../api/useCreateConnection', () => ({
  default: () => ({mutate: mutateMock, reset: resetMock, ...mutationState}),
}));

vi.mock('../../components/ConnectionForm', () => ({
  default: function StubConnectionForm({onFieldChange}: {onFieldChange: (name: string, value: string) => void}) {
    useEffect(() => {
      // Populate both IdP and SMS fields; each type's form only reads the ones it declares.
      onFieldChange('clientId', 'x');
      onFieldChange('clientSecret', 's');
      onFieldChange('accountSid', 'AC00000000000000000000000000000000');
      onFieldChange('authToken', 's');
      onFieldChange('senderId', '+15005550006');
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);
    return (
      <div data-testid="stub-connection-form">
        <button type="button" data-testid="edit-client-id" onClick={() => onFieldChange('clientId', 'changed')}>
          edit
        </button>
      </div>
    );
  },
}));

describe('ConnectionConfigureWizardPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockParams.type = 'google';
    mutationState.isPending = false;
    mutationState.isError = false;
  });

  it('shows a single configure step and creates with the fixed vendor name', () => {
    render(<ConnectionConfigureWizardPage />);

    // Single step: the credentials form is shown with a Create button (no attribute-mapping step).
    expect(screen.getByTestId('stub-connection-form')).toBeInTheDocument();
    expect(screen.getByText('Configure your Google Login connection')).toBeInTheDocument();
    fireEvent.click(screen.getByTestId('wizard-create'));

    expect(mutateMock).toHaveBeenCalledTimes(1);
    const payload = mutateMock.mock.calls[0][0] as {
      name: string;
      clientId: string;
      redirectUri: string;
      scopes?: string[];
      attributeConfiguration?: unknown;
    };
    expect(payload).toMatchObject({
      name: 'Google Login',
      clientId: 'x',
      clientSecret: 's',
      redirectUri: 'https://id.acme.io/gate/callback',
    });
    expect(payload.scopes).toBeUndefined();
    expect(payload.attributeConfiguration).toBeUndefined();
  });

  it('shows the setup hint with the redirect URI to copy for Google', () => {
    render(<ConnectionConfigureWizardPage />);

    const hint = screen.getByTestId('connection-create-hint');
    expect(hint).toBeInTheDocument();
    expect(screen.getByDisplayValue('https://id.acme.io/gate/callback')).toBeInTheDocument();
  });

  it('navigates to the connection detail page after a successful create', () => {
    render(<ConnectionConfigureWizardPage />);

    fireEvent.click(screen.getByTestId('wizard-create'));

    const {onSuccess} = mutateMock.mock.calls[0][1] as {onSuccess: (data: {id: string}) => void};
    onSuccess({id: 'conn-1'});

    expect(navigateMock).toHaveBeenCalledWith('/connections/google/conn-1');
  });

  it('shows a general inline error (no toast) on a 409 create conflict, since the name is not user-editable here', () => {
    mutateMock.mockImplementationOnce((_payload, opts: {onError: (error: unknown) => void}) => {
      opts.onError({response: {status: 409}});
    });
    render(<ConnectionConfigureWizardPage />);

    fireEvent.click(screen.getByTestId('wizard-create'));

    expect(screen.getByText('Failed to create connection.')).toBeInTheDocument();
    expect(showToastMock).not.toHaveBeenCalled();
  });

  it('shows a general inline error (no toast) for a non-conflict create failure', () => {
    mutateMock.mockImplementationOnce((_payload, opts: {onError: (error: unknown) => void}) => {
      opts.onError({response: {status: 500}});
    });
    render(<ConnectionConfigureWizardPage />);

    fireEvent.click(screen.getByTestId('wizard-create'));

    expect(screen.getByText('Failed to create connection.')).toBeInTheDocument();
    expect(showToastMock).not.toHaveBeenCalled();
  });

  it('clears the general create error when a field is edited', () => {
    mutateMock.mockImplementationOnce((_payload, opts: {onError: (error: unknown) => void}) => {
      opts.onError({response: {status: 500}});
    });
    render(<ConnectionConfigureWizardPage />);

    fireEvent.click(screen.getByTestId('wizard-create'));
    expect(screen.getByText('Failed to create connection.')).toBeInTheDocument();

    fireEvent.click(screen.getByTestId('edit-client-id'));

    expect(screen.queryByText('Failed to create connection.')).not.toBeInTheDocument();
  });

  it('does not reset a still-pending mutation when a field is edited', () => {
    mutationState.isPending = true;
    mutationState.isError = false;
    render(<ConnectionConfigureWizardPage />);

    fireEvent.click(screen.getByTestId('edit-client-id'));

    expect(resetMock).not.toHaveBeenCalled();
  });

  it('resets a failed (settled) mutation when a field is edited', () => {
    mutationState.isPending = false;
    mutationState.isError = true;
    render(<ConnectionConfigureWizardPage />);

    fireEvent.click(screen.getByTestId('edit-client-id'));

    expect(resetMock).toHaveBeenCalled();
  });

  it('SMS vendor: single configure step creates without attribute mapping', () => {
    mockParams.type = 'twilio';
    render(<ConnectionConfigureWizardPage />);

    // Single step: the credentials form with a Create button (no attribute-mapping step).
    expect(screen.getByTestId('stub-connection-form')).toBeInTheDocument();
    fireEvent.click(screen.getByTestId('wizard-create'));

    expect(mutateMock).toHaveBeenCalledTimes(1);
    const payload = mutateMock.mock.calls[0][0] as Record<string, unknown>;
    expect(payload).toMatchObject({
      name: 'Twilio SMS',
      accountSid: 'AC00000000000000000000000000000000',
      senderId: '+15005550006',
    });
    expect('attributeConfiguration' in payload).toBe(false);
  });
});
