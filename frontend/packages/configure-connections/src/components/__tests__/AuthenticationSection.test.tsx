// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@thunderid/test-utils';
import {type ComponentProps, useState} from 'react';
import {describe, expect, it, vi} from 'vitest';
import type {OutboundAuthMethod} from '../../models/connection';
import {AUTH_TYPE_FIELD, authPropertyField} from '../../utils/connectionFormMapping';
import AuthenticationSection from '../AuthenticationSection';

vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useToast: () => ({showToast: vi.fn()}),
}));

/** The metadata the server returns for a vendor that supports the built-in methods. */
const BASIC_METHODS: OutboundAuthMethod[] = [
  {displayName: '{{t(connections:outboundAuth.method.none)}}', properties: [], type: 'none'},
  {
    displayName: '{{t(connections:outboundAuth.method.basic)}}',
    properties: [
      {
        displayName: '{{t(connections:outboundAuth.field.username)}}',
        name: 'username',
        required: true,
        type: 'string',
      },
      {
        credential: true,
        displayName: '{{t(connections:outboundAuth.field.password)}}',
        name: 'password',
        required: true,
        type: 'string',
      },
    ],
    type: 'basic',
  },
];

/**
 * The section is controlled, so edits only show up when the parent feeds them back. This mirrors
 * how ConnectionForm's own callers behave.
 */
function ControlledAuthenticationSection({
  values: initialValues,
  onFieldChange,
  ...rest
}: ComponentProps<typeof AuthenticationSection>): ReturnType<typeof AuthenticationSection> {
  const [values, setValues] = useState(initialValues);
  return (
    <AuthenticationSection
      {...rest}
      values={values}
      onFieldChange={(name, value) => {
        onFieldChange(name, value);
        setValues((prev) => ({...prev, [name]: value}));
      }}
    />
  );
}

function renderSection(overrides: Partial<ComponentProps<typeof AuthenticationSection>> = {}): {
  onFieldChange: ReturnType<typeof vi.fn>;
} {
  const onFieldChange = vi.fn();
  render(
    <ControlledAuthenticationSection
      methods={BASIC_METHODS}
      values={{}}
      mode="create"
      secretReplacing={false}
      hasStoredSecret={false}
      onFieldChange={onFieldChange}
      onSecretReplacingChange={vi.fn()}
      {...overrides}
    />,
  );
  return {onFieldChange};
}

function field(name: string): HTMLElement {
  const element = document.getElementById(`connection-field-${name}`);
  if (!element) {
    throw new Error(`Expected field ${name} to exist`);
  }
  return element;
}

