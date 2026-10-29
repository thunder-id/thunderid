// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import {screen, cleanup, fireEvent} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {CONSENT_MODIFIED_KEY_PREFIX, ThemeProvider, type EmbeddedFlowComponent} from '@thunderid/react';
import {TEST_CN_PREFIX} from '@thunderid/test-utils';
import i18next from 'i18next';
import {useState, type JSX, type ReactNode} from 'react';
import {I18nextProvider, initReactI18next} from 'react-i18next';
import {describe, it, expect, afterEach, vi} from 'vitest';
import renderWithDesignProviders from '../../../../test/renderWithProviders';
import FlowComponentRenderer from '../../FlowComponentRenderer';
import ConsentAdapter from '../ConsentAdapter';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

const noop = () => undefined;

// The SDK's consent components read the SDK theme, which the Gate's ThunderIDProvider supplies.
function renderWithProviders(ui: ReactNode) {
  const result = renderWithDesignProviders(<ThemeProvider>{ui}</ThemeProvider>);
  return {...result, rerender: (next: ReactNode) => result.rerender(<ThemeProvider>{next}</ThemeProvider>)};
}

// Mirrors the permission purpose the backend builds at runtime: it is never persisted, so purposeId
// arrives as an empty string and essential arrives as null.
const permissionsPurpose = JSON.stringify([
  {
    purposeName: 'permissions:app1',
    purposeId: '',
    type: 'permissions',
    essential: null,
    optional: [{name: 'system'}],
  },
]);

const nestedPermissionsPurpose = JSON.stringify([
  {
    purposeName: 'permissions:app1',
    purposeId: '',
    type: 'permissions',
    essential: null,
    optional: [{name: 'users'}, {name: 'users:read', parent: 'users'}, {name: 'users:write', parent: 'users'}],
  },
]);

const permissionKey = (name: string) => `__consent_opt____${name}`;

const switchFor = (name: string) => screen.getByRole<HTMLInputElement>('switch', {name});

describe('ConsentAdapter — permission default state', () => {
  it('renders an untouched permission as unchecked', () => {
    renderWithProviders(<ConsentAdapter consentData={permissionsPurpose} formValues={{}} onInputChange={noop} />);

    expect(switchFor('system').checked).toBe(false);
  });

  it('renders a permission the user enabled as checked', () => {
    renderWithProviders(
      <ConsentAdapter
        consentData={permissionsPurpose}
        formValues={{[permissionKey('system')]: 'true'}}
        onInputChange={noop}
      />,
    );

    expect(switchFor('system').checked).toBe(true);
  });

  it('renders a permission the user disabled as unchecked', () => {
    renderWithProviders(
      <ConsentAdapter
        consentData={permissionsPurpose}
        formValues={{[permissionKey('system')]: 'false'}}
        onInputChange={noop}
      />,
    );

    expect(switchFor('system').checked).toBe(false);
  });

  it('leaves every permission in a nested tree unchecked when nothing is selected', () => {
    renderWithProviders(<ConsentAdapter consentData={nestedPermissionsPurpose} formValues={{}} onInputChange={noop} />);

    expect(switchFor('users').checked).toBe(false);
    expect(switchFor('users:read').checked).toBe(false);
    expect(switchFor('users:write').checked).toBe(false);
  });
});

describe('ConsentAdapter — permission rollup', () => {
  it('cascades a parent toggle to its descendants', () => {
    const onInputChange = vi.fn();

    renderWithProviders(
      <ConsentAdapter consentData={nestedPermissionsPurpose} formValues={{}} onInputChange={onInputChange} />,
    );

    fireEvent.click(switchFor('users'));

    expect(onInputChange).toHaveBeenCalledWith(permissionKey('users'), 'true');
    expect(onInputChange).toHaveBeenCalledWith(permissionKey('users:read'), 'true');
    expect(onInputChange).toHaveBeenCalledWith(permissionKey('users:write'), 'true');
  });

  it('shows the parent as checked once every descendant is checked', () => {
    renderWithProviders(
      <ConsentAdapter
        consentData={nestedPermissionsPurpose}
        formValues={{
          [permissionKey('users:read')]: 'true',
          [permissionKey('users:write')]: 'true',
        }}
        onInputChange={noop}
      />,
    );

    expect(switchFor('users').checked).toBe(true);
  });
});

