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

import {render, screen, fireEvent, waitFor} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import CapabilityCatalog from '../CapabilityCatalog';

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  }),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, options?: {count?: number}) => {
      const translations: Record<string, string> = {
        'flows:catalog.title': 'What do you want to build?',
        'flows:catalog.subtitle': 'Explore ready-made capabilities and start from a template',
        'flows:catalog.explore': 'Explore what you can build',
        'flows:catalog.cards.passwords.title': 'Passwords & Credentials',
        'flows:catalog.cards.socialLogin.title': 'Social & Enterprise Login',
        'flows:catalog.cards.mfa.title': 'Multi-Factor Authentication',
        'flows:catalog.cards.passwordless.title': 'Passwordless',
        'flows:catalog.cards.recovery.title': 'Account Recovery',
      };
      if (key === 'flows:catalog.templatesCount') {
        return `${options?.count} templates`;
      }
      return translations[key] || key;
    },
  }),
}));

const mockNavigate = vi.fn();
vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

vi.mock('../../api/useGetFlowsMeta', () => ({
  default: () => ({
    data: {
      templates: [
        {type: 'BLANK', flowType: 'AUTHENTICATION', category: 'STARTER'},
        {type: 'BASIC', flowType: 'AUTHENTICATION', category: 'PASSWORD'},
        {type: 'BASIC', flowType: 'REGISTRATION', category: 'PASSWORD'},
        {type: 'GOOGLE', flowType: 'AUTHENTICATION', category: 'SOCIAL_LOGIN'},
        {type: 'SMS_OTP', flowType: 'AUTHENTICATION', category: 'MFA'},
        {type: 'PASSKEY', flowType: 'AUTHENTICATION', category: 'PASSWORDLESS'},
        {type: 'PASSWORD_RECOVERY', flowType: 'RECOVERY', category: 'STARTER'},
      ],
    },
    error: null,
    isLoading: false,
  }),
}));

describe('CapabilityCatalog', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Full variant', () => {
    it('should render the gallery heading and all capability cards', () => {
      render(<CapabilityCatalog variant="full" />);

      expect(screen.getByText('What do you want to build?')).toBeInTheDocument();
      expect(screen.getByText('Passwords & Credentials')).toBeInTheDocument();
      expect(screen.getByText('Social & Enterprise Login')).toBeInTheDocument();
      expect(screen.getByText('Multi-Factor Authentication')).toBeInTheDocument();
      expect(screen.getByText('Passwordless')).toBeInTheDocument();
      expect(screen.getByText('Account Recovery')).toBeInTheDocument();
    });

    it('should show template counts per capability, excluding blank templates', () => {
      render(<CapabilityCatalog variant="full" />);

      // PASSWORD has two templates; RECOVERY flow type has one non-blank template.
      expect(screen.getByText('2 templates')).toBeInTheDocument();
      expect(screen.getAllByText('1 templates').length).toBeGreaterThanOrEqual(4);
    });

    it('should navigate to the create wizard with the category preselected', async () => {
      render(<CapabilityCatalog variant="full" />);

      fireEvent.click(screen.getByText('Multi-Factor Authentication'));

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/flows/create?category=MFA');
      });
    });

    it('should navigate to the create wizard with the flow type preselected for recovery', async () => {
      render(<CapabilityCatalog variant="full" />);

      fireEvent.click(screen.getByText('Account Recovery'));

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/flows/create?flowType=RECOVERY');
      });
    });
  });

  describe('Compact variant', () => {
    it('should render a collapsed section with an explore title', () => {
      render(<CapabilityCatalog variant="compact" />);

      expect(screen.getByTestId('capability-catalog-compact')).toBeInTheDocument();
      expect(screen.getByText('Explore what you can build')).toBeInTheDocument();
    });

    it('should reveal capability cards when expanded', () => {
      render(<CapabilityCatalog variant="compact" />);

      fireEvent.click(screen.getByText('Explore what you can build'));

      expect(screen.getByText('Passwords & Credentials')).toBeInTheDocument();
    });
  });
});
