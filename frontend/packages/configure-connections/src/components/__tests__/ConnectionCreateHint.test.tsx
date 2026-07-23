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
import {describe, expect, it, vi} from 'vitest';
import ConnectionCreateHint from '../ConnectionCreateHint';

vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useToast: () => ({showToast: vi.fn()}),
}));

describe('ConnectionCreateHint', () => {
  it('renders the instruction and the redirect URI as a read-only copy field', () => {
    render(
      <ConnectionCreateHint
        instruction="Create an OAuth client for your app, then enter the credentials it gives you."
        redirectUri="https://id.acme.io/gate/callback"
      />,
    );

    expect(
      screen.getByText('Create an OAuth client for your app, then enter the credentials it gives you.'),
    ).toBeInTheDocument();
    expect(screen.getByDisplayValue('https://id.acme.io/gate/callback')).toBeInTheDocument();
    expect(screen.getByTestId('create-hint-redirect-uri-copy')).toBeInTheDocument();
  });
});