const paymentDetail = {
  name: 'detail-1',
  detailType: 'payment_initiation',
  summary: 'Pay 45 EUR to Merchant A',
  rows: [
    {
      path: 'instructedAmount.amount',
      label: 'Amount',
      value: '45.00',
      modification: 'lte',
      as: 'decimal',
      origin: 'server',
    },
    {path: 'creditorName', label: 'Creditor', value: 'Merchant A', origin: 'client'},
    {path: 'locations', label: 'Locations', value: ['https://a.example', 'https://b.example']},
    {path: 'creditorAccount', label: 'Account', value: {iban: 'DE02'}},
    {path: 'recurring', label: 'Recurring', value: false},
    {path: 'count', label: 'Count', value: 3},
  ],
};

const detailPurpose = (...elements: object[]) => ({
  purposeName: 'authorization_details:app-1',
  purposeId: 'authorization_details:app-1',
  type: 'authorization_details',
  essential: [],
  optional: elements,
});

const paymentDetailPurpose = detailPurpose(paymentDetail);

const attributesPurpose = {
  purposeName: 'profile',
  purposeId: 'p1',
  essential: [{name: 'email'}],
  optional: [{name: 'phone'}],
};

const detailKey = (name: string) => `__consent_opt__authorization_details:app-1__${name}`;

const detailSwitch = (detailId: string): HTMLInputElement => {
  const input = document.getElementById(`consent_opt_authorization_details:app-1_${detailId}`);
  if (!(input instanceof HTMLInputElement)) throw new Error(`No approve switch for ${detailId}`);
  return input;
};

