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
import SignInSlogan from '../SignInSlogan';

describe('SignInSlogan', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('renders without crashing', () => {
    const {container} = render(<SignInSlogan />);
    expect(container).toBeInTheDocument();
  });

  it('renders all slogan items', () => {
    render(<SignInSlogan />);
    expect(screen.getByText('Flexible Identity Platform')).toBeInTheDocument();
    expect(screen.getByText('Zero-trust Security')).toBeInTheDocument();
    expect(screen.getByText('Developer-first Experience')).toBeInTheDocument();
    expect(screen.getByText('Extensible & Enterprise-ready')).toBeInTheDocument();
  });

  it('renders item descriptions', () => {
    render(<SignInSlogan />);
    expect(screen.getByText(/Centralizes identity management/)).toBeInTheDocument();
    expect(screen.getByText(/Leverage adaptive authentication/)).toBeInTheDocument();
    expect(screen.getByText(/Configure auth flows and manage organizations/)).toBeInTheDocument();
    expect(screen.getByText(/Built for scale/)).toBeInTheDocument();
  });

  it('renders with default logos', () => {
    const {rerender} = render(<SignInSlogan />);
    rerender(<SignInSlogan />);

    const logo = screen.getByAltText('Logo (Light)');

    expect(logo).toBeInTheDocument();
    expect(logo).toHaveAttribute('src', expect.stringContaining('/assets/images/logo.svg'));
    expect(logo).toHaveStyle({height: '50px'});
  });
});
