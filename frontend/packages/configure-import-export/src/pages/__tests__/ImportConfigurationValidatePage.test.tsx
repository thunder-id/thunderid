// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, userEvent, act} from '@thunderid/test-utils';
import {afterEach, describe, expect, it, vi} from 'vitest';

const mockNavigate = vi.fn();
let mockLocationState: unknown = null;

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useLocation: () => ({state: mockLocationState, pathname: '/welcome/import-configuration/validate'}),
  };
});

const mockLogger = {error: vi.fn(), warn: vi.fn(), info: vi.fn(), debug: vi.fn()};

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => mockLogger,
}));

vi.mock('@wso2/oxygen-ui-icons-react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui-icons-react')>();
  return {
    ...actual,
    ChevronRight: () => <span data-testid="icon-chevron-right" />,
    X: () => <span data-testid="icon-x" />,
    CheckCircle: () => <span data-testid="icon-check-circle" />,
    AlertCircle: () => <span data-testid="icon-alert-circle" />,
  };
});

import ImportConfigurationValidatePage from '../ImportConfigurationValidatePage';

afterEach(() => {
  vi.clearAllMocks();
  mockLocationState = null;
});

describe('ImportConfigurationValidatePage', () => {
  it('renders without crashing', () => {
    const {container} = render(<ImportConfigurationValidatePage />);
    expect(container).toBeInTheDocument();
  });

  it('renders validate title', () => {
    render(<ImportConfigurationValidatePage />);
    expect(screen.getByText('Validating configuration')).toBeInTheDocument();
  });

  it('renders the four validation steps', () => {
    render(<ImportConfigurationValidatePage />);
    expect(screen.getByText('Reading configuration file')).toBeInTheDocument();
    expect(screen.getByText('Validating YAML syntax')).toBeInTheDocument();
    expect(screen.getByText('Checking compatibility')).toBeInTheDocument();
    expect(screen.getByText('Validating resources')).toBeInTheDocument();
  });

  it('renders close button', () => {
    render(<ImportConfigurationValidatePage />);
    expect(screen.getByRole('button', {name: 'Close'})).toBeInTheDocument();
  });

  it('navigates to /home on close', async () => {
    const user = userEvent.setup();
    render(<ImportConfigurationValidatePage />);

    await user.click(screen.getByRole('button', {name: 'Close'}));

    expect(mockNavigate).toHaveBeenCalledWith('/home');
  });

  it('navigates to /welcome on cancel (no errors)', () => {
    mockLocationState = {parseErrors: [], configData: {application: []}};
    render(<ImportConfigurationValidatePage />);

    const cancelButton = screen.queryByRole('button', {name: 'Cancel'});
    // Cancel is only shown when there are parse errors
    expect(cancelButton).not.toBeInTheDocument();
  });

  it('shows parse errors when state has parse errors', () => {
    mockLocationState = {
      parseErrors: [{resourceType: 'unknown_type', fileName: 'bad.yaml', error: 'unexpected token'}],
      parseStats: {successCount: 2, failCount: 1},
    };

    render(<ImportConfigurationValidatePage />);

    expect(screen.getByRole('alert')).toBeInTheDocument();
  });

  it('shows upload different file button when parse errors exist', () => {
    mockLocationState = {
      parseErrors: [{resourceType: 'bad_type', fileName: 'config.yaml', error: 'parse error'}],
      parseStats: {successCount: 0, failCount: 1},
    };

    render(<ImportConfigurationValidatePage />);

    expect(screen.getByRole('button', {name: 'Upload different file'})).toBeInTheDocument();
  });

  it('navigates to /welcome/import-configuration when upload different file is clicked', async () => {
    mockLocationState = {
      parseErrors: [{resourceType: 'bad_type', fileName: 'config.yaml', error: 'parse error'}],
      parseStats: {successCount: 0, failCount: 1},
    };

    const user = userEvent.setup();
    render(<ImportConfigurationValidatePage />);

    await user.click(screen.getByRole('button', {name: 'Upload different file'}));

    expect(mockNavigate).toHaveBeenCalledWith('/welcome/import-configuration');
  });

  it('renders breadcrumb with welcome header', () => {
    render(<ImportConfigurationValidatePage />);
    expect(screen.getByText('Welcome')).toBeInTheDocument();
  });

  it('navigates to /welcome when breadcrumb welcome is clicked', async () => {
    const user = userEvent.setup();
    render(<ImportConfigurationValidatePage />);

    await user.click(screen.getByText('Welcome'));

    expect(mockNavigate).toHaveBeenCalledWith('/welcome');
  });

  it('navigates to /home on cancel when parse errors exist', async () => {
    mockLocationState = {
      parseErrors: [{resourceType: 'bad_type', fileName: 'config.yaml', error: 'parse error'}],
      parseStats: {successCount: 0, failCount: 1},
    };

    const user = userEvent.setup();
    render(<ImportConfigurationValidatePage />);

    await user.click(screen.getByRole('button', {name: 'Cancel'}));

    expect(mockNavigate).toHaveBeenCalledWith('/home');
  });

  it('advances validation steps via timer and navigates to summary', () => {
    mockLocationState = {parseErrors: [], configData: {application: []}};

    vi.useFakeTimers();
    render(<ImportConfigurationValidatePage />);

    // Advance through all three intervals (each 1500ms) + timeouts (1000ms each) + final 500ms
    for (let i = 0; i < 3; i++) {
      act(() => {
        vi.advanceTimersByTime(1500);
      });
      act(() => {
        vi.advanceTimersByTime(1000);
      });
    }
    act(() => {
      vi.advanceTimersByTime(500);
    });

    expect(mockNavigate).toHaveBeenCalledWith(
      expect.stringContaining('/import-configuration/summary'),
      expect.anything(),
    );

    vi.useRealTimers();
  });
});
