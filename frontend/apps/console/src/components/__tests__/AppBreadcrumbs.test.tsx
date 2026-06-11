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

import {render, screen, userEvent, fireEvent} from '@thunderid/test-utils';
import {afterEach, describe, expect, it, vi} from 'vitest';

vi.mock('@wso2/oxygen-ui-icons-react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui-icons-react')>();
  return {
    ...actual,
    ChevronRight: () => <span data-testid="icon-chevron-right" />,
  };
});

import AppBreadcrumbs from '../AppBreadcrumbs';

const items3 = [
  {key: 'a', label: 'Alpha', onClick: vi.fn()},
  {key: 'b', label: 'Beta', onClick: vi.fn()},
  {key: 'c', label: 'Gamma'},
];

const items6 = [
  {key: 'a', label: 'Alpha', onClick: vi.fn()},
  {key: 'b', label: 'Beta', onClick: vi.fn()},
  {key: 'c', label: 'Charlie', onClick: vi.fn()},
  {key: 'd', label: 'Delta', onClick: vi.fn()},
  {key: 'e', label: 'Echo', onClick: vi.fn()},
  {key: 'f', label: 'Foxtrot'},
];

afterEach(() => {
  vi.clearAllMocks();
});

describe('AppBreadcrumbs — no truncation', () => {
  it('renders all items when count is within maxItems', () => {
    render(<AppBreadcrumbs items={items3} />);
    expect(screen.getByText('Alpha')).toBeInTheDocument();
    expect(screen.getByText('Beta')).toBeInTheDocument();
    expect(screen.getByText('Gamma')).toBeInTheDocument();
  });

  it('does not render the ellipsis when no truncation is needed', () => {
    render(<AppBreadcrumbs items={items3} />);
    expect(screen.queryByText('...')).not.toBeInTheDocument();
  });

  it('renders the last item as non-interactive text', () => {
    render(<AppBreadcrumbs items={items3} />);
    const last = screen.getByText('Gamma');
    expect(last).not.toHaveAttribute('role', 'button');
    expect(last).not.toHaveAttribute('tabIndex');
  });

  it('renders non-last clickable items as buttons', () => {
    render(<AppBreadcrumbs items={items3} />);
    const alpha = screen.getByText('Alpha');
    expect(alpha).toHaveAttribute('role', 'button');
    expect(alpha).toHaveAttribute('tabIndex', '0');
  });

  it('calls onClick when a non-last item is clicked', async () => {
    const onClick = vi.fn();
    render(
      <AppBreadcrumbs
        items={[
          {key: 'a', label: 'Alpha', onClick},
          {key: 'b', label: 'Beta'},
        ]}
      />,
    );
    await userEvent.click(screen.getByText('Alpha'));
    expect(onClick).toHaveBeenCalledOnce();
  });

  it('calls onClick on Enter keypress', () => {
    const onClick = vi.fn();
    render(
      <AppBreadcrumbs
        items={[
          {key: 'a', label: 'Alpha', onClick},
          {key: 'b', label: 'Beta'},
        ]}
      />,
    );
    fireEvent.keyDown(screen.getByText('Alpha'), {key: 'Enter'});
    expect(onClick).toHaveBeenCalledOnce();
  });

  it('calls onClick on Space keypress', () => {
    const onClick = vi.fn();
    render(
      <AppBreadcrumbs
        items={[
          {key: 'a', label: 'Alpha', onClick},
          {key: 'b', label: 'Beta'},
        ]}
      />,
    );
    fireEvent.keyDown(screen.getByText('Alpha'), {key: ' '});
    expect(onClick).toHaveBeenCalledOnce();
  });

  it('does not call onClick on non-Enter/Space keypress', () => {
    const onClick = vi.fn();
    render(
      <AppBreadcrumbs
        items={[
          {key: 'a', label: 'Alpha', onClick},
          {key: 'b', label: 'Beta'},
        ]}
      />,
    );
    fireEvent.keyDown(screen.getByText('Alpha'), {key: 'Tab'});
    expect(onClick).not.toHaveBeenCalled();
  });

  it('does not call onClick when item has no onClick handler', () => {
    render(
      <AppBreadcrumbs
        items={[
          {key: 'a', label: 'Alpha'},
          {key: 'b', label: 'Beta'},
        ]}
      />,
    );
    fireEvent.keyDown(screen.getByText('Alpha'), {key: 'Enter'});
    expect(screen.getByText('Alpha')).toBeInTheDocument();
  });

  it('renders a single item without crashing', () => {
    render(<AppBreadcrumbs items={[{key: 'a', label: 'Only'}]} />);
    expect(screen.getByText('Only')).toBeInTheDocument();
    expect(screen.queryByText('...')).not.toBeInTheDocument();
  });

  it('respects a custom maxItems prop', () => {
    const fiveItems = [
      {key: 'a', label: 'A', onClick: vi.fn()},
      {key: 'b', label: 'B', onClick: vi.fn()},
      {key: 'c', label: 'C', onClick: vi.fn()},
      {key: 'd', label: 'D', onClick: vi.fn()},
      {key: 'e', label: 'E'},
    ];
    render(<AppBreadcrumbs items={fiveItems} maxItems={5} />);
    expect(screen.queryByText('...')).not.toBeInTheDocument();
    expect(screen.getByText('A')).toBeInTheDocument();
    expect(screen.getByText('E')).toBeInTheDocument();
  });
});

