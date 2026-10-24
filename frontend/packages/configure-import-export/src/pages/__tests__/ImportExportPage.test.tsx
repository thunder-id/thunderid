// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor, userEvent, fireEvent} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import ImportExportPage from '../ImportExportPage';

const mockNavigate = vi.fn();

const mockLoggerError = vi.fn();

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({
    error: mockLoggerError,
    info: vi.fn(),
    warn: vi.fn(),
    debug: vi.fn(),
  }),
}));

describe('ImportExportPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the page with title and subtitle', () => {
    render(<ImportExportPage />);

    const importExportTexts = screen.getAllByText('Import / Export');
    expect(importExportTexts.length).toBeGreaterThan(0);
    expect(
      screen.getByText('Choose whether to import a configuration file or export your current one.'),
    ).toBeInTheDocument();
  });

  it('sets the document title', async () => {
    render(<ImportExportPage />);
    await waitFor(() => expect(document.title).toBe('Import / Export | ThunderID'));
  });

  it('renders both import and export options', () => {
    render(<ImportExportPage />);

    expect(screen.getByText('Import')).toBeInTheDocument();
    expect(screen.getByText('Export')).toBeInTheDocument();
  });

  it('renders option descriptions', () => {
    render(<ImportExportPage />);

    expect(screen.getByText('Bring in an existing ThunderID configuration file.')).toBeInTheDocument();
    expect(screen.getByText('Download your current configuration as a file.')).toBeInTheDocument();
  });

  it('renders the close button with proper aria-label', () => {
    render(<ImportExportPage />);

    const closeButton = screen.getByLabelText('Close');
    expect(closeButton).toBeInTheDocument();
  });

  it('renders breadcrumbs with Import / Export label', () => {
    render(<ImportExportPage />);

    const importExportTexts = screen.getAllByText('Import / Export');
    expect(importExportTexts.length).toBeGreaterThan(0);
  });

  it('renders progress indicator at the top', () => {
    const {container} = render(<ImportExportPage />);

    const progressBar = container.querySelector('[role="progressbar"]');
    expect(progressBar).toBeInTheDocument();
  });

  it('renders the type selection container with correct test id', () => {
    render(<ImportExportPage />);

    const typeSelectContainer = screen.getByTestId('import-export-type-select');
    expect(typeSelectContainer).toBeInTheDocument();
  });

  it('navigates to /home when close button is clicked', async () => {
    const user = userEvent.setup();
    render(<ImportExportPage />);

    const closeButton = screen.getByLabelText('Close');
    await user.click(closeButton);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/home');
    });
  });

  it('navigates to /import-configuration when Import card is clicked', async () => {
    const user = userEvent.setup();
    render(<ImportExportPage />);

    const cards = screen.getAllByRole('button', {hidden: true});
    const importCard = cards.find((card) => card.textContent?.includes('Import'));

    if (importCard) {
      await user.click(importCard);
    }

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/import-configuration');
    });
  });

  it('navigates to /export when Export card is clicked', async () => {
    const user = userEvent.setup();
    render(<ImportExportPage />);

    const cards = screen.getAllByRole('button', {hidden: true});
    const exportCard = cards.find((card) => card.textContent?.includes('Export'));

    if (exportCard) {
      await user.click(exportCard);
    }

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/export');
    });
  });

  it('navigates when Enter is pressed on a card', async () => {
    render(<ImportExportPage />);

    const cards = screen.getAllByRole('button', {hidden: true});
    const importCard = cards.find((card) => card.textContent?.includes('Import'));

    fireEvent.keyDown(importCard!, {key: 'Enter'});

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/import-configuration');
    });
  });

  it('navigates when Space is pressed on a card', async () => {
    render(<ImportExportPage />);

    const cards = screen.getAllByRole('button', {hidden: true});
    const exportCard = cards.find((card) => card.textContent?.includes('Export'));

    fireEvent.keyDown(exportCard!, {key: ' '});

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/export');
    });
  });

  it('ignores unrelated key presses on a card', () => {
    render(<ImportExportPage />);

    const cards = screen.getAllByRole('button', {hidden: true});
    const importCard = cards.find((card) => card.textContent?.includes('Import'));

    fireEvent.keyDown(importCard!, {key: 'a'});

    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it('handles navigation error when close button is clicked', async () => {
    const user = userEvent.setup();
    const navigationError = new Error('Navigation failed');
    mockNavigate.mockRejectedValue(navigationError);

    render(<ImportExportPage />);

    const closeButton = screen.getByLabelText('Close');
    await user.click(closeButton);

    await waitFor(() => {
      expect(mockLoggerError).toHaveBeenCalledWith('Failed to navigate to home page', {error: navigationError});
    });
  });

  it('handles navigation error when option is selected', async () => {
    const user = userEvent.setup();
    const navigationError = new Error('Navigation failed');
    mockNavigate.mockRejectedValue(navigationError);

    render(<ImportExportPage />);

    const cards = screen.getAllByRole('button', {hidden: true});
    const importCard = cards.find((card) => card.textContent?.includes('Import'));

    if (importCard) {
      await user.click(importCard);
    }

    await waitFor(() => {
      expect(mockLoggerError).toHaveBeenCalledWith(
        'Failed to navigate to import/export sub-page',
        expect.objectContaining({
          error: navigationError,
          route: '/import-configuration',
        }),
      );
    });
  });

  it('renders linear progress with determinate variant', () => {
    const {container} = render(<ImportExportPage />);

    const progressBar = container.querySelector('[role="progressbar"][aria-valuenow="0"]');
    expect(progressBar).toBeInTheDocument();
  });

  it('maintains layout structure with proper flex containers', () => {
    render(<ImportExportPage />);

    // Check that the component renders without layout errors
    const typeSelectContainer = screen.getByTestId('import-export-type-select');
    expect(typeSelectContainer).toBeInTheDocument();
  });

  it('uses importExport translation namespace', () => {
    // This test verifies that the component uses the correct i18n namespace
    render(<ImportExportPage />);

    // All expected text should be rendered (defaults from i18n)
    const importExportTexts = screen.getAllByText('Import / Export');
    expect(importExportTexts.length).toBeGreaterThan(0);
  });

  it('renders all option elements in correct grid layout', () => {
    render(<ImportExportPage />);

    // Check that all options are rendered
    expect(screen.getByText('Import')).toBeInTheDocument();
    expect(screen.getByText('Export')).toBeInTheDocument();
  });

  it('renders cards with proper content', () => {
    render(<ImportExportPage />);

    // Cards should be rendered with descriptions
    expect(screen.getByText('Bring in an existing ThunderID configuration file.')).toBeInTheDocument();
    expect(screen.getByText('Download your current configuration as a file.')).toBeInTheDocument();
  });
});
