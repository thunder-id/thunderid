// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@thunderid/test-utils';
import {useState} from 'react';
import {describe, expect, it, vi} from 'vitest';
import {AuthenticationMethods} from '../../models/authentication-methods';
import ServiceAuthenticationMethods from '../ServiceAuthenticationMethods';

function StoredAPIKeyHeaders(): ReturnType<typeof ServiceAuthenticationMethods> {
  const [replacing, setReplacing] = useState(false);

  return (
    <ServiceAuthenticationMethods
      method={AuthenticationMethods.API_KEY}
      bearerToken=""
      basicUsername=""
      basicPassword=""
      httpHeaders=""
      hasStoredHTTPHeaders
      headersReplacing={replacing}
      hasStoredSecret={false}
      replacing={false}
      onChange={vi.fn()}
      onHeadersReplacingChange={setReplacing}
      onReplacingChange={vi.fn()}
    />
  );
}

function StoredBasicCredentials(): ReturnType<typeof ServiceAuthenticationMethods> {
  const [replacing, setReplacing] = useState(false);

  return (
    <ServiceAuthenticationMethods
      method={AuthenticationMethods.BASIC}
      bearerToken=""
      basicUsername=""
      basicPassword=""
      httpHeaders=""
      hasStoredHTTPHeaders={false}
      headersReplacing={false}
      hasStoredSecret
      replacing={replacing}
      onChange={vi.fn()}
      onHeadersReplacingChange={vi.fn()}
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
        headersReplacing={false}
        hasStoredSecret={false}
        replacing={false}
        onChange={onChange}
        onHeadersReplacingChange={vi.fn()}
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
        headersReplacing={false}
        hasStoredSecret={false}
        replacing={false}
        onChange={onChange}
        onHeadersReplacingChange={vi.fn()}
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
        headersReplacing={false}
        hasStoredSecret={false}
        replacing={false}
        onChange={onChange}
        onHeadersReplacingChange={vi.fn()}
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
        headersReplacing={false}
        hasStoredSecret={false}
        replacing={false}
        errors={{
          basicUsername: 'Username is required.',
          basicPassword: 'Password is required.',
        }}
        onChange={vi.fn()}
        onHeadersReplacingChange={vi.fn()}
        onReplacingChange={vi.fn()}
      />,
    );

    expect(screen.getByText('Username is required.')).toBeInTheDocument();
    expect(screen.getByText('Password is required.')).toBeInTheDocument();
  });

  it('reports API-key header changes', () => {
    const onChange = vi.fn();
    render(
      <ServiceAuthenticationMethods
        method={AuthenticationMethods.API_KEY}
        bearerToken=""
        basicUsername=""
        basicPassword=""
        httpHeaders=""
        hasStoredHTTPHeaders={false}
        headersReplacing={false}
        hasStoredSecret={false}
        replacing={false}
        onChange={onChange}
        onHeadersReplacingChange={vi.fn()}
        onReplacingChange={vi.fn()}
      />,
    );

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

  it('masks saved API-key headers until the user chooses to update them', () => {
    render(<StoredAPIKeyHeaders />);

    expect(screen.getAllByDisplayValue('••••••••••••••••')).toHaveLength(2);
    expect(document.getElementById('connection-field-httpHeaders-name-1')).not.toBeInTheDocument();

    fireEvent.click(screen.getByRole('button', {name: 'Update'}));

    expect(document.getElementById('connection-field-httpHeaders-name-1')).toBeInTheDocument();
    expect(screen.queryAllByDisplayValue('••••••••••••••••')).toHaveLength(0);
  });

  it('shows both Basic credentials when updating stored credentials', () => {
    render(<StoredBasicCredentials />);

    expect(document.getElementById('connection-field-basicUsername')).toBeDisabled();
    expect(document.getElementById('connection-field-basicPassword')).toBeDisabled();

    fireEvent.click(screen.getAllByRole('button', {name: 'Update'})[0]);

    expect(document.getElementById('connection-field-basicUsername')).not.toBeDisabled();
    expect(document.getElementById('connection-field-basicPassword')).not.toBeDisabled();
  });
});
