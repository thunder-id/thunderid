// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@thunderid/test-utils';
import {type ComponentProps, useState} from 'react';
import {describe, expect, it, vi} from 'vitest';
import ConnectionForm from '../ConnectionForm';

/**
 * ConnectionForm is controlled: edits only show up if the parent feeds them back through `values`,
 * as ConnectionCreateWizardPage and ConnectionDetailPage do. Tests that assert on what a field
 * displays after an edit render through this harness rather than static props.
 */
function ControlledConnectionForm({
  values: initialValues,
  onFieldChange,
  ...rest
}: ComponentProps<typeof ConnectionForm>): ReturnType<typeof ConnectionForm> {
  const [values, setValues] = useState(initialValues);
  return (
    <ConnectionForm
      {...rest}
      values={values}
      onFieldChange={(name, value) => {
        onFieldChange(name, value);
        setValues((prev) => ({...prev, [name]: value}));
      }}
    />
  );
}

vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useToast: () => ({showToast: vi.fn()}),
}));

function getConnectionField(id: string): HTMLElement {
  const field = document.getElementById(`connection-field-${id}`);
  if (!field) {
    throw new Error(`Expected connection field ${id} to exist`);
  }
  return field;
}

function isFieldMarkedRequired(id: string): boolean {
  const label = document.querySelector(`label[for="connection-field-${id}"]`);
  return Boolean(label?.querySelector('.MuiFormLabel-asterisk'));
}