describe('ConsentAdapter — authorization details', () => {
  it('renders the server summary and read-only rows', () => {
    renderWithProviders(
      <ConsentAdapter consentData={JSON.stringify([paymentDetailPurpose])} formValues={{}} onInputChange={noop} />,
    );

    expect(screen.getByText('Pay 45 EUR to Merchant A')).toBeInTheDocument();
    expect(screen.getByText('Amount')).toBeInTheDocument();
    expect(screen.getByText('45.00')).toBeInTheDocument();
    expect(screen.getByText('Merchant A')).toBeInTheDocument();
    expect(screen.getByText('https://a.example, https://b.example')).toBeInTheDocument();
    expect(screen.getByText('{ "iban": "DE02" }')).toBeInTheDocument();
    expect(screen.getByText('false')).toBeInTheDocument();
    expect(screen.getByText('3')).toBeInTheDocument();
  });

  it('renders a list of plain values as comma separated text without an origin marker', () => {
    const purpose = detailPurpose({
      ...paymentDetail,
      rows: [{path: 'actions', label: 'Actions', value: ['initiate', 'status'], origin: 'server'}],
    });
    renderWithProviders(
      <ConsentAdapter consentData={JSON.stringify([purpose])} formValues={{}} onInputChange={noop} />,
    );

    expect(screen.getByText('initiate, status')).toBeInTheDocument();
    expect(screen.queryByText('consent:authorizationDetails.serverProvided')).not.toBeInTheDocument();
  });

  it('exposes one approve toggle per detail with a stable id, starting approved', () => {
    const onInputChange = vi.fn();
    renderWithProviders(
      <ConsentAdapter
        consentData={JSON.stringify([paymentDetailPurpose])}
        formValues={{}}
        onInputChange={onInputChange}
      />,
    );

    const toggle = detailSwitch('detail-1');
    expect(toggle.id).toBe('consent_opt_authorization_details:app-1_detail-1');
    expect(toggle).toHaveAttribute(
      'aria-describedby',
      'consent_authz_detail_authorization_details:app-1_detail-1_summary',
    );
    expect(toggle.checked).toBe(true);
    expect(onInputChange).toHaveBeenCalledWith(detailKey('detail-1'), 'true');
    expect(document.querySelectorAll('input[id^="consent_opt_authorization_details:app-1_"]')).toHaveLength(1);
  });

  it('reflects a declined detail from the form values', () => {
    renderWithProviders(
      <ConsentAdapter
        consentData={JSON.stringify([paymentDetailPurpose])}
        formValues={{[detailKey('detail-1')]: 'false'}}
        onInputChange={noop}
      />,
    );

    expect(detailSwitch('detail-1').checked).toBe(false);
  });

  it('reflects an approved detail from the form values', () => {
    renderWithProviders(
      <ConsentAdapter
        consentData={JSON.stringify([paymentDetailPurpose])}
        formValues={{[detailKey('detail-1')]: 'true'}}
        onInputChange={noop}
      />,
    );

    expect(detailSwitch('detail-1').checked).toBe(true);
  });

  it('writes the toggle state to the detail element the submitted decision is built from', () => {
    const onInputChange = vi.fn();

    const {rerender} = renderWithProviders(
      <ConsentAdapter
        consentData={JSON.stringify([paymentDetailPurpose])}
        formValues={{[detailKey('detail-1')]: 'true'}}
        onInputChange={onInputChange}
      />,
    );

    fireEvent.click(detailSwitch('detail-1'));
    expect(onInputChange).toHaveBeenLastCalledWith(detailKey('detail-1'), 'false');

    rerender(
      <ConsentAdapter
        consentData={JSON.stringify([paymentDetailPurpose])}
        formValues={{[detailKey('detail-1')]: 'false'}}
        onInputChange={onInputChange}
      />,
    );

    fireEvent.click(detailSwitch('detail-1'));
    expect(onInputChange).toHaveBeenLastCalledWith(detailKey('detail-1'), 'true');
  });

  it('renders a card with its own approve switch for each detail', () => {
    const second = {...paymentDetail, name: 'detail-2', summary: 'Pay 12 EUR to Merchant B'};
    renderWithProviders(
      <ConsentAdapter
        consentData={JSON.stringify([detailPurpose(paymentDetail, second)])}
        formValues={{[detailKey('detail-1')]: 'true', [detailKey('detail-2')]: 'false'}}
        onInputChange={noop}
      />,
    );

    expect(screen.getByText('Pay 45 EUR to Merchant A')).toBeInTheDocument();
    expect(screen.getByText('Pay 12 EUR to Merchant B')).toBeInTheDocument();
    expect(detailSwitch('detail-1').checked).toBe(true);
    expect(detailSwitch('detail-2').checked).toBe(false);
  });

  it('falls back to the detail type when the summary is absent', () => {
    const withoutSummary = detailPurpose({...paymentDetail, summary: undefined});

    renderWithProviders(
      <ConsentAdapter consentData={JSON.stringify([withoutSummary])} formValues={{}} onInputChange={noop} />,
    );

    expect(screen.getByText('payment_initiation')).toBeInTheDocument();
  });

  it('falls back to the detail id when summary and detail type are absent', () => {
    const bare = detailPurpose({...paymentDetail, summary: undefined, detailType: undefined, rows: undefined});

    renderWithProviders(<ConsentAdapter consentData={JSON.stringify([bare])} formValues={{}} onInputChange={noop} />);

    expect(screen.getByText('detail-1')).toBeInTheDocument();
    expect(screen.queryByText('authorization_details:app-1')).not.toBeInTheDocument();
  });

  it('keeps rendering non-detail purposes alongside detail purposes', () => {
    const onInputChange = vi.fn();

    renderWithProviders(
      <ConsentAdapter
        consentData={JSON.stringify([attributesPurpose, paymentDetailPurpose])}
        formValues={{[detailKey('detail-1')]: 'true'}}
        onInputChange={onInputChange}
      />,
    );

    expect(screen.getByText('Essential Attributes')).toBeInTheDocument();
    expect(screen.getByText('Optional Attributes')).toBeInTheDocument();
    expect(switchFor('email').checked).toBe(true);
    expect(switchFor('email').disabled).toBe(true);
    expect(switchFor('phone').checked).toBe(false);

    fireEvent.click(switchFor('phone'));
    expect(onInputChange).toHaveBeenCalledWith('__consent_opt__p1__phone', 'true');
    expect(onInputChange).not.toHaveBeenCalledWith(detailKey('detail-1'), expect.anything());
    expect(detailSwitch('detail-1').checked).toBe(true);
  });
});

const modifiableDetailPurpose = detailPurpose({
  ...paymentDetail,
  rows: [
    {path: 'instructedAmount.amount', label: 'Amount', value: 35, modification: 'decrease', as: 'number'},
    {path: 'actions', label: 'Actions', value: ['initiate', 'status'], modification: 'removeItems'},
    {path: 'toDate', label: 'Until', value: '2026-09-30', modification: 'decrease', as: 'date'},
    {path: 'creditorName', label: 'Creditor', value: 'Merchant A'},
  ],
});

const modifiedKey = (path: string): string =>
  `${CONSENT_MODIFIED_KEY_PREFIX}authorization_details:app-1__detail-1__${path}`;

