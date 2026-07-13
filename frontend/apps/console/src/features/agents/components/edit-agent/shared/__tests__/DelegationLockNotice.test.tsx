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

import {render, screen} from '@testing-library/react';
import {describe, it, expect} from 'vitest';
import DelegationLockNotice from '../DelegationLockNotice';

describe('DelegationLockNotice', () => {
  it('renders only the children when unlocked', () => {
    render(
      <DelegationLockNotice isUnlocked message="Turn on Delegated mode to unlock these settings.">
        <div data-testid="content">content</div>
      </DelegationLockNotice>,
    );

    expect(screen.getByTestId('content')).toBeInTheDocument();
    expect(screen.queryByText('Turn on Delegated mode to unlock these settings.')).not.toBeInTheDocument();
  });

  it('shows the given message and gives the (still visible) children a frozen look when locked', () => {
    render(
      <DelegationLockNotice isUnlocked={false} message="Turn on Delegated mode to unlock these settings.">
        <div data-testid="content">content</div>
      </DelegationLockNotice>,
    );

    expect(screen.getByTestId('content')).toBeInTheDocument();
    expect(screen.getByText('Turn on Delegated mode to unlock these settings.')).toBeInTheDocument();
    expect(screen.getByTestId('content').parentElement).toHaveStyle({pointerEvents: 'none', opacity: '0.6'});
  });
});
