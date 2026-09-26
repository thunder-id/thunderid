// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, waitFor, fireEvent} from '@testing-library/react';
import type {IntegrationGuides} from '@thunderid/configure-applications';
import {LoggerProvider, LogLevel} from '@thunderid/logger';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import TechnologyGuide from '../TechnologyGuide';

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({
      config: {
        brand: {
          product_name: 'ThunderID',
          favicon: {light: 'assets/images/favicon.ico', dark: 'assets/images/favicon-inverted.ico'},
        },
      },
      getDocumentationLink: () => undefined,
    }),
  };
});

const REDIRECT_BASED_PROMPT_URL = 'https://thunderid.dev/prompts/react/redirect-based.txt';
const EMBEDDED_PROMPT_URL = 'https://thunderid.dev/prompts/react/embedded.txt';

const mockIntegrationGuides: IntegrationGuides = {
  REDIRECT_BASED: {
    llm_prompt: {
      docsUrl: REDIRECT_BASED_PROMPT_URL,
    },
  },
  EMBEDDED: {
    llm_prompt: {
      docsUrl: EMBEDDED_PROMPT_URL,
    },
  },
};

const promptFixtures: Record<string, string> = {
  [REDIRECT_BASED_PROMPT_URL]: 'Integrate with clientId: {{clientId}} and applicationId: {{applicationId}}',
  [EMBEDDED_PROMPT_URL]: 'Embedded integration prompt',
};

const mockWriteText = vi.fn();
const mockFetch = vi.fn();

const renderWithProviders = (component: React.ReactElement) =>
  render(<LoggerProvider logger={{level: LogLevel.DEBUG}}>{component}</LoggerProvider>);

