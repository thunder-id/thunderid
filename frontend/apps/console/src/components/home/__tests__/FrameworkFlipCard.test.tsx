// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import FrameworkFlipCard from '../FrameworkFlipCard';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

const mockNavigate = vi.fn();
vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

vi.mock('@wso2/oxygen-ui', async () => {
  const actual = await vi.importActual('@wso2/oxygen-ui');
  return {
    ...actual,
    useColorScheme: () => ({mode: 'dark', systemMode: 'dark'}),
  };
});

describe('FrameworkFlipCard', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigate.mockReturnValue(undefined);
  });

  it('renders the default number of slots', () => {
    render(<FrameworkFlipCard />);
    expect(screen.getAllByRole('button').length).toBeGreaterThan(0);
  });

  it('renders only a single slot when slotCount is 1', () => {
    const {container} = render(<FrameworkFlipCard slotCount={1} />);
    expect(container.querySelectorAll(':scope > div > div').length).toBeGreaterThan(0);
  });

  it('navigates to the application types page with the selected template on click', () => {
    render(<FrameworkFlipCard />);
    const buttons = screen.getAllByRole('button');
    fireEvent.click(buttons[0]);
    expect(mockNavigate).toHaveBeenCalledWith(expect.stringMatching(/^\/applications\/types\?type=.+/));
  });
});
