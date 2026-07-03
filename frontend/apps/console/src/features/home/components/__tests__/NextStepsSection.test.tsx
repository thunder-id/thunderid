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
import {describe, it, expect, vi} from 'vitest';
import NextStepsSection from '../NextStepsSection';

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
      create:
        () =>
        ({children, ...rest}: React.HTMLAttributes<HTMLDivElement>) => <div {...rest}>{children}</div>,
    },
  };
});

vi.mock('../cards/InviteMembersCard', () => ({
  default: () => <div data-testid="invite-members-card" />,
}));

vi.mock('../cards/LoginBoxCard', () => ({
  default: () => <div data-testid="login-box-card" />,
}));

vi.mock('../cards/MFACard', () => ({
  default: () => <div data-testid="mfa-card" />,
}));

vi.mock('../cards/ConnectionsCard', () => ({
  default: () => <div data-testid="connections-card" />,
}));

describe('NextStepsSection', () => {
  it('renders the section title', () => {
    render(<NextStepsSection />);
    expect(screen.getByText('Quick Links')).toBeInTheDocument();
  });

  it('renders InviteMembersCard', () => {
    render(<NextStepsSection />);
    expect(screen.getByTestId('invite-members-card')).toBeInTheDocument();
  });

  it('renders LoginBoxCard', () => {
    render(<NextStepsSection />);
    expect(screen.getByTestId('login-box-card')).toBeInTheDocument();
  });

  it('renders ConnectionsCard', () => {
    render(<NextStepsSection />);
    expect(screen.getByTestId('connections-card')).toBeInTheDocument();
  });

  it('renders MFACard', () => {
    render(<NextStepsSection />);
    expect(screen.getByTestId('mfa-card')).toBeInTheDocument();
  });
});
