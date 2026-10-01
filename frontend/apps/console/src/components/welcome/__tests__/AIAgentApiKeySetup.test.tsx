// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent, act} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';

const {mockUseWayfinderReleases} = vi.hoisted(() => ({mockUseWayfinderReleases: vi.fn()}));

vi.mock('../../../api/useWayfinderReleases', () => ({
  default: (...args: unknown[]): unknown => mockUseWayfinderReleases(...args),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

vi.mock('@wso2/oxygen-ui-icons-react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui-icons-react')>();
  return {
    ...actual,
    Bot: () => <span data-testid="icon-bot" />,
    Check: () => <span data-testid="icon-check" />,
    Copy: () => <span data-testid="icon-copy" />,
    ExternalLink: () => <span data-testid="icon-external-link" />,
    FilePen: () => <span data-testid="icon-file-pen" />,
  };
});

import AIAgentApiKeySetup from '../AIAgentApiKeySetup';

const mockWriteText = vi.fn().mockResolvedValue(undefined);

describe('AIAgentApiKeySetup', () => {
  beforeEach(() => {
    Object.assign(navigator, {clipboard: {writeText: mockWriteText}});
    mockUseWayfinderReleases.mockReturnValue({data: undefined, isError: false});
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('renders the component header title', () => {
    render(<AIAgentApiKeySetup />);
    expect(screen.getByText('common:welcome.aiAgentsTryout.steps.configureSample.title')).toBeInTheDocument();
  });

  it('renders step 1 get key title and description', () => {
    render(<AIAgentApiKeySetup />);
    expect(screen.getByText('common:welcome.aiAgentsTryout.apiKeySetup.getKey.title')).toBeInTheDocument();
    expect(screen.getByText('common:welcome.aiAgentsTryout.apiKeySetup.getKey.description')).toBeInTheDocument();
  });

  it('renders Anthropic Console and Google AI Studio links', () => {
    render(<AIAgentApiKeySetup />);
    const anthropicLink = screen.getByRole('link', {name: /Anthropic Console/});
    expect(anthropicLink).toHaveAttribute('href', 'https://console.anthropic.com/settings/keys');
    expect(anthropicLink).toHaveAttribute('target', '_blank');

    const googleLink = screen.getByRole('link', {name: /Google AI Studio/});
    expect(googleLink).toHaveAttribute('href', 'https://aistudio.google.com/apikey');
    expect(googleLink).toHaveAttribute('target', '_blank');
  });

  it('renders step 2 set key title and description', () => {
    render(<AIAgentApiKeySetup />);
    expect(screen.getByText('common:welcome.aiAgentsTryout.apiKeySetup.setKey.title')).toBeInTheDocument();
    expect(screen.getByText('common:welcome.aiAgentsTryout.apiKeySetup.setKey.description')).toBeInTheDocument();
  });

  it('renders the env file block with filename label', () => {
    render(<AIAgentApiKeySetup />);
    expect(screen.getByText('ai-agent/.env')).toBeInTheDocument();
  });

  it('renders env variable keys and values', () => {
    render(<AIAgentApiKeySetup />);
    expect(screen.getByText('LLM_PROVIDER')).toBeInTheDocument();
    expect(screen.getByText('anthropic')).toBeInTheDocument();
    expect(screen.getByText('LLM_API_KEY')).toBeInTheDocument();
    expect(screen.getByText('your-llm-api-key')).toBeInTheDocument();
  });

  it('copies env content to clipboard when copy button is clicked', () => {
    render(<AIAgentApiKeySetup />);
    const copyButton = screen.getByRole('button', {name: /Copy snippet/i});
    fireEvent.click(copyButton);
    expect(mockWriteText).toHaveBeenCalledOnce();
    const written = mockWriteText.mock.calls[0][0] as string;
    expect(written).toContain('LLM_PROVIDER=anthropic');
    expect(written).toContain('LLM_API_KEY=your-llm-api-key');
  });

  it('shows check icon after copying', async () => {
    render(<AIAgentApiKeySetup />);
    expect(screen.getByTestId('icon-copy')).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', {name: /Copy snippet/i}));
    expect(await screen.findByTestId('icon-check')).toBeInTheDocument();
  });

  it('reverts copy icon back after 2 seconds', async () => {
    vi.useFakeTimers({toFake: ['setTimeout']});
    render(<AIAgentApiKeySetup />);
    await act(async () => {
      fireEvent.click(screen.getByRole('button', {name: /Copy snippet/i}));
      await mockWriteText.mock.results[0]?.value;
    });
    expect(screen.getByTestId('icon-check')).toBeInTheDocument();
    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(screen.getByTestId('icon-copy')).toBeInTheDocument();
    vi.useRealTimers();
  });

  it('shows versioned env file path when release data is available', () => {
    mockUseWayfinderReleases.mockReturnValue({
      data: {
        latestRelease: {
          tagName: 'v1.2.3',
          assets: [
            {
              name: 'sample-app-wayfinder-1.2.3.zip',
              downloadUrl: 'https://example.com/sample-app-wayfinder-1.2.3.zip',
              sizeLabel: '50 MB',
            },
          ],
        },
        releases: [],
      },
      isError: false,
    });
    render(<AIAgentApiKeySetup releasesUrl="https://example.com/releases.json" />);
    expect(screen.getByText('sample-app-wayfinder-1.2.3/ai-agent/.env')).toBeInTheDocument();
  });
});