const modifyInput = (path: string, suffix = ''): HTMLInputElement => {
  const input = document.getElementById(`consent_modify_authorization_details:app-1_detail-1_${path}${suffix}`);
  if (!(input instanceof HTMLInputElement)) throw new Error(`No input for ${path}`);
  return input;
};

describe('ConsentAdapter — modifying authorization details', () => {
  const renderModifiable = (formValues: Record<string, string> = {}) => {
    const onInputChange = vi.fn();
    renderWithProviders(
      <ConsentAdapter
        consentData={JSON.stringify([modifiableDetailPurpose])}
        formValues={formValues}
        onInputChange={onInputChange}
      />,
    );
    return onInputChange;
  };

  it('lets the user lower a number, bounded by the value shown', () => {
    const onInputChange = renderModifiable();
    const amount = modifyInput('instructedAmount.amount');

    expect(amount.value).toBe('35');
    expect(amount.max).toBe('35');

    fireEvent.change(amount, {target: {value: '20'}});
    fireEvent.change(amount, {target: {value: '50'}});

    expect(onInputChange).toHaveBeenCalledWith(modifiedKey('instructedAmount.amount'), '20');
    expect(onInputChange).not.toHaveBeenCalledWith(modifiedKey('instructedAmount.amount'), '50');
  });

  it('lets the user remove items from a list, add them back, and keeps the last one', () => {
    const chip = (index: number): HTMLElement => {
      const element = document.getElementById(`consent_modify_authorization_details:app-1_detail-1_actions_${index}`);
      if (!element) throw new Error(`No chip ${index}`);
      return element;
    };
    const deleteIcon = (index: number) => chip(index).querySelector('.MuiChip-deleteIcon');

    const onInputChange = renderModifiable();
    fireEvent.click(chip(1));
    expect(onInputChange).toHaveBeenCalledWith(modifiedKey('actions'), '["initiate"]');

    cleanup();
    const reAdd = renderModifiable({[modifiedKey('actions')]: '["initiate"]'});
    expect(chip(1).getAttribute('aria-pressed')).toBe('false');
    expect(deleteIcon(0)).toBeNull();
    fireEvent.click(chip(0));
    expect(reAdd).not.toHaveBeenCalledWith(modifiedKey('actions'), expect.anything());
    fireEvent.click(chip(1));
    expect(reAdd).toHaveBeenCalledWith(modifiedKey('actions'), '["initiate","status"]');
  });

  it('lets the user move a date only in the allowed direction', () => {
    const onInputChange = renderModifiable();
    const until = modifyInput('toDate');

    fireEvent.change(until, {target: {value: '2026-10-15'}});
    fireEvent.change(until, {target: {value: '2026-08-31'}});

    expect(until.max).toBe('2026-09-30');
    expect(onInputChange).toHaveBeenCalledWith(modifiedKey('toDate'), '"2026-08-31"');
    expect(onInputChange).not.toHaveBeenCalledWith(modifiedKey('toDate'), '"2026-10-15"');
  });

  it('keeps a field without a modification rule read-only', () => {
    renderModifiable();

    expect(screen.getByText('Merchant A')).toBeInTheDocument();
    expect(document.getElementById('consent_modify_authorization_details:app-1_detail-1_creditorName')).toBeNull();
  });

  it('shows the value the user already chose', () => {
    renderModifiable({[modifiedKey('instructedAmount.amount')]: '20'});

    expect(modifyInput('instructedAmount.amount').value).toBe('20');
  });
});

