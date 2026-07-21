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
import ConnectionsCard from '../ConnectionsCard';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback?: string | {defaultValue: string; [key: string]: string}) => {
      if (typeof fallback === 'string') {
        return fallback;
      }
      if (fallback && typeof fallback === 'object') {
        const {defaultValue, ...vars} = fallback;
        return Object.entries(vars).reduce(
          (text, [name, value]) => text.replaceAll(`{{${name}}}`, value),
          defaultValue,
        );
      }
      return key;
    },
  }),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({config: {brand: {product_name: 'ThunderID'}}}),
  };
});

vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => vi.fn(),
  };
});

describe('ConnectionsCard', () => {
  it('renders the card title', () => {
    render(<ConnectionsCard />);

    expect(screen.getByText('Connections')).toBeInTheDocument();
  });

  it('renders the card description', () => {
    render(<ConnectionsCard />);

    expect(
      screen.getByText(
        'Manage the external services ThunderID connects to for social login, enterprise OIDC, SMS delivery, and more.',
      ),
    ).toBeInTheDocument();
  });

  it('renders the primary action button', () => {
    render(<ConnectionsCard />);

    expect(screen.getByRole('button', {name: 'Manage Connections'})).toBeInTheDocument();
  });
});
