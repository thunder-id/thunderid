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
import {afterEach, describe, expect, it, vi} from 'vitest';

const mockNavigate = vi.fn();

vi.mock('react-i18next', () => ({
  useTranslation: () => ({t: (key: string) => key}),
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
    ChevronRight: () => <span data-testid="icon-chevron-right" />,
    X: () => <span data-testid="icon-x" />,
    Settings: () => <span data-testid="icon-settings" />,
    PlayCircle: () => <span data-testid="icon-play-circle" />,
    CheckCircle: () => <span data-testid="icon-check-circle" />,
  };
});

vi.mock('@/assets/images/illustrations/how-solution-works.svg?react', () => ({
  default: () => <svg data-testid="illustration" />,
}));

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

import CreateProjectPage from '../CreateProjectPage';

afterEach(() => {
  vi.clearAllMocks();
});

describe('CreateProjectPage', () => {
  it('renders without crashing', () => {
    const {container} = render(<CreateProjectPage />);
    expect(container).toBeInTheDocument();
  });

  it('renders close button', () => {
    render(<CreateProjectPage />);
    expect(screen.getByRole('button', {name: 'common:actions.close'})).toBeInTheDocument();
  });

  it('renders the page title', () => {
    render(<CreateProjectPage />);
    expect(screen.getByText('common:welcome.createProject.title')).toBeInTheDocument();
  });

  it('renders the get started button', () => {
    render(<CreateProjectPage />);
    expect(screen.getByRole('button', {name: 'common:welcome.createProject.actions.getStarted'})).toBeInTheDocument();
  });

  it('navigates to /home when close button is clicked', async () => {
    const user = userEvent.setup();
    render(<CreateProjectPage />);

    await user.click(screen.getByRole('button', {name: 'common:actions.close'}));

    expect(mockNavigate).toHaveBeenCalledWith('/home');
  });

  it('navigates to /welcome/get-started when get started button is clicked', async () => {
    const user = userEvent.setup();
    render(<CreateProjectPage />);

    await user.click(screen.getByRole('button', {name: 'common:welcome.createProject.actions.getStarted'}));

    expect(mockNavigate).toHaveBeenCalledWith('/welcome/get-started');
  });

  it('renders breadcrumb with welcome header', () => {
    render(<CreateProjectPage />);
    expect(screen.getByText('common:welcome.header')).toBeInTheDocument();
  });

  it('navigates to /welcome when breadcrumb welcome is clicked', async () => {
    const user = userEvent.setup();
    render(<CreateProjectPage />);

    await user.click(screen.getByText('common:welcome.header'));

    expect(mockNavigate).toHaveBeenCalledWith('/welcome');
  });

  it('navigates to /welcome on breadcrumb welcome Enter keypress', () => {
    render(<CreateProjectPage />);
    fireEvent.keyDown(screen.getByText('common:welcome.header'), {key: 'Enter'});
    expect(mockNavigate).toHaveBeenCalledWith('/welcome');
  });

  it('navigates to /welcome on breadcrumb welcome Space keypress', () => {
    render(<CreateProjectPage />);
    fireEvent.keyDown(screen.getByText('common:welcome.header'), {key: ' '});
    expect(mockNavigate).toHaveBeenCalledWith('/welcome');
  });
});
