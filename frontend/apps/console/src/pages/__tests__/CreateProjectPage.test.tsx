// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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

vi.mock('@wso2/oxygen-ui', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui')>();
  return {
    ...actual,
    AppBreadcrumbs: ({items}: {items: {key: string; label: string; onClick?: () => void}[]}) => (
      <nav>
        {items.map((item) => (
          <span
            key={item.key}
            onClick={item.onClick}
            onKeyDown={(e: React.KeyboardEvent) => (e.key === 'Enter' || e.key === ' ') && item.onClick?.()}
            role={item.onClick ? 'button' : undefined}
            tabIndex={item.onClick ? 0 : undefined}
          >
            {item.label}
          </span>
        ))}
      </nav>
    ),
  };
});

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
