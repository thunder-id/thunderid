// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import HomeNextStepCard from '../HomeNextStepCard';

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

const DEFAULT_PROPS = {
  icon: <span data-testid="card-icon">icon</span>,
  title: 'Test Title',
  description: 'Test description text',
};

describe('HomeNextStepCard', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('renders title and description', () => {
      render(<HomeNextStepCard {...DEFAULT_PROPS} />);

      expect(screen.getByText('Test Title')).toBeInTheDocument();
      expect(screen.getByText('Test description text')).toBeInTheDocument();
    });

    it('renders the icon slot', () => {
      render(<HomeNextStepCard {...DEFAULT_PROPS} />);

      expect(screen.getByTestId('card-icon')).toBeInTheDocument();
    });

    it('renders the preview slot when provided', () => {
      render(<HomeNextStepCard {...DEFAULT_PROPS} preview={<span data-testid="preview-content">preview</span>} />);

      expect(screen.getByTestId('preview-content')).toBeInTheDocument();
    });

    it('does not render preview when not provided', () => {
      render(<HomeNextStepCard {...DEFAULT_PROPS} />);

      expect(screen.queryByTestId('preview-content')).not.toBeInTheDocument();
    });
  });

  describe('Primary action button', () => {
    it('renders primary button when primaryLabel and primaryRoute are provided', () => {
      render(<HomeNextStepCard {...DEFAULT_PROPS} primaryLabel="Go Somewhere" primaryRoute="/somewhere" />);

      expect(screen.getByRole('button', {name: 'Go Somewhere'})).toBeInTheDocument();
    });

    it('does not render primary button when primaryLabel is missing', () => {
      render(<HomeNextStepCard {...DEFAULT_PROPS} primaryRoute="/somewhere" />);

      expect(screen.queryByRole('button')).not.toBeInTheDocument();
    });

    it('does not render primary button when primaryRoute is missing', () => {
      render(<HomeNextStepCard {...DEFAULT_PROPS} primaryLabel="Go" />);

      expect(screen.queryByRole('button')).not.toBeInTheDocument();
    });

    it('calls navigate with the primaryRoute on click', () => {
      mockNavigate.mockReturnValue(undefined);
      render(<HomeNextStepCard {...DEFAULT_PROPS} primaryLabel="Go" primaryRoute="/target" />);

      fireEvent.click(screen.getByRole('button', {name: 'Go'}));

      expect(mockNavigate).toHaveBeenCalledWith('/target');
    });
  });

  describe('Secondary action button', () => {
    it('renders secondary button with navigate when secondaryRoute is provided', () => {
      render(<HomeNextStepCard {...DEFAULT_PROPS} secondaryLabel="Secondary" secondaryRoute="/secondary" />);

      expect(screen.getByRole('button', {name: 'Secondary'})).toBeInTheDocument();
    });

    it('renders secondary button as anchor when only secondaryHref is provided', () => {
      render(<HomeNextStepCard {...DEFAULT_PROPS} secondaryLabel="Docs" secondaryHref="https://example.com" />);

      const link = screen.getByRole('link', {name: 'Docs'});
      expect(link).toBeInTheDocument();
      expect(link).toHaveAttribute('href', 'https://example.com');
    });

    it('falls back to # when secondaryLabel has no href or route', () => {
      render(<HomeNextStepCard {...DEFAULT_PROPS} secondaryLabel="Link" />);

      const link = screen.getByRole('link', {name: 'Link'});
      expect(link).toHaveAttribute('href', '#');
    });

    it('calls navigate with secondaryRoute on click', () => {
      mockNavigate.mockReturnValue(undefined);
      render(<HomeNextStepCard {...DEFAULT_PROPS} secondaryLabel="Go Secondary" secondaryRoute="/sec" />);

      fireEvent.click(screen.getByRole('button', {name: 'Go Secondary'}));

      expect(mockNavigate).toHaveBeenCalledWith('/sec');
    });
  });

  describe('Feature status badge', () => {
    it('does not render a status chip when featureStatus is not provided', () => {
      render(<HomeNextStepCard {...DEFAULT_PROPS} />);

      expect(screen.queryByText('New')).not.toBeInTheDocument();
      expect(screen.queryByText('Coming Soon')).not.toBeInTheDocument();
    });

    it('renders a chip with fallback label "New" for featureStatus="new"', () => {
      render(<HomeNextStepCard {...DEFAULT_PROPS} featureStatus="new" />);

      expect(screen.getByText('New')).toBeInTheDocument();
    });

    it('renders a chip with fallback label "Coming Soon" for featureStatus="coming_soon"', () => {
      render(<HomeNextStepCard {...DEFAULT_PROPS} featureStatus="coming_soon" />);

      expect(screen.getByText('Coming Soon')).toBeInTheDocument();
    });
  });
});
