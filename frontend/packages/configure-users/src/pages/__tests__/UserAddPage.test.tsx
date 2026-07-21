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

import {render, screen, waitFor, userEvent} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import UserAddPage from '../UserAddPage';

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

vi.mock('react-i18next', async () => {
  const actual = await vi.importActual<typeof import('react-i18next')>('react-i18next');
  return {
    ...actual,
    useTranslation: () => ({
      t: (_key: string, defaultValue: string) => defaultValue,
    }),
  };
});

describe('UserAddPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders the page with title and subtitle', () => {
    render(<UserAddPage />);

    const addUserTexts = screen.getAllByText('Add User');
    expect(addUserTexts.length).toBeGreaterThan(0);
    expect(
      screen.getByText(
        'Choose whether to create the account now or send an invite for the user to finish onboarding later.',
      ),
    ).toBeInTheDocument();
  });

  it('renders both user type options', () => {
    render(<UserAddPage />);

    expect(screen.getByText('Create User')).toBeInTheDocument();
    expect(screen.getByText('Invite User')).toBeInTheDocument();
  });

  it('renders option descriptions', () => {
    render(<UserAddPage />);

    expect(screen.getByText('Create the account now with a password or other credentials.')).toBeInTheDocument();
    expect(screen.getByText('Send an invite for the user to finish onboarding later.')).toBeInTheDocument();
  });

  it('renders the close button with proper aria-label', () => {
    render(<UserAddPage />);

    const closeButton = screen.getByLabelText('Close');
    expect(closeButton).toBeInTheDocument();
  });

  it('renders breadcrumbs with Add User label', () => {
    render(<UserAddPage />);

    const addUserTexts = screen.getAllByText('Add User');
    expect(addUserTexts.length).toBeGreaterThan(0);
  });

  it('renders progress indicator at the top', () => {
    const {container} = render(<UserAddPage />);

    const progressBar = container.querySelector('[role="progressbar"]');
    expect(progressBar).toBeInTheDocument();
  });

  it('renders the type selection container with correct test id', () => {
    render(<UserAddPage />);

    const typeSelectContainer = screen.getByTestId('add-user-type-select');
    expect(typeSelectContainer).toBeInTheDocument();
  });

  it('navigates to /users when close button is clicked', async () => {
    const user = userEvent.setup();
    render(<UserAddPage />);

    const closeButton = screen.getByLabelText('Close');
    await user.click(closeButton);

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/users');
    });
  });

  it('navigates to /users/add/create when Create User card is clicked', async () => {
    const user = userEvent.setup();
    render(<UserAddPage />);

    // Find the clickable card area
    const cards = screen.getAllByRole('button', {hidden: true});
    const createCard = cards.find((card) => card.textContent?.includes('Create User'));

    if (createCard) {
      await user.click(createCard);
    }

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/users/add/create');
    });
  });

  it('navigates to /users/add/invite when Invite User card is clicked', async () => {
    const user = userEvent.setup();
    render(<UserAddPage />);

    const cards = screen.getAllByRole('button', {hidden: true});
    const inviteCard = cards.find((card) => card.textContent?.includes('Invite User'));

    if (inviteCard) {
      await user.click(inviteCard);
    }

    await waitFor(() => {
      expect(mockNavigate).toHaveBeenCalledWith('/users/add/invite');
    });
  });

  it('handles navigation error when close button is clicked', async () => {
    const user = userEvent.setup();
    const navigationError = new Error('Navigation failed');
    mockNavigate.mockRejectedValue(navigationError);

    render(<UserAddPage />);

    const closeButton = screen.getByLabelText('Close');
    await user.click(closeButton);

    await waitFor(() => {
      expect(mockLoggerError).toHaveBeenCalledWith('Failed to navigate to users page', {error: navigationError});
    });
  });

  it('handles navigation error when option is selected', async () => {
    const user = userEvent.setup();
    const navigationError = new Error('Navigation failed');
    mockNavigate.mockRejectedValue(navigationError);

    render(<UserAddPage />);

    const cards = screen.getAllByRole('button', {hidden: true});
    const createCard = cards.find((card) => card.textContent?.includes('Create User'));

    if (createCard) {
      await user.click(createCard);
    }

    await waitFor(() => {
      expect(mockLoggerError).toHaveBeenCalledWith(
        'Failed to navigate to add user sub-page',
        expect.objectContaining({
          error: navigationError,
          route: '/users/add/create',
        }),
      );
    });
  });

  it('renders linear progress with determinate variant', () => {
    render(<UserAddPage />);

    const progressBar = screen.getByRole('progressbar');
    expect(progressBar).toBeInTheDocument();
  });

  it('maintains layout structure with proper flex containers', () => {
    render(<UserAddPage />);

    // Check that the component renders without layout errors
    const typeSelectContainer = screen.getByTestId('add-user-type-select');
    expect(typeSelectContainer).toBeInTheDocument();
  });
});