describe('AppBreadcrumbs — truncation', () => {
  it('shows ellipsis when item count exceeds maxItems', () => {
    render(<AppBreadcrumbs items={items6} />);
    expect(screen.getByText('...')).toBeInTheDocument();
  });

  it('always shows the first item', () => {
    render(<AppBreadcrumbs items={items6} />);
    expect(screen.getByText('Alpha')).toBeInTheDocument();
  });

  it('always shows the last two items', () => {
    render(<AppBreadcrumbs items={items6} />);
    expect(screen.getByText('Echo')).toBeInTheDocument();
    expect(screen.getByText('Foxtrot')).toBeInTheDocument();
  });

  it('does not show hidden middle items in the main breadcrumb row', () => {
    render(<AppBreadcrumbs items={items6} />);
    expect(screen.queryByText('Beta')).not.toBeInTheDocument();
    expect(screen.queryByText('Charlie')).not.toBeInTheDocument();
    expect(screen.queryByText('Delta')).not.toBeInTheDocument();
  });

  it('opens the dropdown when ellipsis is clicked', async () => {
    render(<AppBreadcrumbs items={items6} />);
    await userEvent.click(screen.getByText('...'));
    expect(screen.getByText('Beta')).toBeInTheDocument();
    expect(screen.getByText('Charlie')).toBeInTheDocument();
    expect(screen.getByText('Delta')).toBeInTheDocument();
  });

  it('closes the dropdown when ellipsis is clicked again', async () => {
    render(<AppBreadcrumbs items={items6} />);
    const ellipsis = screen.getByText('...');
    await userEvent.click(ellipsis);
    expect(screen.getByText('Beta')).toBeInTheDocument();
    await userEvent.click(ellipsis);
    expect(screen.queryByText('Beta')).not.toBeInTheDocument();
  });

  it('calls the item onClick and closes dropdown when a hidden item is clicked', async () => {
    const onClick = vi.fn();
    const customItems = [
      {key: 'a', label: 'Alpha', onClick: vi.fn()},
      {key: 'b', label: 'Beta', onClick},
      {key: 'c', label: 'Charlie', onClick: vi.fn()},
      {key: 'd', label: 'Delta', onClick: vi.fn()},
      {key: 'e', label: 'Echo', onClick: vi.fn()},
      {key: 'f', label: 'Foxtrot'},
    ];
    render(<AppBreadcrumbs items={customItems} />);
    await userEvent.click(screen.getByText('...'));
    await userEvent.click(screen.getByText('Beta'));
    expect(onClick).toHaveBeenCalledOnce();
    expect(screen.queryByText('Beta')).not.toBeInTheDocument();
  });

  it('closes the dropdown on outside click', async () => {
    render(
      <div>
        <AppBreadcrumbs items={items6} />
        <button type="button">outside</button>
      </div>,
    );
    await userEvent.click(screen.getByText('...'));
    expect(screen.getByText('Beta')).toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', {name: 'outside'}));
    expect(screen.queryByText('Beta')).not.toBeInTheDocument();
  });

  it('sets ellipsisHovered on mouseenter and clears on mouseleave', () => {
    render(<AppBreadcrumbs items={items6} />);
    const ellipsis = screen.getByText('...');
    fireEvent.mouseEnter(ellipsis);
    expect(ellipsis).toBeInTheDocument();
    fireEvent.mouseLeave(ellipsis);
    expect(ellipsis).toBeInTheDocument();
  });

  it('applies hover style while dropdown is open', async () => {
    render(<AppBreadcrumbs items={items6} />);
    const ellipsis = screen.getByText('...');
    await userEvent.click(ellipsis);
    expect(screen.getByText('Beta')).toBeInTheDocument();
  });

  it('triggers truncation exactly at maxItems + 1', () => {
    const fiveItems = [
      {key: 'a', label: 'A', onClick: vi.fn()},
      {key: 'b', label: 'B', onClick: vi.fn()},
      {key: 'c', label: 'C', onClick: vi.fn()},
      {key: 'd', label: 'D', onClick: vi.fn()},
      {key: 'e', label: 'E'},
    ];
    const {rerender} = render(<AppBreadcrumbs items={fiveItems} maxItems={4} />);
    expect(screen.getByText('...')).toBeInTheDocument();

    rerender(<AppBreadcrumbs items={fiveItems.slice(0, 4)} maxItems={4} />);
    expect(screen.queryByText('...')).not.toBeInTheDocument();
  });
});
