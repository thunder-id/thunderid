// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, within} from '@testing-library/react';
import {useState} from 'react';
import {describe, expect, it, vi} from 'vitest';
import {AuthenticationMethods} from '../authentication-methods';
import type {ServiceAuthenticationErrors} from '../outboundAuthentication';
import ServiceAuthenticationMethods from '../ServiceAuthenticationMethods';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string) => fallback ?? key,
  }),
}));

function StoredAPIKeyHeaders(): ReturnType<typeof ServiceAuthenticationMethods> {
  const [headers, setHeaders] = useState('X-API-Key: ******');
  return (
    <ServiceAuthenticationMethods
      method={AuthenticationMethods.API_KEY}
      bearerToken=""
      basicUsername=""
      basicPassword=""
      httpHeaders={headers}
      hasStoredHTTPHeaders
      storedHTTPHeaders="X-API-Key: ******"
      hasStoredSecret={false}
      replacing={false}
      onChange={(_name, value) => setHeaders(value)}
      onReplacingChange={vi.fn()}
    />
  );
}

/** API key headers being edited, as a form holding them would. */
function APIKeyHeaders({
  initial,
  stored = '',
  hasStored = false,
  errors = {},
}: {
  initial: string;
  stored?: string;
  hasStored?: boolean;
  errors?: ServiceAuthenticationErrors;
}): ReturnType<typeof ServiceAuthenticationMethods> {
  const [headers, setHeaders] = useState(initial);
  return (
    <>
      <ServiceAuthenticationMethods
        method={AuthenticationMethods.API_KEY}
        bearerToken=""
        basicUsername=""
        basicPassword=""
        httpHeaders={headers}
        hasStoredHTTPHeaders={hasStored}
        storedHTTPHeaders={stored}
        hasStoredSecret={false}
        replacing={false}
        errors={errors}
        onChange={(_name, value) => setHeaders(value)}
        onReplacingChange={vi.fn()}
      />
      <output data-testid="headers">{headers}</output>
    </>
  );
}

/** A stored secret of the method, with the form values it is edited through. */
function StoredSecret({method}: {method: typeof AuthenticationMethods.BEARER | typeof AuthenticationMethods.BASIC}) {
  const [replacing, setReplacing] = useState(false);
  const [values, setValues] = useState<Record<string, string>>({
    bearerToken: '',
    basicUsername: 'svc',
    basicPassword: '',
  });
  return (
    <>
      <ServiceAuthenticationMethods
        method={method}
        bearerToken={values['bearerToken']}
        basicUsername={values['basicUsername']}
        basicPassword={values['basicPassword']}
        httpHeaders=""
        hasStoredHTTPHeaders={false}
        storedHTTPHeaders=""
        hasStoredSecret
        replacing={replacing}
        onChange={(name, value) => setValues((previous) => ({...previous, [name]: value}))}
        onReplacingChange={setReplacing}
      />
      <output data-testid="values">{JSON.stringify(values)}</output>
    </>
  );
}

function StoredBasicCredentials({onChange = vi.fn()}: {onChange?: (name: string, value: string) => void}) {
  const [replacing, setReplacing] = useState(false);

  return (
    <ServiceAuthenticationMethods
      method={AuthenticationMethods.BASIC}
      bearerToken=""
      basicUsername="test-user"
      basicPassword=""
      httpHeaders=""
      hasStoredHTTPHeaders={false}
      storedHTTPHeaders=""
      hasStoredSecret
      replacing={replacing}
      onChange={onChange}
      onReplacingChange={setReplacing}
    />
  );
}

