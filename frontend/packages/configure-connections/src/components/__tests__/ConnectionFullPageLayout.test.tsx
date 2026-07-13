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

import {render, screen} from '@thunderid/test-utils';
import {describe, expect, it, vi} from 'vitest';
import ConnectionFullPageLayout from '../ConnectionFullPageLayout';

describe('ConnectionFullPageLayout', () => {
  it('renders children inside a left-aligned constrained content wrapper', () => {
    render(
      <ConnectionFullPageLayout label="Configure connection" onClose={vi.fn()} progress={50}>
        <div>Wizard content</div>
      </ConnectionFullPageLayout>,
    );

    const content = screen.getByTestId('connection-fullpage-content');
    const styles = window.getComputedStyle(content);

    expect(screen.getByText('Wizard content')).toBeInTheDocument();
    expect(styles.maxWidth).toBe('920px');
    expect(content).not.toHaveStyle({marginLeft: 'auto'});
    expect(content).not.toHaveStyle({marginRight: 'auto'});
  });
});
