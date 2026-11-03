// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import * as componentsModule from '@thunderid/components';
import {act, renderWithProviders, screen, userEvent, fireEvent, waitFor, within} from '@thunderid/test-utils';
import type {MouseEvent, ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {AuthorizationDetailType} from '../../models/authorization-detail-type';
import AuthorizationDetailTypeEditPage from '../AuthorizationDetailTypeEditPage';

const mockNavigate = vi.fn();
const mockRefetch = vi.fn();

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useParams: () => ({id: 'type-1'}),
    Link: ({
      to,
      children = undefined,
      onClick = undefined,
      ...props
    }: {
      to: string;
      children?: ReactNode;
      onClick?: (event: MouseEvent<HTMLAnchorElement>) => void;
      [key: string]: unknown;
    }) => (
      <a
        {...(props as Record<string, unknown>)}
        href={to}
        onClick={(e) => {
          onClick?.(e);
          if (e.defaultPrevented) return;
          e.preventDefault();
          Promise.resolve(mockNavigate(to)).catch(() => null);
        }}
      >
        {children}
      </a>
    ),
  };
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

vi.mock('@thunderid/components', {spy: true});

vi.mocked(componentsModule.PageLoadingAnimation).mockImplementation(() => <div role="progressbar" />);

const mockUseGetType = vi.fn<(id: string) => unknown>();
const mockUpdate = vi.fn();
const mockDelete = vi.fn();
const mockUseUpdate = vi.fn(() => ({mutate: mockUpdate, isPending: false}));

vi.mock('../../api/useGetAuthorizationDetailType', () => ({
  default: (id: string) => mockUseGetType(id),
}));
vi.mock('../../api/useUpdateAuthorizationDetailType', () => ({
  default: () => mockUseUpdate(),
}));
vi.mock('../../api/useDeleteAuthorizationDetailType', () => ({
  default: () => ({mutate: mockDelete, isPending: false}),
}));

const paymentType: AuthorizationDetailType = {
  id: 'type-1',
  type: 'payment_initiation',
  name: 'Payment initiation',
  version: 3,
  schema: {
    type: 'object',
    properties: {
      amount: {
        type: 'number',
        title: 'Amount',
        maximum: 500,
        policy: {modification: 'decrease', modifiableBy: ['processor']},
      },
    },
  },
  consentTemplate: 'Pay {{amount}}',
  processor: {endpoint: 'https://bank.example.com/process'},
  reuse: false,
};

const paymentRequest = {
  type: 'payment_initiation',
  name: 'Payment initiation',
  schema: paymentType.schema,
  consentTemplate: 'Pay {{amount}}',
  processor: {endpoint: 'https://bank.example.com/process'},
  reuse: false,
};

interface UpdateCallbacks {
  onSuccess: (saved: AuthorizationDetailType) => void;
  onError: (err: Error) => void;
}

const UNSAVED = 'You have unsaved changes.';
const NEW_VERSION = 'Saving creates a new version of this intent. Earlier approvals no longer apply.';

function loaded(data: AuthorizationDetailType | undefined, error: Error | null = null): void {
  mockUseGetType.mockReturnValue({data, isLoading: false, error, refetch: mockRefetch});
}

type User = ReturnType<typeof userEvent.setup>;

/** Types at the end of the name in the header, which its edit button opens, and applies it with Enter. */
async function typeName(user: User, text: string): Promise<void> {
  await user.click(screen.getByRole('button', {name: 'Edit intent name'}));
  await user.type(screen.getByLabelText('Name'), `${text}{Enter}`);
}

/** Types at the end of the description in the header, which its edit button opens, and applies it with Ctrl+Enter. */
async function typeDescription(user: User, text: string): Promise<void> {
  await user.click(screen.getByRole('button', {name: 'Edit intent description'}));
  await user.type(screen.getByLabelText('Description'), text);
  await user.keyboard('{Control>}{Enter}{/Control}');
}

describe('AuthorizationDetailTypeEditPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigate.mockResolvedValue(undefined);
    mockUseUpdate.mockReturnValue({mutate: mockUpdate, isPending: false});
    mockRefetch.mockReset();
    loaded(paymentType);
  });

  it('shows the loading animation while the type loads', () => {
    mockUseGetType.mockReturnValue({data: undefined, isLoading: true, error: null, refetch: mockRefetch});
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('loads the type by its id and shows it across the tabs', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    expect(mockUseGetType).toHaveBeenCalledWith('type-1');
    expect(screen.getByRole('heading', {name: 'Payment initiation'})).toBeInTheDocument();
    expect(screen.getByText('No description')).toBeInTheDocument();
    expect(screen.getByText('payment_initiation'), 'the header shows the type identifier').toBeInTheDocument();
    expect(screen.getByText('Version 3')).toBeInTheDocument();
    expect(screen.queryByLabelText(/^Name/), 'the name is edited in the header').not.toBeInTheDocument();
    expect(screen.queryByLabelText('Description'), 'the description is edited in the header').not.toBeInTheDocument();
    expect(screen.queryByText('Danger zone'), 'deleting is on the Advanced tab').not.toBeInTheDocument();
    expect(screen.getByLabelText('Intent ID')).toHaveValue('type-1');
    expect(screen.getByLabelText('Intent ID')).toHaveAttribute('readonly');
    expect(screen.getByLabelText('Type identifier')).toHaveValue('payment_initiation');
    expect(screen.getByLabelText('Type identifier')).toHaveAttribute('readonly');
    expect(screen.queryByLabelText('Version'), 'the header shows the version').not.toBeInTheDocument();
    expect(screen.getByRole('switch', {name: /Ask every time/})).toBeChecked();
    expect(screen.queryByText(UNSAVED)).not.toBeInTheDocument();
    expect(screen.queryByText(/read-only/)).not.toBeInTheDocument();

    expect(screen.getAllByRole('tab').map((tab) => tab.textContent)).toEqual([
      'General',
      'Schema',
      'External processor',
      'Advanced',
    ]);
    await user.click(screen.getByRole('tab', {name: 'Schema'}));
    expect(screen.getByRole('treeitem', {name: 'Amount'})).toBeInTheDocument();
    expect(screen.queryByLabelText(/^Name/)).not.toBeInTheDocument();
    expect(screen.queryByRole('switch', {name: /Ask every time/}), 'it is on the General tab').not.toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Consent summary'}));
    expect(screen.getByLabelText('Summary')).toHaveValue('Pay {{amount}}');

    await user.click(screen.getByRole('tab', {name: 'External processor'}));
    expect(screen.getByLabelText(/^Endpoint URL/)).toHaveValue('https://bank.example.com/process');
  });

  it('saves an edited name with the whole type, then starts again from what was saved', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await typeName(user, ' v2');
    expect(
      screen.getByRole('heading', {name: 'Payment initiation v2'}),
      'the header shows the change',
    ).toBeInTheDocument();
    expect(screen.getByText(UNSAVED)).toBeInTheDocument();
    expect(screen.queryByText(NEW_VERSION), 'a name change keeps the version').toBeNull();
    await user.click(screen.getByRole('button', {name: 'Save'}));

    expect(mockUpdate).toHaveBeenCalledWith(
      {id: 'type-1', data: {...paymentRequest, name: 'Payment initiation v2'}},
      expect.any(Object),
    );

    const {onSuccess} = mockUpdate.mock.calls[0][1] as UpdateCallbacks;
    // The update hook keeps the saved type as the latest.
    loaded({...paymentType, name: 'Payment initiation v2'});
    onSuccess({...paymentType, name: 'Payment initiation v2'});
    expect(mockShowToast, 'the update hook confirms the save').not.toHaveBeenCalled();
    // The header showed the edited name before the save, so the form starting again is told by the save bar.
    await waitFor(() => expect(screen.queryByText(UNSAVED)).not.toBeInTheDocument());
    expect(screen.getByRole('heading', {name: 'Payment initiation v2'})).toBeInTheDocument();
    await waitFor(() =>
      expect(
        screen.getByRole('heading', {name: 'Payment initiation v2'}),
        'focus stays on the page when the save bar goes',
      ).toHaveFocus(),
    );
  });

  it('saves changes made on several tabs at once', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await typeDescription(user, 'Pays once');
    await user.click(screen.getByRole('switch', {name: /Ask every time/}));
    await user.click(screen.getByRole('tab', {name: 'External processor'}));
    await user.clear(screen.getByLabelText(/^Endpoint URL/));
    await user.type(screen.getByLabelText(/^Endpoint URL/), 'https://bank.example.com/v2');
    await user.click(screen.getByRole('button', {name: 'Save'}));

    expect(mockUpdate).toHaveBeenCalledWith(
      {
        id: 'type-1',
        data: {
          ...paymentRequest,
          description: 'Pays once',
          processor: {endpoint: 'https://bank.example.com/v2'},
          reuse: true,
        },
      },
      expect.any(Object),
    );
  });

  it('warns that saving changed fields or a changed summary creates a new version', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await user.click(screen.getByRole('tab', {name: 'Schema'}));
    await user.click(screen.getByRole('button', {name: 'Consent summary'}));
    fireEvent.change(screen.getByLabelText('Summary'), {target: {value: 'Pay up to {{amount}}'}});
    expect(screen.getByText(NEW_VERSION)).toBeInTheDocument();

    await user.click(screen.getByRole('button', {name: 'Reset'}));
    expect(screen.queryByText(NEW_VERSION)).not.toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Consent summary'}), 'the open tab is kept').toHaveTextContent('Pay 100');

    await user.click(screen.getByRole('button', {name: 'JSON'}));
    const schema = {type: 'object', properties: {amount: {type: 'number'}, payee: {type: 'string'}}};
    fireEvent.change(screen.getByLabelText('Schema JSON'), {target: {value: JSON.stringify(schema)}});
    expect(screen.getByText(NEW_VERSION)).toBeInTheDocument();
  });

  it('copies the intent ID and the type identifier from the General tab', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);
    const writeText = vi.spyOn(navigator.clipboard, 'writeText');

    expect(screen.getByText('Quick copy')).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Copy intent ID'}));
    expect(writeText).toHaveBeenCalledWith('type-1');
    expect(await screen.findByRole('tooltip')).toHaveTextContent('Copied!');

    await user.click(screen.getByRole('button', {name: 'Copy type identifier'}));
    expect(writeText).toHaveBeenLastCalledWith('payment_initiation');
    expect(screen.queryByText(UNSAVED), 'copying changes nothing').not.toBeInTheDocument();
  });

  it('sets whether to ask every time on the General tab, which creates a new version', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    expect(screen.getByText('Consent')).toBeInTheDocument();
    await user.click(screen.getByRole('switch', {name: /Ask every time/}));
    expect(screen.getByRole('switch', {name: /Ask every time/})).not.toBeChecked();
    expect(screen.getByText(NEW_VERSION)).toBeInTheDocument();
    expect(screen.getByText(UNSAVED)).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Save'}));

    expect(mockUpdate).toHaveBeenCalledWith({id: 'type-1', data: {...paymentRequest, reuse: true}}, expect.any(Object));
  });

  it('explains when the version increases on the version in the header', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);
    const hint = /Increases when the fields, the consent summary or the approval reuse setting change/;

    const version = screen.getByText('Version 3').closest<HTMLElement>('.MuiChip-root')!;
    expect(version, 'the explanation is the description of the version').toHaveAttribute(
      'title',
      expect.stringMatching(hint),
    );
    expect(version).toHaveAttribute('tabindex', '0');
    act(() => screen.getByRole('button', {name: 'Edit intent description'}).focus());
    await user.tab();
    expect(version).toHaveFocus();
    expect(await screen.findByRole('tooltip'), 'it shows on keyboard focus').toHaveTextContent(hint);
    expect(version).toHaveAttribute('aria-describedby', screen.getByRole('tooltip').id);
  });

  it('shows a type identifier the server refused on the General tab, and takes focus there', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await user.click(screen.getByRole('tab', {name: 'External processor'}));
    await typeDescription(user, 'Pays once');
    await user.click(screen.getByRole('button', {name: 'Save'}));
    const {onError} = mockUpdate.mock.calls[0][1] as UpdateCallbacks;
    act(() => onError(Object.assign(new Error('bad'), {response: {status: 400, data: {code: 'RAR-1010'}}})));

    const message = 'The type identifier of an intent cannot be changed.';
    expect(await screen.findByRole('tab', {name: /General/})).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tab', {name: /General Has problems/})).toBeInTheDocument();
    expect(screen.getByLabelText('Type identifier')).toHaveAttribute('aria-invalid', 'true');
    expect(screen.getAllByText(message), 'beside the type identifier only, not again in the save bar').toHaveLength(1);
    await waitFor(() => expect(screen.getByLabelText('Type identifier')).toHaveFocus());
  });

  it('does not save an invalid form, and names the tabs to fix', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await user.click(screen.getByRole('tab', {name: 'External processor'}));
    await user.click(screen.getByRole('switch', {name: 'Use an external processor'}));

    expect(screen.getByText('Fix the problems on these tabs before saving: External processor')).toBeInTheDocument();
    expect(screen.getByRole('tab', {name: 'General'}), 'the General tab has no problem').toBeInTheDocument();
    expect(screen.getByRole('tab', {name: /External processor Has problems/})).toBeInTheDocument();
    expect(screen.queryByText('Enter the URL of your external processor.'), 'not before saving').toBeNull();

    await user.click(screen.getByRole('tab', {name: 'General'}));
    await user.click(screen.getByRole('button', {name: 'Save'}));
    expect(mockUpdate).not.toHaveBeenCalled();
    expect(screen.getByRole('tab', {name: /External processor/})).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByText(/Some fields are set or changed by the external processor/)).toBeInTheDocument();
  });

  it('keeps a name that is too long in its field, and takes focus there when saved', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await user.click(screen.getByRole('tab', {name: 'Schema'}));
    await user.click(screen.getByRole('button', {name: 'Edit intent name'}));
    fireEvent.change(screen.getByLabelText('Name'), {target: {value: 'x'.repeat(101)}});
    await user.keyboard('{Enter}');

    expect(screen.getByLabelText('Name'), 'the field stays open with its problem').toHaveAttribute(
      'aria-invalid',
      'true',
    );
    expect(screen.getByText('Enter a name of up to 100 characters.')).toBeInTheDocument();
    expect(screen.getByText(UNSAVED)).toBeInTheDocument();
    expect(screen.getByRole('tab', {name: 'General'}), 'the name is not on the General tab').toBeInTheDocument();

    await user.click(screen.getByRole('tab', {name: 'External processor'}));
    await user.click(screen.getByRole('button', {name: 'Save'}));
    expect(mockUpdate).not.toHaveBeenCalled();
    expect(screen.getByRole('tab', {name: 'External processor'}), 'the open tab is kept').toHaveAttribute(
      'aria-selected',
      'true',
    );
    await waitFor(() => expect(screen.getByLabelText('Name'), 'the name takes focus').toHaveFocus());

    fireEvent.change(screen.getByLabelText('Name'), {target: {value: 'Payment'}});
    await user.keyboard('{Enter}');
    expect(screen.getByRole('heading', {name: 'Payment'})).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Save'}));
    expect(mockUpdate).toHaveBeenCalledWith(
      {id: 'type-1', data: {...paymentRequest, name: 'Payment'}},
      expect.any(Object),
    );
  });

  it('does not apply an empty name, and puts the name back on Escape', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await user.click(screen.getByRole('button', {name: 'Edit intent name'}));
    await user.clear(screen.getByLabelText('Name'));
    expect(screen.getByText('Enter a name.')).toBeInTheDocument();
    await user.keyboard('{Enter}');
    expect(screen.getByRole('heading', {name: 'Payment initiation'})).toBeInTheDocument();
    expect(screen.queryByLabelText('Name')).toBeNull();
    expect(screen.queryByText(UNSAVED)).toBeNull();

    await user.click(screen.getByRole('button', {name: 'Edit intent name'}));
    await user.type(screen.getByLabelText('Name'), ' v2{Escape}');
    expect(screen.getByRole('heading', {name: 'Payment initiation'})).toBeInTheDocument();
    expect(screen.queryByText(UNSAVED)).toBeNull();

    await user.click(screen.getByRole('button', {name: 'Edit intent name'}));
    await user.type(screen.getByLabelText('Name'), ' v2');
    await user.click(screen.getByRole('tab', {name: 'Schema'}));
    expect(
      screen.getByRole('heading', {name: 'Payment initiation v2'}),
      'leaving the field applies it',
    ).toBeInTheDocument();
    expect(screen.getByText(UNSAVED)).toBeInTheDocument();
  });

  it('edits the description in the header, and puts it back on Escape', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await user.click(screen.getByRole('button', {name: 'Edit intent description'}));
    expect(screen.getByLabelText('Description')).toHaveAttribute('placeholder', 'Add a description');
    await user.type(screen.getByLabelText('Description'), 'Pays once{Escape}');
    expect(screen.getByText('No description')).toBeInTheDocument();
    expect(screen.queryByText(UNSAVED)).toBeNull();

    await typeDescription(user, '  Pays once  ');
    expect(screen.getByText('Pays once')).toBeInTheDocument();
    expect(screen.queryByLabelText('Description')).toBeNull();
    await user.click(screen.getByRole('button', {name: 'Save'}));
    expect(mockUpdate).toHaveBeenCalledWith(
      {id: 'type-1', data: {...paymentRequest, description: 'Pays once'}},
      expect.any(Object),
    );
  });

  it('sets the document title to the name of the type', async () => {
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await waitFor(() => expect(document.title).toBe('Payment initiation | ThunderID'));
  });

  it('names the fields and consent tab when the summary names a field the type does not have', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await user.click(screen.getByRole('tab', {name: 'Schema'}));
    await user.click(screen.getByRole('button', {name: 'Consent summary'}));
    fireEvent.change(screen.getByLabelText('Summary'), {target: {value: 'Pay {{payee}}'}});

    expect(screen.getByText('Fix the problems on these tabs before saving: Schema')).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Close summary settings'}));
    await user.click(screen.getByRole('button', {name: 'Save'}));
    expect(mockUpdate).not.toHaveBeenCalled();
    expect(screen.getByLabelText('Summary'), 'the summary opens to show its problem').toHaveValue('Pay {{payee}}');
  });

  it('shows a save failure until the form changes', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await typeDescription(user, 'Pays once');
    await user.click(screen.getByRole('button', {name: 'Save'}));

    const {onError} = mockUpdate.mock.calls[0][1] as UpdateCallbacks;
    onError(Object.assign(new Error('read-only'), {response: {data: {code: 'RAR-1013'}}}));
    const message = 'This intent is defined in a declarative file and cannot be changed here.';
    expect(await screen.findByText(message)).toBeInTheDocument();
    expect(screen.getByText(UNSAVED), 'the changes are kept').toBeInTheDocument();

    await typeDescription(user, '!');
    expect(screen.queryByText(message)).not.toBeInTheDocument();
  });

  it('saves once when Save is clicked twice, and takes no input while the save is in flight', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await typeDescription(user, 'Pays once');
    const save = screen.getByRole('button', {name: 'Save'});
    act(() => {
      save.click();
      save.click();
    });
    await waitFor(() => expect(mockUpdate).toHaveBeenCalledTimes(1));
    expect(screen.getByRole('button', {name: 'Saving…'})).toBeDisabled();
    expect(
      screen.getByRole('tab', {name: 'General'}).closest('[inert]'),
      'a value typed now would be lost when the form starts again from the saved type',
    ).not.toBeNull();
    expect(screen.getByRole('button', {name: 'Edit intent name'})).toBeDisabled();
    expect(screen.getByRole('button', {name: 'Edit intent description'})).toBeDisabled();

    const {onError} = mockUpdate.mock.calls[0][1] as UpdateCallbacks;
    act(() => onError(Object.assign(new Error('boom'), {response: {status: 500}})));
    expect(await screen.findByRole('button', {name: 'Save'})).toBeEnabled();
    expect(screen.getByRole('tab', {name: 'General'}).closest('[inert]')).toBeNull();
    expect(screen.getByRole('button', {name: 'Edit intent description'})).toBeEnabled();
  });

  it('does not count spaces around a value, or keywords in another order, as a change', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await typeName(user, ' ');
    expect(screen.queryByText(UNSAVED)).toBeNull();

    await user.click(screen.getByRole('tab', {name: 'Schema'}));
    await user.click(screen.getByRole('button', {name: 'JSON'}));
    const editor = screen.getByLabelText('Schema JSON');
    const schema = JSON.parse((editor as HTMLTextAreaElement).value) as {properties: Record<string, object>};
    schema.properties.amount = Object.fromEntries(Object.entries(schema.properties.amount).reverse());
    fireEvent.change(editor, {target: {value: JSON.stringify(schema)}});
    expect(screen.queryByText(UNSAVED)).toBeNull();
  });

  it('logs a failed save without the request it sent', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await typeDescription(user, 'Pays once');
    await user.click(screen.getByRole('button', {name: 'Save'}));
    const {onError} = mockUpdate.mock.calls[0][1] as UpdateCallbacks;
    const err = Object.assign(new Error('bad'), {
      config: {
        data: '{"processor":{"authentication":{"properties":{"token":"secret"}}}}',
        headers: {Authorization: 'x'},
      },
      response: {status: 400, data: {code: 'RAR-1008'}},
    });
    act(() => onError(err));
    expect(mockLoggerError).toHaveBeenCalledWith('Failed to save intent', {status: 400, code: 'RAR-1008'});
  });

  it('checks for a change made elsewhere before saving, and saves over it only once told', async () => {
    const user = userEvent.setup();
    const {rerender} = renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await typeDescription(user, 'Pays once');
    // The server keeps no version check, and a name change keeps the version.
    const renamed = {...paymentType, name: 'Renamed elsewhere'};
    mockRefetch.mockImplementation(() => {
      loaded(renamed);
      return Promise.resolve({data: renamed, error: null});
    });
    await user.click(screen.getByRole('button', {name: 'Save'}));
    await waitFor(() => expect(mockRefetch).toHaveBeenCalled());
    rerender(<AuthorizationDetailTypeEditPage />);

    const notice = await screen.findByText(/Someone else changed this intent after you opened it/);
    expect(mockUpdate, 'the change elsewhere is not replaced without a word').not.toHaveBeenCalled();
    await waitFor(() => expect(notice.closest('[role="alert"]')).toHaveFocus());

    await user.click(screen.getByRole('button', {name: 'Save'}));
    await waitFor(() =>
      expect(mockUpdate).toHaveBeenCalledWith(
        {id: 'type-1', data: {...paymentRequest, description: 'Pays once'}},
        expect.any(Object),
      ),
    );
  });

  it('counts a change to the processor credentials made elsewhere', async () => {
    const user = userEvent.setup();
    const {rerender} = renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await typeDescription(user, 'Pays once');
    loaded({
      ...paymentType,
      processor: {
        endpoint: 'https://bank.example.com/process',
        authentication: {type: 'BEARER', properties: {token: '******'}},
      },
    });
    rerender(<AuthorizationDetailTypeEditPage />);

    expect(screen.getByText(/Someone else changed this intent after you opened it/)).toBeInTheDocument();
  });

  it('does not save over a type deleted since the form opened', async () => {
    const user = userEvent.setup();
    const {rerender} = renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await typeDescription(user, 'Pays once');
    const gone = Object.assign(new Error('gone'), {response: {status: 404, data: {code: 'RAR-1002'}}});
    mockRefetch.mockImplementation(() => {
      loaded(paymentType, gone);
      return Promise.resolve({data: paymentType, error: gone});
    });
    await user.click(screen.getByRole('button', {name: 'Save'}));
    await waitFor(() => expect(mockRefetch).toHaveBeenCalled());
    rerender(<AuthorizationDetailTypeEditPage />);

    expect(await screen.findByText('This intent was deleted, so your changes cannot be saved.')).toBeInTheDocument();
    expect(mockUpdate).not.toHaveBeenCalled();
  });

  it('opens a declarative type read-only, with nothing to save or delete', async () => {
    const user = userEvent.setup();
    loaded({...paymentType, isReadOnly: true});
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    expect(screen.getByText('This resource is read-only and cannot be modified.')).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Edit intent name'})).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Edit intent description'})).not.toBeInTheDocument();
    expect(screen.queryByRole('tab', {name: 'Advanced'}), 'there is nothing to delete').not.toBeInTheDocument();
    expect(screen.queryByText('Danger zone')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Delete intent'})).not.toBeInTheDocument();
    expect(screen.getByRole('switch', {name: /Ask every time/})).toBeDisabled();
    const writeText = vi.spyOn(navigator.clipboard, 'writeText');
    await user.click(screen.getByRole('button', {name: 'Copy type identifier'}));
    expect(writeText, 'the identifiers can still be copied').toHaveBeenCalledWith('payment_initiation');

    await user.click(screen.getByRole('tab', {name: 'Schema'}));
    expect(screen.getByText(/Select the summary or a field on the card to see it\./)).toBeInTheDocument();
    expect(screen.queryByText(/to edit it\./)).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Add field'})).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Consent summary'}));
    expect(screen.getByLabelText('Summary')).toBeDisabled();
    await user.click(screen.getByRole('tab', {name: 'External processor'}));
    expect(screen.getByRole('switch', {name: 'Use an external processor'})).toBeDisabled();

    expect(screen.queryByRole('button', {name: 'Save'})).not.toBeInTheDocument();
    expect(mockUpdate).not.toHaveBeenCalled();
  });

  it('deletes the type from the danger zone after confirmation, then returns to the list', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await user.click(screen.getByRole('tab', {name: 'Advanced'}));
    expect(screen.getByText('Danger zone')).toBeInTheDocument();
    expect(screen.getByText('Irreversible actions for this intent.')).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Delete intent'}));
    const dialog = screen.getByRole('dialog');
    expect(within(dialog).getByText('Payment initiation')).toBeInTheDocument();
    expect(mockDelete).not.toHaveBeenCalled();
    await user.click(within(dialog).getByRole('button', {name: 'Delete'}));

    expect(mockDelete).toHaveBeenCalledWith('type-1', expect.any(Object));
    const {onSuccess} = mockDelete.mock.calls[0][1] as {onSuccess: () => void};
    onSuccess();
    expect(mockShowToast, 'the delete hook confirms the delete').not.toHaveBeenCalled();
    expect(mockNavigate).toHaveBeenCalledWith('/authorization-detail-types');
  });

  it('keeps the type open when the delete is cancelled', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await user.click(screen.getByRole('tab', {name: 'Advanced'}));
    await user.click(screen.getByRole('button', {name: 'Delete intent'}));
    await user.click(within(screen.getByRole('dialog')).getByRole('button', {name: 'Cancel'}));

    expect(mockDelete).not.toHaveBeenCalled();
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it('reports a type the first load finds none of in place of the page, and leads back to the list', async () => {
    const user = userEvent.setup();
    loaded(undefined, Object.assign(new Error('raw not found'), {response: {status: 404, data: {code: 'RAR-1002'}}}));
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    expect(screen.getByRole('alert')).toHaveTextContent('Intent not found.');
    expect(screen.queryByText('Failed to load intent')).not.toBeInTheDocument();
    expect(screen.queryByText('raw not found')).not.toBeInTheDocument();
    expect(screen.queryByRole('tab')).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Refresh'}), 'trying again does not find it').toBeNull();
    await user.click(screen.getByRole('button', {name: 'Back to intents'}));
    expect(mockNavigate).toHaveBeenCalledWith('/authorization-detail-types');
  });

  it('reports any other failed load with a way to retry', async () => {
    const user = userEvent.setup();
    loaded(undefined, Object.assign(new Error('raw failure'), {response: {status: 500}}));
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    expect(screen.getByText('Failed to load intent')).toBeInTheDocument();
    expect(screen.queryByText('raw failure')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Refresh'}));
    expect(mockRefetch).toHaveBeenCalled();
  });

  it('reports a load that returns nothing in place of the page, with a way back', async () => {
    const user = userEvent.setup();
    loaded(undefined);
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    expect(screen.getByRole('alert')).toHaveTextContent('Intent not found.');
    expect(screen.queryByText('Failed to load intent')).not.toBeInTheDocument();
    expect(screen.queryByRole('tab')).not.toBeInTheDocument();
    await waitFor(() => expect(document.title).toBe('Intents | ThunderID'));
    await user.click(screen.getByRole('button', {name: 'Back to intents'}));
    expect(mockNavigate).toHaveBeenCalledWith('/authorization-detail-types');
  });

  it('links back to the list', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await user.click(screen.getByText('Back to intents'));
    expect(mockNavigate).toHaveBeenCalledWith('/authorization-detail-types');
  });

  it('asks before leaving through the back link with unsaved changes', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await typeDescription(user, 'Pays once');
    await user.click(screen.getByText('Back to intents'));

    const dialog = screen.getByRole('dialog', {name: 'Discard unsaved changes?'});
    expect(dialog).toHaveTextContent('You have changes to this intent that are not saved.');
    expect(mockNavigate).not.toHaveBeenCalled();
    await user.click(within(dialog).getByRole('button', {name: 'Keep editing'}));
    await waitFor(() => expect(screen.queryByRole('dialog')).toBeNull());
    expect(screen.getByText('Pays once')).toBeInTheDocument();

    await user.click(screen.getByText('Back to intents'));
    await user.click(
      within(screen.getByRole('dialog', {name: 'Discard unsaved changes?'})).getByRole('button', {
        name: 'Discard changes',
      }),
    );
    expect(mockNavigate).toHaveBeenCalledWith('/authorization-detail-types');
  });

  it('selects and focuses the field with a problem when saved from another tab', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await user.click(screen.getByRole('tab', {name: 'Schema'}));
    await user.click(screen.getByRole('treeitem', {name: 'Amount'}));
    await user.click(screen.getByRole('button', {name: /^Limits/}));
    await user.clear(screen.getByLabelText('Maximum'));
    await user.type(screen.getByLabelText('Maximum'), 'lots');
    await user.click(screen.getByRole('button', {name: 'Close field settings'}));
    await user.click(screen.getByRole('tab', {name: /General/}));
    await user.click(screen.getByRole('button', {name: 'Save'}));

    expect(mockUpdate).not.toHaveBeenCalled();
    expect(screen.getByRole('tab', {name: /Schema/})).toHaveAttribute('aria-selected', 'true');
    expect(await screen.findByRole('region', {name: 'Field settings'})).toBeInTheDocument();
    expect(screen.getByText('Enter the minimum and maximum as values of this type.')).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('treeitem', {name: 'Amount'})).toHaveFocus());

    await user.click(screen.getByRole('button', {name: 'Close field settings'}));
    await user.click(screen.getByRole('tab', {name: /General/}));
    await user.click(screen.getByRole('tab', {name: /Schema/}));
    expect(screen.queryByRole('region', {name: 'Field settings'}), 'a problem is selected once per save').toBeNull();
  });

  it('does not warn of a new version when the fields only change order', async () => {
    const user = userEvent.setup();
    loaded({
      ...paymentType,
      schema: {
        type: 'object',
        required: ['amount'],
        properties: {...(paymentType.schema as {properties: object}).properties, payee: {type: 'string'}},
      },
    });
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await user.click(screen.getByRole('tab', {name: 'Schema'}));
    const payee = screen.getByRole('treeitem', {name: 'payee'});
    payee.focus();
    await user.keyboard('{Alt>}{ArrowUp}{/Alt}');
    expect(screen.getAllByRole('treeitem').map((row) => row.getAttribute('aria-label'))).toEqual(['payee', 'Amount']);
    expect(screen.getByText(UNSAVED)).toBeInTheDocument();
    expect(screen.queryByText(NEW_VERSION)).toBeNull();
  });

  it('opens the processor tab to show an endpoint the server refused', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await typeDescription(user, 'Pays once');
    await user.click(screen.getByRole('button', {name: 'Save'}));
    const {onError} = mockUpdate.mock.calls[0][1] as UpdateCallbacks;
    act(() => onError(Object.assign(new Error('bad'), {response: {status: 400, data: {code: 'RAR-1007'}}})));

    const message =
      'The external processor endpoint must be an HTTPS URL, or HTTP on a loopback address, with a valid port and no user name or password.';
    expect(await screen.findByRole('tab', {name: /External processor/})).toHaveAttribute('aria-selected', 'true');
    expect(screen.getByLabelText(/^Endpoint URL/)).toHaveAttribute('aria-invalid', 'true');
    expect(screen.getAllByText(message), 'beside the endpoint only, not again in the save bar').toHaveLength(1);
    expect(screen.getByText('Fix the problems on these tabs before saving: External processor')).toBeInTheDocument();
    await waitFor(() => expect(screen.getByRole('tabpanel')).toContainElement(document.activeElement as HTMLElement));
  });

  it('shows a name the server refused beside the name only, not again in the save bar', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await user.click(screen.getByRole('tab', {name: 'Schema'}));
    await typeName(user, ' v2');
    await user.click(screen.getByRole('button', {name: 'Save'}));
    const {onError} = mockUpdate.mock.calls[0][1] as UpdateCallbacks;
    act(() => onError(Object.assign(new Error('bad'), {response: {status: 400, data: {code: 'RAR-1005'}}})));

    const message = 'The name is required and can be at most 100 characters.';
    expect(await screen.findAllByText(message)).toHaveLength(1);
    expect(screen.getByLabelText('Name'), 'the name opens in the header').toHaveAttribute('aria-invalid', 'true');
    expect(screen.getByRole('tab', {name: 'Schema'}), 'the open tab is kept').toHaveAttribute('aria-selected', 'true');
    expect(screen.getByRole('tab', {name: 'General'}), 'the General tab is not marked').toBeInTheDocument();
    await waitFor(() => expect(screen.getByLabelText('Name'), 'the refused value takes focus').toHaveFocus());

    await user.type(screen.getByLabelText('Name'), '!{Enter}');
    expect(screen.queryByText(message), 'the error goes once the name changes').toBeNull();
    expect(screen.getByRole('heading', {name: 'Payment initiation v2!'})).toBeInTheDocument();
  });

  it('opens the processor tab to show a credential the server refused on the credential', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await user.click(screen.getByRole('tab', {name: 'External processor'}));
    await user.click(screen.getByRole('combobox', {name: /Authentication method/}));
    await user.click(screen.getByRole('option', {name: 'Bearer token'}));
    await user.type(document.getElementById('connection-field-bearerToken')!, 'tok');
    await user.click(screen.getByRole('tab', {name: 'General'}));
    await user.click(screen.getByRole('button', {name: 'Save'}));
    const {onError} = mockUpdate.mock.calls[0][1] as UpdateCallbacks;
    act(() => onError(Object.assign(new Error('bad'), {response: {status: 400, data: {code: 'RAR-1008'}}})));

    const message = 'The external processor authentication is not valid. Check the method and its credentials.';
    expect(await screen.findByRole('tab', {name: /External processor/})).toHaveAttribute('aria-selected', 'true');
    expect(document.getElementById('connection-field-bearerToken')).toHaveAttribute('aria-invalid', 'true');
    expect(screen.getAllByText(message)).toHaveLength(1);
    expect(screen.getByLabelText(/^Endpoint URL/)).not.toHaveAttribute('aria-invalid', 'true');
  });

  it('says the type was deleted when a save finds it gone', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await typeDescription(user, 'Pays once');
    await user.click(screen.getByRole('button', {name: 'Save'}));
    const {onError} = mockUpdate.mock.calls[0][1] as UpdateCallbacks;
    act(() => onError(Object.assign(new Error('gone'), {response: {status: 404, data: {code: 'RAR-1002'}}})));

    expect(
      await screen.findByText('This intent no longer exists. It may have already been deleted.'),
    ).toBeInTheDocument();
    expect(screen.getByText('Pays once'), 'the changes are kept').toBeInTheDocument();
  });

  it('fetches the latest version when the type changed while the save was applied', async () => {
    const user = userEvent.setup();
    const {rerender} = renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await typeDescription(user, 'Pays once');
    await user.click(screen.getByRole('button', {name: 'Save'}));
    const {onError} = mockUpdate.mock.calls[0][1] as UpdateCallbacks;
    const changed = {...paymentType, consentTemplate: 'Pay up to {{amount}}', version: 4};
    mockRefetch.mockImplementation(() => {
      loaded(changed);
      return Promise.resolve({data: changed, error: null});
    });
    act(() => onError(Object.assign(new Error('changed'), {response: {status: 409, data: {code: 'RAR-1016'}}})));

    expect(
      await screen.findByText('This intent changed while it was being saved. Load the latest version and save again.'),
    ).toBeInTheDocument();
    expect(mockRefetch).toHaveBeenCalled();
    rerender(<AuthorizationDetailTypeEditPage />);
    expect(screen.getByText(/Someone else changed this intent after you opened it/)).toBeInTheDocument();
    expect(screen.getByText('Pays once'), 'the changes are kept').toBeInTheDocument();
  });

  it('resolves a save that gets no response through the catalog fallback', async () => {
    const user = userEvent.setup();
    renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await typeDescription(user, 'Pays once');
    await user.click(screen.getByRole('button', {name: 'Save'}));
    const {onError} = mockUpdate.mock.calls[0][1] as UpdateCallbacks;
    act(() => onError(Object.assign(new Error('Network Error'), {code: 'ERR_NETWORK'})));

    expect(await screen.findByText('Failed to save the intent. Please try again.')).toBeInTheDocument();
    expect(screen.queryByText('Network Error')).toBeNull();
  });

  it('warns when the type changed elsewhere, and loads the latest version on request', async () => {
    const user = userEvent.setup();
    const {rerender} = renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await typeDescription(user, 'Pays once');
    loaded({...paymentType, name: 'Payment initiation (renamed)', version: 4});
    rerender(<AuthorizationDetailTypeEditPage />);

    expect(screen.getByText(/Someone else changed this intent after you opened it/)).toBeInTheDocument();
    expect(screen.getByText('Pays once'), 'the open form is kept').toBeInTheDocument();

    await user.click(screen.getByRole('button', {name: 'Load latest'}));
    expect(screen.queryByRole('dialog'), 'loading the latest version does not ask, as Reset does not').toBeNull();
    expect(await screen.findByRole('heading', {name: 'Payment initiation (renamed)'})).toBeInTheDocument();
    expect(screen.getByText('No description')).toBeInTheDocument();
    expect(screen.queryByText(/Someone else changed this intent/)).toBeNull();
  });

  it('keeps the button that loads the latest version on one line', async () => {
    const user = userEvent.setup();
    const {rerender} = renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await typeDescription(user, 'Pays once');
    loaded({...paymentType, name: 'Payment initiation (renamed)', version: 4});
    rerender(<AuthorizationDetailTypeEditPage />);

    expect(getComputedStyle(screen.getByRole('button', {name: 'Load latest'})).whiteSpace).toBe('nowrap');
  });

  it('takes a change made elsewhere at once, without a warning, while the form has no changes', async () => {
    const {rerender} = renderWithProviders(<AuthorizationDetailTypeEditPage />);
    expect(screen.getByText('No description')).toBeInTheDocument();

    loaded({...paymentType, name: 'Payment initiation (renamed)', description: 'Pays once', version: 4});
    rerender(<AuthorizationDetailTypeEditPage />);

    expect(await screen.findByRole('heading', {name: 'Payment initiation (renamed)'})).toBeInTheDocument();
    expect(screen.getByText('Pays once'), 'the form shows the latest version').toBeInTheDocument();
    expect(screen.queryByText(/Someone else changed this intent/)).toBeNull();
    expect(screen.queryByRole('button', {name: 'Load latest'})).toBeNull();
    expect(screen.queryByText(UNSAVED), 'the form starts again from the latest version').toBeNull();
  });

  it.each([
    ['RAR-1002', {status: 404, data: {code: 'RAR-1002'}}],
    ['a 404', {status: 404}],
  ])('keeps the open form when a refresh finds the type deleted (%s), and stops saving', async (_case, response) => {
    const user = userEvent.setup();
    const {rerender} = renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await typeDescription(user, 'Pays once');
    loaded(paymentType, Object.assign(new Error('gone'), {response}));
    rerender(<AuthorizationDetailTypeEditPage />);

    expect(screen.getByText('This intent was deleted, so your changes cannot be saved.')).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Refresh'}), 'not reported as a failed refresh too').toBeNull();
    expect(screen.getByText('Pays once')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();
  });

  it('says a type deleted elsewhere can no longer be edited while the form has no changes', () => {
    const {rerender} = renderWithProviders(<AuthorizationDetailTypeEditPage />);

    loaded(paymentType, Object.assign(new Error('gone'), {response: {status: 404, data: {code: 'RAR-1002'}}}));
    rerender(<AuthorizationDetailTypeEditPage />);

    expect(screen.getByText('This intent was deleted, so it can no longer be edited.')).toBeInTheDocument();
    expect(screen.queryByText(/your changes cannot be saved/)).toBeNull();
  });

  it('keeps the open form when a refresh fails, and says so with a way to retry', async () => {
    const user = userEvent.setup();
    const {rerender} = renderWithProviders(<AuthorizationDetailTypeEditPage />);

    await typeDescription(user, 'Pays once');
    loaded(paymentType, Object.assign(new Error('raw failure'), {response: {status: 500, data: {code: 'RAR-1001'}}}));
    rerender(<AuthorizationDetailTypeEditPage />);

    expect(screen.getByText('The request is malformed or contains invalid data.')).toBeInTheDocument();
    expect(screen.queryByText('raw failure')).toBeNull();
    expect(screen.getByText('Pays once')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Save'})).toBeEnabled();
    await user.click(screen.getByRole('button', {name: 'Refresh'}));
    expect(mockRefetch).toHaveBeenCalled();
  });
});
