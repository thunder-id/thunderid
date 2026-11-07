// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, renderWithProviders, screen, userEvent, fireEvent, waitFor, within} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {AuthorizationDetailType} from '../../models/authorization-detail-type';
import CreateAuthorizationDetailTypePage from '../CreateAuthorizationDetailTypePage';

const mockNavigate = vi.fn();

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {...actual, useNavigate: () => mockNavigate};
});

const mockShowToast = vi.fn();

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useToast: () => ({showToast: mockShowToast})};
});

const mockLoggerError = vi.fn();

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({error: mockLoggerError, info: vi.fn(), debug: vi.fn()}),
}));

vi.mock('@monaco-editor/react', () => ({
  default: ({value, onChange}: {value: string; onChange?: (v: string | undefined) => void}) => (
    <textarea aria-label="Schema JSON" value={value} onChange={(e) => onChange?.(e.target.value)} />
  ),
}));

const mockCreate = vi.fn();
const mockCreateReset = vi.fn();
const mockUseCreate = vi.fn(() => ({mutate: mockCreate, isPending: false, isSuccess: false, reset: mockCreateReset}));

vi.mock('../../api/useCreateAuthorizationDetailType', () => ({
  default: () => mockUseCreate(),
}));

interface CreateCallbacks {
  onSuccess: (created: AuthorizationDetailType) => void;
  onError: (err: Error) => void;
}

const CREATE = 'Create intent';