describe('ConsentAdapter — values that cannot be granted', () => {
  const consentBlock = {
    id: 'consent_block',
    type: 'BLOCK',
    components: [
      {id: 'action_allow', type: 'ACTION', eventType: 'SUBMIT', variant: 'PRIMARY', label: 'Allow'},
      {id: 'action_deny', type: 'ACTION', eventType: 'SUBMIT', variant: 'SECONDARY', label: 'Deny'},
    ],
  } as unknown as EmbeddedFlowComponent;
  const amountKey = modifiedKey('instructedAmount.amount');
  const amountInvalidKey = '__consent_invalid__authorization_details:app-1__detail-1__instructedAmount.amount';
  const dateKey = modifiedKey('toDate');

  // Keeps the form values in state, as the Gate does, so the screen and the submitted decision can be
  // compared after each keystroke.
  const renderPrompt = () => {
    const onInputChange = vi.fn();
    const onSubmit = vi.fn();
    const decision: Record<string, string> = {};
    function Prompt(): JSX.Element {
      const [values, setValues] = useState<Record<string, string>>({});
      return (
        <FlowComponentRenderer
          component={consentBlock}
          index={0}
          values={values}
          isLoading={false}
          resolve={(template) => template}
          onInputChange={(name, value) => {
            onInputChange(name, value);
            decision[name] = value;
            setValues((previous) => ({...previous, [name]: value}));
          }}
          onSubmit={onSubmit}
          additionalData={{consentPrompt: JSON.stringify([modifiableDetailPurpose])}}
        />
      );
    }
    const {container} = renderWithProviders(<Prompt />);
    return {onInputChange, onSubmit, values: () => decision, form: () => container.querySelector('form')!};
  };
  const allow = () => screen.getByRole<HTMLButtonElement>('button', {name: 'Allow'});
  const deny = () => screen.getByRole<HTMLButtonElement>('button', {name: 'Deny'});
  const amountError = 'Enter 35 or less.';

  it('shows an error, blocks Allow and grants neither keystroke when typing past the cap', async () => {
    const user = userEvent.setup();
    const {onInputChange, values} = renderPrompt();
    const amount = modifyInput('instructedAmount.amount');

    await user.tripleClick(amount);
    await user.keyboard('50');

    expect(amount.value).toBe('50');
    expect(amount).toHaveAttribute('aria-invalid', 'true');
    expect(screen.getByText(amountError)).toBeInTheDocument();
    expect(allow().disabled).toBe(true);
    expect(deny().disabled).toBe(false);
    expect(onInputChange).not.toHaveBeenCalledWith(amountKey, '50');
    // The first keystroke was a grantable 5 on its own; it must not survive into the decision.
    expect(values()[amountKey] ?? '').toBe('');
    expect(values()[amountInvalidKey]).toBe('true');

    await user.tab();
    expect(amount.value).toBe('50');
    expect(allow().disabled).toBe(true);
  });

  it('sends a value within the cap and keeps Allow enabled', async () => {
    const user = userEvent.setup();
    const {onInputChange, values} = renderPrompt();
    const amount = modifyInput('instructedAmount.amount');

    await user.tripleClick(amount);
    await user.keyboard('20');

    expect(onInputChange).toHaveBeenLastCalledWith(amountKey, '20');
    expect(values()[amountKey]).toBe('20');
    expect(values()[amountInvalidKey]).toBe('');
    expect(screen.queryByText(amountError)).not.toBeInTheDocument();
    expect(allow().disabled).toBe(false);
  });

  it('re-enables Allow once an invalid value is corrected', async () => {
    const user = userEvent.setup();
    const {values} = renderPrompt();
    const amount = modifyInput('instructedAmount.amount');

    fireEvent.change(amount, {target: {value: '100'}});
    expect(allow().disabled).toBe(true);

    await user.tripleClick(amount);
    await user.keyboard('30');

    expect(values()[amountKey]).toBe('30');
    expect(screen.queryByText(amountError)).not.toBeInTheDocument();
    expect(allow().disabled).toBe(false);
  });

  it('shows an error for a pasted value past the cap and does not send it', () => {
    const {onInputChange, values} = renderPrompt();
    const amount = modifyInput('instructedAmount.amount');

    fireEvent.change(amount, {target: {value: '100'}});

    expect(amount.value).toBe('100');
    expect(screen.getByText(amountError)).toBeInTheDocument();
    expect(allow().disabled).toBe(true);
    expect(onInputChange).not.toHaveBeenCalledWith(amountKey, '100');
    expect(values()[amountKey] ?? '').toBe('');
  });

  it('drops an earlier value when the field is cleared and shows the value granted on blur', async () => {
    const user = userEvent.setup();
    const {values} = renderPrompt();
    const amount = modifyInput('instructedAmount.amount');

    fireEvent.change(amount, {target: {value: '20'}});
    expect(values()[amountKey]).toBe('20');

    await user.clear(amount);
    expect(values()[amountKey]).toBe('');
    expect(allow().disabled).toBe(true);

    await user.tab();
    expect(amount.value).toBe('35');
    expect(screen.queryByText(amountError)).not.toBeInTheDocument();
    expect(allow().disabled).toBe(false);
    expect(values()[amountKey]).toBe('');
  });

  it('ignores Enter while blocked and keeps Deny available', async () => {
    const user = userEvent.setup();
    const {onSubmit, form} = renderPrompt();
    const amount = modifyInput('instructedAmount.amount');

    await user.tripleClick(amount);
    await user.keyboard('50');

    fireEvent.submit(form());
    expect(onSubmit).not.toHaveBeenCalled();

    fireEvent.click(deny());
    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({id: 'action_deny'}), expect.any(Object));
  });

  it('submits the inputs the decision is built from once the value can be granted', async () => {
    const user = userEvent.setup();
    const {onSubmit} = renderPrompt();
    const amount = modifyInput('instructedAmount.amount');

    await user.tripleClick(amount);
    await user.keyboard('50');
    await user.tripleClick(amount);
    await user.keyboard('20');
    fireEvent.click(allow());

    // The SDK's sign-in builds the decision from the modified value and strips both helper keys before
    // sending, so only a value that can be granted reaches the server.
    expect(onSubmit).toHaveBeenCalledTimes(1);
    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({id: 'action_allow'}),
      expect.objectContaining({[amountKey]: '20', [amountInvalidKey]: ''}),
    );
  });

  it('allows approving once the detail with the invalid value is declined', () => {
    renderPrompt();

    fireEvent.change(modifyInput('instructedAmount.amount'), {target: {value: '100'}});
    expect(allow().disabled).toBe(true);

    fireEvent.click(detailSwitch('detail-1'));
    expect(allow().disabled).toBe(false);
  });

  it('shows an error for a date past the cap and does not send it', () => {
    const {onInputChange, values} = renderPrompt();
    const until = modifyInput('toDate');

    fireEvent.change(until, {target: {value: '2026-08-31'}});
    expect(values()[dateKey]).toBe('"2026-08-31"');

    fireEvent.change(until, {target: {value: '2026-10-15'}});

    expect(until.value).toBe('2026-10-15');
    expect(screen.getByText('Choose 2026-09-30 or earlier.')).toBeInTheDocument();
    expect(allow().disabled).toBe(true);
    expect(onInputChange).not.toHaveBeenCalledWith(dateKey, '"2026-10-15"');
    expect(values()[dateKey]).toBe('');
  });
});