describe('ConnectionForm', () => {
  const baseProps = {
    type: 'google' as const,
    mode: 'create' as const,
    values: {
      name: '',
      clientId: '',
      clientSecret: '',
      redirectUri: 'https://id.acme.io/oauth/callback/google',
      scopes: '',
    },
    secretReplacing: false,
    hasStoredSecret: false,
    vendorDisplayName: 'Google Login',
    onFieldChange: vi.fn(),
    onSecretReplacingChange: vi.fn(),
  };

  it('shows field hints by default and replaces them with validation errors after blur', () => {
    render(<ConnectionForm {...baseProps} />);

    expect(screen.getByText('OAuth2 client identifier used for authentication.')).toBeInTheDocument();
    expect(screen.getByText('OAuth2 client secret issued by your identity provider.')).toBeInTheDocument();

    fireEvent.blur(getConnectionField('clientId'));

    expect(screen.getByText('This field is required.')).toBeInTheDocument();
    expect(screen.queryByText('OAuth2 client identifier used for authentication.')).not.toBeInTheDocument();
  });

  it('reports field edits through onFieldChange', () => {
    const onFieldChange = vi.fn();
    render(<ConnectionForm {...baseProps} onFieldChange={onFieldChange} />);

    fireEvent.change(getConnectionField('clientId'), {
      target: {value: 'my-client-id'},
    });

    expect(onFieldChange).toHaveBeenCalledWith('clientId', 'my-client-id');
  });

  it('does not render the redirect URI or scopes fields on create (moved to a create hint / edit view)', () => {
    render(<ConnectionForm {...baseProps} />);

    expect(document.getElementById('connection-field-redirectUri')).not.toBeInTheDocument();
    expect(document.getElementById('connection-field-scopes')).not.toBeInTheDocument();
  });

  it('renders the redirect URI as a read-only copy field and the scopes field on edit', () => {
    render(<ConnectionForm {...baseProps} mode="edit" hasStoredSecret />);

    const field = getConnectionField('redirectUri') as HTMLInputElement;
    expect(field).toHaveValue('https://id.acme.io/oauth/callback/google');
    expect(field).toHaveAttribute('readonly');
    expect(screen.getByTestId('connection-field-redirectUri-copy')).toBeInTheDocument();
    expect(screen.getByText('Add this exact URI to your Google Login OAuth client.')).toBeInTheDocument();
    expect(screen.getByText(/Space-separated scopes to request during sign-in\. Defaults to/)).toBeInTheDocument();
  });

  it('shows the GitHub scopes default instead of the OIDC scopes default', () => {
    render(
      <ConnectionForm {...baseProps} type="github" mode="edit" hasStoredSecret vendorDisplayName="GitHub Login" />,
    );

    expect(getConnectionField('scopes')).toHaveAttribute('placeholder', 'user:email');
    const helperText = document.getElementById('connection-field-scopes-helper-text');
    expect(helperText).toHaveTextContent(
      'Space-separated scopes to request during sign-in. Defaults to user:email if not set.',
    );
    expect(helperText).not.toHaveTextContent('openid email profile');
  });

  describe('OIDC create', () => {
    const oidcCreateProps = {
      ...baseProps,
      type: 'oidc' as const,
      values: {
        name: '',
        clientId: '',
        clientSecret: '',
        authorizationEndpoint: '',
        tokenEndpoint: '',
        redirectUri: 'https://id.acme.io/oauth/callback/oidc',
      },
    };

    it('renders only the required fields and no Federation section', () => {
      render(<ConnectionForm {...oidcCreateProps} />);

      expect(getConnectionField('clientId')).toBeInTheDocument();
      expect(getConnectionField('clientSecret')).toBeInTheDocument();
      expect(getConnectionField('authorizationEndpoint')).toBeInTheDocument();
      expect(getConnectionField('tokenEndpoint')).toBeInTheDocument();
      expect(document.getElementById('connection-field-userInfoEndpoint')).not.toBeInTheDocument();
      expect(document.getElementById('connection-field-issuer')).not.toBeInTheDocument();
      expect(document.getElementById('connection-field-jwksEndpoint')).not.toBeInTheDocument();
      expect(document.getElementById('connection-field-redirectUri')).not.toBeInTheDocument();
      expect(document.getElementById('connection-field-scopes')).not.toBeInTheDocument();
      expect(screen.queryByRole('switch', {name: 'Enable token exchange'})).not.toBeInTheDocument();
      expect(screen.queryByRole('heading', {name: 'Federation'})).not.toBeInTheDocument();
    });
  });

  describe('OIDC edit federation fields', () => {
    const oidcEditProps = {
      ...baseProps,
      type: 'oidc' as const,
      mode: 'edit' as const,
      hasStoredSecret: true,
      values: {
        name: '',
        clientId: '',
        clientSecret: '',
        authorizationEndpoint: '',
        tokenEndpoint: '',
        issuer: '',
        userInfoEndpoint: '',
        jwksEndpoint: '',
        redirectUri: 'https://id.acme.io/oauth/callback/oidc',
        scopes: '',
        tokenExchangeEnabled: 'false',
        trustedTokenAudience: '',
      },
    };

    it('renders the Federation section heading above the tokenExchangeEnabled field', () => {
      render(<ConnectionForm {...oidcEditProps} />);

      expect(screen.getByRole('heading', {name: 'Federation'})).toBeInTheDocument();
    });

    it('renders a switch for the tokenExchangeEnabled field', () => {
      render(<ConnectionForm {...oidcEditProps} />);

      const toggle = screen.getByRole('switch', {name: 'Enable token exchange'});
      expect(toggle).toBeInTheDocument();
      expect(toggle).not.toBeChecked();
    });

    it('reports the switch toggle through onFieldChange as a "true"/"false" string', () => {
      const onFieldChange = vi.fn();
      render(<ConnectionForm {...oidcEditProps} onFieldChange={onFieldChange} />);

      const toggle = screen.getByRole('switch', {name: 'Enable token exchange'});
      fireEvent.click(toggle);

      expect(onFieldChange).toHaveBeenCalledWith('tokenExchangeEnabled', 'true');
    });

    it('hides trustedTokenAudience when tokenExchangeEnabled is off', () => {
      render(<ConnectionForm {...oidcEditProps} />);

      expect(document.getElementById('connection-field-trustedTokenAudience')).not.toBeInTheDocument();
    });

    it('shows trustedTokenAudience when tokenExchangeEnabled is on', () => {
      render(<ConnectionForm {...oidcEditProps} values={{...oidcEditProps.values, tokenExchangeEnabled: 'true'}} />);

      expect(document.getElementById('connection-field-trustedTokenAudience')).toBeInTheDocument();
    });

    it('does not mark issuer/jwksEndpoint required when tokenExchangeEnabled is off', () => {
      render(<ConnectionForm {...oidcEditProps} />);

      expect(isFieldMarkedRequired('issuer')).toBe(false);
      expect(isFieldMarkedRequired('jwksEndpoint')).toBe(false);
    });

    it('marks issuer/jwksEndpoint required when tokenExchangeEnabled is on', () => {
      render(<ConnectionForm {...oidcEditProps} values={{...oidcEditProps.values, tokenExchangeEnabled: 'true'}} />);

      expect(isFieldMarkedRequired('issuer')).toBe(true);
      expect(isFieldMarkedRequired('jwksEndpoint')).toBe(true);
    });
  });

  describe('SMS gateway create', () => {
    const smsGatewayProps = {
      ...baseProps,
      type: 'sms-gateway' as const,
      vendorDisplayName: 'SMS Gateway',
      values: {
        name: '',
        url: '',
        httpMethod: 'POST',
        contentType: 'JSON',
        httpHeaders: '',
      },
    };

    it('renders the gateway fields, with the transport options as selects', () => {
      render(<ConnectionForm {...smsGatewayProps} />);

      expect(getConnectionField('url')).toBeInTheDocument();
      expect(screen.getByTestId('connection-field-httpHeaders-rows')).toBeInTheDocument();
      expect(screen.getByTestId('connection-field-select-httpMethod')).toHaveTextContent('POST');
      expect(screen.getByTestId('connection-field-select-contentType')).toHaveTextContent('JSON');
      expect(isFieldMarkedRequired('url')).toBe(true);
    });

    it('marks only the gateway URL required, leaving the defaulted transport fields unmarked', () => {
      render(<ConnectionForm {...smsGatewayProps} showNameField />);

      expect(isFieldMarkedRequired('name')).toBe(true);
      expect(isFieldMarkedRequired('url')).toBe(true);
      expect(isFieldMarkedRequired('httpMethod')).toBe(false);
      expect(isFieldMarkedRequired('contentType')).toBe(false);
      expect(isFieldMarkedRequired('httpHeaders')).toBe(false);
    });

    it('reports a select change through onFieldChange', async () => {
      const onFieldChange = vi.fn();
      render(<ConnectionForm {...smsGatewayProps} onFieldChange={onFieldChange} />);

      fireEvent.mouseDown(getConnectionField('contentType'));
      fireEvent.click(await screen.findByRole('option', {name: 'FORM'}));

      expect(onFieldChange).toHaveBeenCalledWith('contentType', 'FORM');
    });

    it('renders one header row per stored pair, plus no blank row', () => {
      render(
        <ConnectionForm
          {...smsGatewayProps}
          values={{...smsGatewayProps.values, httpHeaders: 'X-API-Key: abc123, Accept: application/json'}}
        />,
      );

      expect(getConnectionField('httpHeaders-name-1')).toHaveValue('X-API-Key');
      expect(getConnectionField('httpHeaders-value-1')).toHaveValue('abc123');
      expect(getConnectionField('httpHeaders-name-2')).toHaveValue('Accept');
      expect(getConnectionField('httpHeaders-value-2')).toHaveValue('application/json');
      expect(document.getElementById('connection-field-httpHeaders-name-3')).not.toBeInTheDocument();
    });

    it('starts with a single blank row when no headers are stored', () => {
      render(<ConnectionForm {...smsGatewayProps} />);

      expect(getConnectionField('httpHeaders-name-1')).toHaveValue('');
      expect(document.getElementById('connection-field-httpHeaders-name-2')).not.toBeInTheDocument();
      expect(screen.getByTestId('connection-field-httpHeaders-add')).toBeDisabled();
    });

    it('serializes edited rows into the stored comma-separated format', () => {
      const onFieldChange = vi.fn();
      render(<ControlledConnectionForm {...smsGatewayProps} onFieldChange={onFieldChange} />);

      fireEvent.change(getConnectionField('httpHeaders-name-1'), {target: {value: 'X-API-Key'}});
      fireEvent.change(getConnectionField('httpHeaders-value-1'), {target: {value: 'abc123'}});

      expect(onFieldChange).toHaveBeenLastCalledWith('httpHeaders', 'X-API-Key: abc123');

      fireEvent.click(screen.getByTestId('connection-field-httpHeaders-add'));
      fireEvent.change(getConnectionField('httpHeaders-name-2'), {target: {value: 'Accept'}});
      fireEvent.change(getConnectionField('httpHeaders-value-2'), {target: {value: 'application/json'}});

      expect(onFieldChange).toHaveBeenLastCalledWith('httpHeaders', 'X-API-Key: abc123, Accept: application/json');
    });

    it('removes a header row and re-serializes without it', () => {
      const onFieldChange = vi.fn();
      render(
        <ControlledConnectionForm
          {...smsGatewayProps}
          values={{...smsGatewayProps.values, httpHeaders: 'X-API-Key: abc123, Accept: application/json'}}
          onFieldChange={onFieldChange}
        />,
      );

      fireEvent.click(screen.getByTestId('connection-field-httpHeaders-remove-1'));

      expect(onFieldChange).toHaveBeenLastCalledWith('httpHeaders', 'Accept: application/json');
      expect(getConnectionField('httpHeaders-name-1')).toHaveValue('Accept');
    });

    it('omits a header row whose value is empty, keeping the row on screen to finish', () => {
      const onFieldChange = vi.fn();
      render(<ControlledConnectionForm {...smsGatewayProps} onFieldChange={onFieldChange} />);

      fireEvent.change(getConnectionField('httpHeaders-name-1'), {target: {value: 'Content-Type'}});
      fireEvent.change(getConnectionField('httpHeaders-value-1'), {target: {value: 'application/json'}});
      fireEvent.click(screen.getByTestId('connection-field-httpHeaders-add'));
      fireEvent.change(getConnectionField('httpHeaders-name-2'), {target: {value: 'Accept'}});

      expect(onFieldChange).toHaveBeenLastCalledWith('httpHeaders', 'Content-Type: application/json');
      expect(getConnectionField('httpHeaders-name-2')).toHaveValue('Accept');

      fireEvent.change(getConnectionField('httpHeaders-value-2'), {target: {value: 'text/plain'}});

      expect(onFieldChange).toHaveBeenLastCalledWith(
        'httpHeaders',
        'Content-Type: application/json, Accept: text/plain',
      );
    });

    it('re-derives rows when the value changes outside the editor, as the detail page Reset does', () => {
      const {rerender} = render(
        <ConnectionForm {...smsGatewayProps} values={{...smsGatewayProps.values, httpHeaders: 'X-API-Key: abc123'}} />,
      );
      expect(getConnectionField('httpHeaders-name-1')).toHaveValue('X-API-Key');

      rerender(<ConnectionForm {...smsGatewayProps} values={{...smsGatewayProps.values, httpHeaders: ''}} />);

      expect(getConnectionField('httpHeaders-name-1')).toHaveValue('');
      expect(document.getElementById('connection-field-httpHeaders-name-2')).not.toBeInTheDocument();
    });

    it('strips characters the stored format cannot represent', () => {
      const onFieldChange = vi.fn();
      render(<ControlledConnectionForm {...smsGatewayProps} onFieldChange={onFieldChange} />);

      fireEvent.change(getConnectionField('httpHeaders-name-1'), {target: {value: 'X-A:B,C'}});
      fireEvent.change(getConnectionField('httpHeaders-value-1'), {target: {value: 'text/html, application/json'}});

      expect(getConnectionField('httpHeaders-name-1')).toHaveValue('X-ABC');
      expect(onFieldChange).toHaveBeenLastCalledWith('httpHeaders', 'X-ABC: text/html application/json');
    });
  });
});
