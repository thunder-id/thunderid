// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@thunderid/test-utils';
import {afterEach, describe, it, expect, vi, beforeEach} from 'vitest';
import SignIn from '../SignIn';

// Mock child components
vi.mock('../SignInBox', () => ({
  default: ({
    isCompletingSignIn = false,
    onSignInStepReady = undefined,
  }: {
    isCompletingSignIn?: boolean;
    onSignInStepReady?: () => void;
  }) => (
    <div data-testid="signin-box" data-completing={String(isCompletingSignIn)}>
      <button type="button" onClick={onSignInStepReady}>
        step ready
      </button>
    </div>
  ),
}));

vi.mock('../SignInSlogan', () => ({
  default: () => <div data-testid="signin-slogan">SignInSlogan</div>,
}));

vi.mock('../SignInCallbackLoading', () => ({
  default: () => <div data-testid="signin-callback-loading">SignInCallbackLoading</div>,
}));

// Mock useDesign hook
const mockUseDesign = vi.fn();
vi.mock('@thunderid/design', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/design')>();
  return {
    ...actual,
    // eslint-disable-next-line @typescript-eslint/no-unsafe-return
    useDesign: () => mockUseDesign(),
  };
});

describe('SignIn', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseDesign.mockReturnValue({
      isDesignEnabled: false,
      isLoading: false,
    });
  });

  it('renders without crashing', () => {
    const {container} = render(<SignIn />);
    expect(container).toBeInTheDocument();
  });

  it('renders SignInBox component', () => {
    render(<SignIn />);
    expect(screen.getByTestId('signin-box')).toBeInTheDocument();
  });

  it('shows SignInSlogan when design is not enabled and not loading', () => {
    mockUseDesign.mockReturnValue({
      isDesignEnabled: false,
      isLoading: false,
    });
    render(<SignIn />);
    expect(screen.getByTestId('signin-slogan')).toBeInTheDocument();
  });

  it('hides SignInSlogan when design is enabled', () => {
    mockUseDesign.mockReturnValue({
      isDesignEnabled: true,
      isLoading: false,
    });
    render(<SignIn />);
    expect(screen.queryByTestId('signin-slogan')).not.toBeInTheDocument();
  });

  it('hides SignInSlogan while design is loading', () => {
    mockUseDesign.mockReturnValue({
      isDesignEnabled: false,
      isLoading: true,
    });
    render(<SignIn />);
    expect(screen.queryByTestId('signin-slogan')).not.toBeInTheDocument();
  });

  it('does not show the callback loading state on a plain visit', () => {
    render(<SignIn />);
    expect(screen.queryByTestId('signin-callback-loading')).not.toBeInTheDocument();
    expect(screen.getByTestId('signin-box')).toHaveAttribute('data-completing', 'false');
  });

  describe('when returning from a social identity provider', () => {
    afterEach(() => {
      window.history.replaceState({}, '', '/');
    });

    it.each(['?code=abc&state=xyz', '?code=abc', '?error=access_denied', '?error=access_denied&state=xyz'])(
      'shows the callback loading state instead of the slogan for %s',
      (search) => {
        window.history.replaceState({}, '', `/signin${search}`);
        render(<SignIn />);
        expect(screen.getByTestId('signin-callback-loading')).toBeInTheDocument();
        expect(screen.queryByTestId('signin-slogan')).not.toBeInTheDocument();
        expect(screen.getByTestId('signin-box')).toHaveAttribute('data-completing', 'true');
        expect(window.location.search).toBe(search);
      },
    );

    // The SDK neither completes a state without a code or an error nor starts a new flow while it is
    // in the URL, so waiting for it would never end.
    it('drops a state without a code or an error and signs in as usual', () => {
      window.history.replaceState({}, '', '/signin?applicationId=app-1&state=xyz');
      render(<SignIn />);
      expect(screen.queryByTestId('signin-callback-loading')).not.toBeInTheDocument();
      expect(screen.getByTestId('signin-slogan')).toBeInTheDocument();
      expect(screen.getByTestId('signin-box')).toHaveAttribute('data-completing', 'false');
      expect(window.location.pathname).toBe('/signin');
      expect(window.location.search).toBe('?applicationId=app-1');
    });

    it('keeps the callback loading state while design is loading', () => {
      window.history.replaceState({}, '', '/signin?code=abc&state=xyz');
      mockUseDesign.mockReturnValue({
        isDesignEnabled: false,
        isLoading: true,
      });
      render(<SignIn />);
      expect(screen.getByTestId('signin-callback-loading')).toBeInTheDocument();
      expect(screen.queryByTestId('signin-box')).not.toBeInTheDocument();
    });

    it('returns to the slogan and card once the flow has a step to show', () => {
      window.history.replaceState({}, '', '/signin?code=abc&state=xyz');
      render(<SignIn />);
      fireEvent.click(screen.getByRole('button', {name: 'step ready'}));
      expect(screen.queryByTestId('signin-callback-loading')).not.toBeInTheDocument();
      expect(screen.getByTestId('signin-slogan')).toBeInTheDocument();
      expect(screen.getByTestId('signin-box')).toHaveAttribute('data-completing', 'false');
    });
  });
});
