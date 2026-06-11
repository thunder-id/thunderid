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
const mockShowToast = vi.fn();

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
    useToast: () => ({showToast: mockShowToast}),
  };
});

vi.mock('react-i18next', () => ({
  Trans: ({i18nKey, components = {}}: {i18nKey: string; components?: Record<string, React.ReactElement>}) => (
    <span>
      {i18nKey}
      {components?.a}
      {components?.mail}
    </span>
  ),
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, unknown>) => {
      if (opts?.productName) return `${key}:${opts.productName as string}`;
      return key;
    },
  }),
}));

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {...actual, useNavigate: () => mockNavigate};
});

vi.mock('framer-motion', () => ({
  motion: {create: (Component: React.ElementType) => Component},
}));

vi.mock('@wso2/oxygen-ui-icons-react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui-icons-react')>();
  return {
    ...actual,
    AppWindow: () => <span data-testid="icon-app-window" />,
    BookOpen: () => <span data-testid="icon-book-open" />,
    Check: () => <span data-testid="icon-check" />,
    ChevronRight: () => <span data-testid="icon-chevron-right" />,
    Copy: () => <span data-testid="icon-copy" />,
    ExternalLink: () => <span data-testid="icon-external-link" />,
    Eye: () => <span data-testid="icon-eye" />,
    EyeOff: () => <span data-testid="icon-eye-off" />,
    KeyRound: () => <span data-testid="icon-key-round" />,
    LogIn: () => <span data-testid="icon-log-in" />,
    UserCheck: () => <span data-testid="icon-user-check" />,
    UserCircle: () => <span data-testid="icon-user-circle" />,
    UserPlus: () => <span data-testid="icon-user-plus" />,
    X: () => <span data-testid="icon-x" />,
  };
});

vi.mock('../../components/WayfinderSampleSetup', () => ({
  default: () => <div data-testid="wayfinder-sample-setup" />,
}));

vi.mock('@/components/AppBreadcrumbs', () => ({
  default: ({items}: {items: {key: string; label: string; onClick?: () => void}[]}) => (
    <nav>
      {items.map((item) => (
        <span
          key={item.key}
          onClick={item.onClick}
          onKeyDown={(e) => (e.key === 'Enter' || e.key === ' ') && item.onClick?.()}
          role={item.onClick ? 'button' : undefined}
          tabIndex={item.onClick ? 0 : undefined}
        >
          {item.label}
        </span>
      ))}
    </nav>
  ),
}));

import TryoutSecuringApplicationPage from '../TryoutSecuringApplicationPage';

