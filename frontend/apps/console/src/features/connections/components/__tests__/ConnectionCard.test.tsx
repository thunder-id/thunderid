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

import {render, screen, fireEvent} from '@testing-library/react';
import type {JSX} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import type {ConnectionCardModel} from '../../models/connection';
import ConnectionCard from '../ConnectionCard';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({t: (key: string) => key}),
}));

const baseCard: ConnectionCardModel = {
  id: 'google',
  vendorKey: 'google',
  backendType: 'google',
  displayName: 'Google',
  descriptionKey: 'connections:vendor.google.description',
  logo: 'logo' as unknown as JSX.Element,
  categories: ['social-login'],
  status: 'not-configured',
  comingSoon: false,
  navTarget: '/connections/google/configure',
};

describe('ConnectionCard', () => {
  const onAction = vi.fn();
  beforeEach(() => vi.clearAllMocks());

  it('renders the vendor name, status, and hashtag category tags', () => {
    render(<ConnectionCard card={baseCard} onAction={onAction} />);
    expect(screen.getByText('Google')).toBeInTheDocument();
    expect(screen.getByText('card.notConfigured')).toBeInTheDocument();
    expect(screen.getByText('#categories.social-login')).toBeInTheDocument();
  });

  it('invokes onAction when the whole card is clicked', () => {
    render(<ConnectionCard card={baseCard} onAction={onAction} />);
    fireEvent.click(screen.getByTestId('connection-card-action-google'));
    expect(onAction).toHaveBeenCalledWith(baseCard);
  });

  it('disables interaction for coming-soon cards (no clickable action area)', () => {
    const soon: ConnectionCardModel = {
      ...baseCard,
      id: 'twilio',
      vendorKey: 'twilio',
      comingSoon: true,
      navTarget: null,
    };
    render(<ConnectionCard card={soon} onAction={onAction} />);
    expect(screen.queryByTestId('connection-card-action-twilio')).not.toBeInTheDocument();
    expect(screen.getByText('card.comingSoon')).toBeInTheDocument();
  });
});
