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
          favicon: {light: 'assets/images/favicon.ico', dark: 'assets/images/favicon-inverted.ico'},
        },
      },
    }),
    useToast: () => ({showToast: mockShowToast}),
  };
});

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, unknown>) => {
      if (opts?.productName) {
        return `${key}:${opts.productName as string}`;
      }
      return key;
    },
  }),
}));

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {...actual, useNavigate: () => mockNavigate};
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
    AppWindow: () => <span data-testid="icon-app-window" />,
    ChevronRight: () => <span data-testid="icon-chevron-right" />,
    SkipForward: () => <span data-testid="icon-skip-forward" />,
    X: () => <span data-testid="icon-x" />,
  };
});

vi.mock('@/components/AppBreadcrumbs', () => ({
  default: ({items}: {items: {key: string; label: string; onClick?: () => void}[]}) => (
    <nav>
      {items.map((item) => (
        <span
          key={item.key}
          onClick={item.onClick}
          onKeyDown={
            item.onClick
              ? (e: React.KeyboardEvent) => {
                  if (e.key === 'Enter' || e.key === ' ') item.onClick?.();
                }
              : undefined
          }
          role={item.onClick ? 'button' : undefined}
        >
          {item.label}
        </span>
      ))}
    </nav>
  ),
}));

import GetStartedPage from '../GetStartedPage';

describe('GetStartedPage', () => {
  beforeEach(() => {
    vi.stubGlobal('sessionStorage', {
      setItem: mockSessionStorageSetItem,
      getItem: vi.fn(),
      removeItem: vi.fn(),
      clear: vi.fn(),
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  it('renders without crashing', () => {
    const {container} = render(<GetStartedPage />);
    expect(container).toBeInTheDocument();
  });

  it('renders close button', () => {
    render(<GetStartedPage />);
    expect(screen.getByRole('button', {name: /common:actions\.close/i})).toBeInTheDocument();
  });

  it('renders breadcrumb with welcome and create-project links', () => {
    render(<GetStartedPage />);
    expect(screen.getByText('common:welcome.header')).toBeInTheDocument();
    expect(screen.getByText('common:welcome.createProject.breadcrumb')).toBeInTheDocument();
    expect(screen.getByText('common:welcome.getStarted.breadcrumb')).toBeInTheDocument();
  });

  it('renders title and subtitle', () => {
    render(<GetStartedPage />);
    expect(screen.getByText('common:welcome.getStarted.title')).toBeInTheDocument();
    expect(screen.getByText(/common:welcome\.getStarted\.subtitle/i)).toBeInTheDocument();
  });

  it('renders onboard app option', () => {
    render(<GetStartedPage />);
    expect(screen.getByText('common:welcome.getStarted.options.onboardApp.title')).toBeInTheDocument();
    expect(screen.getByText(/common:welcome\.getStarted\.options\.onboardApp\.description/i)).toBeInTheDocument();
  });

  it('renders skip to console button', () => {
    render(<GetStartedPage />);
    expect(screen.getByText('common:welcome.getStarted.actions.skipToConsole')).toBeInTheDocument();
  });

  it('navigates to /welcome/get-started/applications/create on onboard app click', async () => {
    const user = userEvent.setup();
    render(<GetStartedPage />);

    const actionButton = screen.getByText('common:welcome.getStarted.options.onboardApp.action');
    await user.click(actionButton);

    expect(mockNavigate).toHaveBeenCalledWith('/welcome/get-started/applications/create');
  });

  it('navigates to /home and sets session storage on skip', async () => {
    const user = userEvent.setup();
    render(<GetStartedPage />);

    await user.click(screen.getByText('common:welcome.getStarted.actions.skipToConsole'));

    expect(mockSessionStorageSetItem).toHaveBeenCalledWith('thunderid:welcome:dismissed', 'true');
    expect(mockNavigate).toHaveBeenCalledWith('/home');
    expect(mockShowToast).toHaveBeenCalledWith('common:welcome.dismissed', 'info');
  });

  it('navigates to /home on close', async () => {
    const user = userEvent.setup();
    render(<GetStartedPage />);

    await user.click(screen.getByRole('button', {name: /common:actions\.close/i}));

    expect(mockNavigate).toHaveBeenCalledWith('/home');
  });

  it('navigates to /welcome on welcome breadcrumb click', async () => {
    const user = userEvent.setup();
    render(<GetStartedPage />);

    await user.click(screen.getByText('common:welcome.header'));

    expect(mockNavigate).toHaveBeenCalledWith('/welcome');
  });

  it('navigates to /welcome/create-project on create-project breadcrumb click', async () => {
    const user = userEvent.setup();
    render(<GetStartedPage />);

    await user.click(screen.getByText('common:welcome.createProject.breadcrumb'));

    expect(mockNavigate).toHaveBeenCalledWith('/welcome/create-project');
  });

  it('navigates to /welcome on welcome breadcrumb Enter keypress', () => {
    render(<GetStartedPage />);
    fireEvent.keyDown(screen.getByText('common:welcome.header'), {key: 'Enter'});
    expect(mockNavigate).toHaveBeenCalledWith('/welcome');
  });

  it('navigates to /welcome/create-project on create-project breadcrumb Enter keypress', () => {
    render(<GetStartedPage />);
    fireEvent.keyDown(screen.getByText('common:welcome.createProject.breadcrumb'), {key: 'Enter'});
    expect(mockNavigate).toHaveBeenCalledWith('/welcome/create-project');
  });
});
