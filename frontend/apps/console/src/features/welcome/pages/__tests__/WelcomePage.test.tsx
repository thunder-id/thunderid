/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {render, screen, userEvent, fireEvent} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';

const mockNavigate = vi.fn();
const mockSessionStorageSetItem = vi.fn();

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({
      config: {
        brand: {
          product_name: 'ThunderID',
          documentation: {
            baseUrl: 'https://docs.example.com/',
            releasesUrl: 'https://docs.example.com/data/releases.json',
          },
          favicon: {light: 'assets/images/favicon.ico', dark: 'assets/images/favicon-inverted.ico'},
        },
      },
    }),
  };
});

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, unknown>) => {
      if (opts?.productName) {
        const productName = opts.productName as string;
        return `${key}:${productName}`;
      }
      return key;
    },
  }),
}));

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {...actual, useNavigate: () => mockNavigate};
});

vi.mock('@wso2/oxygen-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui')>();
  return {
    ...actual,
    useTheme: () => ({palette: {mode: 'light'}}),
  };
});

vi.mock('framer-motion', () => ({
  motion: {
    create: (Component: React.ElementType) => Component,
  },
}));

vi.mock('@wso2/oxygen-ui-icons-react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui-icons-react')>();
  return {
    ...actual,
    Bot: () => <span data-testid="icon-bot" />,
    ChevronRight: () => <span data-testid="icon-chevron-right" />,
    ExternalLink: () => <span data-testid="icon-external-link" />,
    FolderOpen: () => <span data-testid="icon-folder-open" />,
    Lightbulb: () => <span data-testid="icon-lightbulb" />,
    MCP: () => <span data-testid="icon-mcp" />,
    PackagePlus: () => <span data-testid="icon-package-plus" />,
    Users: () => <span data-testid="icon-users" />,
    X: () => <span data-testid="icon-x" />,
  };
});

import WelcomePage from '../WelcomePage';

