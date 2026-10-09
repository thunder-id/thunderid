// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@thunderid/test-utils';
import {useState} from 'react';
import {describe, expect, it, vi} from 'vitest';
import {AuthenticationMethods} from '../../models/authentication-methods';
import ServiceAuthenticationMethods from '../ServiceAuthenticationMethods';

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
      hasStoredSecret={false}
      replacing={false}
      onChange={(_name, value) => setHeaders(value)}
      onReplacingChange={vi.fn()}
    />
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
    expect(screen.getByText('Username sent with each PDP request.')).toBeInTheDocument();

    fireEvent.change(document.getElementById('connection-field-basicUsername')!, {target: {value: 'new-user'}});
    expect(onChange).toHaveBeenCalledWith('basicUsername', 'new-user');
  });
});
