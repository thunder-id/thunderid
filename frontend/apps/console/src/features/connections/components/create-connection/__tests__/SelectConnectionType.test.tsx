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

import {render, screen, fireEvent} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import SelectConnectionType from '../SelectConnectionType';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({t: (key: string) => key}),
}));

describe('SelectConnectionType', () => {
  const onSelect = vi.fn();
  beforeEach(() => vi.clearAllMocks());

  it('renders the OIDC and SMS gateway options', () => {
    render(<SelectConnectionType selectedType={null} onSelect={onSelect} />);
    expect(screen.getByText('wizard.type.heading')).toBeInTheDocument();
    expect(screen.queryByText('wizard.steps.type')).not.toBeInTheDocument();
    expect(screen.getByTestId('connection-type-option-oidc')).toBeInTheDocument();
    expect(screen.getByTestId('connection-type-option-custom-sms')).toBeInTheDocument();
  });

  it('selects the OIDC type when clicked', () => {
    render(<SelectConnectionType selectedType={null} onSelect={onSelect} />);
    fireEvent.click(screen.getByTestId('connection-type-option-oidc'));
    expect(onSelect).toHaveBeenCalledWith('oidc');
  });

  it('does not select the disabled Custom SMS gateway option', () => {
    render(<SelectConnectionType selectedType={null} onSelect={onSelect} />);
    fireEvent.click(screen.getByTestId('connection-type-option-custom-sms'));
    expect(onSelect).not.toHaveBeenCalled();
  });
});
