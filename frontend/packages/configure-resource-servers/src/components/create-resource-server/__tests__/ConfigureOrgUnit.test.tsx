/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {render, screen} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import ConfigureOrgUnit from '../ConfigureOrgUnit';

vi.mock('@thunderid/configure-organization-units', () => ({
  OrganizationUnitTreePicker: () => <div data-testid="ou-tree-picker" />,
}));

describe('ConfigureOrgUnit', () => {
  it('renders the resource server subtitle when selectedType is not provided', () => {
    render(<ConfigureOrgUnit selectedOuId="" onOuIdChange={vi.fn()} />);

    expect(screen.getByText('Select which organization unit this resource server belongs to.')).toBeInTheDocument();
  });

  it('renders the resource server subtitle when selectedType is API', () => {
    render(<ConfigureOrgUnit selectedOuId="" selectedType="API" onOuIdChange={vi.fn()} />);

    expect(screen.getByText('Select which organization unit this resource server belongs to.')).toBeInTheDocument();
  });

  it('renders the MCP server subtitle when selectedType is MCP', () => {
    render(<ConfigureOrgUnit selectedOuId="" selectedType="MCP" onOuIdChange={vi.fn()} />);

    expect(screen.getByText('Select which organization unit this MCP server belongs to.')).toBeInTheDocument();
  });

  it('renders the organization unit tree picker', () => {
    render(<ConfigureOrgUnit selectedOuId="" onOuIdChange={vi.fn()} />);

    expect(screen.getByTestId('ou-tree-picker')).toBeInTheDocument();
  });

  it('calls onReadyChange with true when selectedOuId is non-empty', () => {
    const onReadyChange = vi.fn();
    render(<ConfigureOrgUnit selectedOuId="ou-1" onOuIdChange={vi.fn()} onReadyChange={onReadyChange} />);

    expect(onReadyChange).toHaveBeenCalledWith(true);
  });

  it('calls onReadyChange with false when selectedOuId is empty', () => {
    const onReadyChange = vi.fn();
    render(<ConfigureOrgUnit selectedOuId="" onOuIdChange={vi.fn()} onReadyChange={onReadyChange} />);

    expect(onReadyChange).toHaveBeenCalledWith(false);
  });
});
