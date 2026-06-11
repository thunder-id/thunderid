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

import {render, screen, fireEvent} from '@thunderid/test-utils';
import {describe, expect, it, vi, beforeEach} from 'vitest';
import ConfigureName from '../ConfigureName';

vi.mock('@thunderid/utils');

const {generateRandomHumanReadableIdentifiers} = await import('@thunderid/utils');

const mockSuggestions = ['Alpha Service', 'Beta Platform', 'Gamma API', 'Delta Hub', 'Epsilon Suite'];

describe('ConfigureName', () => {
  beforeEach(() => {
    vi.mocked(generateRandomHumanReadableIdentifiers).mockReturnValue(mockSuggestions);
  });

  it('renders the name and handle input fields', () => {
    render(<ConfigureName name="" handle="" onNameChange={vi.fn()} onHandleChange={vi.fn()} />);

    expect(screen.getByRole('textbox', {name: /resource server name/i})).toBeInTheDocument();
    expect(screen.getByRole('textbox', {name: /handle/i})).toBeInTheDocument();
  });

  it('calls onNameChange and derives handle when name input changes', () => {
    const onNameChange = vi.fn();
    const onHandleChange = vi.fn();
    render(<ConfigureName name="" handle="" onNameChange={onNameChange} onHandleChange={onHandleChange} />);

    fireEvent.change(screen.getByRole('textbox', {name: /resource server name/i}), {
      target: {value: 'Payments API'},
    });

    expect(onNameChange).toHaveBeenCalledWith('Payments API');
    expect(onHandleChange).toHaveBeenCalledWith('payments-api');
  });

  it('calls onHandleChange with invalid characters stripped when handle input changes', () => {
    const onHandleChange = vi.fn();
    render(<ConfigureName name="Test" handle="test" onNameChange={vi.fn()} onHandleChange={onHandleChange} />);

    fireEvent.change(screen.getByRole('textbox', {name: /handle/i}), {
      target: {value: 'payments-api!!'},
    });

    expect(onHandleChange).toHaveBeenCalledWith('payments-api');
  });

  it('calls onReadyChange with true when name and handle are non-empty', () => {
    const onReadyChange = vi.fn();
    render(
      <ConfigureName
        name="Test"
        handle="test"
        onNameChange={vi.fn()}
        onHandleChange={vi.fn()}
        onReadyChange={onReadyChange}
      />,
    );

    expect(onReadyChange).toHaveBeenCalledWith(true);
  });

  it('calls onReadyChange with false when name is empty', () => {
    const onReadyChange = vi.fn();
    render(
      <ConfigureName
        name=""
        handle="test"
        onNameChange={vi.fn()}
        onHandleChange={vi.fn()}
        onReadyChange={onReadyChange}
      />,
    );

    expect(onReadyChange).toHaveBeenCalledWith(false);
  });

  it('calls onReadyChange with true when name is non-empty even if handle is empty', () => {
    const onReadyChange = vi.fn();
    render(
      <ConfigureName
        name="Test"
        handle=""
        onNameChange={vi.fn()}
        onHandleChange={vi.fn()}
        onReadyChange={onReadyChange}
      />,
    );

    expect(onReadyChange).toHaveBeenCalledWith(true);
  });

  it('renders suggestion chips from the returned suggestions', () => {
    render(<ConfigureName name="" handle="" onNameChange={vi.fn()} onHandleChange={vi.fn()} />);

    expect(screen.getByText('Alpha Service')).toBeInTheDocument();
    expect(screen.getByText('Beta Platform')).toBeInTheDocument();
  });

  it('fills name and handle when a suggestion chip is clicked', () => {
    const onNameChange = vi.fn();
    const onHandleChange = vi.fn();
    render(<ConfigureName name="" handle="" onNameChange={onNameChange} onHandleChange={onHandleChange} />);

    fireEvent.click(screen.getByText('Alpha Service'));

    expect(onNameChange).toHaveBeenCalledWith('Alpha Service');
    expect(onHandleChange).toHaveBeenCalledWith('alpha-service');
  });

  it('derives handle with underscore when delimiter is hyphen', () => {
    const onHandleChange = vi.fn();
    render(<ConfigureName name="" handle="" delimiter="-" onNameChange={vi.fn()} onHandleChange={onHandleChange} />);

    fireEvent.change(screen.getByRole('textbox', {name: /resource server name/i}), {
      target: {value: 'Shaky Trees Refuse'},
    });

    expect(onHandleChange).toHaveBeenCalledWith('shaky_trees_refuse');
  });

  it('does not auto-derive handle when handleEdited is true', () => {
    const onHandleChange = vi.fn();
    render(
      <ConfigureName
        name=""
        handle="custom-handle"
        handleEdited={true}
        onNameChange={vi.fn()}
        onHandleChange={onHandleChange}
      />,
    );

    fireEvent.change(screen.getByRole('textbox', {name: /resource server name/i}), {
      target: {value: 'New Name'},
    });

    expect(onHandleChange).not.toHaveBeenCalled();
  });

  it('strips delimiter character from manual handle input', () => {
    const onHandleChange = vi.fn();
    render(
      <ConfigureName name="Test" handle="test" delimiter="/" onNameChange={vi.fn()} onHandleChange={onHandleChange} />,
    );

    fireEvent.change(screen.getByRole('textbox', {name: /handle/i}), {
      target: {value: 'my/handle'},
    });

    expect(onHandleChange).toHaveBeenCalledWith('myhandle');
  });
});
