// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import HomePage from '../HomePage';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string | object) => (typeof fallback === 'string' ? fallback : key),
  }),
}));

type UserData = {name?: string; email?: string} | null;
let mockUser: UserData = null;

vi.mock('@thunderid/react', () => ({
  User: ({children}: {children: (user: UserData) => React.ReactNode}) => <>{children(mockUser)}</>,
}));

vi.mock('../../components/home/StartBuildingSection', () => ({
  default: () => <div data-testid="start-building-section" />,
}));

vi.mock('../../components/home/NextStepsSection', () => ({
  default: () => <div data-testid="next-steps-section" />,
}));

describe('HomePage', () => {
  beforeEach(() => {
    mockUser = null;
    vi.clearAllMocks();
  });

  describe('Greeting', () => {
    it('renders the greeting text', () => {
      render(<HomePage />);

      expect(screen.getByRole('heading', {level: 1})).toHaveTextContent('Hello,');
    });

    it('renders the subtitle', () => {
      render(<HomePage />);

      expect(screen.getByText('What do you want to secure today?')).toBeInTheDocument();
    });

    it('displays the first name of the authenticated user', () => {
      mockUser = {name: 'Jane Doe', email: 'jane@example.com'};

      render(<HomePage />);

      expect(screen.getByText('Jane')).toBeInTheDocument();
    });

    it('uses only the first word of the name when name has multiple parts', () => {
      mockUser = {name: 'Alice Bob Charlie', email: 'alice@example.com'};

      render(<HomePage />);

      expect(screen.getByText('Alice')).toBeInTheDocument();
      expect(screen.queryByText('Alice Bob Charlie')).not.toBeInTheDocument();
    });

    it('shows the fallback name "there" when no user is signed in', () => {
      mockUser = null;

      render(<HomePage />);

      expect(screen.getByText('there')).toBeInTheDocument();
    });

    it('shows the fallback name "there" when user has no name', () => {
      mockUser = {email: 'noname@example.com'};

      render(<HomePage />);

      expect(screen.getByText('there')).toBeInTheDocument();
    });
  });

  describe('Sections', () => {
    it('renders the StartBuildingSection', () => {
      render(<HomePage />);

      expect(screen.getByTestId('start-building-section')).toBeInTheDocument();
    });

    it('renders the NextStepsSection', () => {
      render(<HomePage />);

      expect(screen.getByTestId('next-steps-section')).toBeInTheDocument();
    });
  });
});
