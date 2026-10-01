// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import LoginBoxCard from '../LoginBoxCard';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string | object) => (typeof fallback === 'string' ? fallback : key),
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

vi.mock('framer-motion', async () => {
  const actual = await vi.importActual<typeof import('framer-motion')>('framer-motion');
  return {
    ...actual,
    motion: {
      ...((actual as {motion: object}).motion ?? {}),
      div: ({children, ...rest}: React.HTMLAttributes<HTMLDivElement>) => <div {...rest}>{children}</div>,
    },
  };
});

describe('LoginBoxCard', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigate.mockReturnValue(undefined);
  });

  it('renders the card title', () => {
    render(<LoginBoxCard />);

    expect(screen.getByText('Sign-in Box')).toBeInTheDocument();
  });

  it('renders the card description', () => {
    render(<LoginBoxCard />);

    expect(
      screen.getByText('Build themes and attach them to your applications to personalise the sign-in experience.'),
    ).toBeInTheDocument();
  });

  it('renders the primary action button', () => {
    render(<LoginBoxCard />);

    expect(screen.getByRole('button', {name: 'Open Design Studio'})).toBeInTheDocument();
  });

  it('navigates to /design when the primary button is clicked', () => {
    render(<LoginBoxCard />);

    fireEvent.click(screen.getByRole('button', {name: 'Open Design Studio'}));

    expect(mockNavigate).toHaveBeenCalledWith('/design');
  });

  it('does not render a feature status badge', () => {
    render(<LoginBoxCard />);

    expect(screen.queryByText('New')).not.toBeInTheDocument();
    expect(screen.queryByText('Coming Soon')).not.toBeInTheDocument();
  });
});
