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

import {fireEvent, render, screen} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import ConnectionNameStep from '../ConnectionNameStep';

describe('ConnectionNameStep', () => {
  it('reports typed names through onNameChange', () => {
    const onNameChange = vi.fn();
    render(<ConnectionNameStep name="" onNameChange={onNameChange} />);

    fireEvent.change(screen.getByTestId('connection-name-input'), {target: {value: 'Acme Connection'}});

    expect(onNameChange).toHaveBeenCalledWith('Acme Connection');
  });

  it('fills the name field when a suggestion chip is clicked', () => {
    const onNameChange = vi.fn();
    render(<ConnectionNameStep name="" onNameChange={onNameChange} />);

    const suggestions = screen.getAllByRole('button');
    fireEvent.click(suggestions[0]);

    expect(onNameChange).toHaveBeenCalledWith(expect.any(String));
    expect(onNameChange.mock.calls[0][0]).not.toBe('');
  });

  it('shows an external name error', () => {
    render(
      <ConnectionNameStep
        name="Taken"
        onNameChange={vi.fn()}
        nameError="A connection with this name already exists."
      />,
    );

    expect(screen.getByText('A connection with this name already exists.')).toBeInTheDocument();
  });
});
