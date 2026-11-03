// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderWithProviders, screen, userEvent, waitFor, within} from '@thunderid/test-utils';
import {useState, type JSX} from 'react';
import {getI18n} from 'react-i18next';
import {describe, it, expect, vi} from 'vitest';
import {page, userEvent as browserUser} from 'vitest/browser';
import type {SchemaField} from '../../../models/authorization-detail-type';
import {flightBookingSchema, flightBookingTemplate} from '../../../utils/__tests__/flightBookingSchema';
import {
  fieldsToSchema,
  newField,
  previewConsentTemplate,
  schemaToFields,
  validateFields,
} from '../../../utils/authorization-detail-schema';
import SchemaComposer from '../SchemaComposer';

function Harness({
  initial,
  onSchema = vi.fn(),
  readOnly = false,
  template: initialTemplate = '',
}: {
  initial: SchemaField[];
  onSchema?: (schema: Record<string, unknown>) => void;
  readOnly?: boolean;
  template?: string;
}): JSX.Element {
  const [fields, setFields] = useState(initial);
  const [template, setTemplate] = useState(initialTemplate);
  // As the form does, a field without a name is not flagged while it is being filled in.
  const issues = validateFields(fields).filter((issue) => !issue.messageKey.endsWith('nameRequired'));
  return (
    <SchemaComposer
      fields={fields}
      issues={issues}
      readOnly={readOnly}
      typeValue="payment_initiation"
      consent={{
        template,
        setTemplate,
        preview: template.trim() ? previewConsentTemplate(template, fields) : null,
      }}
      onChange={(next) => {
        setFields(next);
        onSchema(fieldsToSchema(next));
      }}
    />
  );
}

/** Renders at the given viewport size, and restores the viewport the test started with afterwards. */
async function atViewport(width: number, height: number, run: () => Promise<void> | void): Promise<void> {
  const {innerWidth, innerHeight} = window;
  await page.viewport(width, height);
  try {
    await run();
  } finally {
    await page.viewport(innerWidth, innerHeight);
  }
}

/** Whether an element is wholly within the viewport. */
const inView = (element: Element): boolean => {
  const box = element.getBoundingClientRect();
  return box.top >= 0 && box.bottom <= window.innerHeight;
};

/** The names of the fields on the card, in the order the card shows them. */
const cardOrder = (): (string | null)[] =>
  screen.getAllByRole('treeitem').map((item) => item.getAttribute('aria-label'));

// dnd-kit animates a drop for 250ms, and a drag started before it ends is lost.
const dropAnimation = (): Promise<void> =>
  new Promise((resolve) => {
    setTimeout(resolve, 300);
  });

const lastSchema = (mock: ReturnType<typeof vi.fn>): Record<string, unknown> =>
  mock.mock.calls[mock.mock.calls.length - 1][0] as Record<string, unknown>;

/** Adds a field of a kind from an add button of the card: at its end, or at the named insertion point. */
async function addField(user: ReturnType<typeof userEvent.setup>, kind: string, at = 'Add field'): Promise<void> {
  await user.click(screen.getByRole('button', {name: at}));
  await user.click(screen.getByRole('menuitem', {name: kind}));
}

/** Opens a collapsed section of the side panel by its title. */
async function openSection(user: ReturnType<typeof userEvent.setup>, title: string): Promise<void> {
  await user.click(screen.getByRole('button', {name: new RegExp(`^${title}`)}));
}

const amountField = (): SchemaField => ({
  ...newField('number'),
  name: 'amount',
  title: 'Amount',
  required: true,
  policy: {modification: 'decrease', modifiableBy: ['client'], maximum: '10000.00'},
});

