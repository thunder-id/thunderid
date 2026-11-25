// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, renderWithProviders, screen, userEvent, fireEvent, within} from '@thunderid/test-utils';
import {useEffect, type JSX} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import useAuthorizationDetailTypeForm, {
  type AuthorizationDetailTypeForm,
} from '../../../hooks/useAuthorizationDetailTypeForm';
import type {AuthorizationDetailType, AuthorizationDetailTypeRequest} from '../../../models/authorization-detail-type';
import AskEveryTimeSetting from '../AskEveryTimeSetting';
import FieldsSection from '../FieldsSection';
import GeneralSection from '../GeneralSection';
import ProcessorSection from '../ProcessorSection';

vi.mock('@monaco-editor/react', () => ({
  default: ({value, onChange}: {value: string; onChange?: (v: string | undefined) => void}) => (
    <textarea aria-label="Schema JSON" value={value} onChange={(e) => onChange?.(e.target.value)} />
  ),
}));

/** The schema shown in the JSON editor. */
function schemaJson(): unknown {
  return JSON.parse(screen.getByLabelText<HTMLTextAreaElement>('Schema JSON').value) as unknown;
}

const mockSubmit = vi.fn<(request: AuthorizationDetailTypeRequest) => void>();
let currentForm: AuthorizationDetailTypeForm;

interface FormHarnessProps {
  detailType?: AuthorizationDetailType | null;
  readOnly?: boolean;
  deferProcessorIssues?: boolean;
}

/** Shows every section over one form, as the create wizard and the edit page do across steps and tabs. */
function FormHarness({
  detailType = null,
  readOnly = false,
  deferProcessorIssues = false,
}: FormHarnessProps): JSX.Element {
  const form = useAuthorizationDetailTypeForm(detailType);
  useEffect(() => {
    currentForm = form;
  });

  return (
    <>
      {detailType ? (
        // The edit page edits the name and description in its header, and the reuse setting on General.
        <>
          <label htmlFor="harness-name">Name</label>
          <input
            id="harness-name"
            value={form.name}
            disabled={readOnly}
            onChange={(e) => form.setName(e.target.value)}
          />
          <label htmlFor="harness-description">Description</label>
          <input
            id="harness-description"
            value={form.description}
            disabled={readOnly}
            onChange={(e) => form.setDescription(e.target.value)}
          />
          <AskEveryTimeSetting form={form} readOnly={readOnly} />
        </>
      ) : (
        <GeneralSection form={form} />
      )}
      <FieldsSection form={form} readOnly={readOnly} deferProcessorIssues={deferProcessorIssues} />
      <ProcessorSection form={form} readOnly={readOnly} />
      {form.saveError && <div role="alert">{form.saveError}</div>}
      {form.definitionChanged && <span>definition changed</span>}
      <button
        type="button"
        disabled={!form.isValid || (form.isEdit && !form.hasChanges)}
        onClick={() => mockSubmit(form.buildRequest())}
      >
        Submit
      </button>
    </>
  );
}

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

const SAVE_ERROR = 'Failed to save the intent. Please try again.';

