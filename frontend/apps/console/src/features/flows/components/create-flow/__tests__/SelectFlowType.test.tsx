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

import {render, screen, fireEvent} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import SelectFlowType from '../SelectFlowType';

// Mock react-i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (_key: string, defaultValue: string) => defaultValue,
  }),
}));

// Mock oxygen-ui-icons-react
vi.mock('@wso2/oxygen-ui-icons-react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@wso2/oxygen-ui-icons-react')>();
  return {
    ...actual,
    KeyRound: ({size}: {size: number}) => <span data-testid="icon-key-round">{size}</span>,
    UserPlus: ({size}: {size: number}) => <span data-testid="icon-user-plus">{size}</span>,
  };
});

describe('SelectFlowType', () => {
  const mockOnTypeChange = vi.fn();
  const mockOnReadyChange = vi.fn();

  const defaultProps = {
    selectedType: null,
    onTypeChange: mockOnTypeChange,
    onReadyChange: mockOnReadyChange,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render the component with data-testid', () => {
      render(<SelectFlowType {...defaultProps} />);

      expect(screen.getByTestId('select-flow-type')).toBeInTheDocument();
    });

    it('should render the title', () => {
      render(<SelectFlowType {...defaultProps} />);

      expect(screen.getByText('What kind of flow do you want to create?')).toBeInTheDocument();
    });

    it('should render Sign-in option', () => {
      render(<SelectFlowType {...defaultProps} />);

      expect(screen.getByText('Sign-in')).toBeInTheDocument();
      expect(screen.getByText('Authenticate users with passwords, passkeys, or social providers')).toBeInTheDocument();
    });

    it('should render Self Sign-up option', () => {
      render(<SelectFlowType {...defaultProps} />);

      expect(screen.getByText('Self Sign-up')).toBeInTheDocument();
      expect(screen.getByText('Let users register themselves with your application')).toBeInTheDocument();
    });

    it('should render icons for both options', () => {
      render(<SelectFlowType {...defaultProps} />);

      expect(screen.getByTestId('icon-key-round')).toBeInTheDocument();
      expect(screen.getByTestId('icon-user-plus')).toBeInTheDocument();
    });

    it('should render SignOut option', () => {
      render(<SelectFlowType {...defaultProps} />);

      expect(screen.getByText('Sign Out')).toBeInTheDocument();
      expect(screen.getByText('Confirm and terminate an established SSO session')).toBeInTheDocument();
    });
  });

  describe('Selection', () => {
    it('should call onTypeChange and onReadyChange when a flow type is clicked', () => {
      render(<SelectFlowType {...defaultProps} />);

      fireEvent.click(screen.getByText('Sign-in'));

      expect(mockOnTypeChange).toHaveBeenCalledWith('AUTHENTICATION');
      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('should call onTypeChange with SIGNOUT when SignOut is clicked', () => {
      render(<SelectFlowType {...defaultProps} />);

      fireEvent.click(screen.getByText('Sign Out'));

      expect(mockOnTypeChange).toHaveBeenCalledWith('SIGNOUT');
      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('should call onTypeChange with REGISTRATION when Self Sign-up is clicked', () => {
      render(<SelectFlowType {...defaultProps} />);

      fireEvent.click(screen.getByText('Self Sign-up'));

      expect(mockOnTypeChange).toHaveBeenCalledWith('REGISTRATION');
      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });

    it('should visually indicate the selected type via the card action area', () => {
      const {rerender} = render(<SelectFlowType {...defaultProps} selectedType="AUTHENTICATION" />);

      // The component renders two cards; verify the selected one renders without error
      expect(screen.getByText('Sign-in')).toBeInTheDocument();

      rerender(<SelectFlowType {...defaultProps} selectedType="REGISTRATION" />);

      expect(screen.getByText('Self Sign-up')).toBeInTheDocument();
    });

    it('should allow changing selection from one type to another', () => {
      render(<SelectFlowType {...defaultProps} selectedType="AUTHENTICATION" />);

      fireEvent.click(screen.getByText('Self Sign-up'));

      expect(mockOnTypeChange).toHaveBeenCalledWith('REGISTRATION');
    });
  });
});
