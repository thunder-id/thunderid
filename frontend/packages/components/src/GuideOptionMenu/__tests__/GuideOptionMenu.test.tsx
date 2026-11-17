// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen, within} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import GuideOptionMenu from '../GuideOptionMenu';

const options = [
  {value: 'python', label: 'Python', icon: <span data-testid="python-icon" />},
  {value: 'typescript', label: 'TypeScript'},
];

describe('GuideOptionMenu', () => {
  it('names the choice made, and makes another from its menu', async () => {
    const user = userEvent.setup();
    const onSelect = vi.fn();
    render(
      <GuideOptionMenu
        id="language-menu"
        menuLabel="Language"
        options={options}
        value="typescript"
        onSelect={onSelect}
      />,
    );

    const button = screen.getByRole('button', {name: 'TypeScript'});
    expect(button).toHaveAttribute('aria-expanded', 'false');
    await user.click(button);

    expect(button).toHaveAttribute('aria-expanded', 'true');
    const menu = within(screen.getByRole('menu', {name: 'Language'}));
    expect(menu.getByRole('menuitemradio', {name: 'TypeScript'})).toHaveAttribute('aria-checked', 'true');
    expect(menu.getByRole('menuitemradio', {name: 'Python'})).toHaveAttribute('aria-checked', 'false');
    expect(menu.getByTestId('python-icon')).toBeInTheDocument();

    await user.click(menu.getByRole('menuitemradio', {name: 'Python'}));

    expect(onSelect).toHaveBeenCalledWith('python');
    expect(screen.queryByRole('menu')).not.toBeInTheDocument();
  });

  it('names the first choice when the one made is not among them', () => {
    render(<GuideOptionMenu id="language-menu" menuLabel="Language" options={options} value="go" onSelect={vi.fn()} />);

    expect(screen.getByRole('button', {name: 'Python'})).toBeInTheDocument();
  });
});