describe('ConsentAdapter — adding list items', () => {
  const excludedPurpose = detailPurpose({
    ...paymentDetail,
    rows: [
      {
        path: 'excluded',
        label: 'Excluded',
        value: ['gambling'],
        modification: 'addItems',
        options: ['gambling', 'crypto'],
      },
      {path: 'blocked', label: 'Blocked', value: ['x'], modification: 'addItems'},
    ],
  });
  const render = (formValues: Record<string, string> = {}) => {
    const onInputChange = vi.fn();
    renderWithProviders(
      <ConsentAdapter
        consentData={JSON.stringify([excludedPurpose])}
        formValues={formValues}
        onInputChange={onInputChange}
      />,
    );
    return onInputChange;
  };
  const byId = (id: string): HTMLElement | null =>
    document.getElementById(`consent_modify_authorization_details:app-1_detail-1_${id}`);

  it('offers the allowed values, keeps the shown items, and takes an added item out again', () => {
    const onInputChange = render();

    expect(byId('excluded_0')!.querySelector('.MuiChip-deleteIcon')).toBeNull();
    expect(byId('excluded_offered_gambling')).toBeNull();
    fireEvent.click(byId('excluded_offered_crypto')!);
    expect(onInputChange).toHaveBeenCalledWith(modifiedKey('excluded'), '["gambling","crypto"]');

    cleanup();
    const removeAdded = render({[modifiedKey('excluded')]: '["gambling","crypto"]'});
    expect(byId('excluded_offered_crypto')).toBeNull();
    fireEvent.click(byId('excluded_added_crypto')!);
    expect(removeAdded).toHaveBeenCalledWith(modifiedKey('excluded'), '["gambling"]');
  });

  it('adds a typed value when the list allows any value', () => {
    const onInputChange = render();
    const input = byId('blocked_new') as HTMLInputElement;

    fireEvent.change(input, {target: {value: 'y'}});
    fireEvent.keyDown(input, {key: 'Enter'});

    expect(onInputChange).toHaveBeenCalledWith(modifiedKey('blocked'), '["x","y"]');
  });
});