describe('TryoutSecuringApplicationPage', () => {
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
    const {container} = render(<TryoutSecuringApplicationPage />);
    expect(container).toBeInTheDocument();
  });

  it('renders close button', () => {
    render(<TryoutSecuringApplicationPage />);
    expect(screen.getByRole('button', {name: /common:actions\.close/i})).toBeInTheDocument();
  });

  it('renders breadcrumb', () => {
    render(<TryoutSecuringApplicationPage />);
    expect(screen.getByText('common:welcome.header')).toBeInTheDocument();
    expect(screen.getByText('common:welcome.applicationTryout.breadcrumb')).toBeInTheDocument();
  });

  it('renders overline and title', () => {
    render(<TryoutSecuringApplicationPage />);
    expect(screen.getByText('common:welcome.applicationTryout.overline')).toBeInTheDocument();
    expect(screen.getByText('common:welcome.tryout.title')).toBeInTheDocument();
  });

  it('renders WayfinderSampleSetup', () => {
    render(<TryoutSecuringApplicationPage />);
    expect(screen.getByTestId('wayfinder-sample-setup')).toBeInTheDocument();
  });

  it('renders scenario tabs', () => {
    render(<TryoutSecuringApplicationPage />);
    expect(screen.getByText('common:welcome.applicationTryout.scenarios.tabs.login')).toBeInTheDocument();
    expect(screen.getByText('common:welcome.applicationTryout.scenarios.tabs.signup')).toBeInTheDocument();
    expect(screen.getByText('common:welcome.applicationTryout.scenarios.tabs.profile')).toBeInTheDocument();
    expect(screen.getByText('common:welcome.applicationTryout.scenarios.tabs.recovery')).toBeInTheDocument();
    expect(screen.getByText('common:welcome.applicationTryout.scenarios.tabs.onboard')).toBeInTheDocument();
  });

  it('shows login scenario by default', () => {
    render(<TryoutSecuringApplicationPage />);
    expect(screen.getByText('common:welcome.applicationTryout.scenarios.login.description')).toBeInTheDocument();
  });

  it('navigates to /home and sets session storage on close', async () => {
    const user = userEvent.setup();
    render(<TryoutSecuringApplicationPage />);

    await user.click(screen.getByRole('button', {name: /common:actions\.close/i}));

    expect(mockSessionStorageSetItem).toHaveBeenCalledWith('thunderid:welcome:dismissed', 'true');
    expect(mockNavigate).toHaveBeenCalledWith('/home');
    expect(mockShowToast).toHaveBeenCalledWith('common:welcome.dismissed', 'info');
  });

  it('navigates to /welcome on breadcrumb welcome click', async () => {
    const user = userEvent.setup();
    render(<TryoutSecuringApplicationPage />);

    await user.click(screen.getByText('common:welcome.header'));

    expect(mockNavigate).toHaveBeenCalledWith('/welcome');
  });

  it('opens docs URL on read docs click', async () => {
    const mockOpen = vi.fn();
    vi.stubGlobal('open', mockOpen);
    const user = userEvent.setup();
    render(<TryoutSecuringApplicationPage />);

    await user.click(screen.getByText('common:welcome.tryout.actions.readDocs'));

    expect(mockOpen).toHaveBeenCalledWith(
      'https://docs.example.com/use-cases/b2c/try-it-out',
      '_blank',
      'noopener,noreferrer',
    );
  });

  it('navigates to /welcome on breadcrumb Enter keypress', () => {
    render(<TryoutSecuringApplicationPage />);
    fireEvent.keyDown(screen.getByText('common:welcome.header'), {key: 'Enter'});
    expect(mockNavigate).toHaveBeenCalledWith('/welcome');
  });

  it('shows signup scenario when signup tab is clicked', async () => {
    const user = userEvent.setup();
    render(<TryoutSecuringApplicationPage />);

    await user.click(screen.getByText('common:welcome.applicationTryout.scenarios.tabs.signup'));

    expect(
      screen.getByText('common:welcome.applicationTryout.scenarios.signup.description:ThunderID'),
    ).toBeInTheDocument();
  });

  it('shows profile scenario when profile tab is clicked', async () => {
    const user = userEvent.setup();
    render(<TryoutSecuringApplicationPage />);

    await user.click(screen.getByText('common:welcome.applicationTryout.scenarios.tabs.profile'));

    expect(screen.getByText('common:welcome.applicationTryout.scenarios.profile.description')).toBeInTheDocument();
  });

  it('shows recovery scenario with sample-app and mail-inbox links when recovery tab is clicked', async () => {
    const user = userEvent.setup();
    render(<TryoutSecuringApplicationPage />);

    await user.click(screen.getByText('common:welcome.applicationTryout.scenarios.tabs.recovery'));

    expect(screen.getByText('common:welcome.applicationTryout.scenarios.recovery.description')).toBeInTheDocument();
    const hrefs = screen.getAllByRole('link').map((a) => a.getAttribute('href'));
    expect(hrefs).toContain('http://localhost:5173');
    expect(hrefs).toContain('http://localhost:8788');
  });

  it('shows onboard scenario with a mail-inbox link when onboard tab is clicked', async () => {
    const user = userEvent.setup();
    render(<TryoutSecuringApplicationPage />);

    await user.click(screen.getByText('common:welcome.applicationTryout.scenarios.tabs.onboard'));

    expect(
      screen.getByText('common:welcome.applicationTryout.scenarios.onboard.description:ThunderID'),
    ).toBeInTheDocument();
    const hrefs = screen.getAllByRole('link').map((a) => a.getAttribute('href'));
    expect(hrefs).toContain('http://localhost:8788');
  });

  describe('credential interactions', () => {
    let writeTextSpy: ReturnType<typeof vi.fn>;

    beforeAll(() => {
      Object.defineProperty(navigator, 'clipboard', {
        value: {writeText: vi.fn()},
        writable: true,
        configurable: true,
      });
    });

    beforeEach(() => {
      writeTextSpy = vi.spyOn(navigator.clipboard, 'writeText').mockResolvedValue(undefined);
    });

    afterEach(() => {
      vi.restoreAllMocks();
    });

    it('toggles password visibility in credentials block', async () => {
      const user = userEvent.setup();
      render(<TryoutSecuringApplicationPage />);

      await user.click(screen.getAllByRole('button', {name: 'Show password'})[0]);

      expect(screen.getAllByRole('button', {name: 'Hide password'})[0]).toBeInTheDocument();
    });

    it('copies username to clipboard in credentials block', async () => {
      const user = userEvent.setup();
      render(<TryoutSecuringApplicationPage />);

      await user.click(screen.getAllByRole('button', {name: 'Copy username'})[0]);

      expect(writeTextSpy).toHaveBeenCalledWith('john.doe');
    });

    it('copies a form field when signup tab copy button is clicked', async () => {
      const user = userEvent.setup();
      render(<TryoutSecuringApplicationPage />);

      await user.click(screen.getByText('common:welcome.applicationTryout.scenarios.tabs.signup'));
      const copyButtons = screen.getAllByRole('button', {
        name: /^Copy common:welcome\.applicationTryout\.scenarios\.signup\.sampleFields\./,
      });
      await user.click(copyButtons[0]);

      expect(writeTextSpy).toHaveBeenCalledWith('emma.wilson');
    });
  });
});