describe('AuthenticationSection', () => {
  // A vendor whose credentials are a fixed contract advertises no methods, and must render
  // nothing rather than an empty section.
  it('renders nothing when the vendor advertises no methods', () => {
    renderSection({methods: []});
    expect(screen.queryByTestId('connection-authentication-section')).toBeNull();
  });

  it('renders the section heading by default', () => {
    renderSection();
    expect(screen.getByRole('heading', {name: 'Authentication'})).toBeInTheDocument();
  });

  it('omits the section heading when showHeading is false', () => {
    renderSection({showHeading: false});
    expect(screen.getByTestId('connection-authentication-section')).toBeInTheDocument();
    expect(screen.queryByRole('heading', {name: 'Authentication'})).toBeNull();
  });

  it('defaults to the method that sends no credentials', () => {
    renderSection();
    expect(screen.getByTestId('connection-authentication-section')).toBeInTheDocument();
    // No method is selected yet, so no credential fields are asked for.
    expect(document.getElementById(`connection-field-${authPropertyField('username')}`)).toBeNull();
  });

  // A method that takes no fields may reach the console with `properties` absent rather than as
  // an empty array. Iterating it blindly blanked the whole wizard step.
  it('renders the section for a selected method that carries no fields at all', () => {
    const methods: OutboundAuthMethod[] = [{displayName: 'None', type: 'none'}];
    renderSection({methods, values: {[AUTH_TYPE_FIELD]: 'none'}});

    expect(screen.getByTestId('connection-authentication-section')).toBeInTheDocument();
    expect(document.querySelectorAll('[id^="connection-field-authentication.properties."]')).toHaveLength(0);
  });

  it('renders the selected method fields in the order the server gave them', () => {
    renderSection({values: {[AUTH_TYPE_FIELD]: 'basic'}});

    const rendered = Array.from(document.querySelectorAll('[id^="connection-field-authentication.properties."]')).map(
      (element) => element.id,
    );
    expect(rendered).toEqual([
      `connection-field-${authPropertyField('username')}`,
      `connection-field-${authPropertyField('password')}`,
    ]);
  });

  it('resolves server-supplied labels through the locale bundle', () => {
    renderSection({values: {[AUTH_TYPE_FIELD]: 'basic'}});
    const label = document.querySelector(`label[for="connection-field-${authPropertyField('username')}"]`);
    expect(label?.textContent).toContain('Username');
  });

  // A method added server-side has no locale entry, so the raw field name is all the console
  // can show. It must still render rather than showing a blank label.
  it('falls back to the raw name for a label with no locale entry', () => {
    renderSection({
      methods: [
        {
          displayName: '{{t(connections:outboundAuth.method.clientCredentials)}}',
          properties: [
            {displayName: '{{t(connections:outboundAuth.field.tokenEndpoint)}}', name: 'tokenEndpoint', type: 'string'},
          ],
          type: 'client_credentials',
        },
      ],
      values: {[AUTH_TYPE_FIELD]: 'client_credentials'},
    });

    const label = document.querySelector(`label[for="connection-field-${authPropertyField('tokenEndpoint')}"]`);
    expect(label?.textContent).toContain('tokenEndpoint');
  });

  it('reports the selected method under the namespaced key', () => {
    const {onFieldChange} = renderSection();

    fireEvent.mouseDown(screen.getByRole('combobox'));
    fireEvent.click(screen.getByRole('option', {name: 'Username and Password'}));

    expect(onFieldChange).toHaveBeenCalledWith(AUTH_TYPE_FIELD, 'basic');
  });

  it('reports a field edit under its namespaced key', () => {
    const {onFieldChange} = renderSection({values: {[AUTH_TYPE_FIELD]: 'basic'}});

    fireEvent.change(field(authPropertyField('username')), {target: {value: 'mailer'}});

    expect(onFieldChange).toHaveBeenCalledWith(authPropertyField('username'), 'mailer');
  });

  // A credential must never render as a plain visible input, which is what separates it from a
  // header pasted into a free-text field.
  it('renders a credential as a masked field', () => {
    renderSection({values: {[AUTH_TYPE_FIELD]: 'basic'}});
    expect(field(authPropertyField('password')).getAttribute('type')).toBe('password');
  });

  it('renders an enum field as a select of its choices', () => {
    renderSection({
      methods: [
        {
          displayName: 'API key',
          properties: [
            {displayName: 'Placement', enum: ['header', 'query'], name: 'in', required: true, type: 'string'},
          ],
          type: 'api_key',
        },
      ],
      values: {[AUTH_TYPE_FIELD]: 'api_key'},
    });

    const selects = screen.getAllByRole('combobox');
    // The method selector plus the enum field.
    expect(selects).toHaveLength(2);
  });

  // An unrecognized field type renders nothing rather than guessing at a widget.
  it('skips a field whose type it does not recognize', () => {
    renderSection({
      methods: [
        {
          displayName: 'Exotic',
          properties: [{displayName: 'Payload', name: 'payload', type: 'object'}],
          type: 'exotic',
        },
      ],
      values: {[AUTH_TYPE_FIELD]: 'exotic'},
    });

    expect(document.getElementById(`connection-field-${authPropertyField('payload')}`)).toBeNull();
  });
});