describe('ConsentAdapter — large content', () => {
  const renderDetail = (detail: object) => {
    const onInputChange = vi.fn();
    renderWithProviders(
      <ConsentAdapter
        consentData={JSON.stringify([detailPurpose(detail)])}
        formValues={{}}
        onInputChange={onInputChange}
      />,
    );
    return onInputChange;
  };
  const items = (count: number) => Array.from({length: count}, (_, index) => `item-${index}`);

  it('clamps a long summary behind a toggle that reports its state', () => {
    const summary = `Pay ${'a very long description '.repeat(10)}`.trim();
    renderDetail({...paymentDetail, summary, rows: []});

    const toggle = screen.getByRole('button', {name: 'authorizationDetails.showMore'});
    expect(toggle).toHaveAttribute('aria-expanded', 'false');
    expect(toggle).toHaveAttribute(
      'aria-controls',
      'consent_authz_detail_authorization_details:app-1_detail-1_summary',
    );
    expect(screen.getByText(summary)).toBeInTheDocument();

    fireEvent.click(toggle);
    expect(screen.getByRole('button', {name: 'authorizationDetails.showLess'})).toHaveAttribute(
      'aria-expanded',
      'true',
    );
  });

  it('leaves a short summary and value without a toggle', () => {
    renderDetail(paymentDetail);

    expect(screen.queryByRole('button', {name: 'authorizationDetails.showMore'})).not.toBeInTheDocument();
  });

  it('shows the first items of a long read-only list and the rest on request', () => {
    renderDetail({...paymentDetail, rows: [{path: 'regions', label: 'Regions', value: items(20)}]});

    expect(screen.getByText(items(8).join(', '))).toBeInTheDocument();
    const toggle = screen.getByRole('button', {name: 'authorizationDetails.moreItems'});
    expect(toggle).toHaveAttribute('aria-expanded', 'false');

    fireEvent.click(toggle);
    expect(screen.getByText(items(20).join(', '))).toBeInTheDocument();
  });

  it('collapses a long list of removable chips and keeps the hidden ones reachable', () => {
    const onInputChange = renderDetail({
      ...paymentDetail,
      rows: [{path: 'stores', label: 'Stores', value: items(12), modification: 'removeItems'}],
    });
    const chip = (index: number) =>
      document.getElementById(`consent_modify_authorization_details:app-1_detail-1_stores_${index}`);

    expect(chip(7)).toBeInTheDocument();
    expect(chip(8)).toBeNull();

    fireEvent.click(screen.getByRole('button', {name: 'authorizationDetails.moreItems'}));
    expect(chip(11)).toBeInTheDocument();
    fireEvent.click(chip(11)!.querySelector('.MuiChip-deleteIcon')!);
    expect(onInputChange).toHaveBeenCalledWith(modifiedKey('stores'), JSON.stringify(items(11)));
  });

  it('counts offered chips toward the collapsed list', () => {
    renderDetail({
      ...paymentDetail,
      rows: [{path: 'excluded', label: 'Excluded', value: ['a'], modification: 'addItems', options: items(10)}],
    });

    expect(
      document.getElementById('consent_modify_authorization_details:app-1_detail-1_excluded_offered_item-6'),
    ).toBeInTheDocument();
    expect(
      document.getElementById('consent_modify_authorization_details:app-1_detail-1_excluded_offered_item-7'),
    ).toBeNull();
  });

  it('gives each chip its full value as a tooltip', () => {
    const iban = 'DE89370400440532013000DE89370400440532013000';
    renderDetail({
      ...paymentDetail,
      rows: [{path: 'accounts', label: 'Accounts', value: [iban, 'x'], modification: 'removeItems'}],
    });

    expect(document.getElementById('consent_modify_authorization_details:app-1_detail-1_accounts_0')).toHaveAttribute(
      'title',
      iban,
    );
  });

  it('clamps a long free-text value behind a toggle', () => {
    const note = 'word '.repeat(60).trim();
    renderDetail({...paymentDetail, rows: [{path: 'note', label: 'Note', value: note}]});

    expect(screen.getByText(note)).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'authorizationDetails.showMore'})).toBeInTheDocument();
  });

  it('keeps long content inside a narrow card and stacks long fields below their label', () => {
    const iban = 'DE89370400440532013000DE89370400440532013000DE89370400440532013000';
    const detail = {
      ...paymentDetail,
      summary: `Pay to ${iban}`,
      rows: [
        {path: 'creditorName', label: 'Pay to', value: 'Merchant A'},
        {path: 'account', label: 'Account', value: `${iban}X`},
        {path: 'accounts', label: 'Accounts', value: [iban, 'x'], modification: 'removeItems'},
        {path: 'schedule', label: 'Schedule', value: [{date: '2026-11-01', memo: iban}]},
      ],
    };
    renderWithProviders(
      <div style={{width: 280}}>
        <ConsentAdapter consentData={JSON.stringify([detailPurpose(detail)])} formValues={{}} onInputChange={noop} />
      </div>,
    );

    const card = document
      .getElementById('consent_authz_detail_authorization_details:app-1_detail-1_summary')!
      .closest<HTMLElement>(`.${TEST_CN_PREFIX}Flow--consent-authorization-detail`)!;
    expect(card.getBoundingClientRect().width).toBeLessThanOrEqual(280);
    expect(card.scrollWidth).toBeLessThanOrEqual(card.clientWidth);

    const top = (text: string) => screen.getByText(text).getBoundingClientRect().top;
    expect(top('Merchant A')).toBe(top('Pay to'));
    expect(top(`${iban}X`)).toBeGreaterThan(top('Account'));
  });
});

