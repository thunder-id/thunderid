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

import {render, screen, userEvent, act, fireEvent} from '@thunderid/test-utils';
import {afterEach, beforeAll, beforeEach, describe, expect, it, vi} from 'vitest';

vi.mock('framer-motion', () => ({
  motion: {
    create: (Component: React.ElementType) => Component,
  },
}));

vi.mock('@wso2/oxygen-ui-icons-react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui-icons-react')>();
  return {
    ...actual,
    Check: () => <span data-testid="icon-check" />,
    Copy: () => <span data-testid="icon-copy" />,
    Play: () => <span data-testid="icon-play" />,
  };
});

import TerminalBlock from '../TerminalBlock';

describe('TerminalBlock', () => {
  let writeTextSpy: ReturnType<typeof vi.fn>;

  beforeAll(() => {
    Object.defineProperty(navigator, 'clipboard', {
      value: {writeText: vi.fn()},
      writable: true,
      configurable: true,
    });
  });

  beforeEach(() => {
    writeTextSpy = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined);
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('renders without crashing', () => {
    const {container} = render(<TerminalBlock command="echo hello" />);
    expect(container).toBeInTheDocument();
  });

  it('renders the command text', () => {
    render(<TerminalBlock command="echo hello" />);
    expect(screen.getByText('echo hello')).toBeInTheDocument();
  });

  it('renders tabs when provided', () => {
    render(<TerminalBlock command="echo hello" tabs={<div data-testid="tab-content">Tab 1</div>} />);
    expect(screen.getByTestId('tab-content')).toBeInTheDocument();
    expect(screen.getByText('Tab 1')).toBeInTheDocument();
  });

  it('does not render tabs section when not provided', () => {
    render(<TerminalBlock command="echo hello" />);
    expect(screen.queryByTestId('tab-content')).not.toBeInTheDocument();
  });

  // Swapped order: check icon test first, then writeText call test
  it('copy button shows check icon after copying', async () => {
    const user = userEvent.setup();
    render(<TerminalBlock command="echo hello" />);

    expect(screen.getByTestId('icon-copy')).toBeInTheDocument();
    expect(screen.queryByTestId('icon-check')).not.toBeInTheDocument();

    await user.click(screen.getByRole('button', {name: /copy command/i}));

    expect(await screen.findByTestId('icon-check')).toBeInTheDocument();
    expect(screen.queryByTestId('icon-copy')).not.toBeInTheDocument();
  });

  it('copy button calls clipboard.writeText with the command', async () => {
    const user = userEvent.setup();
    render(<TerminalBlock command="echo hello" />);

    await user.click(screen.getByRole('button', {name: /copy command/i}));

    expect(writeTextSpy).toHaveBeenCalledWith('echo hello');
  });

  it('copy button reverts to copy icon after 2 seconds', async () => {
    vi.useFakeTimers({toFake: ['setTimeout', 'clearTimeout']});
    render(<TerminalBlock command="echo hello" />);

    fireEvent.click(screen.getByRole('button', {name: /copy command/i}));

    // Flush the .then() microtask → setCopied(true) → React re-render
    await act(async () => {
      await Promise.resolve();
    });
    expect(screen.getByTestId('icon-check')).toBeInTheDocument();

    // Advance the 2000ms timer → setCopied(false) → React re-render
    act(() => {
      vi.advanceTimersByTime(2001);
    });
    expect(screen.getByTestId('icon-copy')).toBeInTheDocument();

    vi.useRealTimers();
  });
});
