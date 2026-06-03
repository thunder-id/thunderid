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

import {render} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import App from '../App';

vi.mock('../pages/AcceptInvitePage', () => ({default: () => null}));
vi.mock('../pages/ErrorPage', () => ({default: () => null}));
vi.mock('../pages/RecoveryPage', () => ({default: () => null}));
vi.mock('../pages/SignInPage', () => ({default: () => null}));
vi.mock('../pages/SignUpPage', () => ({default: () => null}));

describe('App', () => {
  it('renders without crashing', () => {
    const {container} = render(<App />);
    expect(container).toBeInTheDocument();
  });
});
