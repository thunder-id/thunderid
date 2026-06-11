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

const mockSessionStorageGetItem = vi.fn();
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
        },
      },
    }),
  };
});

vi.mock('react-i18next', () => ({
  Trans: ({i18nKey}: {i18nKey: string}) => i18nKey,
  useTranslation: () => ({
    t: (key: string, opts?: Record<string, unknown>) => {
      if (opts?.productName) return `${key}:${opts.productName as string}`;
      return key;
    },
  }),
}));

vi.mock('../TerminalBlock', () => ({
  default: ({command}: {command: string}) => (
    <div>
      <pre data-testid="terminal-block">{command}</pre>
    </div>
  ),
}));

vi.mock('../WayfinderConfigImport', () => ({
  default: ({onSuccess}: {onSuccess?: () => void}) => (
    <div data-testid="wayfinder-config-import">
      <button onClick={onSuccess} type="button">
        mock-import-success
      </button>
    </div>
  ),
}));

vi.mock('../WayfinderSampleDownload', () => ({
  default: () => <div data-testid="wayfinder-sample-download" />,
}));

vi.mock('@wso2/oxygen-ui-icons-react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui-icons-react')>();
  return {
    ...actual,
    CheckCircle: () => <span data-testid="icon-check-circle" />,
    ChevronRight: () => <span data-testid="icon-chevron-right" />,
    Database: () => <span data-testid="icon-database" />,
    Download: () => <span data-testid="icon-download" />,
    Play: () => <span data-testid="icon-play" />,
    Settings: () => <span data-testid="icon-settings" />,
  };
});

import getWayfinderConfiguredStorageKey from '../../utils/getWayfinderConfiguredStorageKey';
import getWayfinderSetupExpandedStorageKey from '../../utils/getWayfinderSetupExpandedStorageKey';
import WayfinderSampleSetup from '../WayfinderSampleSetup';

const PRODUCT_NAME = 'ThunderID';
const IMPORTED_KEY = getWayfinderConfiguredStorageKey(PRODUCT_NAME);
const EXPANDED_KEY = getWayfinderSetupExpandedStorageKey(PRODUCT_NAME);

describe('WayfinderSampleSetup', () => {
  beforeEach(() => {
    vi.stubGlobal('sessionStorage', {
      getItem: mockSessionStorageGetItem,
      setItem: mockSessionStorageSetItem,
      removeItem: vi.fn(),
      clear: vi.fn(),
    });
    mockSessionStorageGetItem.mockReturnValue(null);
  });

  afterEach(() => {
    vi.clearAllMocks();
    vi.unstubAllGlobals();
  });

  it('renders without crashing', () => {
    const {container} = render(<WayfinderSampleSetup />);
    expect(container).toBeInTheDocument();
  });

  it('renders the setup title', () => {
    render(<WayfinderSampleSetup />);
    expect(screen.getByText('common:welcome.wayfinderSampleSetup.title')).toBeInTheDocument();
  });

  it('renders the one-time setup badge', () => {
    render(<WayfinderSampleSetup />);
    expect(screen.getByText('common:welcome.wayfinderSampleSetup.oneTimeSetup')).toBeInTheDocument();
  });

  it('expands by default when not previously imported', () => {
    mockSessionStorageGetItem.mockReturnValue(null);
    render(<WayfinderSampleSetup />);
    expect(screen.getByTestId('wayfinder-config-import')).toBeInTheDocument();
    expect(screen.getByTestId('terminal-block')).toBeInTheDocument();
  });

  it('collapses by default when previously imported', () => {
    mockSessionStorageGetItem.mockImplementation((key: string) => {
      if (key === IMPORTED_KEY) return '1234567890';
      return null;
    });
    render(<WayfinderSampleSetup />);
    expect(screen.queryByTestId('wayfinder-config-import')).not.toBeInTheDocument();
  });

  it('shows setupComplete message when done and collapsed', () => {
    mockSessionStorageGetItem.mockImplementation((key: string) => {
      if (key === IMPORTED_KEY) return '1234567890';
      if (key === EXPANDED_KEY) return 'false';
      return null;
    });
    render(<WayfinderSampleSetup />);
    expect(screen.getByText('common:welcome.wayfinderSampleSetup.setupComplete')).toBeInTheDocument();
  });

  it('toggles expand/collapse when header is clicked', async () => {
    const user = userEvent.setup();
    render(<WayfinderSampleSetup />);

    expect(screen.getByTestId('terminal-block')).toBeInTheDocument();

    const header = screen.getByRole('button', {name: /wayfinderSampleSetup\.title/i});
    await user.click(header);

    expect(screen.queryByTestId('terminal-block')).not.toBeInTheDocument();
    expect(mockSessionStorageSetItem).toHaveBeenCalledWith(EXPANDED_KEY, 'false');
  });

  it('respects sessionStorage expanded=true even when already imported', () => {
    mockSessionStorageGetItem.mockImplementation((key: string) => {
      if (key === IMPORTED_KEY) return '1234567890';
      if (key === EXPANDED_KEY) return 'true';
      return null;
    });
    render(<WayfinderSampleSetup />);
    expect(screen.getByTestId('terminal-block')).toBeInTheDocument();
  });

  it('shows WayfinderSampleDownload when expanded', () => {
    render(<WayfinderSampleSetup />);
    expect(screen.getByTestId('wayfinder-sample-download')).toBeInTheDocument();
  });

  it('shows step titles when expanded', () => {
    render(<WayfinderSampleSetup />);
    expect(screen.getByText('common:welcome.wayfinderSampleSetup.steps.getSample.title')).toBeInTheDocument();
    expect(screen.getByText(/wayfinderSampleSetup.steps.configure.title/)).toBeInTheDocument();
    expect(screen.getByText('common:welcome.wayfinderSampleSetup.steps.run.title')).toBeInTheDocument();
  });

  it('shows npm run command', () => {
    render(<WayfinderSampleSetup />);
    expect(screen.getByTestId('terminal-block')).toHaveTextContent('npm i && npm run dev');
  });

  it('stays expanded after import success but persists collapsed intent for next visit', async () => {
    const user = userEvent.setup();
    render(<WayfinderSampleSetup />);

    expect(screen.getByTestId('wayfinder-config-import')).toBeInTheDocument();
    expect(screen.getByTestId('terminal-block')).toBeInTheDocument();

    await user.click(screen.getByText('mock-import-success'));

    expect(screen.getByTestId('wayfinder-config-import')).toBeInTheDocument();
    expect(screen.getByTestId('terminal-block')).toBeInTheDocument();
    expect(mockSessionStorageSetItem).toHaveBeenCalledWith(EXPANDED_KEY, 'false');
  });

  it('toggles expand/collapse when header receives Enter keypress', () => {
    render(<WayfinderSampleSetup />);

    expect(screen.getByTestId('terminal-block')).toBeInTheDocument();

    const header = screen.getByRole('button', {name: /wayfinderSampleSetup\.title/i});
    fireEvent.keyDown(header, {key: 'Enter'});

    expect(screen.queryByTestId('terminal-block')).not.toBeInTheDocument();
  });
});
