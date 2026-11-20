// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import type {AggregatedUserTypeAttribute} from '@thunderid/configure-user-types';
import {useState, type ReactElement} from 'react';
import {describe, it, expect, vi} from 'vitest';
import UserAttributeAutocomplete, {type UserAttributeAutocompletePropsInterface} from '../UserAttributeAutocomplete';

const options: AggregatedUserTypeAttribute[] = [
  {attribute: 'email', credential: false, unique: true, userTypes: ['Person', 'Employee']},
  {attribute: 'username', credential: false, unique: true, userTypes: ['Person']},
];

type HarnessProps = Partial<UserAttributeAutocompletePropsInterface> &
  Pick<UserAttributeAutocompletePropsInterface, 'onChange'>;

function Harness({onChange, ...rest}: HarnessProps): ReactElement {
  const [inputValue, setInputValue] = useState<string>('');
  return (
    <UserAttributeAutocomplete
      id="attribute"
      label="Attribute"
      options={options}
      inputValue={inputValue}
      onInputChange={(value: string) => setInputValue(value)}
      onChange={onChange}
      {...rest}
    />
  );
}

describe('UserAttributeAutocomplete', () => {
  it('should label the input and suggest attributes with the user types declaring them', async () => {
    const user = userEvent.setup();
    render(<Harness onChange={vi.fn()} />);

    await user.click(screen.getByLabelText('Attribute'));

    expect(screen.getByRole('option', {name: /email/})).toHaveTextContent('Person, Employee');
    expect(screen.getByRole('option', {name: /username/})).toHaveTextContent('Person');
  });

  it('should report the name of a picked suggestion', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness onChange={onChange} />);

    await user.click(screen.getByLabelText('Attribute'));
    await user.click(screen.getByRole('option', {name: /email/}));

    expect(onChange).toHaveBeenCalledWith('email');
  });

  it('should report a typed attribute no user type declares', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(<Harness onChange={onChange} />);

    await user.type(screen.getByLabelText('Attribute'), 'mobileNumber{Enter}');

    expect(onChange).toHaveBeenCalledWith('mobileNumber');
  });

  it('should report clearing as an input change', async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    const onInputChange = vi.fn();
    render(<Harness onChange={onChange} onInputChange={onInputChange} inputValue="email" />);

    await user.click(screen.getByLabelText('Attribute'));
    await user.click(screen.getByRole('button', {name: 'Clear'}));

    expect(onInputChange).toHaveBeenCalledWith('', 'clear');
    expect(onChange).not.toHaveBeenCalled();
  });

  it('should pass the input props through and report blur', async () => {
    const user = userEvent.setup();
    const onBlur = vi.fn();
    render(
      <Harness
        onChange={vi.fn()}
        placeholder="Select an attribute"
        size="small"
        error
        loading
        disableClearable
        onBlur={onBlur}
        sx={{width: '100%'}}
      />,
    );

    const input = screen.getByLabelText('Attribute');
    expect(input).toHaveAttribute('placeholder', 'Select an attribute');
    expect(input).toHaveAttribute('aria-invalid', 'true');

    await user.click(input);
    await user.tab();

    expect(onBlur).toHaveBeenCalled();
  });
});
