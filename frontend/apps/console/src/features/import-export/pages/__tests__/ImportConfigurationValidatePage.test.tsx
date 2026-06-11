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

import {render, screen, userEvent, act} from '@thunderid/test-utils';
import {afterEach, describe, expect, it, vi} from 'vitest';

const mockNavigate = vi.fn();
let mockLocationState: unknown = null;

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, unknown>) => (opts ? `${key}:${JSON.stringify(opts)}` : key),
  }),
}));

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useLocation: () => ({state: mockLocationState, pathname: '/welcome/open-project/validate'}),
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
    expect(screen.getByText('validate.title')).toBeInTheDocument();
  });

  it('renders the four validation steps', () => {
    render(<ImportConfigurationValidatePage />);
    expect(screen.getByText('validate.steps.readingFile')).toBeInTheDocument();
    expect(screen.getByText('validate.steps.validatingYaml')).toBeInTheDocument();
    expect(screen.getByText('validate.steps.checkingCompatibility')).toBeInTheDocument();
    expect(screen.getByText('validate.steps.validatingResources')).toBeInTheDocument();
  });

  it('renders close button', () => {
    render(<ImportConfigurationValidatePage />);
    expect(screen.getByRole('button', {name: 'common:actions.close'})).toBeInTheDocument();
  });

  it('navigates to /home on close', async () => {
    const user = userEvent.setup();
    render(<ImportConfigurationValidatePage />);

    await user.click(screen.getByRole('button', {name: 'common:actions.close'}));

    expect(mockNavigate).toHaveBeenCalledWith('/home');
  });

  it('navigates to /welcome on cancel (no errors)', () => {
    mockLocationState = {parseErrors: [], configData: {application: []}};
    render(<ImportConfigurationValidatePage />);

    const cancelButton = screen.queryByRole('button', {name: 'common:actions.cancel'});
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

    expect(screen.getByRole('button', {name: 'validate.actions.uploadDifferentFile'})).toBeInTheDocument();
  });

  it('navigates to /welcome/open-project when upload different file is clicked', async () => {
    mockLocationState = {
      parseErrors: [{resourceType: 'bad_type', fileName: 'config.yaml', error: 'parse error'}],
      parseStats: {successCount: 0, failCount: 1},
    };

    const user = userEvent.setup();
    render(<ImportConfigurationValidatePage />);

    await user.click(screen.getByRole('button', {name: 'validate.actions.uploadDifferentFile'}));

    expect(mockNavigate).toHaveBeenCalledWith('/welcome/open-project');
  });

  it('renders breadcrumb with welcome header', () => {
    render(<ImportConfigurationValidatePage />);
    expect(screen.getByText('common:welcome.header')).toBeInTheDocument();
  });

  it('navigates to /welcome when breadcrumb welcome is clicked', async () => {
    const user = userEvent.setup();
    render(<ImportConfigurationValidatePage />);

    await user.click(screen.getByText('common:welcome.header'));

    expect(mockNavigate).toHaveBeenCalledWith('/welcome');
  });

  it('navigates to /home on cancel when parse errors exist', async () => {
    mockLocationState = {
      parseErrors: [{resourceType: 'bad_type', fileName: 'config.yaml', error: 'parse error'}],
      parseStats: {successCount: 0, failCount: 1},
    };

    const user = userEvent.setup();
    render(<ImportConfigurationValidatePage />);

    await user.click(screen.getByRole('button', {name: 'common:actions.cancel'}));

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

    expect(mockNavigate).toHaveBeenCalledWith(expect.stringContaining('/open-project/summary'), expect.anything());

    vi.useRealTimers();
  });
});
