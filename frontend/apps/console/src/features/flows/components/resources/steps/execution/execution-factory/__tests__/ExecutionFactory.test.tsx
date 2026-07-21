/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import ExecutionFactory from '../ExecutionFactory';
import type {Step} from '@/features/flows/models/steps';
import {ExecutionTypes} from '@/features/flows/models/steps';

// Use vi.hoisted to define mock function before vi.mock hoisting
const mockUseColorScheme = vi.hoisted(() =>
  vi.fn(() => ({
    mode: 'light',
    systemMode: 'light',
  })),
);

// Mock react-i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string) => fallback ?? key,
  }),
}));

// Mock useColorScheme
vi.mock('@wso2/oxygen-ui', async () => {
  const actual = await vi.importActual('@wso2/oxygen-ui');
  return {
    ...actual,
    useColorScheme: () => mockUseColorScheme(),
  };
});

// Mock resolveStaticResourcePath
vi.mock('@/features/flows/utils/resolveStaticResourcePath', () => ({
  default: (path: string) => `/static/${path}`,
}));

// Create mock resource
const createMockResource = (overrides: Partial<Step> = {}): Step =>
  ({
    id: 'execution-1',
    type: 'TASK_EXECUTION',
    position: {x: 0, y: 0},
    size: {width: 200, height: 100},
    display: {
      label: 'Test Executor',
      image: 'assets/images/icons/test.svg',
      showOnResourcePanel: true,
    },
    data: {
      action: {
        executor: {
          name: 'TestExecutor',
        },
      },
    },
    config: {},
    ...overrides,
  }) as Step;

describe('ExecutionFactory', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseColorScheme.mockReturnValue({
      mode: 'light',
      systemMode: 'light',
    });
  });

  describe('Display Metadata Rendering', () => {
    it('should render image and label for executors with display.image', () => {
      const resource = createMockResource({
        display: {
          label: 'Custom Executor',
          image: 'assets/images/icons/custom.svg',
          showOnResourcePanel: true,
        },
        data: {
          action: {
            executor: {
              name: 'CustomExecutor',
            },
          },
        },
      });
      render(<ExecutionFactory resource={resource} />);

      const img = screen.getByRole('img');
      expect(img).toHaveAttribute('src', '/static/assets/images/icons/custom.svg');
      expect(img).toHaveAttribute('alt', 'Custom Executor');
      expect(screen.getByText('Custom Executor')).toBeInTheDocument();
    });

    it('should render Google federation from its display metadata like any other executor', () => {
      const resource = createMockResource({
        display: {
          label: 'Google',
          image: 'assets/images/icons/google.svg',
          preserveImageColor: true,
          showOnResourcePanel: true,
        },
        data: {
          action: {
            executor: {
              name: ExecutionTypes.GoogleFederation,
            },
          },
        },
      });
      render(<ExecutionFactory resource={resource} />);

      expect(screen.getByRole('img')).toHaveAttribute('src', '/static/assets/images/icons/google.svg');
      expect(screen.getByText('Google')).toBeInTheDocument();
    });

    it('should render GitHub federation from its display metadata like any other executor', () => {
      const resource = createMockResource({
        display: {
          label: 'GitHub',
          image: 'assets/images/icons/github.svg',
          showOnResourcePanel: true,
        },
        data: {
          action: {
            executor: {
              name: ExecutionTypes.GithubFederation,
            },
          },
        },
      });
      render(<ExecutionFactory resource={resource} />);

      expect(screen.getByRole('img')).toHaveAttribute('src', '/static/assets/images/icons/github.svg');
      expect(screen.getByText('GitHub')).toBeInTheDocument();
    });

    it('should render icon-library names in display.image the same way as the resource panel', () => {
      const resource = createMockResource({
        display: {
          label: 'Widget Executor',
          image: 'UserPlus',
          showOnResourcePanel: true,
        },
        data: {
          action: {
            executor: {
              name: 'WidgetExecutor',
            },
          },
        },
      });
      const {container} = render(<ExecutionFactory resource={resource} />);

      expect(container.querySelector('svg')).toBeInTheDocument();
      expect(screen.getByText('Widget Executor')).toBeInTheDocument();
    });

    it('should use the fallback label when displayLabel is undefined', () => {
      const resource = createMockResource({
        display: {
          label: undefined as unknown as string,
          image: 'assets/images/icons/custom.svg',
          showOnResourcePanel: true,
        },
        data: {
          action: {
            executor: {
              name: 'CustomExecutor',
            },
          },
        },
      });
      render(<ExecutionFactory resource={resource} />);

      expect(screen.getByText('Executor')).toBeInTheDocument();
    });
  });

  describe('Description', () => {
    it('should not render the description text in the node body (shown as a header hint instead)', () => {
      const resource = createMockResource({
        display: {
          label: 'Check SSO Session',
          description: 'Can the following authentication be skipped by reusing the existing session?',
          image: 'assets/images/icons/magnifying-glass.svg',
          showOnResourcePanel: true,
        },
        data: {
          action: {
            executor: {
              name: 'SSOCheckExecutor',
            },
          },
        },
      });
      render(<ExecutionFactory resource={resource} />);

      expect(
        screen.queryByText('Can the following authentication be skipped by reusing the existing session?'),
      ).not.toBeInTheDocument();
    });
  });

  describe('Fallback Executor without Display Image', () => {
    it('should render only label when display.image is not provided', () => {
      const resource = createMockResource({
        display: {
          label: 'No Image Executor',
          image: '',
          showOnResourcePanel: true,
        },
        data: {
          action: {
            executor: {
              name: 'NoImageExecutor',
            },
          },
        },
      });
      render(<ExecutionFactory resource={resource} />);

      expect(screen.getByText('No Image Executor')).toBeInTheDocument();
      expect(screen.queryByRole('img')).not.toBeInTheDocument();
    });

    it('should use the fallback label when display is completely undefined', () => {
      const resource = createMockResource({
        display: undefined,
        data: {
          action: {
            executor: {
              name: 'UndefinedDisplayExecutor',
            },
          },
        },
      });
      render(<ExecutionFactory resource={resource} />);

      expect(screen.getByText('Executor')).toBeInTheDocument();
    });
  });

  describe('Edge Cases', () => {
    it('should handle undefined data', () => {
      const resource = createMockResource({
        display: undefined,
        data: undefined,
      });
      render(<ExecutionFactory resource={resource} />);

      expect(screen.getByText('Executor')).toBeInTheDocument();
    });

    it('should handle undefined action', () => {
      const resource = createMockResource({
        display: undefined,
        data: {},
      });
      render(<ExecutionFactory resource={resource} />);

      expect(screen.getByText('Executor')).toBeInTheDocument();
    });
  });
});
