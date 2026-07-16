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

import {render, screen, fireEvent} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import StartBuildingSection from '../StartBuildingSection';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string | object) => (typeof fallback === 'string' ? fallback : key),
  }),
}));

vi.mock('framer-motion', async () => {
  const actual = await vi.importActual<typeof import('framer-motion')>('framer-motion');
  return {
    ...actual,
    motion: {
      ...((actual as {motion: object}).motion ?? {}),
      div: ({children, ...rest}: React.HTMLAttributes<HTMLDivElement>) => <div {...rest}>{children}</div>,
    },
  };
});

const mockNavigate = vi.fn();
vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

const mockUseGetApplications = vi.fn();
vi.mock('../../../applications/api/useGetApplications', () => ({
  default: (args: unknown) => mockUseGetApplications(args) as unknown,
}));

vi.mock('../HomeFloatingLogos', () => ({
  default: () => <div data-testid="home-floating-logos" />,
}));

describe('StartBuildingSection', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockNavigate.mockReturnValue(undefined);
    mockUseGetApplications.mockReturnValue({data: undefined});
  });

  describe('No applications state', () => {
    beforeEach(() => {
      mockUseGetApplications.mockReturnValue({data: {totalResults: 0}});
    });

    it('renders "Create Application" button when no apps exist', () => {
      render(<StartBuildingSection />);
      expect(screen.getByRole('button', {name: 'Create Application'})).toBeInTheDocument();
    });

    it('navigates to /applications/types when the button is clicked', () => {
      render(<StartBuildingSection />);
      fireEvent.click(screen.getByRole('button', {name: 'Create Application'}));
      expect(mockNavigate).toHaveBeenCalledWith('/applications/types');
    });

    it('does not render the "Create Applications" view button when totalResults is 0', () => {
      render(<StartBuildingSection />);
      expect(screen.queryByRole('button', {name: 'Create Applications'})).not.toBeInTheDocument();
    });
  });

  describe('Has applications state', () => {
    beforeEach(() => {
      mockUseGetApplications.mockReturnValue({data: {totalResults: 3}});
    });

    it('renders "Create Applications" button when apps exist', () => {
      render(<StartBuildingSection />);
      expect(screen.getByRole('button', {name: 'Create Applications'})).toBeInTheDocument();
    });

    it('navigates to /applications when the button is clicked', () => {
      render(<StartBuildingSection />);
      fireEvent.click(screen.getByRole('button', {name: 'Create Applications'}));
      expect(mockNavigate).toHaveBeenCalledWith('/applications/types');
    });

    it('does not render the create-only button when apps exist', () => {
      render(<StartBuildingSection />);
      expect(screen.queryByRole('button', {name: 'Create Application'})).not.toBeInTheDocument();
    });
  });

  describe('Content', () => {
    it('renders the hero description text', () => {
      render(<StartBuildingSection />);
      expect(
        screen.getByText('Add secure login, token management, and user sessions to your app in minutes.'),
      ).toBeInTheDocument();
    });

    it('renders the floating logos decorator', () => {
      render(<StartBuildingSection />);
      expect(screen.getByTestId('home-floating-logos')).toBeInTheDocument();
    });
  });
});