describe('authorization details type sections', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('builds a new type from the form', async () => {
    const user = userEvent.setup();
    renderWithProviders(<FormHarness />);

    await user.type(screen.getByLabelText(/^Name/), 'Flight booking');
    expect(screen.getByLabelText(/^Type identifier/)).toHaveValue('flight_booking');
    await user.click(screen.getByRole('button', {name: 'Add field'}));
    await user.click(screen.getByRole('menuitem', {name: 'Text'}));
    await user.type(screen.getByLabelText(/^Field name/), 'route');
    await user.click(screen.getByRole('button', {name: 'Consent summary'}));
    fireEvent.change(screen.getByLabelText('Summary'), {target: {value: 'Book'}});
    await user.click(screen.getByRole('button', {name: 'Insert route'}));
    expect(screen.getByLabelText('Summary')).toHaveValue('Book {{route}}');
    expect(screen.getByTestId('canvas-summary')).toHaveTextContent('Book ‹route›');
    await user.click(screen.getByRole('button', {name: 'Close summary settings'}));
    expect(screen.getByRole('switch', {name: /Ask every time/})).toBeChecked();
    await user.click(screen.getByRole('switch', {name: /Ask every time/}));
    await user.click(screen.getByRole('button', {name: 'Submit'}));

    expect(mockSubmit).toHaveBeenCalledWith({
      type: 'flight_booking',
      name: 'Flight booking',
      schema: {type: 'object', properties: {route: {type: 'string', maxLength: 100}}},
      consentTemplate: 'Book {{route}}',
      reuse: true,
    });
  });

  it('stops following the name once the identifier is edited', async () => {
    const user = userEvent.setup();
    renderWithProviders(<FormHarness />);

    await user.type(screen.getByLabelText(/^Type identifier/), 'urn:pay');
    await user.type(screen.getByLabelText(/^Name/), 'Payment');
    expect(screen.getByLabelText(/^Type identifier/)).toHaveValue('urn:pay');
  });

  it('does not save fields that rely on a processor that is not set', async () => {
    const user = userEvent.setup();
    renderWithProviders(<FormHarness detailType={paymentType} />);
    const error = /Some fields are set or changed by the external processor/;

    expect(screen.queryByText(error)).not.toBeInTheDocument();
    await user.click(screen.getByRole('switch', {name: 'Use an external processor'}));
    expect(screen.getByText(error)).toBeInTheDocument();
    const amount = screen.getByRole('treeitem', {name: 'Amount'});
    expect(within(amount).getByRole('img', {name: 'This field needs attention'})).toBeInTheDocument();
    await user.click(amount);
    expect(
      screen.getByText('The external processor sets or changes this field, but no processor is set.'),
    ).toBeInTheDocument();
    expect(currentForm.sectionValid).toEqual({general: true, fields: true, processor: false});
    expect(screen.getByRole('button', {name: 'Submit'})).toBeDisabled();
  });

  it('leaves the missing processor off the fields where the processor is set later', async () => {
    const user = userEvent.setup();
    renderWithProviders(<FormHarness detailType={paymentType} deferProcessorIssues />);

    await user.click(screen.getByRole('switch', {name: 'Use an external processor'}));
    expect(screen.getByText(/Some fields are set or changed by the external processor/)).toBeInTheDocument();
    await user.click(screen.getByRole('treeitem', {name: 'Amount'}));
    expect(
      screen.queryByText('The external processor sets or changes this field, but no processor is set.'),
    ).not.toBeInTheDocument();
  });

  it('does not save while the form is invalid', async () => {
    const user = userEvent.setup();
    renderWithProviders(<FormHarness />);

    const submit = screen.getByRole('button', {name: 'Submit'});
    expect(submit).toBeDisabled();
    expect(currentForm.sectionValid).toEqual({general: false, fields: true, processor: true});

    await user.type(screen.getByLabelText(/^Type identifier/), 'has space');
    fireEvent.change(screen.getByLabelText(/^Name/), {target: {value: 'x'.repeat(101)}});
    await user.click(screen.getByRole('button', {name: 'Add field'}));
    await user.click(screen.getByRole('menuitem', {name: 'Text'}));
    await user.click(screen.getByRole('switch', {name: 'Use an external processor'}));
    await user.type(screen.getByLabelText(/^Endpoint URL/), 'http://bank.example.com');

    expect(screen.getByText('Use up to 512 ASCII letters, digits or symbols, without spaces.')).toBeInTheDocument();
    expect(screen.getByText('Enter a name of up to 100 characters.')).toBeInTheDocument();
    expect(screen.queryByText('Enter a field name.'), 'a new field is not flagged before it is named').toBeNull();
    expect(
      screen.getByText('Use HTTPS. HTTP is accepted only for a loopback address, such as localhost or 127.0.0.1.'),
    ).toBeInTheDocument();
    expect(currentForm.sectionValid).toEqual({general: false, fields: false, processor: false});
    expect(submit).toBeDisabled();
  });

  it('edits a type, and shows a save error until the form changes', async () => {
    const user = userEvent.setup();
    renderWithProviders(<FormHarness detailType={paymentType} />);

    expect(screen.getByLabelText(/^Endpoint URL/)).toHaveValue('https://bank.example.com/process');
    expect(screen.getByRole('treeitem', {name: 'Amount'})).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Submit'}), 'an unchanged type has nothing to save').toBeDisabled();
    await user.type(screen.getByLabelText('Description'), 'Pays once');
    await user.click(screen.getByRole('button', {name: 'Submit'}));

    expect(mockSubmit).toHaveBeenCalledWith({
      type: 'payment_initiation',
      name: 'Payment initiation',
      description: 'Pays once',
      schema: paymentType.schema,
      consentTemplate: 'Pay {{amount}}',
      processor: {endpoint: 'https://bank.example.com/process'},
      reuse: false,
    });

    act(() => {
      currentForm.reportSaveError(new Error('boom'));
    });
    expect(await screen.findByText(SAVE_ERROR)).toBeInTheDocument();
    await user.type(screen.getByLabelText(/^Name/), ' v2');
    expect(screen.queryByText(SAVE_ERROR)).not.toBeInTheDocument();
  });

  it('reports a changed definition only when the fields, the summary or the approval reuse change', async () => {
    const user = userEvent.setup();
    renderWithProviders(<FormHarness detailType={paymentType} />);

    await user.type(screen.getByLabelText(/^Name/), ' v2');
    expect(screen.queryByText('definition changed')).not.toBeInTheDocument();
    await user.click(screen.getByRole('switch', {name: /Ask every time/}));
    expect(screen.getByText('definition changed'), 'reusing approvals moves the version').toBeInTheDocument();
    await user.click(screen.getByRole('switch', {name: /Ask every time/}));
    expect(screen.queryByText('definition changed')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', {name: 'Consent summary'}));
    fireEvent.change(screen.getByLabelText('Summary'), {target: {value: 'Pay up to {{amount}}'}});
    expect(screen.getByText('definition changed')).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Summary'), {target: {value: 'Pay {{amount}}'}});
    expect(screen.queryByText('definition changed')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', {name: 'Add field'}));
    await user.click(screen.getByRole('menuitem', {name: 'Text'}));
    expect(screen.getByText('definition changed')).toBeInTheDocument();
  });

  it('reuses a valid approval unless asked every time, and sends the processor fallback', async () => {
    const user = userEvent.setup();
    const detailType: AuthorizationDetailType = {
      ...paymentType,
      schema: {type: 'object', properties: {note: {type: 'string', maxLength: 140}}},
      consentTemplate: 'Note {{note}}',
      processor: {endpoint: 'https://bank.example.com/process'},
      reuse: true,
    };
    renderWithProviders(<FormHarness detailType={detailType} />);

    expect(screen.queryByText(/features the composer cannot show/)).not.toBeInTheDocument();
    await user.click(screen.getByRole('treeitem', {name: 'note'}));
    expect(screen.getByRole('button', {name: /^Limits/})).toHaveTextContent('Up to 140 characters');
    await user.click(screen.getByRole('button', {name: /^Limits/}));
    expect(screen.getByLabelText('Maximum length')).toHaveValue('140');
    await user.click(screen.getByRole('button', {name: 'Close field settings'}));
    expect(screen.getByRole('switch', {name: /Ask every time/})).not.toBeChecked();
    await user.type(screen.getByLabelText('Description'), 'Notes');
    await user.click(screen.getByRole('button', {name: 'Submit'}));

    expect(mockSubmit).toHaveBeenCalledWith({
      type: 'payment_initiation',
      name: 'Payment initiation',
      description: 'Notes',
      schema: detailType.schema,
      consentTemplate: 'Note {{note}}',
      processor: {endpoint: 'https://bank.example.com/process'},
      reuse: true,
    });
  });

  it('builds a type that asks every time by default', async () => {
    const user = userEvent.setup();
    renderWithProviders(<FormHarness />);

    await user.type(screen.getByLabelText(/^Name/), 'Flight booking');
    await user.click(screen.getByRole('button', {name: 'Submit'}));

    expect(mockSubmit).toHaveBeenCalledWith({
      type: 'flight_booking',
      name: 'Flight booking',
      schema: {type: 'object', properties: {}},
      reuse: false,
    });
  });

  it('does not save while the summary names a field the type does not have', async () => {
    const user = userEvent.setup();
    renderWithProviders(<FormHarness detailType={paymentType} />);

    await user.click(screen.getByRole('button', {name: 'Consent summary'}));
    fireEvent.change(screen.getByLabelText('Summary'), {target: {value: 'Pay {{payee}}'}});
    expect(screen.getByText('No field matches payee. Use a field name from the list.')).toBeInTheDocument();
    expect(currentForm.sectionValid.fields).toBe(false);
    expect(screen.getByRole('button', {name: 'Submit'})).toBeDisabled();
  });

  it('does not save while the summary has text in braces that is not a placeholder', async () => {
    const user = userEvent.setup();
    renderWithProviders(<FormHarness detailType={paymentType} />);

    await user.click(screen.getByRole('button', {name: 'Consent summary'}));
    fireEvent.change(screen.getByLabelText('Summary'), {target: {value: 'Pay {{amount}} to {{the payee}}'}});
    expect(screen.getByTestId('template-malformed')).toHaveTextContent(
      '{{the payee}} is not a field placeholder. Write a field name in double braces, with no spaces inside.',
    );
    expect(currentForm.sectionValid.fields).toBe(false);
    expect(screen.getByRole('button', {name: 'Submit'})).toBeDisabled();
  });

  it('does not save while the summary has a control or text direction character', async () => {
    const user = userEvent.setup();
    renderWithProviders(<FormHarness detailType={paymentType} />);

    await user.click(screen.getByRole('button', {name: 'Consent summary'}));
    fireEvent.change(screen.getByLabelText('Summary'), {target: {value: 'Pay {{amount}}\u202e'}});
    expect(screen.getByTestId('template-unsafe')).toHaveTextContent(
      'Remove the control or text direction characters from the summary.',
    );
    expect(currentForm.sectionValid.fields).toBe(false);

    fireEvent.change(screen.getByLabelText('Summary'), {target: {value: 'Pay {{amount}}\nnow'}});
    expect(screen.queryByTestId('template-unsafe'), 'a line break is allowed').toBeNull();
    expect(currentForm.sectionValid.fields).toBe(true);
  });

  it('marks and selects the field a schema error names, without the server text', async () => {
    renderWithProviders(<FormHarness detailType={paymentType} />);

    const reason = 'policy of amount: limit a number with its own keywords';
    let target: unknown;
    act(() => {
      target = currentForm.reportSaveError(
        Object.assign(new Error('bad schema'), {
          response: {
            data: {code: 'RAR-1006', description: {defaultValue: 'raw server text', params: {path: 'amount', reason}}},
          },
        }),
      );
    });
    expect(target).toBe('schema');
    expect(await screen.findByTestId('schema-save-error')).toHaveTextContent(
      'The schema is not valid. Check the field types and their rules.',
    );
    expect(screen.queryByText(reason, {exact: false})).toBeNull();
    expect(screen.queryByText('raw server text')).toBeNull();

    // The field the server names is selected with the problem on it.
    expect(screen.getByRole('region', {name: 'Field settings'})).toHaveTextContent('Amount');
    expect(screen.getByRole('treeitem', {name: 'Amount'})).toHaveAttribute('aria-selected', 'true');
    expect(
      screen.getByText("The server refused this field's rules. Check its type, limits and who can change it."),
    ).toBeInTheDocument();
  });

  it.each([
    ['the schema itself', ''],
    ['a field the type does not have', 'fee'],
    ['no field', undefined],
  ])('shows a schema error about %s above the card only', async (_case, path) => {
    renderWithProviders(<FormHarness detailType={paymentType} />);

    act(() => {
      currentForm.reportSaveError(
        Object.assign(new Error('bad schema'), {
          response: {data: {code: 'RAR-1006', description: {params: {path, reason: 'policy of fee: no such field'}}}},
        }),
      );
    });
    expect(await screen.findByTestId('schema-save-error')).toHaveTextContent(
      'The schema is not valid. Check the field types and their rules.',
    );
    expect(screen.queryByText('policy of fee: no such field', {exact: false})).toBeNull();
    expect(screen.queryByRole('region', {name: 'Field settings'})).toBeNull();
  });

  it('takes a summary that names an object in the composer and as JSON, as the server does', async () => {
    const user = userEvent.setup();
    const schema = {
      type: 'object',
      properties: {payee: {type: 'object', properties: {iban: {type: 'string', title: 'IBAN'}}}},
    };
    renderWithProviders(<FormHarness detailType={{...paymentType, processor: undefined, schema}} />);

    fireEvent.change(screen.getByLabelText('Description'), {target: {value: 'x'}});
    await user.click(screen.getByRole('button', {name: 'Consent summary'}));
    fireEvent.change(screen.getByLabelText('Summary'), {target: {value: 'Pay {{payee}}'}});
    expect(screen.getByTestId('canvas-summary')).toHaveTextContent('Pay {"iban":"‹IBAN›"}');
    expect(currentForm.sectionValid.fields).toBe(true);

    await user.click(screen.getByRole('button', {name: 'JSON'}));
    expect(screen.queryByText(/No field matches/)).toBeNull();
    expect(currentForm.sectionValid.fields).toBe(true);
    expect(screen.getByRole('button', {name: 'Submit'})).toBeEnabled();
  });

  it('names where in the JSON a schema error is, without the server text', () => {
    const schema = {
      type: 'object',
      properties: {payee: {type: 'object', properties: {iban: {type: 'string', pattern: '^[A-Z0-9]+$'}}}},
    };
    renderWithProviders(<FormHarness detailType={{...paymentType, consentTemplate: '', schema}} />);
    const refuse = (path: string): void =>
      act(() => {
        currentForm.reportSaveError(
          Object.assign(new Error('bad schema'), {
            response: {
              data: {
                code: 'RAR-1006',
                description: {defaultValue: 'raw server text', params: {path, reason: 'title must not be empty'}},
              },
            },
          }),
        );
      });
    const message = 'The schema is not valid. Check the field types and their rules.';

    refuse('payee.iban');
    expect(screen.getByTestId('schema-save-error')).toHaveTextContent(`payee.iban: ${message}`);
    expect(screen.queryByText('title must not be empty', {exact: false})).toBeNull();
    expect(screen.queryByText('raw server text', {exact: false})).toBeNull();

    refuse('');
    expect(screen.getByTestId('schema-save-error'), 'an error about the whole schema names no place').toHaveTextContent(
      new RegExp(`^${message}$`),
    );
  });

  it('shows a summary the server refused on the summary, without the server text', async () => {
    const user = userEvent.setup();
    renderWithProviders(<FormHarness detailType={paymentType} />);
    const refuse = (): void =>
      act(() => {
        currentForm.reportSaveError(
          Object.assign(new Error('bad summary'), {
            response: {data: {code: 'RAR-1011', description: {params: {reason: 'unknown placeholder amount'}}}},
          }),
        );
      });
    const message = 'The consent summary is not valid. Each placeholder must name a field of the intent.';

    refuse();
    expect(await screen.findByTestId('template-save-error')).toHaveTextContent(message);
    expect(screen.queryByText('unknown placeholder amount', {exact: false})).toBeNull();
    expect(screen.getByLabelText('Summary'), 'the summary is selected to show it').toHaveAttribute(
      'aria-invalid',
      'true',
    );
    expect(screen.queryByTestId('schema-save-error')).toBeNull();

    await user.click(screen.getByRole('button', {name: 'JSON'}));
    refuse();
    expect(await screen.findByTestId('template-save-error')).toHaveTextContent(message);
    expect(screen.queryByTestId('schema-save-error'), 'not above the JSON editor').toBeNull();
  });

  it.each([
    ['RAR-1008', 'The external processor authentication is not valid. Check the method and its credentials.'],
    ['RAR-1009', 'Enter the external processor credentials again, since the endpoint changed.'],
  ])('shows a %s save error on the processor credential', async (code, message) => {
    const user = userEvent.setup();
    renderWithProviders(<FormHarness detailType={paymentType} />);
    await user.click(screen.getByRole('combobox', {name: /Authentication method/}));
    await user.click(screen.getByRole('option', {name: 'Bearer token'}));
    await user.type(document.getElementById('connection-field-bearerToken')!, 'tok');

    let target: unknown;
    act(() => {
      target = currentForm.reportSaveError(
        Object.assign(new Error('bad credential'), {response: {data: {code, description: {defaultValue: 'raw'}}}}),
      );
    });
    expect(target).toBe('authentication');
    const credential = document.getElementById('connection-field-bearerToken')!;
    expect(credential).toHaveAttribute('aria-invalid', 'true');
    expect(credential.closest('.MuiFormControl-root')).toHaveTextContent(message);
    expect(screen.getByLabelText(/^Endpoint URL/)).not.toHaveAttribute('aria-invalid', 'true');
    expect(screen.getByRole('alert'), 'the form level error the harness shows').toHaveTextContent(message);
  });

  it('shows a processor credential error for the whole form while the stored credential is kept', () => {
    const withBearer: AuthorizationDetailType = {
      ...paymentType,
      processor: {
        endpoint: 'https://bank.example.com/process',
        authentication: {type: 'bearer', properties: {token: '******'}},
      },
    };
    renderWithProviders(<FormHarness detailType={withBearer} />);

    let target: unknown = 'unset';
    act(() => {
      target = currentForm.reportSaveError(
        Object.assign(new Error('bad credential'), {response: {data: {code: 'RAR-1009'}}}),
      );
    });
    expect(target).toBeNull();
  });

  it('shows a processor credential error for the whole form when there is no credential to show it on', () => {
    renderWithProviders(<FormHarness detailType={paymentType} />);

    let target: unknown = 'unset';
    act(() => {
      target = currentForm.reportSaveError(
        Object.assign(new Error('bad credential'), {response: {data: {code: 'RAR-1008'}}}),
      );
    });
    expect(target).toBeNull();
    expect(screen.getByRole('alert')).toHaveTextContent(
      'The external processor authentication is not valid. Check the method and its credentials.',
    );
  });

  it('checks the summary of a schema the composer cannot show against the fields it declares', () => {
    const schema = {
      type: 'object',
      properties: {
        amount: {type: 'number', enum: [10, 20]},
        payee: {type: 'object', properties: {iban: {type: 'string', pattern: '^[A-Z0-9]+$'}}},
      },
    };
    renderWithProviders(<FormHarness detailType={{...paymentType, schema}} />);

    expect(screen.getByText(/features the composer cannot show/)).toBeInTheDocument();
    fireEvent.change(screen.getByLabelText('Summary'), {
      target: {value: 'Pay {{amount}} to {{payee.iban}} as {{type}}'},
    });
    expect(currentForm.sectionValid.fields).toBe(true);
    fireEvent.change(screen.getByLabelText('Summary'), {target: {value: 'Pay {{amount}} to {{payee.name}}'}});
    expect(screen.getByText('No field matches payee.name. Use a field name from the list.')).toBeInTheDocument();
    expect(currentForm.sectionValid.fields).toBe(false);
    expect(screen.getByRole('button', {name: 'Submit'})).toBeDisabled();
  });

  it('does not report a changed definition when the fields only change order', async () => {
    const user = userEvent.setup();
    const detailType: AuthorizationDetailType = {
      ...paymentType,
      schema: {
        type: 'object',
        required: ['type', 'amount'],
        properties: {
          amount: {
            type: 'number',
            policy: {suppliedBy: 'client', modification: 'decrease', modifiableBy: ['client', 'processor']},
          },
          payee: {type: 'string'},
        },
      },
    };
    renderWithProviders(<FormHarness detailType={detailType} />);

    screen.getByRole('treeitem', {name: 'payee'}).focus();
    await user.keyboard('{Alt>}{ArrowUp}{/Alt}');
    expect(currentForm.hasChanges).toBe(true);
    expect(screen.queryByText('definition changed')).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Submit'}));
    expect(Object.keys((mockSubmit.mock.calls[0][0].schema as {properties: object}).properties)).toEqual([
      'payee',
      'amount',
    ]);
  });

  it('resolves any other server error through the catalog', async () => {
    renderWithProviders(<FormHarness detailType={paymentType} />);

    act(() => {
      currentForm.reportSaveError(
        Object.assign(new Error('conflict'), {
          response: {data: {code: 'RAR-1003', description: {defaultValue: 'raw server text'}}},
        }),
      );
    });
    expect(await screen.findByRole('alert')).not.toHaveTextContent('raw server text');
    act(() => currentForm.clearSaveError());
    expect(screen.queryByRole('alert')).not.toBeInTheDocument();
  });

  it('sends the processor credential the admin chooses', async () => {
    const user = userEvent.setup();
    renderWithProviders(<FormHarness detailType={paymentType} />);

    await user.click(screen.getByRole('combobox', {name: /Authentication method/}));
    await user.click(screen.getByRole('option', {name: 'API key'}));
    expect(screen.getByRole('button', {name: 'Submit'})).toBeDisabled();
    await user.type(document.getElementById('connection-field-httpHeaders-name-1')!, 'X-Api-Key');
    await user.type(document.getElementById('connection-field-httpHeaders-value-1')!, 'secret');
    await user.click(screen.getByRole('button', {name: 'Add header'}));
    expect(document.getElementById('connection-field-httpHeaders-name-2')).toBeInTheDocument();
    await user.click(screen.getByTestId('connection-field-httpHeaders-remove-2'));
    await user.click(screen.getByRole('button', {name: 'Submit'}));

    const [[request]] = mockSubmit.mock.calls;
    expect(request.processor).toEqual({
      endpoint: 'https://bank.example.com/process',
      authentication: {type: 'api_key', properties: {'X-Api-Key': 'secret'}},
    });
  });

  it('keeps a stored credential by leaving the authentication out', async () => {
    const user = userEvent.setup();
    const withBasic: AuthorizationDetailType = {
      ...paymentType,
      processor: {
        endpoint: 'https://bank.example.com/process',
        authentication: {type: 'basic', properties: {username: 'svc', password: '******'}},
      },
    };
    renderWithProviders(<FormHarness detailType={withBasic} />);

    expect(screen.getByLabelText(/^Username/)).toHaveValue('svc');
    expect(screen.getByLabelText(/^Username/)).toBeDisabled();
    await user.type(screen.getByLabelText(/^Name/), ' v2');
    await user.click(screen.getByRole('button', {name: 'Submit'}));

    const [[request]] = mockSubmit.mock.calls;
    expect(request.processor).toEqual({endpoint: 'https://bank.example.com/process'});
  });

  it('sends a replaced stored credential', async () => {
    const user = userEvent.setup();
    const withBearer: AuthorizationDetailType = {
      ...paymentType,
      processor: {
        endpoint: 'https://bank.example.com/process',
        authentication: {type: 'bearer', properties: {token: '******'}},
      },
    };
    renderWithProviders(<FormHarness detailType={withBearer} />);

    await user.click(screen.getByTestId('connection-field-bearerToken-replace'));
    expect(screen.getByRole('button', {name: 'Submit'})).toBeDisabled();
    await user.type(document.getElementById('connection-field-bearerToken')!, 'new-token');
    await user.click(screen.getByRole('button', {name: 'Submit'}));

    const [[request]] = mockSubmit.mock.calls;
    expect(request.processor?.authentication).toEqual({type: 'bearer', properties: {token: 'new-token'}});
  });

  it('requires the stored credential again when the endpoint changes', async () => {
    const user = userEvent.setup();
    const withBearer: AuthorizationDetailType = {
      ...paymentType,
      processor: {
        endpoint: 'https://bank.example.com/process',
        authentication: {type: 'bearer', properties: {token: '******'}},
      },
    };
    renderWithProviders(<FormHarness detailType={withBearer} />);

    await user.clear(screen.getByLabelText(/^Endpoint URL/));
    await user.type(screen.getByLabelText(/^Endpoint URL/), 'https://bank.example.com/v2');
    expect(screen.getByRole('button', {name: 'Submit'})).toBeDisabled();
    expect(document.getElementById('connection-field-bearerToken')).not.toBeDisabled();
    await user.type(document.getElementById('connection-field-bearerToken')!, 'tok');
    await user.click(screen.getByRole('button', {name: 'Submit'}));

    const [[request]] = mockSubmit.mock.calls;
    expect(request.processor).toEqual({
      endpoint: 'https://bank.example.com/v2',
      authentication: {type: 'bearer', properties: {token: 'tok'}},
    });
  });

  it('requires the credentials of the chosen scheme', async () => {
    const user = userEvent.setup();
    renderWithProviders(<FormHarness detailType={paymentType} />);

    await user.click(screen.getByRole('combobox', {name: /Authentication method/}));
    await user.click(screen.getByRole('option', {name: 'Bearer token'}));
    expect(screen.getByRole('button', {name: 'Submit'})).toBeDisabled();
    await user.type(document.getElementById('connection-field-bearerToken')!, 'tok');
    await user.click(screen.getByRole('button', {name: 'Submit'}));

    const [[request]] = mockSubmit.mock.calls;
    expect(request.processor?.authentication).toEqual({type: 'bearer', properties: {token: 'tok'}});
  });

  it('names what the server would refuse in a schema the composer cannot show, before saving', () => {
    const schema = {
      type: 'object',
      properties: {
        o: {type: 'object', properties: {q: {type: 'string', oneOf: [{type: 'string'}]}}},
        s: {type: 'string', format: 'email', maxLength: 9223372036854775808},
      },
    };
    renderWithProviders(<FormHarness detailType={{...paymentType, schema}} />);

    expect(screen.getByTestId('schema-json-issues')).toHaveTextContent(
      [
        'o.q: oneOf is not supported here.',
        's: maxLength must be a whole number up to 2147483647.',
        's: Use date or date-time as the format.',
      ].join(''),
    );
    fireEvent.change(screen.getByLabelText('Schema JSON'), {
      target: {value: JSON.stringify({type: 'object', properties: {n: {type: 'integer', minimum: 0}}})},
    });
    expect(screen.getByTestId('schema-json-issues')).toHaveTextContent(
      'n: Use number instead of integer, with a multipleOf of 1 for whole numbers.',
    );
    fireEvent.change(screen.getByLabelText('Schema JSON'), {
      target: {value: JSON.stringify({type: 'object', properties: {n: {type: 'number', minimum: 0, multipleOf: 1}}})},
    });
    expect(screen.queryByTestId('schema-json-issues')).toBeNull();
  });

  it('opens a schema the composer cannot show as JSON, and saves it as it is', async () => {
    const user = userEvent.setup();
    const schema = {type: 'object', properties: {amount: {type: 'number', enum: [10, 20]}}};
    renderWithProviders(<FormHarness detailType={{...paymentType, schema}} />);

    expect(screen.getByText(/features the composer cannot show/)).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Composer'})).toBeDisabled();
    expect(schemaJson()).toEqual(schema);
    await user.type(screen.getByLabelText('Description'), 'Pays once');
    await user.click(screen.getByRole('button', {name: 'Submit'}));
    const [[request]] = mockSubmit.mock.calls;
    expect(request.schema).toEqual(schema);
  });

  it('switches between the composer and JSON, keeping the schema', async () => {
    const user = userEvent.setup();
    renderWithProviders(<FormHarness />);

    await user.click(screen.getByRole('button', {name: 'Add field'}));
    await user.click(screen.getByRole('menuitem', {name: 'Text'}));
    await user.type(screen.getByLabelText(/^Field name/), 'route');
    await user.click(screen.getByRole('button', {name: 'JSON'}));
    expect(schemaJson()).toEqual({
      type: 'object',
      properties: {route: {type: 'string', maxLength: 100}},
    });

    const edited = {type: 'object', properties: {route: {type: 'string', maxLength: 100}, seats: {type: 'number'}}};
    fireEvent.change(screen.getByLabelText('Schema JSON'), {target: {value: JSON.stringify(edited)}});
    await user.click(screen.getByRole('button', {name: 'Composer'}));
    expect(screen.queryByLabelText('Schema JSON')).not.toBeInTheDocument();
    expect(screen.getByRole('treeitem', {name: 'route'})).toBeInTheDocument();
    expect(screen.getByRole('treeitem', {name: 'seats'})).toBeInTheDocument();
  });

  it('round-trips a composed schema through JSON without losing anything', async () => {
    const user = userEvent.setup();
    renderWithProviders(<FormHarness detailType={paymentType} />);

    await user.click(screen.getByRole('button', {name: 'Add field'}));
    await user.click(screen.getByRole('menuitem', {name: 'Money'}));
    await user.type(screen.getByLabelText(/^Field name/), 'fee');
    // The money preset has an amount of its own; the type's amount comes first.
    await user.click(screen.getAllByRole('treeitem', {name: 'Amount'})[0]);
    await user.click(screen.getByRole('button', {name: /^Who sets and changes it/}));
    await user.click(screen.getByRole('checkbox', {name: 'The user, on the consent screen'}));
    await user.click(screen.getByRole('button', {name: 'JSON'}));
    const composed = schemaJson();
    expect(composed).toEqual({
      type: 'object',
      properties: {
        amount: {
          type: 'number',
          title: 'Amount',
          maximum: 500,
          policy: {modification: 'decrease', modifiableBy: ['processor', 'user']},
        },
        fee: {
          type: 'object',
          required: ['amount', 'currency'],
          properties: {
            amount: {type: 'number', title: 'Amount'},
            currency: {type: 'string', title: 'Currency', maxLength: 3},
          },
        },
      },
    });

    await user.click(screen.getByRole('button', {name: 'Composer'}));
    await user.click(screen.getByRole('button', {name: 'JSON'}));
    expect(schemaJson()).toEqual(composed);
  });

  it('saves a schema edited as JSON', async () => {
    const user = userEvent.setup();
    renderWithProviders(<FormHarness />);
    await user.type(screen.getByLabelText(/^Name/), 'Flight booking');
    await user.click(screen.getByRole('button', {name: 'JSON'}));
    const schema = {type: 'object', properties: {route: {type: 'string', pattern: '^[A-Z]{3}-[A-Z]{3}$'}}};
    fireEvent.change(screen.getByLabelText('Schema JSON'), {target: {value: JSON.stringify(schema)}});
    await user.click(screen.getByRole('button', {name: 'Submit'}));

    const [[request]] = mockSubmit.mock.calls;
    expect(request.schema).toEqual(schema);
  });

  it('does not save JSON that is not a schema object', async () => {
    const user = userEvent.setup();
    renderWithProviders(<FormHarness />);
    await user.type(screen.getByLabelText(/^Name/), 'Flight booking');
    await user.click(screen.getByRole('button', {name: 'JSON'}));
    fireEvent.change(screen.getByLabelText('Schema JSON'), {
      target: {value: '{\n  "type": "object",\n  "properties": {}\n'},
    });

    expect(
      screen.getByText('The JSON is not valid near line 4, column 1. Check for a missing comma, quote, or bracket.'),
    ).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Composer'})).toBeDisabled();

    fireEvent.change(screen.getByLabelText('Schema JSON'), {target: {value: '["type"]'}});
    expect(screen.getByText('Enter the schema as a JSON object.')).toBeInTheDocument();
    expect(currentForm.sectionValid.fields).toBe(false);
    expect(screen.getByRole('button', {name: 'Submit'})).toBeDisabled();
  });

  it('does not save a schema larger than the server stores', async () => {
    const user = userEvent.setup();
    renderWithProviders(<FormHarness />);
    await user.type(screen.getByLabelText(/^Name/), 'Flight booking');
    await user.click(screen.getByRole('button', {name: 'JSON'}));
    const schema = {type: 'object', description: 'x'.repeat(64 * 1024), properties: {}};
    fireEvent.change(screen.getByLabelText('Schema JSON'), {target: {value: JSON.stringify(schema)}});

    expect(screen.getByTestId('schema-too-large')).toHaveTextContent(
      'The schema can be at most 64 KiB. Remove fields or shorten their settings.',
    );
    expect(currentForm.sectionValid.fields).toBe(false);
    expect(screen.getByRole('button', {name: 'Submit'})).toBeDisabled();
  });

  it('reports field problems found in JSON with the field name', async () => {
    const user = userEvent.setup();
    renderWithProviders(<FormHarness />);
    await user.type(screen.getByLabelText(/^Name/), 'Flight booking');
    await user.click(screen.getByRole('button', {name: 'JSON'}));
    const schema = {
      type: 'object',
      properties: {fare: {type: 'number', minimum: 500, maximum: 100}},
    };
    fireEvent.change(screen.getByLabelText('Schema JSON'), {target: {value: JSON.stringify(schema)}});

    expect(screen.getByText(/^fare: /)).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Submit'})).toBeDisabled();
  });

  it('shows a read-only type without letting it change', async () => {
    const user = userEvent.setup();
    renderWithProviders(<FormHarness detailType={paymentType} readOnly />);

    expect(screen.getByLabelText(/^Name/)).toBeDisabled();
    expect(screen.getByLabelText('Description')).toBeDisabled();
    expect(screen.getByLabelText(/^Endpoint URL/)).toBeDisabled();
    expect(screen.getByRole('combobox', {name: /Authentication method/})).toHaveAttribute('aria-disabled', 'true');
    expect(screen.getByRole('switch', {name: /Ask every time/})).toBeDisabled();
    expect(screen.queryByRole('button', {name: 'Add field'})).not.toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Consent summary'}));
    expect(screen.getByLabelText('Summary')).toBeDisabled();
    expect(screen.queryByRole('button', {name: 'Insert amount'})).not.toBeInTheDocument();
  });

  it('edits the summary beside the JSON editor, where there is no card to select it on', async () => {
    const user = userEvent.setup();
    renderWithProviders(<FormHarness detailType={paymentType} />);

    await user.click(screen.getByRole('button', {name: 'JSON'}));
    expect(screen.getByLabelText('Summary')).toHaveValue('Pay {{amount}}');
    fireEvent.change(screen.getByLabelText('Summary'), {target: {value: 'Pay {{fee}}'}});
    expect(screen.getByText('No field matches fee. Use a field name from the list.')).toBeInTheDocument();
    expect(currentForm.sectionValid.fields).toBe(false);
    await user.click(screen.getByRole('button', {name: 'Insert amount'}));
    expect(screen.getByLabelText('Summary')).toHaveValue('Pay {{fee}} {{amount}}');
    expect(screen.getByRole('switch', {name: /Ask every time/})).toBeChecked();
  });
});