describe('CreateAuthorizationDetailTypePage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigate.mockResolvedValue(undefined);
    mockUseCreate.mockReturnValue({mutate: mockCreate, isPending: false, isSuccess: false, reset: mockCreateReset});
  });

  it('sets the document title', async () => {
    renderWithProviders(<CreateAuthorizationDetailTypePage />);

    await waitFor(() => expect(document.title).toBe('Add intent | ThunderID'));
  });

  it('creates a one-time type from its name alone, then opens it', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateAuthorizationDetailTypePage />);

    expect(screen.getByRole('heading', {name: "Let's collect some details about your intent"})).toBeVisible();
    expect(screen.queryByText('Enter a name.'), 'nothing missing is reported while the user fills it in').toBeNull();
    expect(screen.queryByRole('button', {name: 'Back'})).not.toBeInTheDocument();
    expect(screen.getByLabelText(/^Name/), 'the wizard opens on the name').toHaveFocus();
    expect(screen.getByRole('progressbar')).toHaveAttribute('aria-valuenow', '33');

    await user.type(screen.getByLabelText(/^Name/), 'Flight booking');
    expect(screen.getByLabelText(/^Type identifier/)).toHaveValue('flight_booking');
    expect(screen.getByRole('switch', {name: /Ask every time/}), 'asked every time by default').toBeChecked();
    await user.click(screen.getByRole('button', {name: 'Continue'}));

    expect(screen.getByRole('heading', {name: 'What does the user approve?'})).toBeVisible();
    expect(screen.getByRole('heading', {name: 'Schema'}), 'the step after the details').toBeVisible();
    expect(screen.getByRole('button', {name: 'Add field'})).toBeInTheDocument();
    expect(screen.queryByRole('switch', {name: /Ask every time/}), 'it is set with the details').toBeNull();
    await user.click(screen.getByRole('button', {name: 'Continue'}));

    expect(screen.getByRole('heading', {name: 'Should your system check each request?'})).toBeVisible();
    expect(screen.getByRole('switch', {name: 'Use an external processor'})).not.toBeChecked();
    await user.click(screen.getByRole('button', {name: CREATE}));

    expect(mockCreate).toHaveBeenCalledWith(
      {type: 'flight_booking', name: 'Flight booking', schema: {type: 'object', properties: {}}, reuse: false},
      expect.any(Object),
    );
    const {onSuccess} = mockCreate.mock.calls[0][1] as CreateCallbacks;
    onSuccess({
      id: 'type-9',
      type: 'flight_booking',
      name: 'Flight booking',
      version: 1,
      schema: {type: 'object', properties: {}},
      reuse: false,
    });
    expect(mockShowToast, 'the create hook confirms the create').not.toHaveBeenCalled();
    expect(mockNavigate, 'going back does not open the wizard again').toHaveBeenCalledWith(
      '/authorization-detail-types/type-9',
      {replace: true},
    );
  });

  it('creates a type with its fields, consent, and processor', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateAuthorizationDetailTypePage />);

    await user.type(screen.getByLabelText(/^Name/), 'Flight booking');
    await user.type(screen.getByLabelText('Description'), 'Books one flight');
    await user.click(screen.getByRole('switch', {name: /Ask every time/}));
    await user.click(screen.getByRole('button', {name: 'Continue'}));

    await user.click(screen.getByRole('button', {name: 'Add field'}));
    await user.click(screen.getByRole('menuitem', {name: 'Text'}));
    await user.type(screen.getByLabelText(/^Field name/), 'route');
    await user.click(screen.getByRole('button', {name: 'Consent summary'}));
    fireEvent.change(screen.getByLabelText('Summary'), {target: {value: 'Book'}});
    await user.click(screen.getByRole('button', {name: 'Insert route'}));
    expect(screen.getByRole('button', {name: 'Consent summary'})).toHaveTextContent('Book ‹route›');
    await user.click(screen.getByRole('button', {name: 'Close summary settings'}));
    await user.click(screen.getByRole('button', {name: 'Continue'}));

    await user.click(screen.getByRole('switch', {name: 'Use an external processor'}));
    await user.click(screen.getByRole('button', {name: CREATE}));
    expect(mockCreate, 'a processor needs an endpoint').not.toHaveBeenCalled();
    expect(screen.getByText('Enter the URL of your external processor.')).toBeInTheDocument();
    expect(screen.getByLabelText(/^Endpoint URL/), 'the missing endpoint takes focus').toHaveFocus();
    await user.type(screen.getByLabelText(/^Endpoint URL/), 'https://bank.example.com/process');
    await user.click(screen.getByRole('button', {name: CREATE}));

    expect(mockCreate).toHaveBeenCalledWith(
      {
        type: 'flight_booking',
        name: 'Flight booking',
        description: 'Books one flight',
        schema: {type: 'object', properties: {route: {type: 'string', maxLength: 100}}},
        consentTemplate: 'Book {{route}}',
        processor: {endpoint: 'https://bank.example.com/process'},
        reuse: true,
      },
      expect.any(Object),
    );
  });

  it('does not continue past a step that is not valid', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateAuthorizationDetailTypePage />);

    await user.type(screen.getByLabelText(/^Name/), 'Flight booking');
    await user.clear(screen.getByLabelText(/^Type identifier/));
    await user.type(screen.getByLabelText(/^Type identifier/), 'has space');
    expect(screen.getByText('Use up to 512 ASCII letters, digits or symbols, without spaces.')).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Continue'}));
    expect(screen.getByLabelText(/^Type identifier/), 'the step stays open on its problem').toHaveFocus();

    await user.clear(screen.getByLabelText(/^Type identifier/));
    await user.type(screen.getByLabelText(/^Type identifier/), 'urn:flight');
    await user.click(screen.getByRole('button', {name: 'Continue'}));

    await user.click(screen.getByRole('button', {name: 'JSON'}));
    fireEvent.change(screen.getByLabelText('Schema JSON'), {target: {value: '{"type": "object",'}});
    expect(screen.getByText(/^The JSON is not valid near line 1, column 19\./)).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Continue'}));
    expect(screen.getByRole('heading', {name: 'What does the user approve?'})).toBeVisible();

    fireEvent.change(screen.getByLabelText('Schema JSON'), {target: {value: '{"type": "object", "properties": {}}'}});
    fireEvent.change(screen.getByLabelText('Summary'), {target: {value: 'Pay {{payee}}'}});
    expect(screen.getByText('No field matches payee. Use a field name from the list.')).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Continue'}));
    expect(screen.getByRole('heading', {name: 'What does the user approve?'})).toBeVisible();
    expect(mockCreate).not.toHaveBeenCalled();
  });

  it('asks for a processor on the last step when the fields need one', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateAuthorizationDetailTypePage />);

    await user.type(screen.getByLabelText(/^Name/), 'Payment');
    await user.click(screen.getByRole('button', {name: 'Continue'}));

    await user.click(screen.getByRole('button', {name: 'JSON'}));
    const schema = {type: 'object', properties: {fee: {type: 'number', policy: {suppliedBy: 'processor'}}}};
    fireEvent.change(screen.getByLabelText('Schema JSON'), {target: {value: JSON.stringify(schema)}});
    expect(screen.queryByText(/no processor is set/), 'the processor is set on a later step').toBeNull();
    await user.click(screen.getByRole('button', {name: 'Continue'}));

    expect(screen.getByText(/Some fields are set or changed by the external processor/)).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: CREATE}));
    expect(mockCreate).not.toHaveBeenCalled();

    await user.click(screen.getByRole('switch', {name: 'Use an external processor'}));
    await user.type(screen.getByLabelText(/^Endpoint URL/), 'https://bank.example.com/process');
    await user.click(screen.getByRole('button', {name: CREATE}));
    expect(mockCreate).toHaveBeenCalled();
  });

  it('goes back through the steps, keeping what was entered', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateAuthorizationDetailTypePage />);

    await user.type(screen.getByLabelText(/^Name/), 'Flight booking');
    await user.click(screen.getByRole('button', {name: 'Continue'}));
    await user.click(screen.getByRole('button', {name: 'Continue'}));
    expect(screen.getByRole('heading', {name: 'Should your system check each request?'})).toBeVisible();

    await user.click(screen.getByRole('button', {name: 'Back'}));
    expect(screen.getByRole('button', {name: 'Add field'})).toBeInTheDocument();

    await user.click(screen.getByText('Details'));
    expect(screen.getByLabelText(/^Name/)).toHaveValue('Flight booking');
  });

  it('shows a create failure until the form changes or it is dismissed', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateAuthorizationDetailTypePage />);

    await user.type(screen.getByLabelText(/^Name/), 'Flight booking');
    await user.click(screen.getByRole('button', {name: 'Continue'}));
    await user.click(screen.getByRole('button', {name: 'Continue'}));
    await user.click(screen.getByRole('button', {name: CREATE}));

    const {onError} = mockCreate.mock.calls[0][1] as CreateCallbacks;
    onError(new Error('Request failed with status code 500'));
    const message = 'Failed to save the intent. Please try again.';
    expect(await screen.findByText(message)).toBeInTheDocument();
    expect(mockNavigate).not.toHaveBeenCalled();

    await user.click(within(screen.getByRole('alert')).getByRole('button', {name: 'Close'}));
    expect(screen.queryByText(message)).not.toBeInTheDocument();
    expect(mockCreateReset).toHaveBeenCalled();

    await user.click(screen.getByRole('button', {name: CREATE}));
    const {onError: onSecondError} = mockCreate.mock.calls[1][1] as CreateCallbacks;
    onSecondError(new Error('boom'));
    const fallback = 'Failed to save the intent. Please try again.';
    expect(await screen.findByText(fallback)).toBeInTheDocument();
    await user.click(screen.getByRole('switch', {name: 'Use an external processor'}));
    expect(screen.queryByText(fallback)).not.toBeInTheDocument();
  });

  it('does not create twice while the first create is in flight or done', async () => {
    const user = userEvent.setup();
    const {rerender} = renderWithProviders(<CreateAuthorizationDetailTypePage />);

    await user.type(screen.getByLabelText(/^Name/), 'Flight booking');
    await user.click(screen.getByRole('button', {name: 'Continue'}));
    await user.click(screen.getByRole('button', {name: 'Continue'}));

    mockUseCreate.mockReturnValue({mutate: mockCreate, isPending: true, isSuccess: false, reset: mockCreateReset});
    rerender(<CreateAuthorizationDetailTypePage />);
    expect(screen.getByRole('button', {name: 'Creating…'})).toBeDisabled();
    expect(screen.getByRole('button', {name: 'Back'})).toBeDisabled();

    mockUseCreate.mockReturnValue({mutate: mockCreate, isPending: false, isSuccess: true, reset: mockCreateReset});
    rerender(<CreateAuthorizationDetailTypePage />);
    expect(screen.getByRole('button', {name: 'Creating…'})).toBeDisabled();
  });

  it('returns to the list when closed', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateAuthorizationDetailTypePage />);

    await user.click(screen.getByRole('button', {name: 'Close'}));
    expect(mockNavigate).toHaveBeenCalledWith('/authorization-detail-types');
  });

  it('says what is missing when going on from a step that is not ready, without flagging it before', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateAuthorizationDetailTypePage />);

    expect(screen.getByRole('button', {name: 'Continue'}), 'going on shows what is missing').toBeEnabled();
    expect(screen.getByLabelText(/^Name/)).not.toHaveAttribute('aria-invalid', 'true');
    expect(screen.queryByText('Enter a name.')).toBeNull();
    expect(screen.queryByRole('alert')).toBeNull();

    await user.click(screen.getByRole('button', {name: 'Continue'}));
    expect(screen.getByRole('heading', {name: "Let's collect some details about your intent"})).toBeVisible();
    expect(screen.getByText('Enter a name.')).toBeInTheDocument();
    expect(screen.getByLabelText(/^Name/)).toHaveFocus();
    expect(screen.getByLabelText(/^Name/)).toBeRequired();
    expect(screen.getByLabelText(/^Type identifier/)).toBeRequired();

    await user.type(screen.getByLabelText(/^Name/), 'Flight booking');
    await user.click(screen.getByRole('button', {name: 'Continue'}));
    expect(screen.getByRole('heading', {name: 'What does the user approve?'})).toBeVisible();
  });

  it('says a field has no name when going on from the schema', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateAuthorizationDetailTypePage />);

    await user.type(screen.getByLabelText(/^Name/), 'Flight booking');
    await user.click(screen.getByRole('button', {name: 'Continue'}));
    await user.click(screen.getByRole('button', {name: 'Add field'}));
    await user.click(screen.getByRole('menuitem', {name: 'Number'}));
    expect(screen.queryByText('Enter a field name.'), 'a new field is not flagged before it is filled in').toBeNull();

    await user.click(screen.getByRole('button', {name: 'Continue'}));
    expect(screen.getByRole('heading', {name: 'What does the user approve?'})).toBeVisible();
    expect(screen.getByText('Enter a field name.')).toBeInTheDocument();

    await user.type(screen.getByLabelText(/^Field name/), 'amount');
    await user.click(screen.getByRole('button', {name: 'Continue'}));
    expect(screen.getByRole('heading', {name: 'Should your system check each request?'})).toBeVisible();
  });

  it('asks for a type identifier when the name has no letter it can take', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateAuthorizationDetailTypePage />);

    await user.type(screen.getByLabelText(/^Name/), '支払い');
    expect(screen.getByLabelText(/^Type identifier/)).toHaveValue('');
    expect(screen.getByText('Enter the type identifier clients send.')).toBeInTheDocument();

    await user.clear(screen.getByLabelText(/^Name/));
    await user.type(screen.getByLabelText(/^Name/), "Paiement à l'étranger");
    expect(screen.getByLabelText(/^Type identifier/), 'accented letters keep their base letter').toHaveValue(
      'paiement_a_l_etranger',
    );
  });

  it('creates once when Create is clicked twice before the page updates', async () => {
    renderWithProviders(<CreateAuthorizationDetailTypePage />);
    const user = userEvent.setup();

    await user.type(screen.getByLabelText(/^Name/), 'Flight booking');
    await user.click(screen.getByRole('button', {name: 'Continue'}));
    await user.click(screen.getByRole('button', {name: 'Continue'}));
    const create = screen.getByRole('button', {name: CREATE});
    act(() => {
      create.click();
      create.click();
    });
    expect(mockCreate).toHaveBeenCalledTimes(1);

    const {onError} = mockCreate.mock.calls[0][1] as CreateCallbacks;
    act(() => onError(Object.assign(new Error('boom'), {response: {status: 500, data: {code: 'SSE-5000'}}})));
    await user.click(screen.getByRole('button', {name: CREATE}));
    expect(mockCreate, 'a failed create can be tried again').toHaveBeenCalledTimes(2);
  });

  it('logs a failed create without the request it sent', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateAuthorizationDetailTypePage />);

    await user.type(screen.getByLabelText(/^Name/), 'Flight booking');
    await user.click(screen.getByRole('button', {name: 'Continue'}));
    await user.click(screen.getByRole('button', {name: 'Continue'}));
    await user.click(screen.getByRole('button', {name: CREATE}));

    const {onError} = mockCreate.mock.calls[0][1] as CreateCallbacks;
    const err = Object.assign(new Error('conflict'), {
      config: {
        data: '{"processor":{"authentication":{"properties":{"token":"secret"}}}}',
        headers: {Authorization: 'x'},
      },
      response: {status: 409, data: {code: 'RAR-1003'}},
    });
    act(() => onError(err));
    expect(mockLoggerError).toHaveBeenCalledWith('Failed to create intent', {status: 409, code: 'RAR-1003'});
  });

  it('opens the details step to show a type identifier that is already taken', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateAuthorizationDetailTypePage />);

    await user.type(screen.getByLabelText(/^Name/), 'Flight booking');
    await user.click(screen.getByRole('button', {name: 'Continue'}));
    await user.click(screen.getByRole('button', {name: 'Continue'}));
    await user.click(screen.getByRole('button', {name: CREATE}));

    const {onError} = mockCreate.mock.calls[0][1] as CreateCallbacks;
    act(() => onError(Object.assign(new Error('conflict'), {response: {status: 409, data: {code: 'RAR-1003'}}})));

    const typeInput = await screen.findByLabelText(/^Type identifier/);
    expect(typeInput).toHaveAttribute('aria-invalid', 'true');
    await waitFor(() => expect(typeInput, 'the refused value takes focus').toHaveFocus());
    expect(screen.getAllByText('An intent with the same type identifier already exists.')).toHaveLength(1);
    expect(screen.queryByRole('alert'), 'shown beside the value only').toBeNull();

    await user.type(typeInput, '_v2');
    expect(screen.queryByText('An intent with the same type identifier already exists.')).toBeNull();
  });

  it('shows an invalid schema beside the fields, without the server text', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateAuthorizationDetailTypePage />);

    await user.type(screen.getByLabelText(/^Name/), 'Flight booking');
    await user.click(screen.getByRole('button', {name: 'Continue'}));
    await user.click(screen.getByRole('button', {name: 'Continue'}));
    await user.click(screen.getByRole('button', {name: CREATE}));

    const {onError} = mockCreate.mock.calls[0][1] as CreateCallbacks;
    act(() =>
      onError(
        Object.assign(new Error('bad schema'), {
          response: {
            status: 400,
            data: {code: 'RAR-1006', description: {params: {path: '', reason: 'amount: maximum must be a number'}}},
          },
        }),
      ),
    );

    const alert = await screen.findByTestId('schema-save-error');
    expect(alert).toHaveTextContent('The schema is not valid. Check the field types and their rules.');
    expect(alert).not.toHaveTextContent('amount: maximum must be a number');
    expect(screen.getByRole('heading', {name: 'What does the user approve?'})).toBeVisible();
  });

  it('resolves a create that gets no response through the catalog fallback', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateAuthorizationDetailTypePage />);

    await user.type(screen.getByLabelText(/^Name/), 'Flight booking');
    await user.click(screen.getByRole('button', {name: 'Continue'}));
    await user.click(screen.getByRole('button', {name: 'Continue'}));
    await user.click(screen.getByRole('button', {name: CREATE}));

    const {onError} = mockCreate.mock.calls[0][1] as CreateCallbacks;
    act(() => onError(Object.assign(new Error('Network Error'), {code: 'ERR_NETWORK'})));

    expect(await screen.findByText('Failed to save the intent. Please try again.')).toBeInTheDocument();
    expect(screen.queryByText('Network Error')).toBeNull();
  });

  it('asks before closing once something is entered', async () => {
    const user = userEvent.setup();
    renderWithProviders(<CreateAuthorizationDetailTypePage />);

    await user.type(screen.getByLabelText(/^Name/), 'Flight booking');
    await user.click(screen.getByRole('button', {name: 'Close'}));

    const dialog = screen.getByRole('dialog', {name: 'Discard unsaved changes?'});
    expect(dialog).toHaveTextContent('You have changes to this intent that are not saved.');
    await user.click(within(dialog).getByRole('button', {name: 'Keep editing'}));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expect(mockNavigate).not.toHaveBeenCalled();
    expect(screen.getByLabelText(/^Name/)).toHaveValue('Flight booking');

    await user.click(screen.getByRole('button', {name: 'Close'}));
    await user.click(
      within(screen.getByRole('dialog', {name: 'Discard unsaved changes?'})).getByRole('button', {
        name: 'Discard changes',
      }),
    );
    expect(mockNavigate).toHaveBeenCalledWith('/authorization-detail-types');
  });
});
