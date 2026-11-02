// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent, waitFor, act} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {BrowserRouter} from 'react-router';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import IntegrationGuide from '../IntegrationGuide';

// Mock react-i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

// Mock navigate function
const mockNavigate = vi.fn();
vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

// Mock TechnologyGuide component
vi.mock('../../edit-application/integration-guides/TechnologyGuide', () => ({
  default: () => <div data-testid="technology-guide">Technology Guide</div>,
}));

describe('IntegrationGuide', () => {
  const defaultProps = {
    appName: 'Test Application',
    appLogo: 'https://example.com/logo.png',
    selectedColor: '#FF5733',
    hasOAuthConfig: false,
    applicationId: 'app-123',
  };

  const renderWithRouter = (ui: React.ReactElement) => render(<BrowserRouter>{ui}</BrowserRouter>);

  // Store mock at module level so tests can access it
  let clipboardWriteTextMock: ReturnType<typeof vi.fn>;
  // Store original document.execCommand to restore after tests
  let originalExecCommand: typeof document.execCommand;

  beforeEach(() => {
    vi.clearAllMocks();
    vi.useFakeTimers({shouldAdvanceTime: true});
    // Mock clipboard API using defineProperty
    clipboardWriteTextMock = vi.fn().mockResolvedValue(undefined);
    Object.defineProperty(navigator, 'clipboard', {
      value: {
        writeText: clipboardWriteTextMock,
      },
      writable: true,
      configurable: true,
    });
    // Save original execCommand before any test modifies it
    // eslint-disable-next-line @typescript-eslint/unbound-method
    originalExecCommand = document.execCommand;
  });

  afterEach(() => {
    vi.useRealTimers();
    // Restore original execCommand to prevent test pollution
    document.execCommand = originalExecCommand;
  });

  describe('Rendering', () => {
    it('should render the component', () => {
      renderWithRouter(<IntegrationGuide {...defaultProps} />);

      expect(screen.getByText('applications:onboarding.summary.title')).toBeInTheDocument();
    });

    it('should display app name', () => {
      renderWithRouter(<IntegrationGuide {...defaultProps} />);

      expect(screen.getByText(defaultProps.appName)).toBeInTheDocument();
    });

    it('should display success message when OAuth not configured', () => {
      renderWithRouter(<IntegrationGuide {...defaultProps} hasOAuthConfig={false} />);

      expect(screen.getByText('applications:onboarding.summary.subtitle')).toBeInTheDocument();
    });

    it('should display guides subtitle when integrationGuides are provided', () => {
      const props = {
        ...defaultProps,
        integrationGuides: {
          react: {
            llm_prompt: {
              docsUrl: 'https://example.com/docs/test-guide',
            },
          },
        },
      };

      renderWithRouter(<IntegrationGuide {...props} />);

      expect(screen.getByText('applications:onboarding.summary.guides.subtitle')).toBeInTheDocument();
    });

    it('should render success icon', () => {
      renderWithRouter(<IntegrationGuide {...defaultProps} />);

      expect(screen.getByRole('img', {name: 'Success'})).toBeInTheDocument();
    });
  });

  describe('OAuth 2 Configuration', () => {
    it('should display client ID when hasOAuthConfig is true', () => {
      const props = {
        ...defaultProps,
        hasOAuthConfig: true,
        clientId: 'test_client_id',
      };

      renderWithRouter(<IntegrationGuide {...props} />);

      expect(screen.getByDisplayValue('test_client_id')).toBeInTheDocument();
    });

    it('should display client secret when provided', () => {
      const props = {
        ...defaultProps,
        hasOAuthConfig: true,
        clientId: 'test_client_id',
        clientSecret: 'test_secret',
      };

      renderWithRouter(<IntegrationGuide {...props} />);

      // Secret should be hidden by default
      const secretInput = screen.getByDisplayValue('test_secret');
      expect(secretInput).toHaveAttribute('type', 'password');
    });

    it('should not display OAuth credentials when hasOAuthConfig is false', () => {
      renderWithRouter(<IntegrationGuide {...defaultProps} hasOAuthConfig={false} />);

      expect(screen.queryByText('applications:create.integrationGuide.clientId')).not.toBeInTheDocument();
    });

    it('should display warning alert when client secret is present', () => {
      const props = {
        ...defaultProps,
        hasOAuthConfig: true,
        clientId: 'test_client_id',
        clientSecret: 'test_secret',
      };

      renderWithRouter(<IntegrationGuide {...props} />);

      expect(screen.getByText('applications:clientSecret.warning')).toBeInTheDocument();
    });

    it('should not display warning alert for public clients (no secret)', () => {
      const props = {
        ...defaultProps,
        hasOAuthConfig: true,
        clientId: 'test_client_id',
        clientSecret: '',
      };

      renderWithRouter(<IntegrationGuide {...props} />);

      expect(screen.queryByText('applications:clientSecret.warning')).not.toBeInTheDocument();
    });

    it('should toggle client secret visibility', async () => {
      const user = userEvent.setup({advanceTimers: vi.advanceTimersByTime});
      const props = {
        ...defaultProps,
        hasOAuthConfig: true,
        clientId: 'test_client_id',
        clientSecret: 'test_secret',
      };

      renderWithRouter(<IntegrationGuide {...props} />);

      const secretInput = screen.getByDisplayValue('test_secret');
      expect(secretInput).toHaveAttribute('type', 'password');

      // Find the visibility toggle button - it's in the client secret field
      // The buttons are: app card, client ID copy, visibility toggle, client secret copy
      const buttons = screen.getAllByRole('button');
      // Visibility toggle is the second-to-last button (before client secret copy)
      const visibilityButton = buttons[buttons.length - 2];

      await user.click(visibilityButton);
      await waitFor(() => {
        expect(secretInput).toHaveAttribute('type', 'text');
      });

      await user.click(visibilityButton);
      await waitFor(() => {
        expect(secretInput).toHaveAttribute('type', 'password');
      });
    });

    it('should copy client ID to clipboard and show copied message', async () => {
      const user = userEvent.setup({advanceTimers: vi.advanceTimersByTime});
      const props = {
        ...defaultProps,
        hasOAuthConfig: true,
        clientId: 'test_client_id',
      };

      renderWithRouter(<IntegrationGuide {...props} />);

      // Find the copy button for client ID
      // Buttons are: app card (button role), client ID copy button
      const copyButtons = screen.getAllByRole('button');
      // The client ID copy button is the second button (index 1) - after app card
      const clientIdCopyButton = copyButtons[1];

      await user.click(clientIdCopyButton);

      // Check copied message appears (indicates copy was successful)
      await waitFor(() => {
        expect(screen.getByText('applications:clientSecret.copied')).toBeInTheDocument();
      });

      // Advance timers to clear the copied state
      act(() => {
        vi.advanceTimersByTime(2500);
      });

      // After timeout, copied message should disappear
      await waitFor(() => {
        expect(screen.queryByText('applications:clientSecret.copied')).not.toBeInTheDocument();
      });
    });

    it('should copy client secret to clipboard and show copied message', async () => {
      const user = userEvent.setup({advanceTimers: vi.advanceTimersByTime});
      const props = {
        ...defaultProps,
        hasOAuthConfig: true,
        clientId: 'test_client_id',
        clientSecret: 'test_secret',
      };

      renderWithRouter(<IntegrationGuide {...props} />);

      // Find all copy buttons - the last one should be for client secret
      const copyButtons = screen.getAllByRole('button');
      // Get the last copy button (for client secret)
      const secretCopyButton = copyButtons[copyButtons.length - 1];

      await user.click(secretCopyButton);

      // Check copied message appears (indicates copy was successful)
      await waitFor(() => {
        // There may be two copied messages (one for client ID area, one for secret)
        const copiedMessages = screen.getAllByText('applications:clientSecret.copied');
        expect(copiedMessages.length).toBeGreaterThan(0);
      });
    });

    it('should use fallback copy method when clipboard API fails', async () => {
      const user = userEvent.setup({advanceTimers: vi.advanceTimersByTime});

      // Mock clipboard to fail using defineProperty
      Object.defineProperty(navigator, 'clipboard', {
        value: {
          writeText: vi.fn().mockRejectedValue(new Error('Clipboard not available')),
        },
        writable: true,
        configurable: true,
      });

      // Mock document.execCommand
      const execCommandMock = vi.fn().mockReturnValue(true);
      document.execCommand = execCommandMock;

      const props = {
        ...defaultProps,
        hasOAuthConfig: true,
        clientId: 'test_client_id',
      };

      renderWithRouter(<IntegrationGuide {...props} />);

      // Buttons are: app card, client ID copy button
      const copyButtons = screen.getAllByRole('button');
      await user.click(copyButtons[1]);

      await waitFor(() => {
        expect(execCommandMock).toHaveBeenCalledWith('copy');
      });
    });

    it('should handle fallback copy failure gracefully when execCommand throws', async () => {
      const user = userEvent.setup({advanceTimers: vi.advanceTimersByTime});

      // Mock clipboard to fail
      Object.defineProperty(navigator, 'clipboard', {
        value: {
          writeText: vi.fn().mockRejectedValue(new Error('Clipboard not available')),
        },
        writable: true,
        configurable: true,
      });

      // Mock document.execCommand to throw an error
      const execCommandMock = vi.fn().mockImplementation(() => {
        throw new Error('execCommand not supported');
      });
      document.execCommand = execCommandMock;

      const props = {
        ...defaultProps,
        hasOAuthConfig: true,
        clientId: 'test_client_id',
      };

      renderWithRouter(<IntegrationGuide {...props} />);

      // Buttons are: app card, client ID copy button
      const copyButtons = screen.getAllByRole('button');

      // Should not throw even when both copy methods fail
      await expect(user.click(copyButtons[1])).resolves.not.toThrow();

      // execCommand should have been called
      expect(execCommandMock).toHaveBeenCalledWith('copy');
    });

    it('should handle fallback copy failure gracefully when execCommand returns false', async () => {
      const user = userEvent.setup({advanceTimers: vi.advanceTimersByTime});

      // Mock clipboard to fail
      Object.defineProperty(navigator, 'clipboard', {
        value: {
          writeText: vi.fn().mockRejectedValue(new Error('Clipboard not available')),
        },
        writable: true,
        configurable: true,
      });

      // Mock document.execCommand to return false (copy failed)
      const execCommandMock = vi.fn().mockReturnValue(false);
      document.execCommand = execCommandMock;

      const props = {
        ...defaultProps,
        hasOAuthConfig: true,
        clientId: 'test_client_id',
      };

      renderWithRouter(<IntegrationGuide {...props} />);

      const copyButtons = screen.getAllByRole('button');

      // Should not throw
      await expect(user.click(copyButtons[1])).resolves.not.toThrow();
    });

    it('should show copied state on successful fallback copy', async () => {
      const user = userEvent.setup({advanceTimers: vi.advanceTimersByTime});

      // Mock clipboard to fail
      Object.defineProperty(navigator, 'clipboard', {
        value: {
          writeText: vi.fn().mockRejectedValue(new Error('Clipboard not available')),
        },
        writable: true,
        configurable: true,
      });

      // Mock document.execCommand to succeed
      document.execCommand = vi.fn().mockReturnValue(true);

      const props = {
        ...defaultProps,
        hasOAuthConfig: true,
        clientId: 'test_client_id',
      };

      renderWithRouter(<IntegrationGuide {...props} />);

      const copyButtons = screen.getAllByRole('button');
      await user.click(copyButtons[1]);

      // Should show copied message after fallback copy succeeds
      await waitFor(() => {
        expect(screen.getByText('applications:clientSecret.copied')).toBeInTheDocument();
      });

      // Advance timers to clear copied state
      act(() => {
        vi.advanceTimersByTime(2500);
      });

      await waitFor(() => {
        expect(screen.queryByText('applications:clientSecret.copied')).not.toBeInTheDocument();
      });
    });

    it('should clear existing timeout before setting new one in fallback copy', async () => {
      const user = userEvent.setup({advanceTimers: vi.advanceTimersByTime});

      // Mock clipboard to fail
      Object.defineProperty(navigator, 'clipboard', {
        value: {
          writeText: vi.fn().mockRejectedValue(new Error('Clipboard not available')),
        },
        writable: true,
        configurable: true,
      });

      // Mock document.execCommand to succeed
      document.execCommand = vi.fn().mockReturnValue(true);

      const props = {
        ...defaultProps,
        hasOAuthConfig: true,
        clientId: 'test_client_id',
      };

      renderWithRouter(<IntegrationGuide {...props} />);

      const copyButtons = screen.getAllByRole('button');

      // Click twice rapidly to trigger timeout clearing logic
      await user.click(copyButtons[1]);
      await user.click(copyButtons[1]);

      // Should still show copied message
      await waitFor(() => {
        expect(screen.getByText('applications:clientSecret.copied')).toBeInTheDocument();
      });
    });
  });

  describe('Integration Guides', () => {
    it('should render TechnologyGuide when integrationGuides are provided', () => {
      const props = {
        ...defaultProps,
        integrationGuides: {
          react: {
            llm_prompt: {
              docsUrl: 'https://example.com/docs/test-guide',
            },
          },
        },
      };

      renderWithRouter(<IntegrationGuide {...props} />);

      expect(screen.getByTestId('technology-guide')).toBeInTheDocument();
    });

    it('should not render TechnologyGuide when integrationGuides are null', () => {
      const props = {
        ...defaultProps,
        integrationGuides: null,
      };

      renderWithRouter(<IntegrationGuide {...props} />);

      expect(screen.queryByTestId('technology-guide')).not.toBeInTheDocument();
    });

    it('should not render TechnologyGuide when integrationGuides are undefined', () => {
      renderWithRouter(<IntegrationGuide {...defaultProps} />);

      expect(screen.queryByTestId('technology-guide')).not.toBeInTheDocument();
    });

    it('should not render app details section when integrationGuides are provided', () => {
      const props = {
        ...defaultProps,
        integrationGuides: {
          react: {
            llm_prompt: {
              docsUrl: 'https://example.com/docs/test-guide',
            },
          },
        },
      };

      renderWithRouter(<IntegrationGuide {...props} />);

      expect(screen.queryByText('applications:onboarding.summary.appDetails')).not.toBeInTheDocument();
    });
  });

  describe('App Logo', () => {
    it('should display app logo when provided', () => {
      renderWithRouter(<IntegrationGuide {...defaultProps} />);

      const logo = screen.getByAltText('Test Application logo');
      expect(logo).toHaveAttribute('src', defaultProps.appLogo);
    });

    it('should handle null logo gracefully', () => {
      const props = {
        ...defaultProps,
        appLogo: null,
      };

      renderWithRouter(<IntegrationGuide {...props} />);

      // Should still render without crashing
      expect(screen.getByText(defaultProps.appName)).toBeInTheDocument();
    });

    it('should display first letter avatar when logo is null', () => {
      const props = {
        ...defaultProps,
        appLogo: null,
      };

      renderWithRouter(<IntegrationGuide {...props} />);

      // Should display 'T' for 'Test Application'
      expect(screen.getByText('T')).toBeInTheDocument();
    });
  });

  describe('Navigation', () => {
    it('should navigate to application details on click when applicationId is provided', async () => {
      const user = userEvent.setup({advanceTimers: vi.advanceTimersByTime});
      mockNavigate.mockResolvedValue(undefined);

      renderWithRouter(<IntegrationGuide {...defaultProps} />);

      const appCard = screen.getByRole('button', {name: 'applications:onboarding.summary.viewAppAriaLabel'});
      await user.click(appCard);

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/applications/app-123');
      });
    });

    it('should navigate on Enter key press', async () => {
      mockNavigate.mockResolvedValue(undefined);

      renderWithRouter(<IntegrationGuide {...defaultProps} />);

      const appCard = screen.getByRole('button', {name: 'applications:onboarding.summary.viewAppAriaLabel'});
      fireEvent.keyDown(appCard, {key: 'Enter'});

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/applications/app-123');
      });
    });

    it('should navigate on Space key press', async () => {
      mockNavigate.mockResolvedValue(undefined);

      renderWithRouter(<IntegrationGuide {...defaultProps} />);

      const appCard = screen.getByRole('button', {name: 'applications:onboarding.summary.viewAppAriaLabel'});
      fireEvent.keyDown(appCard, {key: ' '});

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/applications/app-123');
      });
    });

    it('should not navigate on other key press', () => {
      renderWithRouter(<IntegrationGuide {...defaultProps} />);

      const appCard = screen.getByRole('button', {name: 'applications:onboarding.summary.viewAppAriaLabel'});
      fireEvent.keyDown(appCard, {key: 'Tab'});

      expect(mockNavigate).not.toHaveBeenCalled();
    });

    it('should not be clickable when applicationId is null', () => {
      const props = {
        ...defaultProps,
        applicationId: null,
      };

      renderWithRouter(<IntegrationGuide {...props} />);

      // Should not have button role when applicationId is null
      expect(
        screen.queryByRole('button', {name: 'applications:onboarding.summary.viewAppAriaLabel'}),
      ).not.toBeInTheDocument();
    });

    it('should handle navigation errors gracefully', async () => {
      const user = userEvent.setup({advanceTimers: vi.advanceTimersByTime});
      mockNavigate.mockRejectedValue(new Error('Navigation failed'));

      renderWithRouter(<IntegrationGuide {...defaultProps} />);

      const appCard = screen.getByRole('button', {name: 'applications:onboarding.summary.viewAppAriaLabel'});

      // Should not throw
      await user.click(appCard);

      expect(mockNavigate).toHaveBeenCalled();
    });
  });

  describe('Cleanup', () => {
    it('should clean up copy timeouts on unmount', async () => {
      const user = userEvent.setup({advanceTimers: vi.advanceTimersByTime});
      const props = {
        ...defaultProps,
        hasOAuthConfig: true,
        clientId: 'test_client_id',
      };

      const {unmount} = renderWithRouter(<IntegrationGuide {...props} />);

      const copyButtons = screen.getAllByRole('button');
      await user.click(copyButtons[0]);

      // Unmount before timeout completes
      unmount();

      // Advance timers - should not cause errors
      expect(() => {
        act(() => {
          vi.advanceTimersByTime(3000);
        });
      }).not.toThrow();
    });
  });
});
