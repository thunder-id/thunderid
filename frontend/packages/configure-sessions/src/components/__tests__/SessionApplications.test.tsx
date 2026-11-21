// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, waitFor} from '@thunderid/test-utils';
import {describe, expect, it} from 'vitest';
import type {SessionParticipant} from '../../models/session';
import SessionApplications from '../SessionApplications';

const apps = (...names: string[]): SessionParticipant[] =>
  names.map((appName) => ({appId: `app-${appName}`, appName, firstJoinedAt: '', lastActiveAt: ''}));

const many = apps('A', 'B', 'C', 'D', 'E', 'F', 'G', 'H', 'I', 'J');

describe('SessionApplications', () => {
  it('shows every application as an icon named for assistive technology', () => {
    render(<SessionApplications participants={apps('Console', 'Store')} size={24} />);

    expect(screen.getByRole('img', {name: 'Console'})).toBeInTheDocument();
    expect(screen.getByRole('img', {name: 'Store'})).toBeInTheDocument();
  });

  it('names an application by its id when its name is unknown', () => {
    render(<SessionApplications participants={[{appId: 'app-9', firstJoinedAt: '', lastActiveAt: ''}]} size={24} />);

    expect(screen.getByRole('img', {name: 'app-9'})).toBeInTheDocument();
  });

  it('shows the name in a tooltip on hover', async () => {
    render(<SessionApplications participants={apps('Console')} size={24} />);

    fireEvent.mouseOver(screen.getByRole('img', {name: 'Console'}));

    expect(await screen.findByRole('tooltip')).toHaveTextContent('Console');
  });

  it('offers no scroll arrows when every icon fits', () => {
    render(<SessionApplications participants={apps('A', 'B')} size={24} />);

    expect(screen.getByRole('region', {name: 'Active applications'})).not.toHaveAttribute('tabindex');
    expect(screen.queryByRole('button')).not.toBeInTheDocument();
  });

  it('offers an arrow on each side that has more to scroll to', async () => {
    render(
      <div style={{width: 100}}>
        <SessionApplications participants={many} size={28} />
      </div>,
    );
    const strip = screen.getByRole('region', {name: 'Active applications'});

    expect(screen.getAllByRole('img')).toHaveLength(10);
    expect(strip).toHaveAttribute('tabindex', '0');
    expect(await screen.findByRole('button', {name: 'Scroll applications right'})).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Scroll applications left'})).not.toBeInTheDocument();

    strip.scrollLeft = strip.scrollWidth;
    fireEvent.scroll(strip);

    expect(await screen.findByRole('button', {name: 'Scroll applications left'})).toBeInTheDocument();
    await waitFor(() =>
      expect(screen.queryByRole('button', {name: 'Scroll applications right'})).not.toBeInTheDocument(),
    );
  });

  it('scrolls the strip when an arrow is pressed', async () => {
    render(
      <div style={{width: 100}}>
        <SessionApplications participants={many} size={28} />
      </div>,
    );
    const strip = screen.getByRole('region', {name: 'Active applications'});

    fireEvent.click(await screen.findByRole('button', {name: 'Scroll applications right'}));

    await waitFor(() => expect(strip.scrollLeft).toBeGreaterThan(0));
  });

  it('renders nothing for a session with no applications', () => {
    const {container} = render(<SessionApplications participants={[]} size={24} />);

    expect(container).toBeEmptyDOMElement();
  });
});