describe('SchemaComposer', () => {
  it('starts with an empty card, adds a field on it, selects it, and writes its name and display name', async () => {
    const user = userEvent.setup();
    const onSchema = vi.fn();
    renderWithProviders(<Harness initial={[]} onSchema={onSchema} />);

    const card = screen.getByRole('region', {name: 'Consent card'});
    expect(card).toHaveTextContent('Add the values a request carries, such as an amount.');
    expect(within(card).getByTestId('canvas-summary'), 'without a summary, the type identifier').toHaveTextContent(
      'payment_initiation',
    );
    expect(within(card).getByLabelText('Approve')).toBeChecked();
    expect(screen.getAllByRole('button', {name: 'Add field'})).toHaveLength(1);
    await addField(user, 'Text');
    expect(screen.getByLabelText(/^Field name/)).toHaveFocus();
    await user.type(screen.getByLabelText(/^Field name/), 'creditorName');
    await user.type(screen.getByLabelText('Display name'), 'Pay to');

    expect(lastSchema(onSchema)).toEqual({
      type: 'object',
      properties: {creditorName: {type: 'string', title: 'Pay to', maxLength: 100}},
    });
    const row = screen.getByRole('treeitem', {name: 'Pay to'});
    expect(row).toHaveAttribute('aria-selected', 'true');
    expect(row, 'the row shows the field as the consent screen does').toHaveTextContent('Pay to‹Pay to›');
    expect(screen.getByRole('button', {name: /^Who sets and changes it/})).toHaveTextContent(
      'The client sets this. Nobody can change it after that.',
    );
    await openSection(user, 'Who sets and changes it');
    expect(
      screen.queryByRole('radiogroup', {name: 'Can it change after the request?'}),
      'text cannot change after the request, so the question is not asked',
    ).not.toBeInTheDocument();
    expect(screen.queryByRole('group', {name: 'Who can change it?'})).not.toBeInTheDocument();
    expect(
      screen.getByText(
        'Only numbers, dates, and lists can change after the request, so this value stays as requested.',
      ),
    ).toBeInTheDocument();
  });

  it('shows each field as a row of the consent screen with its rule, and edits the selected one', async () => {
    const user = userEvent.setup();
    const iban: SchemaField = {...newField(), name: 'iban', policy: {suppliedBy: 'processor'}};
    const note: SchemaField = {...newField(), name: 'note'};
    renderWithProviders(<Harness initial={[amountField(), iban, note]} />);

    const row = screen.getByRole('treeitem', {name: 'Amount'});
    expect(row).toHaveTextContent('Amount100');
    expect(within(row).getByRole('img', {name: 'The client sets this. The client can lower it.'})).toBeInTheDocument();
    expect(
      within(screen.getByRole('treeitem', {name: 'iban'})).getByRole('img', {
        name: 'Your external processor fills this in after the request. Nobody can change it after that.',
      }),
    ).toBeInTheDocument();
    expect(
      within(screen.getByRole('treeitem', {name: 'note'})).queryByRole('img', {name: /sets this|fills this in/}),
      'a field without a rule has no marker',
    ).toBeNull();
    expect(screen.getByRole('region', {name: 'Overview'})).toHaveTextContent(
      'Select an element on the card to edit it.',
    );

    await user.click(row);
    expect(row).toHaveAttribute('aria-selected', 'true');
    expect(screen.queryByRole('region', {name: 'Overview'})).not.toBeInTheDocument();
    expect(screen.getByLabelText(/^Field name/)).toHaveValue('amount');
    expect(screen.queryByLabelText('Maximum'), 'the limits start collapsed').not.toBeInTheDocument();
    expect(screen.getByRole('button', {name: /^Limits/})).toHaveTextContent('Up to 10000.00');
    await openSection(user, 'Limits');
    expect(screen.getByLabelText('Maximum')).toHaveValue('10000.00');

    await user.click(screen.getByRole('button', {name: 'Close field settings'}));
    expect(screen.getByRole('region', {name: 'Overview'})).toBeInTheDocument();
    expect(row).toHaveAttribute('aria-selected', 'false');
  });

  it('builds a lowerable amount with a maximum', async () => {
    const user = userEvent.setup();
    const onSchema = vi.fn();
    renderWithProviders(<Harness initial={[]} onSchema={onSchema} />);

    await addField(user, 'Number');
    await user.type(screen.getByLabelText(/^Field name/), 'amount');
    await openSection(user, 'Who sets and changes it');
    const change = screen.getByRole('radiogroup', {name: 'Can it change after the request?'});
    expect(
      within(change)
        .getAllByRole('radio')
        .map((radio) => radio.closest('label')?.textContent),
      'a number goes down or up',
    ).toEqual(['No, it stays as requested', 'Yes, it can only go down', 'Yes, it can only go up']);
    await user.click(within(change).getByRole('radio', {name: 'Yes, it can only go down'}));
    expect(screen.getByText('Choose who can change this field.'), 'a change needs someone to make it').toBeVisible();
    expect(screen.getByTestId(/-rule$/)).toHaveTextContent('The client sets this. Choose who can change it.');
    const who = screen.getByRole('group', {name: 'Who can change it?'});
    expect(within(who).getByRole('checkbox', {name: 'Your external processor'})).toHaveAccessibleDescription(
      'Can adjust it before the user is asked.',
    );
    await user.click(within(who).getByRole('checkbox', {name: 'Your external processor'}));
    await user.click(within(who).getByRole('checkbox', {name: 'The client'}));
    await openSection(user, 'Limits');
    await user.type(screen.getByLabelText('Maximum'), '10000.00');

    expect(lastSchema(onSchema)).toEqual({
      type: 'object',
      properties: {
        amount: {
          type: 'number',
          maximum: 10000,
          policy: {modification: 'decrease', modifiableBy: ['processor', 'client']},
        },
      },
    });
    expect(screen.getByTestId(/-rule$/), 'the rule reads as a sentence').toHaveTextContent(
      'The client sets this. Your external processor and the client can lower it.',
    );
    expect(screen.queryByText('Choose who can change this field.')).not.toBeInTheDocument();
    expect(screen.queryByLabelText('Maximum length')).not.toBeInTheDocument();
    expect(
      within(screen.getByRole('treeitem', {name: 'amount'})).getByRole('img', {
        name: 'The client sets this. Your external processor and the client can lower it.',
      }),
    ).toBeInTheDocument();
  });

  it('lets items be added to a list and warns what that is for', async () => {
    const user = userEvent.setup();
    const onSchema = vi.fn();
    const excluded: SchemaField = {...newField('array'), name: 'excludedMerchants'};
    renderWithProviders(<Harness initial={[excluded]} onSchema={onSchema} />);

    await user.click(screen.getByRole('treeitem', {name: 'excludedMerchants'}));
    expect(screen.getByRole('button', {name: /^Limits/})).toHaveTextContent('No limits');
    expect(screen.getByRole('button', {name: /^Allowed values/})).toHaveTextContent('Any value');
    await openSection(user, 'Who sets and changes it');
    const change = screen.getByRole('radiogroup', {name: 'Can it change after the request?'});
    expect(within(change).getAllByRole('radio'), 'a list has no order').toHaveLength(3);
    expect(within(change).queryByRole('radio', {name: /go down/})).not.toBeInTheDocument();
    await user.click(within(change).getByRole('radio', {name: 'Yes, items can only be added'}));
    await user.click(screen.getByRole('checkbox', {name: 'The user, on the consent screen'}));

    expect(lastSchema(onSchema)).toEqual({
      type: 'object',
      properties: {
        excludedMerchants: {
          type: 'array',
          items: {type: 'string'},
          policy: {modification: 'addItems', modifiableBy: ['user']},
        },
      },
    });
    expect(screen.getByText('The client sets this. The user can add items to it.')).toBeInTheDocument();
    expect(screen.getByText(/only for lists whose items take permission away/)).toBeInTheDocument();
  });

  it('lists the values a list allows', async () => {
    const user = userEvent.setup();
    const onSchema = vi.fn();
    renderWithProviders(<Harness initial={[{...newField('array'), name: 'actions'}]} onSchema={onSchema} />);

    await user.click(screen.getByRole('treeitem', {name: 'actions'}));
    await openSection(user, 'Allowed values');
    await user.type(screen.getByRole('combobox', {name: 'Allowed values'}), 'read{Enter}write{Enter}');
    expect(lastSchema(onSchema)).toEqual({
      type: 'object',
      properties: {actions: {type: 'array', items: {type: 'string', enum: ['read', 'write']}}},
    });
  });

  it('clears the change rule when the processor supplies the value', async () => {
    const user = userEvent.setup();
    const onSchema = vi.fn();
    renderWithProviders(<Harness initial={[amountField()]} onSchema={onSchema} />);

    await user.click(screen.getByRole('treeitem', {name: 'Amount'}));
    await openSection(user, 'Who sets and changes it');
    const source = screen.getByRole('radiogroup', {name: 'Who provides this value?'});
    expect(within(source).getByRole('radio', {name: 'The client, in its request'})).toBeChecked();
    expect(
      within(source).getByRole('radio', {name: 'Your external processor, after the request'}),
    ).toHaveAccessibleDescription(
      'The external processor, set on the External processor tab, fills it in. Clients cannot send it.',
    );
    await user.click(within(source).getByRole('radio', {name: 'Your external processor, after the request'}));

    expect(screen.queryByRole('radiogroup', {name: 'Can it change after the request?'})).not.toBeInTheDocument();
    expect(screen.queryByRole('group', {name: 'Who can change it?'})).not.toBeInTheDocument();
    expect(lastSchema(onSchema)).toEqual({
      type: 'object',
      required: ['amount'],
      properties: {amount: {type: 'number', title: 'Amount', maximum: 10000, policy: {suppliedBy: 'processor'}}},
    });
    expect(
      within(screen.getByRole('treeitem', {name: 'Amount'})).getByRole('img', {
        name: 'Your external processor fills this in after the request. Nobody can change it after that.',
      }),
    ).toBeInTheDocument();
  });

  it('limits a date field', async () => {
    const user = userEvent.setup();
    const onSchema = vi.fn();
    renderWithProviders(<Harness initial={[]} onSchema={onSchema} />);

    await addField(user, 'Text');
    await user.type(screen.getByLabelText(/^Field name/), 'until');
    await user.click(screen.getByRole('combobox', {name: 'Type'}));
    await user.click(screen.getByRole('option', {name: 'Date'}));
    expect(screen.getByRole('button', {name: /^Limits/})).toHaveTextContent('No limits');
    await openSection(user, 'Limits');
    await user.type(screen.getByLabelText('Maximum'), '2026-12-31');

    expect(screen.queryByLabelText('Maximum length'), 'a date has no length').not.toBeInTheDocument();
    expect(lastSchema(onSchema)).toEqual({
      type: 'object',
      properties: {until: {type: 'string', format: 'date', policy: {maximum: '2026-12-31'}}},
    });
  });

  it('limits the length of a new text field, which can be changed or cleared', async () => {
    const user = userEvent.setup();
    const onSchema = vi.fn();
    renderWithProviders(<Harness initial={[]} onSchema={onSchema} />);

    await addField(user, 'Text');
    await user.type(screen.getByLabelText(/^Field name/), 'note');
    expect(screen.getByRole('button', {name: /^Limits/})).toHaveTextContent('Up to 100 characters');
    await openSection(user, 'Limits');
    const maxLength = screen.getByLabelText('Maximum length');
    expect(maxLength).toHaveValue('100');

    await user.clear(maxLength);
    await user.type(maxLength, '40');
    expect(lastSchema(onSchema)).toEqual({type: 'object', properties: {note: {type: 'string', maxLength: 40}}});
    await user.clear(maxLength);
    expect(lastSchema(onSchema)).toEqual({type: 'object', properties: {note: {type: 'string'}}});
  });

  it('nests fields in objects, adds fields into an object, and removes them together', async () => {
    const user = userEvent.setup();
    const onSchema = vi.fn();
    const group = {...newField('object'), name: 'account', children: [{...newField(), name: 'id'}]};
    renderWithProviders(<Harness initial={[group]} onSchema={onSchema} />);

    const account = screen.getByRole('treeitem', {name: 'account'});
    expect(account).toHaveAttribute('aria-level', '1');
    expect(screen.getByRole('treeitem', {name: 'id'})).toHaveAttribute('aria-level', '2');
    expect(screen.getByRole('treeitem', {name: 'id'}), 'as the consent screen shows it').toHaveTextContent(
      'account.id‹id›',
    );
    await user.click(screen.getByRole('treeitem', {name: 'id'}));
    expect(screen.getByRole('region', {name: 'Field settings'})).toHaveTextContent('account.id');

    await addField(user, 'Money', 'Add field to account');
    await user.type(screen.getByLabelText(/^Field name/), 'limit');
    expect(lastSchema(onSchema)).toEqual({
      type: 'object',
      properties: {
        account: {
          type: 'object',
          properties: {
            id: {type: 'string'},
            limit: {
              type: 'object',
              required: ['amount', 'currency'],
              properties: {
                amount: {type: 'number', title: 'Amount'},
                currency: {type: 'string', title: 'Currency', maxLength: 3},
              },
            },
          },
        },
      },
    });

    // An object's name shows over the row above it while the pointer is on one of its own rows.
    await browserUser.hover(screen.getByRole('treeitem', {name: 'id'}));
    await user.click(account);
    expect(screen.getByLabelText(/^Field name/)).toHaveValue('account');
    await user.click(within(account).getByRole('button', {name: 'Field actions'}));
    await user.click(screen.getByRole('menuitem', {name: 'Remove field'}));
    expect(lastSchema(onSchema)).toEqual({type: 'object', properties: {}});
    expect(screen.getByRole('region', {name: 'Overview'})).toBeInTheDocument();
  });

  it('moves fields with the row menu and the keyboard', async () => {
    const user = userEvent.setup();
    const onSchema = vi.fn();
    const fields = [
      {...newField(), name: 'note'},
      {...newField('object'), name: 'payee', children: [{...newField(), name: 'iban'}]},
    ];
    renderWithProviders(<Harness initial={fields} onSchema={onSchema} />);
    const order = (): unknown => JSON.parse(JSON.stringify(lastSchema(onSchema).properties));

    // A row shows its toolbar while the pointer is on it.
    const note = screen.getByRole('treeitem', {name: 'note'});
    await browserUser.hover(note);
    await user.click(within(note).getByRole('button', {name: 'Field actions'}));
    expect(screen.getByRole('menuitem', {name: 'Move up'})).toHaveAttribute('aria-disabled', 'true');
    await user.click(screen.getByRole('menuitem', {name: 'Move down'}));
    expect(Object.keys(order() as object)).toEqual(['payee', 'note']);

    await browserUser.hover(screen.getByRole('treeitem', {name: 'note'}));
    await user.click(within(screen.getByRole('treeitem', {name: 'note'})).getByRole('button', {name: 'Field actions'}));
    await user.click(screen.getByRole('menuitem', {name: 'Move into the object above'}));
    expect(order()).toEqual({payee: {type: 'object', properties: {iban: {type: 'string'}, note: {type: 'string'}}}});
    expect(screen.getByRole('treeitem', {name: 'note'})).toHaveAttribute('aria-level', '2');

    screen.getByRole('treeitem', {name: 'iban'}).focus();
    await user.keyboard('{Alt>}{ArrowLeft}{/Alt}');
    expect(Object.keys(order() as object)).toEqual(['payee', 'iban']);
    screen.getByRole('treeitem', {name: 'iban'}).focus();
    await user.keyboard('{Alt>}{ArrowUp}{/Alt}');
    expect(Object.keys(order() as object)).toEqual(['iban', 'payee']);
  });

  it('duplicates a field under a free name', async () => {
    const user = userEvent.setup();
    const onSchema = vi.fn();
    renderWithProviders(<Harness initial={[amountField()]} onSchema={onSchema} />);

    await browserUser.hover(screen.getByRole('treeitem', {name: 'Amount'}));
    await user.click(
      within(screen.getByRole('treeitem', {name: 'Amount'})).getByRole('button', {name: 'Field actions'}),
    );
    await user.click(screen.getByRole('menuitem', {name: 'Duplicate'}));
    expect(Object.keys(lastSchema(onSchema).properties as object)).toEqual(['amount', 'amount_copy']);
    expect(screen.getByLabelText(/^Field name/)).toHaveValue('amount_copy');
  });

  it('adds the standard fields a type does not declare yet', async () => {
    const user = userEvent.setup();
    const onSchema = vi.fn();
    renderWithProviders(<Harness initial={[{...newField('array'), name: 'actions'}]} onSchema={onSchema} />);

    await addField(user, 'Add standard fields (locations)');
    expect(lastSchema(onSchema)).toEqual({
      type: 'object',
      properties: {
        actions: {type: 'array', items: {type: 'string'}},
        locations: {type: 'array', title: 'Locations', items: {type: 'string'}},
      },
    });
    await user.click(screen.getByRole('button', {name: 'Add field'}));
    expect(screen.queryByRole('menuitem', {name: /Add standard fields/})).not.toBeInTheDocument();
  });

  it('flags the fields that have issues, and names the issue on the selected one', async () => {
    const user = userEvent.setup();
    renderWithProviders(<Harness initial={[{...newField(), name: 'a.b'}]} />);

    const row = screen.getByRole('treeitem', {name: 'a.b'});
    expect(within(row).getByRole('img', {name: 'This field needs attention'})).toBeInTheDocument();
    await user.click(row);
    expect(screen.getByText('A field name cannot contain a dot.')).toBeInTheDocument();
  });

  it('opens the section that has an issue', async () => {
    const user = userEvent.setup();
    const amount = {...amountField(), policy: {modification: 'decrease' as const}};
    renderWithProviders(<Harness initial={[amount]} />);

    await user.click(screen.getByRole('treeitem', {name: 'Amount'}));
    expect(screen.getByText('Choose who can change this field.')).toBeInTheDocument();
    expect(screen.getByRole('checkbox', {name: 'The client'}), 'the policy section starts open').toBeInTheDocument();
  });

  it('sums up who can change what, and selects a field from it', async () => {
    const user = userEvent.setup();
    const iban: SchemaField = {...newField(), name: 'iban', policy: {suppliedBy: 'processor'}};
    const actions: SchemaField = {
      ...newField('array'),
      name: 'actions',
      policy: {modification: 'removeItems', modifiableBy: ['user']},
    };
    const account: SchemaField = {...newField('object'), name: 'account', children: [iban]};
    renderWithProviders(<Harness initial={[amountField(), account, actions]} />);

    expect(screen.getByRole('button', {name: /^Who can change what/})).toHaveTextContent(
      '2 of 3 fields can change after the request',
    );
    await openSection(user, 'Who can change what');
    const map = screen.getByRole('list', {name: 'Who can change what'});
    expect(
      within(map)
        .getAllByRole('button')
        .map((item) => item.getAttribute('aria-label')),
      'each field that holds a value, in plain words',
    ).toEqual([
      'Amount: The client sets this. The client can lower it.',
      'iban: Your external processor fills this in after the request. Nobody can change it after that.',
      'actions: The client sets this. The user can remove items from it.',
    ]);
    expect(screen.getByTestId(`policy-row-${iban.key}`)).toHaveTextContent(
      'ibanaccount.ibanYour external processor fills this in after the request. Nobody can change it after that.',
    );

    await user.click(within(map).getByRole('button', {name: /^actions:/}));
    expect(screen.getByLabelText(/^Field name/)).toHaveValue('actions');
  });

  it('renders the card as the consent screen does, with the controls of its fields inert', () => {
    const actions: SchemaField = {
      ...newField('array'),
      name: 'actions',
      allowedValues: ['read', 'write', 'delete'],
      policy: {modification: 'removeItems', modifiableBy: ['user']},
    };
    const payee: SchemaField = {...newField(), name: 'payee', title: 'Pay to'};
    const amount = {...amountField(), policy: {modification: 'decrease' as const, modifiableBy: ['user' as const]}};
    renderWithProviders(<Harness initial={[amount, payee, actions]} template="Pay {{amount}} to {{payee}}" />);

    const card = screen.getByRole('region', {name: 'Consent card'});
    expect(within(card).getByTestId('canvas-summary'), 'numbers read as on the rows').toHaveTextContent(
      'Pay 100 to ‹Pay to›',
    );
    expect(screen.getByRole('treeitem', {name: 'Pay to'})).toHaveTextContent('Pay to‹Pay to›');
    expect(within(card).getByRole('group', {name: 'actions'})).toHaveTextContent('readwrite');
    const amountInput = within(card).getByRole('spinbutton', {name: 'Amount'});
    expect(amountInput).toHaveValue(100);
    expect(amountInput.closest('[inert]'), 'selecting the row edits the field instead').not.toBeNull();
    expect(within(card).getByLabelText('Approve').closest('[inert]')).not.toBeNull();
  });

  it('shows the fields in the order they are declared, nested ones in place, and follows a move', async () => {
    const user = userEvent.setup();
    const fields = [
      {...newField(), name: 'b'},
      {
        ...newField('object'),
        name: 'obj',
        children: [
          {...newField(), name: 'y'},
          {...newField(), name: 'x'},
        ],
      },
      {...newField(), name: 'a'},
    ];
    renderWithProviders(<Harness initial={fields} />);

    expect(cardOrder()).toEqual(['b', 'obj', 'y', 'x', 'a']);
    screen.getByRole('treeitem', {name: 'x'}).focus();
    await user.keyboard('{Alt>}{ArrowUp}{/Alt}');
    expect(cardOrder()).toEqual(['b', 'obj', 'x', 'y', 'a']);
    screen.getByRole('treeitem', {name: 'a'}).focus();
    await user.keyboard('{Alt>}{ArrowUp}{/Alt}');
    expect(cardOrder()).toEqual(['b', 'a', 'obj', 'x', 'y']);
  });

  it('inserts a field where the plus between rows is', async () => {
    const user = userEvent.setup();
    const onSchema = vi.fn();
    renderWithProviders(
      <Harness
        initial={[
          {...newField(), name: 'note'},
          {...newField(), name: 'until'},
        ]}
        onSchema={onSchema}
      />,
    );

    await addField(user, 'Number', 'Insert a field before until');
    await user.type(screen.getByLabelText(/^Field name/), 'amount');
    expect(Object.keys(lastSchema(onSchema).properties as object)).toEqual(['note', 'amount', 'until']);
    expect(cardOrder()).toEqual(['note', 'amount', 'until']);
  });

  it('edits the summary once it is selected on the card, with fields inserted where the cursor is', async () => {
    const user = userEvent.setup();
    const payee: SchemaField = {...newField(), name: 'payee', title: 'Pay to'};
    renderWithProviders(<Harness initial={[amountField(), payee]} />);

    const summary = screen.getByRole('button', {name: 'Consent summary'});
    expect(summary).toHaveTextContent('payment_initiation');
    await user.click(summary);
    expect(summary).toHaveAttribute('aria-pressed', 'true');
    const input = screen.getByLabelText<HTMLTextAreaElement>('Summary');
    expect(input).toHaveFocus();
    expect(
      screen.getByText('Without a summary, the consent screen shows the type identifier, payment_initiation.'),
    ).toBeInTheDocument();

    await user.type(input, 'Pay  now');
    input.setSelectionRange(4, 4);
    await user.click(screen.getByRole('button', {name: 'Insert amount'}));
    expect(input).toHaveValue('Pay {{amount}} now');
    expect(summary).toHaveTextContent('Pay 100 now');

    await user.clear(input);
    await user.type(input, 'Pay {{{{fee}}');
    expect(screen.getByText('No field matches fee. Use a field name from the list.')).toBeInTheDocument();

    await user.click(screen.getByRole('button', {name: 'Close summary settings'}));
    expect(summary).toHaveAttribute('aria-pressed', 'false');
    expect(screen.getByRole('region', {name: 'Overview'})).toBeInTheDocument();
  });

  it('leaves whether an approval is reused out of the overview', () => {
    renderWithProviders(<Harness initial={[amountField()]} />);

    const overview = screen.getByRole('region', {name: 'Overview'});
    expect(within(overview).queryByRole('switch', {name: 'Ask every time'})).toBeNull();
    expect(
      within(overview).queryByRole('button', {name: /^Consent/}),
      'the consent section shows only with a notice about the summary',
    ).toBeNull();
  });

  it('adds a field from the palette at the end of the card', async () => {
    const user = userEvent.setup();
    const onSchema = vi.fn();
    renderWithProviders(<Harness initial={[amountField()]} onSchema={onSchema} />);

    await openSection(user, 'Add a field');
    await user.click(within(screen.getByRole('group', {name: 'Add a field'})).getByRole('button', {name: 'Boolean'}));
    await user.type(screen.getByLabelText(/^Field name/), 'recurring');
    expect(Object.keys(lastSchema(onSchema).properties as object)).toEqual(['amount', 'recurring']);
  });

  it('sums up the example request a client sends', async () => {
    const user = userEvent.setup();
    const payee: SchemaField = {...newField(), name: 'payee', title: 'Pay to'};
    renderWithProviders(<Harness initial={[amountField(), payee]} />);

    await openSection(user, 'Example request');
    expect(JSON.parse(screen.getByTestId('composer-example-detail').textContent ?? '')).toEqual({
      type: 'payment_initiation',
      amount: 100,
      payee: '‹Pay to›',
    });
  });

  it('shows a read-only type without letting it change', async () => {
    const user = userEvent.setup();
    renderWithProviders(<Harness initial={[amountField()]} readOnly />);

    expect(screen.queryByRole('button', {name: /^Add field/})).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Drag Amount'})).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Field actions'})).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: /^Add a field/})).not.toBeInTheDocument();
    expect(screen.getByRole('region', {name: 'Overview'})).toHaveTextContent(
      'Select an element on the card to see it.',
    );

    await user.click(screen.getByRole('treeitem', {name: 'Amount'}));
    expect(screen.getByLabelText(/^Field name/)).toBeDisabled();
    await openSection(user, 'Limits');
    expect(screen.getByLabelText('Maximum')).toBeDisabled();
    await openSection(user, 'Who sets and changes it');
    expect(screen.getByRole('radio', {name: 'The client, in its request'})).toBeDisabled();
    expect(screen.getByRole('radio', {name: 'Yes, it can only go down'})).toBeDisabled();
    expect(screen.getByRole('checkbox', {name: 'The client'})).toBeDisabled();
    await user.click(screen.getByRole('button', {name: 'Consent summary'}));
    expect(screen.getByLabelText('Summary')).toBeDisabled();
  });

  it('renders the flight booking type on the card, as the consent screen shows it', () => {
    renderWithProviders(<Harness initial={schemaToFields(flightBookingSchema)!} template={flightBookingTemplate} />);

    const card = screen.getByRole('region', {name: 'Consent card'});
    expect(within(card).getByTestId('canvas-summary')).toHaveTextContent(
      'Book flight ‹Flight› from ‹From› to ‹To›, spending up to 100 EUR',
    );
    expect(cardOrder()).toEqual([
      'Actions',
      'flight',
      'From',
      'Flight',
      'To',
      'Goal',
      'Applies to',
      'maxPrice',
      'Spend up to',
      'Currency',
      'Travelers',
    ]);
    expect(screen.getByRole('treeitem', {name: 'Actions'})).toHaveTextContent('Actionsbook');
    expect(screen.getByRole('treeitem', {name: 'Applies to'})).toHaveTextContent('Applies tofirst, second');
    expect(within(card).getByRole('spinbutton', {name: 'Spend up to'})).toHaveValue(100);
    expect(
      within(card).getByRole('spinbutton', {name: 'Travelers'}),
      'a sample stays within the limits, and the user can only lower it',
    ).toHaveValue(9);
    expect(within(card).getByRole('spinbutton', {name: 'Travelers'})).toHaveAttribute('max', '9');
  });

  it('keeps the name of an object, and the toolbar of a row, inside the canvas', async () => {
    await page.viewport(1100, 900);
    const user = userEvent.setup();
    const amount = {...amountField(), policy: {modification: 'decrease' as const, modifiableBy: ['user' as const]}};
    const money: SchemaField = {...newField('object'), name: 'instructedAmount', title: 'Amount', children: [amount]};
    renderWithProviders(<Harness initial={[{...newField(), name: 'note'}, money]} />);

    const canvas = screen.getByTestId('consent-canvas').getBoundingClientRect();
    const card = screen.getByRole('region', {name: 'Consent card'}).getBoundingClientRect();
    const tag = screen.getByTestId(`field-row-${money.key}`);
    expect(getComputedStyle(tag).position, 'the name takes no room on the card').toBe('absolute');
    expect(getComputedStyle(tag).opacity, 'the consent screen does not show objects').toBe('0');
    const tagBox = tag.getBoundingClientRect();
    expect(tagBox.left).toBeGreaterThanOrEqual(card.left);
    expect(tagBox.right).toBeLessThanOrEqual(card.right);
    expect(tag, 'named by its display name, as the consent screen names a field').toHaveTextContent('Amount');
    expect(tag).not.toHaveTextContent('instructedAmount');

    const row = screen.getByTestId(`field-row-${amount.key}`);
    await user.click(row);
    const toolbar = row.querySelector('.canvas-toolbar')!.getBoundingClientRect();
    expect(toolbar.left).toBeGreaterThanOrEqual(canvas.left);
    expect(toolbar.right, 'the toolbar is never cut off by the canvas').toBeLessThanOrEqual(canvas.right);
    const stepperShown = Array.from(document.styleSheets).some((sheet) =>
      Array.from(sheet.cssRules).some(
        (rule) => rule.cssText.includes('::-webkit-inner-spin-button') && rule.cssText.includes('opacity: 1'),
      ),
    );
    expect(stepperShown, 'the stepper shows on the inert card as on the consent screen under the pointer').toBe(true);
  });

  it('asks about a date in the words of a date, and limits how few items a list holds', async () => {
    const user = userEvent.setup();
    const onSchema = vi.fn();
    const until: SchemaField = {...newField('date'), name: 'until', title: 'Valid until'};
    const stores: SchemaField = {...newField('array'), name: 'stores'};
    renderWithProviders(<Harness initial={[until, stores]} onSchema={onSchema} />);

    await user.click(screen.getByRole('treeitem', {name: 'Valid until'}));
    await openSection(user, 'Who sets and changes it');
    await user.click(screen.getByRole('radio', {name: 'Yes, it can only move earlier'}));
    await user.click(screen.getByRole('checkbox', {name: 'The user, on the consent screen'}));
    expect(screen.getByTestId(/-rule$/)).toHaveTextContent('The client sets this. The user can move it earlier.');

    await user.click(screen.getByRole('treeitem', {name: 'stores'}));
    await openSection(user, 'Limits');
    await user.type(screen.getByLabelText('Minimum number of items'), '1');
    expect(lastSchema(onSchema)).toEqual({
      type: 'object',
      properties: {
        until: {
          type: 'string',
          format: 'date',
          title: 'Valid until',
          policy: {modification: 'decrease', modifiableBy: ['user']},
        },
        stores: {type: 'array', items: {type: 'string'}, minItems: 1},
      },
    });
  });

  it('says a read-only type without fields has none', () => {
    renderWithProviders(<Harness initial={[]} readOnly />);
    expect(screen.getByText('This intent has no fields.')).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Add field'})).not.toBeInTheDocument();
  });

  it('drags fields to reorder them and to nest them in an object', async () => {
    await page.viewport(1400, 1000);
    const onSchema = vi.fn();
    const fields = [
      {...newField(), name: 'note'},
      {...newField('object'), name: 'payee'},
    ];
    renderWithProviders(<Harness initial={fields} onSchema={onSchema} />);

    // A handle shows, and can be taken, while the pointer is on its field.
    await browserUser.hover(screen.getByRole('treeitem', {name: 'payee'}));
    await browserUser.dragAndDrop(screen.getByRole('button', {name: 'Drag payee'}), screen.getByTestId('gap-root-0'), {
      steps: 20,
    });
    await waitFor(() => expect(Object.keys(lastSchema(onSchema).properties as object)).toEqual(['payee', 'note']));
    await dropAnimation();

    await browserUser.hover(screen.getByRole('treeitem', {name: 'note'}));
    await browserUser.dragAndDrop(
      screen.getByRole('button', {name: 'Drag note'}),
      screen.getByTestId(`gap-${fields[1].key}-0`),
      {
        steps: 20,
      },
    );
    await waitFor(() =>
      expect(lastSchema(onSchema)).toEqual({
        type: 'object',
        properties: {payee: {type: 'object', properties: {note: {type: 'string'}}}},
      }),
    );
    await dropAnimation();
    expect(screen.getByRole('treeitem', {name: 'note'})).toHaveAttribute('aria-level', '2');
  });

  it('makes each field of an editable type draggable', async () => {
    renderWithProviders(<Harness initial={[amountField()]} />);
    expect(screen.getByRole('button', {name: 'Drag Amount'})).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByRole('button', {name: 'Drag Amount'})).toHaveAttribute('aria-roledescription', 'draggable');
    });
  });

  it('selects a field on the card with Enter or Space, and shows its rule to assistive technology', async () => {
    const user = userEvent.setup();
    renderWithProviders(<Harness initial={[amountField()]} />);

    const row = screen.getByRole('treeitem', {name: 'Amount'});
    expect(row).toHaveAccessibleDescription('Number, Required. The client sets this. The client can lower it.');
    row.focus();
    await user.keyboard('{Enter}');
    expect(screen.getByRole('region', {name: 'Field settings'})).toBeInTheDocument();
    expect(row).toHaveAttribute('aria-selected', 'true');

    await user.click(screen.getByRole('button', {name: 'Close field settings'}));
    row.focus();
    await user.keyboard(' ');
    expect(screen.getByRole('region', {name: 'Field settings'})).toBeInTheDocument();
  });

  it('blocks two fields with the same name at one level, and says so', async () => {
    const user = userEvent.setup();
    renderWithProviders(<Harness initial={[amountField(), {...newField('number'), name: 'fee'}]} />);

    await user.click(screen.getByRole('treeitem', {name: 'fee'}));
    await user.clear(screen.getByLabelText(/^Field name/));
    await user.type(screen.getByLabelText(/^Field name/), 'amount');

    expect(screen.getByText('Another field at this level has the same name.')).toBeInTheDocument();
    expect(screen.getByLabelText(/^Field name/)).toHaveAttribute('aria-invalid', 'true');
  });

  it('offers to update the summary when a field it names is renamed', async () => {
    const user = userEvent.setup();
    renderWithProviders(<Harness initial={[amountField()]} template="Pay up to {{amount}}" />);

    await user.click(screen.getByRole('treeitem', {name: 'Amount'}));
    await user.clear(screen.getByLabelText(/^Field name/));
    expect(screen.queryByTestId('summary-rename-notice'), 'no notice while the name is empty').toBeNull();
    await user.type(screen.getByLabelText(/^Field name/), 'limit');

    const notice = screen.getByTestId('summary-rename-notice');
    expect(notice).toHaveTextContent(
      'The summary uses {{amount}}, which no longer names a field. Update it to {{limit}}?',
    );
    await user.click(within(notice).getByRole('button', {name: 'Update summary'}));

    expect(screen.queryByTestId('summary-rename-notice')).toBeNull();
    expect(screen.getByTestId('canvas-summary')).toHaveTextContent('Pay up to');
    await user.click(screen.getByTestId('canvas-summary'));
    expect(screen.getByLabelText('Summary')).toHaveValue('Pay up to {{limit}}');
  });

  it('warns when a removed field is named in the summary, and undoes the removal', async () => {
    const user = userEvent.setup();
    const onSchema = vi.fn();
    renderWithProviders(
      <Harness
        initial={[amountField(), {...newField('string'), name: 'payee'}]}
        template="Pay {{amount}}"
        onSchema={onSchema}
      />,
    );

    await browserUser.hover(screen.getByRole('treeitem', {name: 'Amount'}));
    await user.click(
      within(screen.getByRole('treeitem', {name: 'Amount'})).getByRole('button', {name: 'Field actions'}),
    );
    await user.click(screen.getByRole('menuitem', {name: 'Remove field'}));

    expect(screen.queryByRole('treeitem', {name: 'Amount'})).toBeNull();
    const snackbar = await screen.findByText(/Removed Amount\. The summary still uses \{\{amount\}\}/);
    expect(snackbar).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Undo'}));

    expect(screen.getByRole('treeitem', {name: 'Amount'})).toBeInTheDocument();
    expect(cardOrder()).toEqual(['Amount', 'payee']);
    expect(Object.keys((lastSchema(onSchema).properties ?? {}) as Record<string, unknown>)).toEqual([
      'amount',
      'payee',
    ]);
  });

  it('says how many fields went with a removed object', async () => {
    const user = userEvent.setup();
    const money: SchemaField = {
      ...newField('object'),
      name: 'instructedAmount',
      children: [
        {...newField('number'), name: 'amount'},
        {...newField('string'), name: 'currency'},
      ],
    };
    renderWithProviders(<Harness initial={[money]} />);

    await browserUser.hover(screen.getByTestId(`field-group-${money.key}`));
    await user.click(
      within(screen.getByRole('treeitem', {name: 'instructedAmount'})).getByRole('button', {name: 'Field actions'}),
    );
    await user.click(screen.getByRole('menuitem', {name: 'Remove field'}));

    expect(await screen.findByText('Removed instructedAmount and the 2 fields in it.')).toBeInTheDocument();
  });

  it('cleans settings a new kind cannot keep, says which, and can undo the change', async () => {
    const user = userEvent.setup();
    const onSchema = vi.fn();
    renderWithProviders(<Harness initial={[amountField()]} onSchema={onSchema} />);

    await user.click(screen.getByRole('treeitem', {name: 'Amount'}));
    await user.click(screen.getByRole('combobox', {name: 'Type'}));
    await user.click(screen.getByRole('option', {name: 'Text'}));

    const notice = screen.getByTestId('kind-change-notice');
    expect(notice).toHaveTextContent(
      'Changing the type removed the minimum and maximum and the rule for changing it, which the new type cannot keep.',
    );
    expect(lastSchema(onSchema)).toEqual({
      type: 'object',
      properties: {amount: {type: 'string', title: 'Amount'}},
      required: ['amount'],
    });

    await user.click(within(notice).getByRole('button', {name: 'Undo'}));
    expect(screen.queryByTestId('kind-change-notice')).toBeNull();
    expect(lastSchema(onSchema)).toEqual(fieldsToSchema([amountField()]));
  });

  it('joins the removed settings in the language the Console is shown in', async () => {
    const user = userEvent.setup();
    const i18n = getI18n();
    const language = i18n.language;
    await i18n.changeLanguage('fr');
    try {
      renderWithProviders(<Harness initial={[amountField()]} />);

      await user.click(screen.getByRole('treeitem', {name: 'Amount'}));
      await user.click(screen.getByRole('combobox', {name: 'Type'}));
      await user.click(screen.getByRole('option', {name: 'Text'}));

      expect(screen.getByTestId('kind-change-notice')).toHaveTextContent(
        'the minimum and maximum et the rule for changing it',
      );
    } finally {
      await i18n.changeLanguage(language);
    }
  });

  it('keeps the plus at the end of an object apart from the plus after the object', async () => {
    await page.viewport(1100, 900);
    const inner: SchemaField = {
      ...newField('object'),
      name: 'bank',
      title: 'Bank',
      children: [{...newField(), name: 'bic'}],
    };
    const payee: SchemaField = {
      ...newField('object'),
      name: 'payee',
      title: 'Payee',
      children: [{...newField(), name: 'iban'}, inner],
    };
    renderWithProviders(<Harness initial={[{...newField(), name: 'note'}, payee]} />);

    const ends = ['Add field to Bank', 'Add field to Payee', 'Insert a field at the end'].map((name) =>
      screen.getByRole('button', {name}),
    );
    const boxes = ends.map((button) => button.getBoundingClientRect());
    for (let i = 0; i < boxes.length; i += 1) {
      for (let j = i + 1; j < boxes.length; j += 1) {
        const [a, b] = [boxes[i], boxes[j]];
        const overlap = a.left < b.right && b.left < a.right && a.top < b.bottom && b.top < a.bottom;
        expect(overlap, `${ends[i].getAttribute('aria-label')} and ${ends[j].getAttribute('aria-label')}`).toBe(false);
      }
    }
    // Each plus is the one a pointer reaches at its place.
    ends.forEach((button, index) => {
      const box = boxes[index];
      const hit = document.elementFromPoint(box.left + box.width / 2, box.top + box.height / 2);
      expect(button.contains(hit), button.getAttribute('aria-label') ?? '').toBe(true);
    });
  });

  it('names an object by its display name on both of its add buttons', () => {
    const payee: SchemaField = {...newField('object'), name: 'payee', title: 'Payee'};
    renderWithProviders(<Harness initial={[payee]} />);

    expect(screen.getAllByRole('button', {name: 'Add field to Payee'})).toHaveLength(2);
    expect(screen.queryByRole('button', {name: 'Add field to payee'})).toBeNull();
  });

  it('closes the field settings with Escape and returns focus to the field', async () => {
    const user = userEvent.setup();
    renderWithProviders(<Harness initial={[amountField()]} />);

    const row = screen.getByRole('treeitem', {name: 'Amount'});
    await user.click(row);
    await user.click(screen.getByLabelText(/^Field name/));
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('region', {name: 'Field settings'})).toBeNull();
    expect(row).toHaveFocus();

    await user.click(row);
    await user.click(screen.getByRole('combobox', {name: 'Type'}));
    await user.keyboard('{Escape}');
    expect(screen.queryByRole('listbox'), 'Escape first closes the open list').toBeNull();
    expect(screen.getByRole('region', {name: 'Field settings'})).toBeInTheDocument();

    await user.click(screen.getByRole('button', {name: 'Close field settings'}));
    expect(row, 'closing returns focus to the field too').toHaveFocus();
  });

  it('moves focus between the fields with the arrow keys, Home, and End', async () => {
    const user = userEvent.setup();
    const fields = [
      {...newField(), name: 'note'},
      {...newField('object'), name: 'payee', children: [{...newField(), name: 'iban'}]},
      {...newField(), name: 'memo'},
    ];
    renderWithProviders(<Harness initial={fields} />);
    const item = (name: string): HTMLElement => screen.getByRole('treeitem', {name});

    item('note').focus();
    await user.keyboard('{ArrowDown}');
    expect(item('payee')).toHaveFocus();
    await user.keyboard('{ArrowDown}');
    expect(item('iban')).toHaveFocus();
    await user.keyboard('{End}');
    expect(item('memo')).toHaveFocus();
    await user.keyboard('{ArrowDown}');
    expect(item('memo'), 'the last field keeps focus').toHaveFocus();
    await user.keyboard('{ArrowUp}');
    expect(item('iban')).toHaveFocus();
    await user.keyboard('{Home}');
    expect(item('note')).toHaveFocus();
    expect(cardOrder(), 'moving focus moves no field').toEqual(['note', 'payee', 'iban', 'memo']);

    await user.tab();
    expect(item('note'), 'Tab still moves on to the next control').not.toHaveFocus();
  });

  it('brings the settings of a field selected on the card into view where they sit below the card', async () => {
    const fields = Array.from({length: 10}, (_, index) => ({...newField(), name: `field${index}`}));
    await atViewport(768, 700, async () => {
      const user = userEvent.setup();
      renderWithProviders(<Harness initial={fields} />);

      const row = screen.getByRole('treeitem', {name: 'field1'});
      await user.click(row);
      const settings = screen.getByRole('region', {name: 'Field settings'});
      expect(settings.getBoundingClientRect().top, 'the settings sit below the card').toBeGreaterThan(
        screen.getByRole('region', {name: 'Consent card'}).getBoundingClientRect().bottom,
      );
      await waitFor(() => expect(inView(within(settings).getByRole('heading', {name: 'field1'}))).toBe(true));
      expect(inView(screen.getByLabelText(/^Field name/))).toBe(true);
      expect(
        screen.getByLabelText(/^Field name/),
        'focus moves to the settings, as when a field is added',
      ).toHaveFocus();
      expect(screen.getByLabelText(/^Field name/)).toHaveValue('field1');

      window.scrollTo(0, 0);
      await user.click(screen.getByRole('treeitem', {name: 'field2'}));
      await waitFor(() => expect(inView(screen.getByLabelText(/^Field name/))).toBe(true));
      expect(screen.getByLabelText(/^Field name/)).toHaveValue('field2');
      expect(screen.getByLabelText(/^Field name/)).toHaveFocus();
    });
  });

  it('moves focus to the heading of the settings of a read-only field, whose controls are disabled', async () => {
    const user = userEvent.setup();
    renderWithProviders(<Harness initial={[amountField()]} readOnly />);

    await user.click(screen.getByRole('treeitem', {name: 'Amount'}));
    const settings = screen.getByRole('region', {name: 'Field settings'});
    expect(within(settings).getByRole('heading', {name: 'Amount'})).toHaveFocus();
  });

  it('keeps the toolbar of a row inside the row on a narrow card, clear of the rows around it, without moving the value', async () => {
    const user = userEvent.setup();
    const fields = [amountField(), {...newField(), name: 'note'}, {...newField('date'), name: 'until'}];
    renderWithProviders(
      <div style={{width: 480}}>
        <Harness initial={fields} />
      </div>,
    );

    for (const field of fields) {
      const row = screen.getByTestId(`field-row-${field.key}`);
      await user.hover(row);
      const rowBox = row.getBoundingClientRect();
      const toolbar = row.querySelector('.canvas-toolbar')!.getBoundingClientRect();
      expect(toolbar.top, field.name).toBeGreaterThanOrEqual(rowBox.top);
      expect(toolbar.bottom, field.name).toBeLessThanOrEqual(rowBox.bottom);
      expect(toolbar.right, field.name).toBeLessThanOrEqual(rowBox.right);
      const value = row.querySelector('[inert]')!.getBoundingClientRect();
      expect(rowBox.right - value.right, `${field.name}: the value ends where the consent screen ends it`).toBe(8);
    }
  });

  it('leaves a tap on the end of a row to the row, until the row shows its toolbar', async () => {
    const user = userEvent.setup();
    const fields = [amountField(), {...newField(), name: 'note'}];
    renderWithProviders(
      <div style={{width: 480}}>
        <Harness initial={fields} />
      </div>,
    );

    const row = screen.getByTestId(`field-row-${fields[1].key}`);
    const toolbar = row.querySelector('.canvas-toolbar')!;
    expect(getComputedStyle(toolbar).opacity).toBe('0');
    expect(getComputedStyle(toolbar).pointerEvents, 'a hidden toolbar takes no tap').toBe('none');

    await user.click(row);
    expect(getComputedStyle(toolbar).pointerEvents, 'the toolbar of the selected row can be used').toBe('auto');
    await waitFor(() => expect(getComputedStyle(toolbar).opacity).toBe('1'));
  });

  it('places the side panel beside the card only where the composer leaves the card its width', async () => {
    const card = (): DOMRect => screen.getByRole('region', {name: 'Consent card'}).getBoundingClientRect();
    const overview = (): DOMRect => screen.getByRole('region', {name: 'Overview'}).getBoundingClientRect();
    await atViewport(1400, 900, () => {
      const {unmount} = renderWithProviders(<Harness initial={[amountField()]} />);
      expect(overview().left, 'beside the card').toBeGreaterThan(card().right);
      expect(card().width).toBe(426);
      unmount();

      // A wide screen can still leave the composer narrow, as beside the Console's expanded sidebar.
      renderWithProviders(
        <div style={{width: 820}}>
          <Harness initial={[amountField()]} />
        </div>,
      );
      expect(overview().top, 'below the card').toBeGreaterThan(card().bottom);
      expect(card().width).toBe(426);
    });
  });

  it('lines the values of the rows up at the end of the card, and stacks a long one, as the consent screen does', async () => {
    const short = [amountField(), {...newField(), name: 'note'}, {...newField('date'), name: 'until'}];
    const long: SchemaField = {
      ...newField(),
      name: 'reference',
      title: 'A display name too long to sit beside its value',
    };
    const check = (width: number): void => {
      const rows = short.map((field) => screen.getByTestId(`field-row-${field.key}`));
      const ends = rows.map((row) => row.querySelector('[inert]')!.getBoundingClientRect().right);
      for (const end of ends) expect(end, `${width}: every value ends where the others do`).toBeCloseTo(ends[0], 0);
      const longRow = screen.getByTestId(`field-row-${long.key}`);
      const label = longRow.firstElementChild!.getBoundingClientRect();
      const value = longRow.querySelector('[inert]')!.getBoundingClientRect();
      expect(value.top, `${width}: a long field's value is below its label`).toBeGreaterThanOrEqual(label.bottom);
      expect(value.left, `${width}: and starts where its label does`).toBeCloseTo(label.left, 0);
    };

    await atViewport(1100, 900, () => {
      const {unmount} = renderWithProviders(<Harness initial={[...short, long]} />);
      check(1100);
      const row = screen.getByTestId(`field-row-${short[0].key}`).getBoundingClientRect();
      const value = screen.getByTestId(`field-row-${short[0].key}`).querySelector('[inert]')!.getBoundingClientRect();
      expect(row.right - value.right, 'with the toolbar beside the card, the row is as on the consent screen').toBe(8);
      unmount();
    });
    renderWithProviders(
      <div style={{width: 480}}>
        <Harness initial={[...short, long]} />
      </div>,
    );
    check(480);
    const row = screen.getByTestId(`field-row-${short[0].key}`).getBoundingClientRect();
    const value = screen.getByTestId(`field-row-${short[0].key}`).querySelector('[inert]')!.getBoundingClientRect();
    expect(row.right - value.right, 'on a narrow card too, the row is as on the consent screen').toBe(8);
  });

  it('does not mark the fields of a read-only type as disabled, since they can still be selected', async () => {
    const user = userEvent.setup();
    const group: SchemaField = {...newField('object'), name: 'account', children: [{...newField(), name: 'id'}]};
    renderWithProviders(<Harness initial={[amountField(), group]} readOnly />);

    await new Promise((resolve) => {
      setTimeout(resolve, 100);
    });
    for (const item of screen.getAllByRole('treeitem')) {
      expect(item).not.toHaveAttribute('aria-disabled');
      expect(item).not.toHaveAttribute('aria-roledescription');
    }
    await user.click(screen.getByRole('treeitem', {name: 'id'}));
    expect(screen.getByRole('region', {name: 'Field settings'})).toBeInTheDocument();
  });

  it('takes one stop in the Tab order for the fields, with the controls of the field that has it', async () => {
    const user = userEvent.setup();
    const fields = [
      {...newField(), name: 'note'},
      {...newField('object'), name: 'payee', children: [{...newField(), name: 'iban'}]},
      {...newField(), name: 'memo'},
    ];
    renderWithProviders(<Harness initial={fields} />);
    const item = (name: string): HTMLElement => screen.getByRole('treeitem', {name});
    const tabStops = (): (string | null)[] =>
      screen
        .getAllByRole('treeitem')
        .filter((element) => element.tabIndex === 0)
        .map((element) => element.getAttribute('aria-label'));

    expect(tabStops(), 'the first field, while none was focused or selected').toEqual(['note']);
    expect(screen.getByRole('button', {name: 'Drag note'})).toHaveAttribute('tabindex', '0');
    expect(screen.getByRole('button', {name: 'Drag payee'})).toHaveAttribute('tabindex', '-1');

    item('note').focus();
    await user.keyboard('{ArrowDown}');
    expect(item('payee')).toHaveFocus();
    expect(tabStops()).toEqual(['payee']);
    expect(screen.getByRole('button', {name: 'Drag note'})).toHaveAttribute('tabindex', '-1');
    expect(screen.getByRole('button', {name: 'Insert a field before note'})).toHaveAttribute('tabindex', '-1');

    await user.tab();
    expect(screen.getByRole('button', {name: 'Drag payee'}), 'the controls of the field follow it').toHaveFocus();
    await user.tab();
    expect(within(item('payee')).getByRole('button', {name: 'Field actions'})).toHaveFocus();
    await user.tab();
    expect(screen.getByRole('button', {name: 'Add field'}), 'Tab then leaves the fields').toHaveFocus();
    await user.tab({shift: true});
    await user.tab({shift: true});
    await user.tab({shift: true});
    expect(item('payee')).toHaveFocus();
    await user.tab({shift: true});
    expect(screen.getByRole('button', {name: 'Insert a field before payee'}), 'the plus before it too').toHaveFocus();
    await user.tab({shift: true});
    expect(screen.getByRole('button', {name: 'Consent summary'}), 'and no other field is a stop').toHaveFocus();
  });

  it('shows a summary that names an object with the object as the server writes it', () => {
    const money: SchemaField = {
      ...newField('object'),
      name: 'instructedAmount',
      title: 'Amount',
      children: [
        {...newField('number'), name: 'amount'},
        {...newField(), name: 'currency'},
      ],
    };
    renderWithProviders(<Harness initial={[money]} template="Pay {{instructedAmount}}" />);

    expect(screen.getByTestId('canvas-summary')).toHaveTextContent('Pay {"amount":100,"currency":"EUR"}');
    expect(screen.getByRole('treeitem', {name: 'Amount'}), 'the object is named by its display name').toHaveTextContent(
      'Amount',
    );
  });

  it('does not offer the user to change a date and time, which the consent screen cannot', async () => {
    const user = userEvent.setup();
    const departure: SchemaField = {
      ...newField('dateTime'),
      name: 'departure',
      policy: {modification: 'decrease', modifiableBy: ['client']},
    };
    const arrival: SchemaField = {
      ...newField('dateTime'),
      name: 'arrival',
      policy: {modification: 'decrease', modifiableBy: ['user']},
    };
    renderWithProviders(<Harness initial={[departure, arrival]} />);

    await user.click(screen.getByRole('treeitem', {name: 'departure'}));
    await openSection(user, 'Who sets and changes it');
    expect(screen.getByRole('checkbox', {name: /Your external processor/})).toBeInTheDocument();
    expect(screen.getByRole('checkbox', {name: /The client/})).toBeChecked();
    expect(screen.queryByRole('checkbox', {name: /The user/})).toBeNull();

    await user.click(screen.getByRole('treeitem', {name: 'arrival'}));
    const message =
      'The consent screen cannot change a date and time, so only the client or the external processor can change it.';
    expect(screen.getByText(message)).toBeInTheDocument();
    await user.click(screen.getByRole('checkbox', {name: /The user/}));
    await user.click(screen.getByRole('checkbox', {name: /The client/}));
    expect(screen.queryByText(message)).toBeNull();
    expect(screen.queryByRole('checkbox', {name: /The user/}), 'a choice taken back is not offered again').toBeNull();
  });
});