describe('ServiceAuthenticationMethods', () => {
  it('reports authentication method changes', () => {
    const onChange = vi.fn();
    render(
      <ServiceAuthenticationMethods
        method={AuthenticationMethods.NONE}
        bearerToken=""
        basicUsername=""
        basicPassword=""
        httpHeaders=""
        hasStoredHTTPHeaders={false}
        storedHTTPHeaders=""
        hasStoredSecret={false}
        replacing={false}
        onChange={onChange}
        onReplacingChange={vi.fn()}
      />,
    );

    fireEvent.mouseDown(screen.getByRole('combobox'));
    fireEvent.click(screen.getByRole('option', {name: 'Bearer token'}));

    expect(onChange).toHaveBeenCalledWith('authenticationScheme', AuthenticationMethods.BEARER);
  });

  it('reports a replacement bearer token', () => {
    const onChange = vi.fn();
    const onReplacingChange = vi.fn();
    render(
      <ServiceAuthenticationMethods
        method={AuthenticationMethods.BEARER}
        bearerToken=""
        basicUsername=""
        basicPassword=""
        httpHeaders=""
        hasStoredHTTPHeaders={false}
        storedHTTPHeaders=""
        hasStoredSecret={false}
        replacing={false}
        onChange={onChange}
        onReplacingChange={onReplacingChange}
      />,
    );

    fireEvent.change(document.getElementById('connection-field-bearerToken')!, {target: {value: 'token'}});

    expect(onReplacingChange).not.toHaveBeenCalled();
    expect(onChange).toHaveBeenCalledWith('bearerToken', 'token');
  });

  it('reports replacement Basic credentials', () => {
    const onChange = vi.fn();
    const onReplacingChange = vi.fn();
    render(
      <ServiceAuthenticationMethods
        method={AuthenticationMethods.BASIC}
        bearerToken=""
        basicUsername=""
        basicPassword=""
        httpHeaders=""
        hasStoredHTTPHeaders={false}
        storedHTTPHeaders=""
        hasStoredSecret={false}
        replacing={false}
        onChange={onChange}
        onReplacingChange={onReplacingChange}
      />,
    );

    fireEvent.change(document.getElementById('connection-field-basicUsername')!, {target: {value: 'user'}});
    fireEvent.change(document.getElementById('connection-field-basicPassword')!, {target: {value: 'password'}});

    expect(onReplacingChange).not.toHaveBeenCalled();
    expect(onChange).toHaveBeenNthCalledWith(1, 'basicUsername', 'user');
    expect(onChange).toHaveBeenNthCalledWith(2, 'basicPassword', 'password');
  });

  it('shows validation errors on missing Basic credentials', () => {
    render(
      <ServiceAuthenticationMethods
        method={AuthenticationMethods.BASIC}
        bearerToken=""
        basicUsername=""
        basicPassword=""
        httpHeaders=""
        hasStoredHTTPHeaders={false}
        storedHTTPHeaders=""
        hasStoredSecret={false}
        replacing={false}
        errors={{
          basicUsername: 'Username is required.',
          basicPassword: 'Password is required.',
        }}
        onChange={vi.fn()}
        onReplacingChange={vi.fn()}
      />,
    );

    expect(screen.getByText('Username is required.')).toBeInTheDocument();
    expect(screen.getByText('Password is required.')).toBeInTheDocument();
  });

  it('reports API-key header changes', () => {
    const onChange = vi.fn();
    function APIKeyEditor(): ReturnType<typeof ServiceAuthenticationMethods> {
      const [headers, setHeaders] = useState('');
      return (
        <ServiceAuthenticationMethods
          method={AuthenticationMethods.API_KEY}
          bearerToken=""
          basicUsername=""
          basicPassword=""
          httpHeaders={headers}
          hasStoredHTTPHeaders={false}
          storedHTTPHeaders=""
          hasStoredSecret={false}
          replacing={false}
          onChange={(name, value) => {
            onChange(name, value);
            setHeaders(value);
          }}
          onReplacingChange={vi.fn()}
        />
      );
    }
    render(<APIKeyEditor />);

    fireEvent.change(document.getElementById('connection-field-httpHeaders-name-1')!, {
      target: {value: 'X-API-Key'},
    });
    fireEvent.change(document.getElementById('connection-field-httpHeaders-value-1')!, {
      target: {value: 'secret'},
    });

    expect(onChange).toHaveBeenLastCalledWith('httpHeaders', 'X-API-Key: secret');
    expect(document.getElementById('connection-field-httpHeaders-value-1')).toHaveAttribute('type', 'password');

    fireEvent.click(screen.getByRole('button', {name: 'Show value'}));
    expect(document.getElementById('connection-field-httpHeaders-value-1')).toHaveAttribute('type', 'text');

    fireEvent.click(screen.getByRole('button', {name: 'Hide value'}));
    expect(document.getElementById('connection-field-httpHeaders-value-1')).toHaveAttribute('type', 'password');
  });

  it('keeps each saved API-key header masked until its row is updated', () => {
    render(<StoredAPIKeyHeaders />);

    expect(document.getElementById('connection-field-httpHeaders-name-1')).toHaveValue('X-API-Key');
    expect(document.getElementById('connection-field-httpHeaders-value-1')).toHaveValue('••••••••••••••••');
    expect(document.getElementById('connection-field-httpHeaders-value-1')).toBeDisabled();

    fireEvent.click(screen.getByTestId('connection-field-httpHeaders-update-1'));

    expect(document.getElementById('connection-field-httpHeaders-value-1')).not.toBeDisabled();
  });

  it('shows the saved Basic username while keeping the password masked', () => {
    const onChange = vi.fn();
    render(<StoredBasicCredentials onChange={onChange} />);

    const credentialFields = screen.getByTestId('basic-credentials-fields');
    expect(credentialFields).toContainElement(document.getElementById('connection-field-basicUsername'));
    expect(credentialFields).toContainElement(document.getElementById('connection-field-basicPassword'));
    expect(credentialFields).toContainElement(screen.getByRole('button', {name: 'Update'}));
    expect(document.getElementById('connection-field-basicUsername')).toHaveValue('test-user');
    expect(document.getElementById('connection-field-basicUsername')).toBeDisabled();
    expect(document.getElementById('connection-field-basicPassword')).toBeDisabled();
    expect(document.getElementById('connection-field-basicPassword')).toHaveValue('••••••••••••••••');
    expect(screen.queryByText('Leave unchanged to keep the stored secret.')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', {name: 'Update'}));

    expect(document.getElementById('connection-field-basicUsername')).not.toBeDisabled();
    expect(document.getElementById('connection-field-basicUsername')).toHaveValue('test-user');
    expect(document.getElementById('connection-field-basicPassword')).not.toBeDisabled();
    expect(screen.getByText('Username sent with each request.')).toBeInTheDocument();

    fireEvent.change(document.getElementById('connection-field-basicUsername')!, {target: {value: 'new-user'}});
    expect(onChange).toHaveBeenCalledWith('basicUsername', 'new-user');
  });
  it('restores the stored API key headers when API key is chosen again', () => {
    const onChange = vi.fn();
    render(
      <ServiceAuthenticationMethods
        method={AuthenticationMethods.BEARER}
        bearerToken=""
        basicUsername=""
        basicPassword=""
        httpHeaders=""
        hasStoredHTTPHeaders={false}
        storedHTTPHeaders="X-API-Key: ******"
        hasStoredSecret={false}
        replacing={false}
        onChange={onChange}
        onReplacingChange={vi.fn()}
      />,
    );

    fireEvent.mouseDown(screen.getByRole('combobox'));
    fireEvent.click(screen.getByRole('option', {name: 'API key'}));

    expect(onChange).toHaveBeenNthCalledWith(1, 'authenticationScheme', AuthenticationMethods.API_KEY);
    expect(onChange).toHaveBeenNthCalledWith(2, 'httpHeaders', 'X-API-Key: ******');
  });

  it('marks the bearer token as required', () => {
    render(
      <ServiceAuthenticationMethods
        method={AuthenticationMethods.BEARER}
        bearerToken=""
        basicUsername=""
        basicPassword=""
        httpHeaders=""
        hasStoredHTTPHeaders={false}
        storedHTTPHeaders=""
        hasStoredSecret={false}
        replacing={false}
        onChange={vi.fn()}
        onReplacingChange={vi.fn()}
      />,
    );

    expect(document.querySelector('label[for="connection-field-bearerToken"]')).toHaveTextContent('*');
  });

  it('cancels replacing a stored bearer token, and moves focus with Update and Cancel', () => {
    render(<StoredSecret method={AuthenticationMethods.BEARER} />);

    fireEvent.click(screen.getByTestId('connection-field-bearerToken-replace'));
    expect(document.getElementById('connection-field-bearerToken')).toHaveFocus();
    fireEvent.change(document.getElementById('connection-field-bearerToken')!, {target: {value: 'typed'}});

    fireEvent.click(screen.getByTestId('connection-field-bearerToken-cancel'));

    expect(screen.getByTestId('connection-field-bearerToken-replace')).toHaveFocus();
    expect(document.getElementById('connection-field-bearerToken')).toBeDisabled();
    expect(JSON.parse(screen.getByTestId('values').textContent ?? '{}')).toMatchObject({bearerToken: ''});
  });

  it('cancels replacing stored Basic credentials, and moves focus with Update and Cancel', () => {
    render(<StoredSecret method={AuthenticationMethods.BASIC} />);

    fireEvent.click(screen.getByRole('button', {name: 'Update'}));
    expect(document.getElementById('connection-field-basicUsername')).toHaveFocus();
    fireEvent.change(document.getElementById('connection-field-basicUsername')!, {target: {value: 'other'}});
    fireEvent.change(document.getElementById('connection-field-basicPassword')!, {target: {value: 'pw'}});

    fireEvent.click(screen.getByTestId('basic-credentials-cancel'));

    expect(screen.getByRole('button', {name: 'Update'})).toHaveFocus();
    expect(document.getElementById('connection-field-basicUsername')).toBeDisabled();
    expect(JSON.parse(screen.getByTestId('values').textContent ?? '{}')).toMatchObject({
      basicUsername: 'svc',
      basicPassword: '',
    });
  });

  it('shows a wrong API key header on its row as soon as it is entered', () => {
    render(<APIKeyHeaders initial="" />);

    fireEvent.change(document.getElementById('connection-field-httpHeaders-name-1')!, {target: {value: 'Host'}});

    expect(screen.getByTestId('connection-field-httpHeaders-error-1')).toHaveTextContent(
      'This header is reserved. Use another name.',
    );
    expect(document.getElementById('connection-field-httpHeaders-name-1')).toHaveAttribute('aria-invalid', 'true');
    expect(document.getElementById('connection-field-httpHeaders-value-1')).toHaveAttribute('aria-invalid', 'false');
  });

  it.each([
    ['X API Key', 'A header name can contain only letters'],
    ['x-api-key', 'This header is already added. Header names are not case sensitive.'],
  ])('explains why the API key header %s is refused', (name, message) => {
    render(<APIKeyHeaders initial="X-API-Key: one" />);

    fireEvent.click(screen.getByTestId('connection-field-httpHeaders-add'));
    fireEvent.change(document.getElementById('connection-field-httpHeaders-name-2')!, {target: {value: name}});
    fireEvent.change(document.getElementById('connection-field-httpHeaders-value-2')!, {target: {value: 'two'}});

    expect(screen.getByTestId('connection-field-httpHeaders-error-2')).toHaveTextContent(message);
    expect(screen.queryByTestId('connection-field-httpHeaders-error-1')).not.toBeInTheDocument();
  });

  it('shows a missing header value only once errors are shown', () => {
    const {rerender} = render(<APIKeyHeaders initial="X-API-Key:" />);
    expect(screen.queryByTestId('connection-field-httpHeaders-error-1')).not.toBeInTheDocument();

    rerender(<APIKeyHeaders initial="X-API-Key:" errors={{httpHeaders: 'This field is required.'}} />);

    expect(screen.getByTestId('connection-field-httpHeaders-error-1')).toHaveTextContent('Enter a value.');
    expect(screen.queryByText('This field is required.')).not.toBeInTheDocument();
  });

  it('asks again for a stored header value that can no longer be kept', () => {
    render(<APIKeyHeaders initial="X-API-Key: ******" stored="X-API-Key: ******" />);

    const value = document.getElementById('connection-field-httpHeaders-value-1')!;
    expect(value).not.toBeDisabled();
    expect(value).toHaveValue('');
    expect(screen.getByTestId('connection-field-httpHeaders-error-1')).toHaveTextContent(
      "Enter this header's value again.",
    );

    fireEvent.change(value, {target: {value: 'fresh'}});

    expect(screen.getByTestId('headers')).toHaveTextContent('X-API-Key: fresh');
    expect(screen.queryByTestId('connection-field-httpHeaders-error-1')).not.toBeInTheDocument();
  });

  it('hides the value toggle of a locked stored header', () => {
    render(<APIKeyHeaders initial="X-API-Key: ******" stored="X-API-Key: ******" hasStored />);

    expect(screen.queryByRole('button', {name: 'Show value'})).not.toBeInTheDocument();

    fireEvent.click(screen.getByTestId('connection-field-httpHeaders-update-1'));

    expect(screen.getByRole('button', {name: 'Show value'})).toBeInTheDocument();
  });

  it('moves focus to the header value on Update and back to Update on Cancel', () => {
    render(<APIKeyHeaders initial="X-API-Key: ******" stored="X-API-Key: ******" hasStored />);

    fireEvent.click(screen.getByTestId('connection-field-httpHeaders-update-1'));
    expect(document.getElementById('connection-field-httpHeaders-value-1')).toHaveFocus();

    fireEvent.click(screen.getByTestId('connection-field-httpHeaders-cancel-1'));
    expect(screen.getByTestId('connection-field-httpHeaders-update-1')).toHaveFocus();
    expect(screen.getByTestId('headers')).toHaveTextContent('X-API-Key: ******');
  });

  it('lines up the inputs of stored and new header rows', () => {
    render(<APIKeyHeaders initial="X-API-Key: ******" stored="X-API-Key: ******" hasStored />);

    fireEvent.click(screen.getByTestId('connection-field-httpHeaders-add'));

    const width = (elementId: string): number =>
      document.getElementById(elementId)!.closest('.MuiTextField-root')!.getBoundingClientRect().width;
    expect(width('connection-field-httpHeaders-name-2')).toBe(width('connection-field-httpHeaders-name-1'));
    expect(width('connection-field-httpHeaders-value-2')).toBe(width('connection-field-httpHeaders-value-1'));
    expect(
      within(screen.getByTestId('connection-field-httpHeaders-rows')).getAllByRole('button', {name: 'Remove'}),
    ).toHaveLength(2);
  });
});