describe('ConsentAdapter — Gate text', () => {
  // Gate translations, as the locale bundle or the server provides them, with wording unlike the SDK's.
  const gateI18n = i18next.createInstance();
  gateI18n
    .use(initReactI18next)
    .init({
      lng: 'en',
      resources: {
        en: {
          consent: {
            'authorizationDetails.numberMax': 'No more than {{value}}, please.',
            'authorizationDetails.removeItem': 'Take out {{item}}',
            'authorizationDetails.addItem': 'Put in {{item}}',
            'authorizationDetails.addBackItem': 'Bring back {{item}}',
            'authorizationDetails.newItem': 'Add to {{label}}',
          },
        },
      },
      interpolation: {escapeValue: false},
    })
    .catch(() => undefined);
  function GateText({children}: {children: ReactNode}): JSX.Element {
    return <I18nextProvider i18n={gateI18n}>{children}</I18nextProvider>;
  }
  const renderWithGateText = (purpose: object, formValues: Record<string, string> = {}) =>
    renderWithProviders(
      <GateText>
        <ConsentAdapter consentData={JSON.stringify([purpose])} formValues={formValues} onInputChange={noop} />
      </GateText>,
    );

  it('shows a validation message from the Gate translations', () => {
    renderWithGateText(modifiableDetailPurpose);

    fireEvent.change(modifyInput('instructedAmount.amount'), {target: {value: '100'}});

    expect(screen.getByText('No more than 35, please.')).toBeInTheDocument();
    expect(screen.queryByText('Enter 35 or less.')).not.toBeInTheDocument();
  });

  it('falls back to the SDK text for a message the Gate does not translate', () => {
    renderWithGateText(modifiableDetailPurpose);

    fireEvent.change(modifyInput('toDate'), {target: {value: '2026-10-15'}});

    expect(screen.getByText('Choose 2026-09-30 or earlier.')).toBeInTheDocument();
  });

  it('labels the chips that add and remove items from the Gate translations', () => {
    const purpose = detailPurpose({
      ...paymentDetail,
      rows: [
        {path: 'excluded', label: 'Excluded', value: ['gambling'], modification: 'addItems', options: ['crypto']},
        {path: 'blocked', label: 'Blocked', value: ['x'], modification: 'addItems'},
      ],
    });
    renderWithGateText(purpose, {[modifiedKey('excluded')]: '["gambling","crypto"]'});

    expect(
      document.getElementById('consent_modify_authorization_details:app-1_detail-1_excluded_added_crypto'),
    ).toHaveAttribute('aria-label', 'Take out crypto');
    expect(document.getElementById('consent_modify_authorization_details:app-1_detail-1_blocked_new')).toHaveAttribute(
      'aria-label',
      'Add to Blocked',
    );
  });

  it('labels the removable chips by what toggling them does', () => {
    renderWithGateText(modifiableDetailPurpose, {[modifiedKey('actions')]: '["initiate"]'});

    const removed = document.getElementById('consent_modify_authorization_details:app-1_detail-1_actions_1');
    expect(removed).toHaveAttribute('aria-label', 'Bring back status');
    expect(removed).toHaveAttribute('aria-pressed', 'false');
    // The last item kept cannot be removed, so it is not offered as a toggle.
    const last = document.getElementById('consent_modify_authorization_details:app-1_detail-1_actions_0');
    expect(last).not.toHaveAttribute('aria-label');
    expect(last).not.toHaveAttribute('aria-pressed');

    cleanup();
    renderWithGateText(modifiableDetailPurpose);
    expect(document.getElementById('consent_modify_authorization_details:app-1_detail-1_actions_0')).toHaveAttribute(
      'aria-label',
      'Take out initiate',
    );
  });
});