describe('TechnologyGuide', () => {
  const originalClipboard = navigator.clipboard;

  beforeEach(() => {
    vi.useFakeTimers({shouldAdvanceTime: true});
    vi.clearAllMocks();
    mockWriteText.mockResolvedValue(undefined);
    mockFetch.mockImplementation((url: string) =>
      Promise.resolve({
        ok: true,
        status: 200,
        text: () => Promise.resolve(promptFixtures[url] ?? ''),
      }),
    );
    vi.stubGlobal('fetch', mockFetch);
    Object.defineProperty(navigator, 'clipboard', {
      value: {
        writeText: mockWriteText,
      },
      writable: true,
      configurable: true,
    });
  });

  afterEach(() => {
    vi.runOnlyPendingTimers();
    vi.useRealTimers();
    vi.unstubAllGlobals();
    Object.defineProperty(navigator, 'clipboard', {
      value: originalClipboard,
      writable: true,
      configurable: true,
    });
  });

  describe('Rendering', () => {
    it('should return null when guides is null', () => {
      const {container} = renderWithProviders(<TechnologyGuide guides={null} />);

      expect(container.firstChild?.firstChild).toBeFalsy();
    });

    it('should return null when selected guide is not found', () => {
      const guidesWithoutRedirectBased: IntegrationGuides = {
        OTHER: mockIntegrationGuides.REDIRECT_BASED,
      };

      const {container} = renderWithProviders(
        <TechnologyGuide guides={guidesWithoutRedirectBased} templateId="react" />,
      );

      expect(container.firstChild?.firstChild).toBeFalsy();
    });

    it('should render redirect-based guide for non-embedded template', async () => {
      renderWithProviders(<TechnologyGuide guides={mockIntegrationGuides} templateId="react" />);

      fireEvent.click(screen.getByTestId('copy-prompt-button'));

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalledWith(REDIRECT_BASED_PROMPT_URL);
        expect(mockWriteText).toHaveBeenCalledWith(
          'Integrate with clientId: {{clientId}} and applicationId: {{applicationId}}',
        );
      });
    });

    it('should render embedded guide for embedded template', async () => {
      renderWithProviders(<TechnologyGuide guides={mockIntegrationGuides} templateId="react-embedded" />);

      fireEvent.click(screen.getByTestId('copy-prompt-button'));

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalledWith(EMBEDDED_PROMPT_URL);
        expect(mockWriteText).toHaveBeenCalledWith('Embedded integration prompt');
      });
    });

    it('should default to redirect-based guide when templateId is null', () => {
      renderWithProviders(<TechnologyGuide guides={mockIntegrationGuides} templateId={null} />);

      expect(screen.getByText('Integrate with a coding agent')).toBeInTheDocument();
    });
  });

  describe('LLM Prompt Section', () => {
    it('should render LLM prompt card with title and description', () => {
      renderWithProviders(<TechnologyGuide guides={mockIntegrationGuides} templateId="react" clientId="client-123" />);

      expect(screen.getByText('Integrate with a coding agent')).toBeInTheDocument();
      expect(screen.getByText('Copy a ready-made prompt for Claude, Cursor, or any agent.')).toBeInTheDocument();
    });

    it('should render copy prompt button', () => {
      renderWithProviders(<TechnologyGuide guides={mockIntegrationGuides} templateId="react" />);

      expect(screen.getByTestId('copy-prompt-button')).toBeInTheDocument();
      expect(screen.getByText('Copy Prompt')).toBeInTheDocument();
    });
  });

  describe('Empty States', () => {
    it('should not render copy prompt button when llm_prompt has no docsUrl', () => {
      const guidesWithoutDocsUrl: IntegrationGuides = {
        REDIRECT_BASED: {
          llm_prompt: {},
        },
      };

      renderWithProviders(<TechnologyGuide guides={guidesWithoutDocsUrl} templateId="react" />);

      expect(screen.queryByTestId('copy-prompt-button')).not.toBeInTheDocument();
    });
  });

  describe('Placeholder Replacement', () => {
    it('should replace {{applicationId}} placeholder in LLM prompt when copied', async () => {
      renderWithProviders(
        <TechnologyGuide
          guides={mockIntegrationGuides}
          templateId="react"
          clientId="test-client"
          applicationId="test-app-id"
        />,
      );

      const copyButton = screen.getByTestId('copy-prompt-button');
      fireEvent.click(copyButton);

      await waitFor(() => {
        expect(mockWriteText).toHaveBeenCalledWith(
          'Integrate with clientId: test-client and applicationId: test-app-id',
        );
      });
    });

    it('should not replace applicationId placeholder when applicationId is empty', async () => {
      renderWithProviders(
        <TechnologyGuide guides={mockIntegrationGuides} templateId="react" clientId="test-client" applicationId="" />,
      );

      const copyButton = screen.getByTestId('copy-prompt-button');
      fireEvent.click(copyButton);

      await waitFor(() => {
        expect(mockWriteText).toHaveBeenCalledWith(
          'Integrate with clientId: test-client and applicationId: {{applicationId}}',
        );
      });
    });
  });

  describe('Copy Functionality', () => {
    it('should copy LLM prompt to clipboard when copy button is clicked', async () => {
      renderWithProviders(<TechnologyGuide guides={mockIntegrationGuides} templateId="react" />);

      const copyButton = screen.getByTestId('copy-prompt-button');
      fireEvent.click(copyButton);

      await waitFor(() => {
        expect(mockWriteText).toHaveBeenCalledWith(
          'Integrate with clientId: {{clientId}} and applicationId: {{applicationId}}',
        );
      });
    });

    it('should show copied feedback after copying prompt', async () => {
      renderWithProviders(<TechnologyGuide guides={mockIntegrationGuides} templateId="react" />);

      const copyButton = screen.getByTestId('copy-prompt-button');
      fireEvent.click(copyButton);

      // The copied feedback is shown in the Tooltip
      await waitFor(() => {
        expect(screen.getByText('Copied to clipboard')).toBeInTheDocument();
      });
    });

    it('should not call fetch when prompt has no docsUrl', () => {
      const guidesWithoutDocsUrl: IntegrationGuides = {
        REDIRECT_BASED: {
          llm_prompt: {
            docsUrl: '',
          },
        },
      };

      renderWithProviders(<TechnologyGuide guides={guidesWithoutDocsUrl} templateId="react" />);

      // Button should not render when docsUrl is an empty string
      expect(screen.queryByTestId('copy-prompt-button')).toBeNull();
      expect(mockFetch).not.toHaveBeenCalled();
      expect(mockWriteText).not.toHaveBeenCalled();
    });

    it('should log an error and not copy when fetching the prompt fails', async () => {
      mockFetch.mockResolvedValue({ok: false, status: 500, text: () => Promise.resolve('')});

      renderWithProviders(<TechnologyGuide guides={mockIntegrationGuides} templateId="react" />);

      const copyButton = screen.getByTestId('copy-prompt-button');
      fireEvent.click(copyButton);

      await waitFor(() => {
        expect(mockFetch).toHaveBeenCalledWith(REDIRECT_BASED_PROMPT_URL);
      });
      expect(mockWriteText).not.toHaveBeenCalled();
    });

    describe('Clipboard Fallback', () => {
      it('should use fallback method when clipboard API fails for prompt', async () => {
        mockWriteText.mockRejectedValue(new Error('Clipboard API failed'));

        const mockExecCommand = vi.fn().mockReturnValue(true);
        document.execCommand = mockExecCommand;

        renderWithProviders(<TechnologyGuide guides={mockIntegrationGuides} templateId="react" />);

        const copyButton = screen.getByTestId('copy-prompt-button');
        fireEvent.click(copyButton);

        await waitFor(() => {
          expect(mockExecCommand).toHaveBeenCalledWith('copy');
        });
      });

      it('should handle fallback failure gracefully for prompt', () => {
        mockWriteText.mockRejectedValue(new Error('Clipboard API failed'));

        const mockExecCommand = vi.fn().mockImplementation(() => {
          throw new Error('execCommand failed');
        });
        document.execCommand = mockExecCommand;

        renderWithProviders(<TechnologyGuide guides={mockIntegrationGuides} templateId="react" />);

        const copyButton = screen.getByTestId('copy-prompt-button');

        // Should not throw - component handles error gracefully
        expect(() => fireEvent.click(copyButton)).not.toThrow();
      });
    });
  });
});