describe('SchemaComposer, after review', () => {
  const limitField = (): SchemaField => ({
    ...newField('number'),
    name: 'limit',
    title: 'Limit',
    policy: {modification: 'decrease', modifiableBy: ['user']},
  });

  /** Opens the menu of a field on the card and picks an item of it. */
  async function fromMenu(user: ReturnType<typeof userEvent.setup>, name: string, item: string): Promise<void> {
    const row = screen.getByRole('treeitem', {name});
    await browserUser.hover(row);
    await user.click(within(row).getByRole('button', {name: 'Field actions'}));
    await user.click(screen.getByRole('menuitem', {name: item}));
  }

  it('lets nothing inside an object the processor supplies change, as the server requires', async () => {
    const user = userEvent.setup();
    const onSchema = vi.fn();
    const grp: SchemaField = {...newField('object'), name: 'grp', title: 'Group', children: [limitField()]};
    renderWithProviders(<Harness initial={[grp]} onSchema={onSchema} />);

    expect(within(screen.getByRole('treeitem', {name: 'Limit'})).getByRole('spinbutton', {hidden: true})).toBeTruthy();
    await browserUser.hover(screen.getByTestId(`field-group-${grp.key}`));
    await user.click(screen.getByRole('treeitem', {name: 'Group'}));
    await openSection(user, 'Who sets and changes it');
    await user.click(screen.getByRole('radio', {name: 'Your external processor, after the request'}));
    expect(lastSchema(onSchema)).toEqual({
      type: 'object',
      properties: {
        grp: {
          type: 'object',
          title: 'Group',
          policy: {suppliedBy: 'processor'},
          properties: {limit: {type: 'number', title: 'Limit'}},
        },
      },
    });

    await user.click(screen.getByRole('treeitem', {name: 'Limit'}));
    await openSection(user, 'Who sets and changes it');
    expect(screen.getByTestId(/-rule$/)).toHaveTextContent(
      'Your external processor fills this in after the request. Nobody can change it after that.',
    );
    expect(screen.queryByRole('radiogroup', {name: 'Can it change after the request?'})).toBeNull();
    expect(
      within(screen.getByRole('treeitem', {name: 'Limit'})).queryByRole('spinbutton', {hidden: true}),
      'the card shows the value as fixed',
    ).toBeNull();
  });

  it('flags a change rule a field kept inside an object the processor supplies, and lets it be taken back', async () => {
    const user = userEvent.setup();
    const grp: SchemaField = {
      ...newField('object'),
      name: 'grp',
      policy: {suppliedBy: 'processor'},
      children: [limitField()],
    };
    renderWithProviders(<Harness initial={[grp]} />);

    await user.click(screen.getByRole('treeitem', {name: 'Limit'}));
    expect(
      screen.getByText(
        'Your external processor fills in the object this field belongs to, so nobody can change it after the request.',
      ),
    ).toBeInTheDocument();
    await user.click(screen.getByRole('radio', {name: 'No, it stays as requested'}));
    expect(screen.queryByText(/so nobody can change it after the request/)).toBeNull();
    expect(screen.queryByRole('radiogroup', {name: 'Can it change after the request?'})).toBeNull();
  });

  it('fills {{type}} with the type identifier, and marks a type field the consent screen never shows', async () => {
    const user = userEvent.setup();
    const typeField: SchemaField = {...newField(), name: 'type', title: 'Request type'};
    renderWithProviders(<Harness initial={[typeField, amountField()]} template="Pay {{amount}} for {{type}}" />);

    expect(screen.getByTestId('canvas-summary')).toHaveTextContent('Pay 100 for payment_initiation');
    const row = screen.getByRole('treeitem', {name: 'Request type'});
    expect(row).toHaveTextContent('Request typeNot shown on the consent screen');
    await user.click(row);
    expect(screen.getByLabelText(/^Field name/), 'it can still be selected and edited').toHaveValue('type');
  });

  it('refuses a space inside a field name, and says when the summary has text in braces that is not a placeholder', async () => {
    const user = userEvent.setup();
    renderWithProviders(<Harness initial={[{...newField(), name: 'z z'}]} template="{{z z}} Send" />);

    await user.click(screen.getByRole('treeitem', {name: 'z z'}));
    expect(screen.getByText('A field name cannot contain a space.')).toBeInTheDocument();
    await user.click(screen.getByTestId('canvas-summary'));
    expect(screen.getByTestId('template-malformed')).toHaveTextContent(
      '{{z z}} is not a field placeholder. Write a field name in double braces, with no spaces inside.',
    );
  });

  it('leaves out of the example request what a client cannot send', async () => {
    const user = userEvent.setup();
    const fields: SchemaField[] = [
      amountField(),
      {...newField(), name: 'iban', policy: {suppliedBy: 'processor'}},
      {...newField('object'), name: 'empty'},
    ];
    renderWithProviders(<Harness initial={fields} />);

    await openSection(user, 'Example request');
    expect(JSON.parse(screen.getByTestId('composer-example-detail').textContent ?? '')).toEqual({
      type: 'payment_initiation',
      amount: 100,
    });
  });

  it('offers to update the summary after a move with the keyboard or the menu, and keeps focus on the moved field', async () => {
    const user = userEvent.setup();
    const payee: SchemaField = {...newField('object'), name: 'payee', children: [{...newField(), name: 'iban'}]};
    renderWithProviders(<Harness initial={[payee, {...newField(), name: 'note'}]} template="To {{payee.iban}}" />);

    const iban = screen.getByRole('treeitem', {name: 'iban'});
    iban.focus();
    await user.keyboard('{Alt>}{ArrowLeft}{/Alt}');
    expect(screen.getByRole('treeitem', {name: 'iban'})).toHaveFocus();
    expect(screen.getByRole('treeitem', {name: 'iban'})).toHaveAttribute('aria-level', '1');
    expect(screen.getByTestId('summary-rename-notice')).toHaveTextContent(
      'The summary uses {{payee.iban}}, which no longer names a field. Update it to {{iban}}?',
    );

    await fromMenu(user, 'note', 'Move up');
    expect(cardOrder()).toEqual(['payee', 'note', 'iban']);
    expect(screen.getByRole('treeitem', {name: 'note'}), 'focus stays on the moved field').toHaveFocus();
  });

  it('moves focus to the next field after a removal, and back to the field it brings back', async () => {
    const user = userEvent.setup();
    renderWithProviders(<Harness initial={[amountField(), {...newField(), name: 'payee'}]} />);

    await fromMenu(user, 'Amount', 'Remove field');
    expect(screen.getByRole('treeitem', {name: 'payee'})).toHaveFocus();
    await user.click(screen.getByRole('button', {name: 'Undo'}));
    expect(screen.getByRole('treeitem', {name: 'Amount'})).toHaveFocus();

    await fromMenu(user, 'payee', 'Remove field');
    expect(screen.getByRole('treeitem', {name: 'Amount'}), 'or to the field before it').toHaveFocus();
    await fromMenu(user, 'Amount', 'Remove field');
    expect(screen.getByRole('button', {name: 'Undo'}), 'or to the undo, with no field left').toHaveFocus();
  });

  it('trims an allowed value, and refuses a blank or repeated one with a reason', async () => {
    const user = userEvent.setup();
    const onSchema = vi.fn();
    renderWithProviders(<Harness initial={[{...newField('array'), name: 'tags'}]} onSchema={onSchema} />);

    await user.click(screen.getByRole('treeitem', {name: 'tags'}));
    await openSection(user, 'Allowed values');
    const input = screen.getByRole('combobox', {name: 'Allowed values'});
    await user.type(input, ' x {Enter}');
    expect(lastSchema(onSchema)).toEqual({
      type: 'object',
      properties: {tags: {type: 'array', items: {type: 'string', enum: ['x']}}},
    });
    await user.type(input, '   {Enter}');
    expect(screen.getByText('Enter a value, not only spaces.')).toBeInTheDocument();
    await user.clear(input);
    await user.type(input, 'x{Enter}');
    expect(screen.getByText('x is already allowed.')).toBeInTheDocument();
    expect(input).toHaveValue('x');
    // The refused value is selected, so the next value replaces it rather than joining it.
    await user.keyboard('y{Enter}');
    expect(input).toHaveValue('');
    expect(lastSchema(onSchema)).toEqual({
      type: 'object',
      properties: {tags: {type: 'array', items: {type: 'string', enum: ['x', 'y']}}},
    });
  });

  it('warns when two rows of the consent screen would show the same label', async () => {
    const user = userEvent.setup();
    renderWithProviders(<Harness initial={[amountField()]} />);

    expect(screen.queryByTestId('composer-repeated-labels')).toBeNull();
    await user.click(screen.getByRole('button', {name: /^Add a field/}));
    await user.click(screen.getByRole('button', {name: 'Money'}));
    await user.keyboard('{Escape}');
    expect(screen.getByTestId('composer-repeated-labels')).toHaveTextContent(
      'The consent screen shows Amount on more than one row',
    );
  });

  it('inserts a field at the end of a summary just opened, apart from its text, and closes with Escape', async () => {
    const user = userEvent.setup();
    renderWithProviders(<Harness initial={[amountField()]} template="Pay now" />);

    const summary = screen.getByTestId('canvas-summary');
    await user.click(summary);
    await user.click(screen.getByRole('button', {name: 'Insert amount'}));
    expect(screen.getByLabelText('Summary')).toHaveValue('Pay now {{amount}}');

    screen.getByLabelText<HTMLTextAreaElement>('Summary').setSelectionRange(0, 0);
    await user.click(screen.getByRole('button', {name: 'Insert amount'}));
    expect(screen.getByLabelText('Summary')).toHaveValue('{{amount}} Pay now {{amount}}');

    await user.keyboard('{Escape}');
    expect(screen.queryByRole('region', {name: 'Consent summary'})).toBeNull();
    expect(summary).toHaveFocus();
  });

  it('writes counts of one in the singular', async () => {
    const user = userEvent.setup();
    const obj: SchemaField = {...newField('object'), name: 'obj', children: [{...newField(), name: 'x'}]};
    const fields: SchemaField[] = [
      obj,
      {...newField('array'), name: 'tags', minItems: '1'},
      {...newField(), name: 'code', maxLength: '1'},
      amountField(),
    ];
    const {unmount} = renderWithProviders(<Harness initial={[amountField()]} />);
    expect(screen.getByRole('button', {name: /^Who can change what/})).toHaveTextContent(
      '1 of 1 field can change after the request',
    );
    unmount();
    renderWithProviders(<Harness initial={fields} />);

    await user.click(screen.getByRole('treeitem', {name: 'tags'}));
    expect(screen.getByRole('button', {name: /^Limits/})).toHaveTextContent('At least 1 item');
    await user.click(screen.getByRole('treeitem', {name: 'code'}));
    expect(screen.getByRole('button', {name: /^Limits/})).toHaveTextContent('Up to 1 character');
    await browserUser.hover(screen.getByTestId(`field-group-${obj.key}`));
    await fromMenu(user, 'obj', 'Remove field');
    expect(await screen.findByText('Removed obj and the field in it.')).toBeInTheDocument();
  });

  it('offers each field once to insert, even when two share a name', async () => {
    const user = userEvent.setup();
    const errors = vi.spyOn(console, 'error').mockImplementation(() => undefined);
    renderWithProviders(
      <Harness
        initial={[
          {...newField(), name: 'a'},
          {...newField(), name: 'a'},
        ]}
      />,
    );

    await user.click(screen.getByTestId('canvas-summary'));
    expect(screen.getAllByRole('button', {name: 'Insert a'})).toHaveLength(1);
    expect(errors.mock.calls.some(([message]) => String(message).includes('same key'))).toBe(false);
    errors.mockRestore();
  });

  it('says when the summary names a field a request can leave out', async () => {
    const user = userEvent.setup();
    renderWithProviders(
      <Harness initial={[{...newField(), name: 'note'}, amountField()]} template="{{amount}} {{note}}" />,
    );

    await user.click(screen.getByTestId('canvas-summary'));
    expect(screen.getByTestId('template-optional')).toHaveTextContent(
      'A request can leave out {{note}}. When it does, the consent screen shows the type identifier instead of this summary.',
    );
  });

  it('marks an object as a field of the tree, without claiming to hold the fields after it', () => {
    const obj: SchemaField = {...newField('object'), name: 'obj', children: [{...newField(), name: 'x'}]};
    renderWithProviders(<Harness initial={[obj]} />);

    expect(screen.getByRole('treeitem', {name: 'obj'})).not.toHaveAttribute('aria-expanded');
  });

  it('keeps offering to update the summary after the renamed field is closed, on the summary and the overview', async () => {
    const user = userEvent.setup();
    renderWithProviders(<Harness initial={[amountField()]} template="Pay {{amount}}" />);

    await user.click(screen.getByRole('treeitem', {name: 'Amount'}));
    await user.clear(screen.getByLabelText(/^Field name/));
    await user.type(screen.getByLabelText(/^Field name/), 'total');
    await user.click(screen.getByRole('button', {name: 'Close field settings'}));
    expect(
      within(screen.getByRole('region', {name: 'Overview'})).getByTestId('summary-rename-notice'),
    ).toBeInTheDocument();

    await user.click(screen.getByRole('treeitem', {name: 'Amount'}));
    expect(screen.getByTestId('summary-rename-notice'), 'again when the field is selected').toBeInTheDocument();
    await user.click(screen.getByTestId('canvas-summary'));
    const notice = within(screen.getByRole('region', {name: 'Consent summary'})).getByTestId('summary-rename-notice');
    await user.click(within(notice).getByRole('button', {name: 'Update summary'}));
    expect(screen.getByLabelText('Summary')).toHaveValue('Pay {{total}}');
    expect(screen.queryByTestId('summary-rename-notice')).toBeNull();
  });
});
