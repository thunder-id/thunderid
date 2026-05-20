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

import {render, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import Recovery from '../Recovery';

// Mock child component
vi.mock('../RecoveryBox', () => ({
  default: () => <div data-testid="recovery-box">RecoveryBox</div>,
}));

// Mock useThunderID hook
const mockUseThunderID = vi.fn();
vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/react')>();
  return {
    ...actual,
    // eslint-disable-next-line @typescript-eslint/no-unsafe-return
    useThunderID: () => mockUseThunderID(),
  };
});

// Mock AuthPageLayout
vi.mock('@thunderid/design', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/design')>();
  return {
    ...actual,
    AuthPageLayout: ({children}: {children: React.ReactNode}) => <div data-testid="auth-page-layout">{children}</div>,
  };
});

describe('Recovery', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseThunderID.mockReturnValue({
      isMetaLoading: false,
    });
  });

  it('renders without crashing', () => {
    const {container} = render(<Recovery />);
    expect(container).toBeInTheDocument();
  });

  it('renders RecoveryBox component', () => {
    render(<Recovery />);
    expect(screen.getByTestId('recovery-box')).toBeInTheDocument();
  });

  it('renders AuthPageLayout', () => {
    render(<Recovery />);
    expect(screen.getByTestId('auth-page-layout')).toBeInTheDocument();
  });

  it('renders when isMetaLoading is true', () => {
    mockUseThunderID.mockReturnValue({
      isMetaLoading: true,
    });
    render(<Recovery />);
    expect(screen.getByTestId('auth-page-layout')).toBeInTheDocument();
  });
});
