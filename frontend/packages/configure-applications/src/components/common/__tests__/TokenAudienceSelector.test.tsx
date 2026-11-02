// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi} from 'vitest';
import TokenAudienceSelector, {type TokenAudienceOption} from '../TokenAudienceSelector';

// Both audiences selectable: the list only renders when there is a choice to make.
const options: TokenAudienceOption[] = [
  {value: 'application', label: 'Application', description: 'M2M access token'},
  {value: 'user', label: 'User', description: 'Tokens for a signed-in user'},
];

const renderSelector = (value = 'application', onChange = vi.fn(), disabled = false) => {
  const result = render(
    <TokenAudienceSelector
      title="Issued to"
      options={options}
      value={value}
      onChange={onChange}
      footnote="Attribute sets are configured independently for each audience."
      disabled={disabled}
    >
      <div data-testid="panel-content" />
    </TokenAudienceSelector>,
  );
  return {...result, onChange};
};

describe('TokenAudienceSelector', () => {
  it('renders a vertical tab list named after the title', () => {
    renderSelector();

    const tablist = screen.getByRole('tablist', {name: 'Issued to'});
    expect(tablist).toHaveAttribute('aria-orientation', 'vertical');
    expect(screen.getAllByRole('tab')).toHaveLength(2);
  });

  it('names each tab by its label alone and references the description separately', () => {
    renderSelector();

    const userTab = screen.getByRole('tab', {name: 'User'});
    const describedBy = userTab.getAttribute('aria-describedby');

    expect(describedBy).not.toBeNull();
    expect(document.getElementById(describedBy!)).toHaveTextContent('Tokens for a signed-in user');
  });

  it('associates every tab with the panel, and the panel with the selected tab', () => {
    renderSelector();

    const panel = screen.getByRole('tabpanel');
    const selectedTab = screen.getByRole('tab', {name: 'Application'});

    screen.getAllByRole('tab').forEach((tab) => {
      expect(tab).toHaveAttribute('aria-controls', panel.id);
    });
    expect(panel).toHaveAttribute('aria-labelledby', selectedTab.id);
    expect(screen.getByTestId('panel-content')).toBeInTheDocument();
  });

  it('keeps only the selected tab in the tab sequence', async () => {
    const user = userEvent.setup();
    renderSelector();

    const applicationTab = screen.getByRole('tab', {name: 'Application'});
    const userTab = screen.getByRole('tab', {name: 'User'});

    expect(applicationTab).toHaveAttribute('tabindex', '0');
    expect(userTab).toHaveAttribute('tabindex', '-1');

    // One Tab press reaches the widget; the next must leave it rather than land on the second tab.
    await user.tab();
    expect(applicationTab).toHaveFocus();

    await user.tab();
    expect(userTab).not.toHaveFocus();
  });

  it('moves focus between tabs with the arrow keys', async () => {
    const user = userEvent.setup();
    renderSelector();

    await user.tab();
    expect(screen.getByRole('tab', {name: 'Application'})).toHaveFocus();

    await user.keyboard('{ArrowDown}');
    expect(screen.getByRole('tab', {name: 'User'})).toHaveFocus();

    await user.keyboard('{ArrowUp}');
    expect(screen.getByRole('tab', {name: 'Application'})).toHaveFocus();
  });

  it('moves focus to the first and last tab with Home and End', async () => {
    const user = userEvent.setup();
    renderSelector();

    await user.tab();

    await user.keyboard('{End}');
    expect(screen.getByRole('tab', {name: 'User'})).toHaveFocus();

    await user.keyboard('{Home}');
    expect(screen.getByRole('tab', {name: 'Application'})).toHaveFocus();
  });

  it('selects the focused tab on Enter, so moving focus alone does not mount another panel', async () => {
    const user = userEvent.setup();
    const {onChange} = renderSelector();

    await user.tab();
    await user.keyboard('{ArrowDown}');

    // Manual activation: arrowing to a tab must not select it, since each panel mounts its own
    // settings and fetches on mount.
    expect(onChange).not.toHaveBeenCalled();

    await user.keyboard('{Enter}');
    expect(onChange).toHaveBeenCalledWith('user');
  });

  it('selects a tab on click', async () => {
    const user = userEvent.setup();
    const {onChange} = renderSelector();

    await user.click(screen.getByRole('tab', {name: 'User'}));

    expect(onChange).toHaveBeenCalledWith('user');
  });
  it('collapses to the settings alone when only one audience is selectable', () => {
    render(
      <TokenAudienceSelector
        title="Issued to"
        options={[
          {value: 'application', label: 'Application', description: 'M2M access token'},
          {value: 'user', label: 'User', description: 'Tokens for a signed-in user', isLocked: true},
        ]}
        value="application"
        onChange={vi.fn()}
        footnote="Attribute sets are configured independently for each audience."
      >
        <div data-testid="panel-content" />
      </TokenAudienceSelector>,
    );

    // Nothing to choose between, so the picker and the locked audience are both gone.
    expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
    expect(screen.queryByRole('tab')).not.toBeInTheDocument();
    expect(screen.getByTestId('panel-content')).toBeInTheDocument();
  });

  it('collapses to the settings alone when every audience is locked', () => {
    render(
      <TokenAudienceSelector
        title="Issued to"
        options={[
          {value: 'application', label: 'Application', description: 'M2M access token', isLocked: true},
          {value: 'user', label: 'User', description: 'Tokens for a signed-in user', isLocked: true},
        ]}
        value="user"
        onChange={vi.fn()}
      >
        <div data-testid="panel-content" />
      </TokenAudienceSelector>,
    );

    expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
    expect(screen.getByTestId('panel-content')).toBeInTheDocument();
  });

  describe('disabled', () => {
    it('marks every tab disabled', () => {
      renderSelector('application', vi.fn(), true);

      screen.getAllByRole('tab').forEach((tab) => {
        expect(tab).toBeDisabled();
      });
    });
  });
});
