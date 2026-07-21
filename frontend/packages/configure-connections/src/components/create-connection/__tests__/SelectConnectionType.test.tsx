/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {render, screen, fireEvent} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import SelectConnectionType from '../SelectConnectionType';

describe('SelectConnectionType', () => {
  const onSelect = vi.fn();
  beforeEach(() => vi.clearAllMocks());

  it('renders the OIDC, OAuth2, and SMS gateway options without a trusted-idp card by default', () => {
    render(<SelectConnectionType selectedType={null} onSelect={onSelect} />);
    expect(screen.getByText('What kind of connection do you want to add?')).toBeInTheDocument();
    expect(screen.queryByText('Connection type')).not.toBeInTheDocument();
    expect(screen.getByTestId('connection-type-option-oidc')).toBeInTheDocument();
    expect(screen.getByTestId('connection-type-option-oauth')).toBeInTheDocument();
    expect(screen.getByTestId('connection-type-option-custom-sms')).toBeInTheDocument();
    expect(screen.queryByTestId('connection-type-option-trusted-idp')).not.toBeInTheDocument();
  });

  it('renders the Trusted Token Issuer option only when its key is in customTypes', () => {
    render(<SelectConnectionType selectedType={null} onSelect={onSelect} customTypes={['trusted-idp']} />);
    expect(screen.getByTestId('connection-type-option-trusted-idp')).toBeInTheDocument();
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

  it('does not select the disabled Custom SMS gateway option', () => {
    render(<SelectConnectionType selectedType={null} onSelect={onSelect} />);
    fireEvent.click(screen.getByTestId('connection-type-option-custom-sms'));
    expect(onSelect).not.toHaveBeenCalled();
  });

  it('selects the Trusted Token Issuer type when clicked', () => {
    render(<SelectConnectionType selectedType={null} onSelect={onSelect} customTypes={['trusted-idp']} />);
    fireEvent.click(screen.getByTestId('connection-type-option-trusted-idp'));
    expect(onSelect).toHaveBeenCalledWith('trusted-idp');
  });

  it('renders the Trusted Token Issuer option before the coming-soon SMS gateway option', () => {
    render(<SelectConnectionType selectedType={null} onSelect={onSelect} customTypes={['trusted-idp']} />);

    const optionIds = screen.getAllByTestId(/^connection-type-option-/).map((el) => el.getAttribute('data-testid'));
    expect(optionIds.indexOf('connection-type-option-trusted-idp')).toBeLessThan(
      optionIds.indexOf('connection-type-option-custom-sms'),
    );
  });
});
