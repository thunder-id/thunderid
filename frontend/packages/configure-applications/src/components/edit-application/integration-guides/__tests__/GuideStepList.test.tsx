// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen, within} from '@thunderid/test-utils';
import {describe, expect, it} from 'vitest';
import GuideStepList from '../GuideStepList';

const stepsWithControl = [
  {
    key: 'long',
    title: 'Build it',
    content: (
      <div>
        <div style={{height: 900}}>A whole sample file.</div>
        <button type="button">Copy</button>
      </div>
    ),
  },
];

const steps = [
  {key: 'short', title: 'Install it', content: <p>One line.</p>},
  {key: 'long', title: 'Build it', content: <div style={{height: 900}}>A whole sample file.</div>},
];

describe('GuideStepList', () => {
  it('expands a collapsed step when the keyboard reaches a control below its cut', async () => {
    const user = userEvent.setup();
    render(<GuideStepList steps={stepsWithControl} collapseLongSteps />);
    await screen.findByRole('button', {name: 'Expand step'});

    await user.tab();

    expect(screen.getByRole('button', {name: 'Copy'})).toHaveFocus();
    expect(screen.getByRole('button', {name: 'Collapse step'})).toHaveAttribute('aria-expanded', 'true');
  });

  it('shows every step whole unless long steps are collapsed', () => {
    render(<GuideStepList steps={steps} />);

    expect(screen.getByRole('heading', {name: 'Install it'})).toBeInTheDocument();
    expect(screen.getByRole('heading', {name: 'Build it'})).toBeInTheDocument();
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('collapses a long step until it is expanded, and leaves a short one as it is', async () => {
    const user = userEvent.setup();
    render(<GuideStepList steps={steps} collapseLongSteps />);

    const [shortStep, longStep] = screen.getAllByRole('listitem');
    expect(within(shortStep).queryByRole('button')).not.toBeInTheDocument();
    const expand = await within(longStep).findByRole('button', {name: 'Expand step'});
    expect(expand).toHaveAttribute('aria-expanded', 'false');
    const content = document.getElementById(expand.getAttribute('aria-controls')!)!;
    expect(content.getBoundingClientRect().height).toBe(320);

    await user.click(expand);

    const collapse = within(longStep).getByRole('button', {name: 'Collapse step'});
    expect(collapse).toHaveAttribute('aria-expanded', 'true');
    expect(content.getBoundingClientRect().height).toBe(900);

    await user.click(collapse);

    expect(within(longStep).getByRole('button', {name: 'Expand step'})).toBeInTheDocument();
    expect(content.getBoundingClientRect().height).toBe(320);
  });
});
