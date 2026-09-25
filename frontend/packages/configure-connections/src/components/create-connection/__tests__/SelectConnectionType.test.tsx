// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import SelectConnectionType from '../SelectConnectionType';

describe('SelectConnectionType', () => {
  const onSelect = vi.fn();
  beforeEach(() => vi.clearAllMocks());

  it('renders the OIDC, OAuth2, trusted-idp, SMS gateway, and SMTP options', () => {
    render(<SelectConnectionType selectedType={null} onSelect={onSelect} />);
    expect(screen.getByText('What kind of connection do you want to add?')).toBeInTheDocument();
    expect(screen.queryByText('Connection type')).not.toBeInTheDocument();
    expect(screen.getByTestId('connection-type-option-oidc')).toBeInTheDocument();
    expect(screen.getByTestId('connection-type-option-oauth')).toBeInTheDocument();
    expect(screen.getByTestId('connection-type-option-trusted-idp')).toBeInTheDocument();
    expect(screen.getByTestId('connection-type-option-sms-gateway')).toBeInTheDocument();
    expect(screen.getByTestId('connection-type-option-email-smtp')).toBeInTheDocument();
    expect(screen.getByText('Email Provider (SMTP)')).toBeInTheDocument();
  });

  it('selects the OIDC type when clicked', () => {
    render(<SelectConnectionType selectedType={null} onSelect={onSelect} />);
    fireEvent.click(screen.getByTestId('connection-type-option-oidc'));
    expect(onSelect).toHaveBeenCalledWith('oidc');
  });

  it('selects the OAuth2 type when clicked', () => {
    render(<SelectConnectionType selectedType={null} onSelect={onSelect} />);
    fireEvent.click(screen.getByTestId('connection-type-option-oauth'));
    expect(onSelect).toHaveBeenCalledWith('oauth');
  });

  it('selects the SMS gateway type when clicked', () => {
    render(<SelectConnectionType selectedType={null} onSelect={onSelect} />);
    fireEvent.click(screen.getByTestId('connection-type-option-sms-gateway'));
    expect(onSelect).toHaveBeenCalledWith('sms-gateway');
  });

  it('selects the SMTP type when clicked', () => {
    render(<SelectConnectionType selectedType={null} onSelect={onSelect} />);
    fireEvent.click(screen.getByTestId('connection-type-option-email-smtp'));
    expect(onSelect).toHaveBeenCalledWith('email-smtp');
  });

  it('selects the Trusted Token Issuer type when clicked', () => {
    render(<SelectConnectionType selectedType={null} onSelect={onSelect} />);
    fireEvent.click(screen.getByTestId('connection-type-option-trusted-idp'));
    expect(onSelect).toHaveBeenCalledWith('trusted-idp');
  });

  it('selects a type when Enter is pressed', () => {
    render(<SelectConnectionType selectedType={null} onSelect={onSelect} />);
    fireEvent.keyDown(screen.getByTestId('connection-type-option-oidc'), {key: 'Enter'});
    expect(onSelect).toHaveBeenCalledWith('oidc');
  });

  it('selects a type when Space is pressed', () => {
    render(<SelectConnectionType selectedType={null} onSelect={onSelect} />);
    fireEvent.keyDown(screen.getByTestId('connection-type-option-oidc'), {key: ' '});
    expect(onSelect).toHaveBeenCalledWith('oidc');
  });

  it('ignores unrelated key presses', () => {
    render(<SelectConnectionType selectedType={null} onSelect={onSelect} />);
    fireEvent.keyDown(screen.getByTestId('connection-type-option-oidc'), {key: 'a'});
    expect(onSelect).not.toHaveBeenCalled();
  });

  it('marks the selected type as pressed and shows the selection checkmark', () => {
    render(<SelectConnectionType selectedType="oidc" onSelect={onSelect} />);
    expect(screen.getByTestId('connection-type-option-oidc')).toHaveAttribute('aria-pressed', 'true');
    expect(screen.getByTestId('connection-type-option-oauth')).toHaveAttribute('aria-pressed', 'false');
  });

  it('renders the Trusted Token Issuer option before the SMS gateway option', () => {
    render(<SelectConnectionType selectedType={null} onSelect={onSelect} />);

    const optionIds = screen.getAllByTestId(/^connection-type-option-/).map((el) => el.getAttribute('data-testid'));
    expect(optionIds.indexOf('connection-type-option-trusted-idp')).toBeLessThan(
      optionIds.indexOf('connection-type-option-sms-gateway'),
    );
  });
});
