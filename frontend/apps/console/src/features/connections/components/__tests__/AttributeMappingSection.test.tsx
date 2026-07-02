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

import {fireEvent, render, screen} from '@testing-library/react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {AttributeConfiguration} from '../../models/connection';
import AttributeMappingSection from '../AttributeMappingSection';

vi.mock('react-i18next', () => ({useTranslation: () => ({t: (key: string) => key})}));

vi.mock('@thunderid/configure-user-types', () => ({
  useGetUserTypes: () => ({data: {types: [{id: 'u1', name: 'Person'}]}}),
  useGetUserType: () => ({data: {schema: {firstName: {type: 'string'}}}}),
}));

describe('AttributeMappingSection', () => {
  const onChange = vi.fn();
  beforeEach(() => vi.clearAllMocks());

  it('renders the section with an add-mapping button', () => {
    render(<AttributeMappingSection onChange={onChange} />);
    expect(screen.getByTestId('attribute-mapping-section')).toBeInTheDocument();
    expect(screen.getByTestId('attribute-mapping-add')).toBeInTheDocument();
  });

  it('emits the existing config and a valid state when seeded (edit prefill)', () => {
    const initial: AttributeConfiguration = {
      userTypeResolution: {default: 'Person'},
      userTypeAttributeMappings: [
        {userType: 'Person', attributes: [{externalAttribute: 'given_name', localAttribute: 'firstName'}]},
      ],
    };
    render(<AttributeMappingSection initialConfig={initial} onChange={onChange} />);
    expect(onChange).toHaveBeenLastCalledWith(initial, true);
  });

  it('adds a row and reports invalid when mappings exist without a user type', () => {
    render(<AttributeMappingSection onChange={onChange} />);
    // Mount with no state → undefined config, valid.
    expect(onChange).toHaveBeenLastCalledWith(undefined, true);

    fireEvent.click(screen.getByTestId('attribute-mapping-add'));
    fireEvent.change(screen.getByLabelText('attributeMapping.externalAttribute.label'), {
      target: {value: 'given_name'},
    });

    expect(onChange).toHaveBeenLastCalledWith(undefined, false);
  });
});
