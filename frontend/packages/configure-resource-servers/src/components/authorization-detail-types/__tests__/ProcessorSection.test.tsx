// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderWithProviders, screen, userEvent, fireEvent} from '@thunderid/test-utils';
import type {JSX} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import useAuthorizationDetailTypeForm from '../../../hooks/useAuthorizationDetailTypeForm';
import type {AuthorizationDetailType, AuthorizationDetailTypeRequest} from '../../../models/authorization-detail-type';
import ProcessorSection from '../ProcessorSection';

const mockSubmit = vi.fn<(request: AuthorizationDetailTypeRequest) => void>();

/** The processor settings over a form, with a button that saves it while it is valid. */
function FormHarness({detailType}: {detailType: AuthorizationDetailType}): JSX.Element {
  const form = useAuthorizationDetailTypeForm(detailType);
  return (
    <>
      <ProcessorSection form={form} />
      <button type="button" disabled={!form.isValid} onClick={() => mockSubmit(form.buildRequest())}>
        Submit
      </button>
    </>
  );
}

const ENDPOINT = 'https://bank.example.com/process';

function withAuthentication(authentication: {type: string; properties?: Record<string, string>}) {
  return {
    id: 'type-1',
    type: 'payment_initiation',
    name: 'Payment initiation',
    version: 3,
    schema: {type: 'object', properties: {amount: {type: 'number', title: 'Amount'}}},
    processor: {endpoint: ENDPOINT, authentication},
    reuse: false,
  } as AuthorizationDetailType;
}

const API_KEY_TYPE = withAuthentication({type: 'api_key', properties: {'X-Api-Key': '******'}});

async function chooseMethod(name: string): Promise<void> {
  const user = userEvent.setup();
  await user.click(screen.getByRole('combobox', {name: /Authentication method/}));
  await user.click(screen.getByRole('option', {name}));
}

describe('ProcessorSection', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('keeps the stored API key headers when another method is chosen and API key again', async () => {
    const user = userEvent.setup();
    renderWithProviders(<FormHarness detailType={API_KEY_TYPE} />);

    await chooseMethod('Bearer token');
    await chooseMethod('API key');

    expect(document.getElementById('connection-field-httpHeaders-name-1')).toHaveValue('X-Api-Key');
    expect(document.getElementById('connection-field-httpHeaders-value-1')).toBeDisabled();
    await user.click(screen.getByRole('button', {name: 'Submit'}));
    expect(mockSubmit.mock.calls[0][0].processor).toEqual({endpoint: ENDPOINT});
  });

  it('never saves API key authentication without headers as no authentication', async () => {
    const user = userEvent.setup();
    renderWithProviders(<FormHarness detailType={API_KEY_TYPE} />);

    await user.click(screen.getByTestId('connection-field-httpHeaders-remove-1'));

    expect(screen.getByRole('button', {name: 'Submit'})).toBeDisabled();
  });

  it('refuses an API key header the server refuses, on its row', async () => {
    const user = userEvent.setup();
    renderWithProviders(<FormHarness detailType={API_KEY_TYPE} />);

    await user.click(screen.getByTestId('connection-field-httpHeaders-add'));
    fireEvent.change(document.getElementById('connection-field-httpHeaders-name-2')!, {target: {value: 'Host'}});
    fireEvent.change(document.getElementById('connection-field-httpHeaders-value-2')!, {target: {value: 'x'}});

    expect(screen.getByTestId('connection-field-httpHeaders-error-2')).toHaveTextContent(
      'This header is reserved. Use another name.',
    );
    expect(screen.getByRole('button', {name: 'Submit'})).toBeDisabled();
  });

  it('explains that the credentials are needed again when the endpoint changes', () => {
    renderWithProviders(<FormHarness detailType={API_KEY_TYPE} />);
    const notice = 'The endpoint changed. Enter the credentials again before saving.';
    expect(screen.queryByText(notice)).not.toBeInTheDocument();

    fireEvent.change(screen.getByLabelText(/^Endpoint URL/), {target: {value: 'https://bank.example.com/v2'}});

    expect(screen.getByText(notice)).toBeInTheDocument();
    expect(document.getElementById('connection-field-httpHeaders-value-1')).not.toBeDisabled();
    expect(document.getElementById('connection-field-httpHeaders-value-1')).toHaveValue('');

    fireEvent.change(document.getElementById('connection-field-httpHeaders-value-1')!, {target: {value: 'fresh'}});

    expect(screen.queryByText(notice)).not.toBeInTheDocument();
  });

  it('marks where a save error about the endpoint or the authentication is shown', () => {
    renderWithProviders(<FormHarness detailType={API_KEY_TYPE} />);

    const endpoint = document.querySelector('[data-save-error-target="endpoint"]');
    const authentication = document.querySelector('[data-save-error-target="authentication"]');
    expect(endpoint).toContainElement(screen.getByLabelText(/^Endpoint URL/));
    expect(authentication).toContainElement(document.getElementById('connection-field-httpHeaders-name-1'));
  });

  it.each([
    ['ftp://bank.example.com/process', 'Use HTTPS. HTTP is accepted only for a loopback address'],
    ['http://bank.example.com/process', 'Use HTTPS. HTTP is accepted only for a loopback address'],
    ['https://user:pw@bank.example.com/process', 'Remove the user name and password from the URL.'],
    ['https://bank.example.com:0/process', 'Enter a port from 1 to 65535.'],
    ['https://bank.example.com/%zz', 'Enter a full URL, such as https://api.example.com/authorize.'],
    ['https://bank example.com/process', 'Enter a full URL, such as https://api.example.com/authorize.'],
  ])('explains why the endpoint %s is refused', (endpoint, message) => {
    renderWithProviders(<FormHarness detailType={API_KEY_TYPE} />);

    fireEvent.change(screen.getByLabelText(/^Endpoint URL/), {target: {value: endpoint}});

    expect(screen.getByText(new RegExp(message.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')))).toBeInTheDocument();
  });

  it('names the fields that need the missing processor', async () => {
    const user = userEvent.setup();
    const detailType = {
      ...API_KEY_TYPE,
      schema: {
        type: 'object',
        properties: {
          amount: {type: 'number', policy: {modification: 'decrease', modifiableBy: ['processor']}},
          creditor: {
            type: 'object',
            properties: {iban: {type: 'string', maxLength: 34, policy: {suppliedBy: 'processor'}}},
          },
        },
      },
    } as AuthorizationDetailType;
    renderWithProviders(<FormHarness detailType={detailType} />);

    await user.click(screen.getByRole('switch', {name: 'Use an external processor'}));

    expect(
      screen.getByText(
        'Some fields are set or changed by the external processor: amount, creditor.iban. Add a processor, or change those fields.',
      ),
    ).toBeInTheDocument();
  });
});
