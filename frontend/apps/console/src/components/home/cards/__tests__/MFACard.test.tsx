// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import MFACard from '../MFACard';

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

describe('MFACard', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigate.mockReturnValue(undefined);
  });

  it('renders the card title', () => {
    render(<MFACard />);

    expect(screen.getByText('Multi-factor Authentication')).toBeInTheDocument();
  });

  it('renders the card description', () => {
    render(<MFACard />);

    expect(
      screen.getByText('Protect users by enabling an additional verification factor to the sign-in process.'),
    ).toBeInTheDocument();
  });

  it('renders the primary action button', () => {
    render(<MFACard />);

    expect(screen.getByRole('button', {name: 'Configure Flows'})).toBeInTheDocument();
  });

  it('navigates to /flows when the primary button is clicked', () => {
    render(<MFACard />);

    fireEvent.click(screen.getByRole('button', {name: 'Configure Flows'}));

    expect(mockNavigate).toHaveBeenCalledWith('/flows');
  });

  it('does not render a feature status badge', () => {
    render(<MFACard />);

    expect(screen.queryByText('New')).not.toBeInTheDocument();
    expect(screen.queryByText('Coming Soon')).not.toBeInTheDocument();
  });
});