describe('WelcomePage', () => {
  beforeEach(() => {
    vi.stubGlobal('sessionStorage', {
      setItem: mockSessionStorageSetItem,
      getItem: vi.fn(),
      removeItem: vi.fn(),
      clear: vi.fn(),
    });
    vi.stubGlobal('open', vi.fn());
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  it('renders without crashing', () => {
    const {container} = render(<WelcomePage />);
    expect(container).toBeInTheDocument();
  });

  it('renders close button', () => {
    render(<WelcomePage />);
    expect(screen.getByRole('button', {name: /common:actions\.close/i})).toBeInTheDocument();
  });

  it('navigates to /home and sets session storage on close', async () => {
    const user = userEvent.setup();
    render(<WelcomePage />);

    await user.click(screen.getByRole('button', {name: /common:actions\.close/i}));

    expect(mockSessionStorageSetItem).toHaveBeenCalledWith('thunderid:welcome:dismissed', 'true');
    expect(mockNavigate).toHaveBeenCalledWith('/home');
  });

  it('navigates to /welcome/create-project on new project click', async () => {
    const user = userEvent.setup();
    render(<WelcomePage />);

    const newProjectButton = screen.getByText('common:welcome.start.newProject');
    await user.click(newProjectButton.closest('[role="button"]') ?? newProjectButton);

    expect(mockSessionStorageSetItem).toHaveBeenCalledWith('thunderid:welcome:dismissed', 'true');
    expect(mockNavigate).toHaveBeenCalledWith('/welcome/create-project');
  });

  it('renders start action items', () => {
    render(<WelcomePage />);
    expect(screen.getByText('common:welcome.start.newProject')).toBeInTheDocument();
    expect(screen.getByText('common:welcome.start.openImport')).toBeInTheDocument();
  });

  it('renders learn product items', () => {
    render(<WelcomePage />);
    expect(screen.getByText('common:welcome.tryoutProduct.securingApplication')).toBeInTheDocument();
    expect(screen.getByText('common:welcome.tryoutProduct.aiAgents')).toBeInTheDocument();
  });

  it('renders walkthrough items', () => {
    render(<WelcomePage />);
    expect(screen.getByText('common:welcome.walkthrough.learnFundamentals')).toBeInTheDocument();
  });

  it('opens external link with noopener,noreferrer on walkthrough click', async () => {
    const mockOpen = vi.fn();
    vi.stubGlobal('open', mockOpen);
    const user = userEvent.setup();
    render(<WelcomePage />);

    const learnFundamentals = screen.getByText('common:welcome.walkthrough.learnFundamentals');
    await user.click(learnFundamentals);

    expect(mockOpen).toHaveBeenCalledWith(expect.any(String), '_blank', 'noopener,noreferrer');
  });

  it('renders learn product items that navigate on click', () => {
    render(<WelcomePage />);
    expect(screen.getByText('common:welcome.tryoutProduct.securingApplication')).toBeInTheDocument();
    expect(screen.getByText('common:welcome.tryoutProduct.aiAgents')).toBeInTheDocument();
  });

  it('uses product name from config', () => {
    render(<WelcomePage />);
    // The openImportDesc key is interpolated with productName
    expect(screen.getByText(/openImportDesc.*ThunderID/i)).toBeInTheDocument();
  });

  it('navigates to /welcome/open-project when open import is clicked', async () => {
    const user = userEvent.setup();
    render(<WelcomePage />);

    await user.click(screen.getByText('common:welcome.start.openImport'));

    expect(mockSessionStorageSetItem).toHaveBeenCalledWith('thunderid:welcome:dismissed', 'true');
    expect(mockNavigate).toHaveBeenCalledWith('/welcome/open-project');
  });

  it('navigates to /welcome/tryout/securing-application when securing application item is clicked', async () => {
    const user = userEvent.setup();
    render(<WelcomePage />);

    await user.click(screen.getByText('common:welcome.tryoutProduct.securingApplication'));

    expect(mockSessionStorageSetItem).toHaveBeenCalledWith('thunderid:welcome:dismissed', 'true');
    expect(mockNavigate).toHaveBeenCalledWith('/welcome/tryout/securing-application');
  });

  it('opens AI agents docs URL on ai-agents item click', async () => {
    const mockOpen = vi.fn();
    vi.stubGlobal('open', mockOpen);
    const user = userEvent.setup();
    render(<WelcomePage />);

    await user.click(screen.getByText('common:welcome.tryoutProduct.aiAgents'));

    expect(mockOpen).toHaveBeenCalledWith(
      'https://docs.example.com/use-cases/ai-agents/try-it-out',
      '_blank',
      'noopener,noreferrer',
    );
  });

  it('opens MCP docs URL on mcp item click', async () => {
    const mockOpen = vi.fn();
    vi.stubGlobal('open', mockOpen);
    const user = userEvent.setup();
    render(<WelcomePage />);

    await user.click(screen.getByText('common:welcome.tryoutProduct.mcp'));

    expect(mockOpen).toHaveBeenCalledWith(
      'https://docs.example.com/use-cases/ai-agents/mcp-authorization/try-it-out',
      '_blank',
      'noopener,noreferrer',
    );
  });

  it('triggers new project action on start card Enter keypress', () => {
    render(<WelcomePage />);
    const card = screen.getByText('common:welcome.start.newProject').closest('[role="button"]')!;
    fireEvent.keyDown(card, {key: 'Enter'});
    expect(mockSessionStorageSetItem).toHaveBeenCalledWith('thunderid:welcome:dismissed', 'true');
    expect(mockNavigate).toHaveBeenCalledWith('/welcome/create-project');
  });

  it('triggers open import action on start card Space keypress', () => {
    render(<WelcomePage />);
    const card = screen.getByText('common:welcome.start.openImport').closest('[role="button"]')!;
    fireEvent.keyDown(card, {key: ' '});
    expect(mockSessionStorageSetItem).toHaveBeenCalledWith('thunderid:welcome:dismissed', 'true');
    expect(mockNavigate).toHaveBeenCalledWith('/welcome/open-project');
  });

  it('triggers walkthrough action on Enter keypress', () => {
    const mockOpen = vi.fn();
    vi.stubGlobal('open', mockOpen);
    render(<WelcomePage />);
    const item = screen.getByText('common:welcome.walkthrough.learnFundamentals').closest('[role="button"]')!;
    fireEvent.keyDown(item, {key: 'Enter'});
    expect(mockOpen).toHaveBeenCalledWith('https://docs.example.com', '_blank', 'noopener,noreferrer');
  });

  it('triggers walkthrough action on Space keypress', () => {
    const mockOpen = vi.fn();
    vi.stubGlobal('open', mockOpen);
    render(<WelcomePage />);
    const item = screen.getByText('common:welcome.walkthrough.learnFundamentals').closest('[role="button"]')!;
    fireEvent.keyDown(item, {key: ' '});
    expect(mockOpen).toHaveBeenCalledWith('https://docs.example.com', '_blank', 'noopener,noreferrer');
  });

  it('ignores non-Enter/Space keypresses on start card', () => {
    render(<WelcomePage />);
    const card = screen.getByText('common:welcome.start.newProject').closest('[role="button"]')!;
    fireEvent.keyDown(card, {key: 'Tab'});
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it('triggers securing application action on learn product Enter keypress', () => {
    render(<WelcomePage />);
    const item = screen.getByText('common:welcome.tryoutProduct.securingApplication').closest('[role="button"]')!;
    fireEvent.keyDown(item, {key: 'Enter'});
    expect(mockSessionStorageSetItem).toHaveBeenCalledWith('thunderid:welcome:dismissed', 'true');
    expect(mockNavigate).toHaveBeenCalledWith('/welcome/tryout/securing-application');
  });

  it('triggers ai-agents action on learn product Space keypress', () => {
    const mockOpen = vi.fn();
    vi.stubGlobal('open', mockOpen);
    render(<WelcomePage />);
    const item = screen.getByText('common:welcome.tryoutProduct.aiAgents').closest('[role="button"]')!;
    fireEvent.keyDown(item, {key: ' '});
    expect(mockOpen).toHaveBeenCalledWith(
      'https://docs.example.com/use-cases/ai-agents/try-it-out',
      '_blank',
      'noopener,noreferrer',
    );
  });

  it('renders productName in hero title', () => {
    render(<WelcomePage />);
    expect(screen.getByText('ThunderID')).toBeInTheDocument();
  });

  it('renders hero subtitle', () => {
    render(<WelcomePage />);
    expect(screen.getByText('common:welcome.hero.subtitle')).toBeInTheDocument();
  });

  it('renders sections headings', () => {
    render(<WelcomePage />);
    expect(screen.getByText('common:welcome.sections.start')).toBeInTheDocument();
  });
});
