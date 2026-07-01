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
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import {ApplicationCreateFlowSignInApproach} from '../../../models/application-create-flow';
import ConfigureExperience from '../ConfigureExperience';

// Mock react-i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
}));

vi.mock('@thunderid/contexts', () => ({
  useConfig: () => ({
    getFeatureConfig: () => ({}),
    config: {brand: {}},
  }),
}));

describe('ConfigureExperience', () => {
  const mockOnApproachChange = vi.fn();
  const mockOnReadyChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render the component with both approach options', () => {
      render(
        <ConfigureExperience
          selectedApproach={ApplicationCreateFlowSignInApproach.INBUILT}
          onApproachChange={mockOnApproachChange}
        />,
      );

      expect(screen.getByText('applications:onboarding.configure.approach.inbuilt.title')).toBeInTheDocument();
      expect(screen.getByText('applications:onboarding.configure.approach.native.title')).toBeInTheDocument();
    });

    it('should select INBUILT approach by default', () => {
      render(
        <ConfigureExperience
          selectedApproach={ApplicationCreateFlowSignInApproach.INBUILT}
          onApproachChange={mockOnApproachChange}
        />,
      );

      const inbuiltRadio = screen.getAllByRole('radio')[0];
      expect(inbuiltRadio).toBeChecked();
    });

    it('should select EMBEDDED approach when prop is set', () => {
      render(
        <ConfigureExperience
          selectedApproach={ApplicationCreateFlowSignInApproach.EMBEDDED}
          onApproachChange={mockOnApproachChange}
        />,
      );

      const embeddedRadio = screen.getAllByRole('radio')[1];
      expect(embeddedRadio).toBeChecked();
    });

    it('should hide the embedded approach when allowEmbeddedApproach is false', () => {
      render(
        <ConfigureExperience
          selectedApproach={ApplicationCreateFlowSignInApproach.INBUILT}
          onApproachChange={mockOnApproachChange}
          allowEmbeddedApproach={false}
        />,
      );

      expect(screen.getByText('applications:onboarding.configure.approach.inbuilt.title')).toBeInTheDocument();
      expect(screen.queryByText('applications:onboarding.configure.approach.native.title')).not.toBeInTheDocument();
      expect(screen.getAllByRole('radio')).toHaveLength(1);
    });

    it('should reset to INBUILT when embedded is selected but not allowed', () => {
      render(
        <ConfigureExperience
          selectedApproach={ApplicationCreateFlowSignInApproach.EMBEDDED}
          onApproachChange={mockOnApproachChange}
          allowEmbeddedApproach={false}
        />,
      );

      expect(mockOnApproachChange).toHaveBeenCalledWith(ApplicationCreateFlowSignInApproach.INBUILT);
    });
  });

  describe('User Interactions', () => {
    it('should call onApproachChange when INBUILT is clicked', async () => {
      const user = userEvent.setup();
      render(
        <ConfigureExperience
          selectedApproach={ApplicationCreateFlowSignInApproach.EMBEDDED}
          onApproachChange={mockOnApproachChange}
        />,
      );

      const inbuiltRadio = screen.getAllByRole('radio')[0];
      await user.click(inbuiltRadio);

      expect(mockOnApproachChange).toHaveBeenCalledWith(ApplicationCreateFlowSignInApproach.INBUILT);
    });

    it('should call onApproachChange when EMBEDDED is clicked', async () => {
      const user = userEvent.setup();
      render(
        <ConfigureExperience
          selectedApproach={ApplicationCreateFlowSignInApproach.INBUILT}
          onApproachChange={mockOnApproachChange}
        />,
      );

      const embeddedRadio = screen.getAllByRole('radio')[1];
      await user.click(embeddedRadio);

      expect(mockOnApproachChange).toHaveBeenCalledWith(ApplicationCreateFlowSignInApproach.EMBEDDED);
    });
  });

  describe('Ready State', () => {
    it('should call onReadyChange with true on mount', () => {
      render(
        <ConfigureExperience
          selectedApproach={ApplicationCreateFlowSignInApproach.INBUILT}
          onApproachChange={mockOnApproachChange}
          onReadyChange={mockOnReadyChange}
        />,
      );

      expect(mockOnReadyChange).toHaveBeenCalledWith(true);
    });
  });

  describe('User Types Selection', () => {
    const mockUserTypes = [
      {id: '1', name: 'Internal', ouId: 'INTERNAL', allowSelfRegistration: true},
      {id: '2', name: 'External', ouId: 'EXTERNAL', allowSelfRegistration: false},
    ];
    const mockOnUserTypesChange = vi.fn();

    beforeEach(() => {
      mockOnUserTypesChange.mockClear();
    });

    it('should render user types selection when userTypes prop is provided', () => {
      render(
        <ConfigureExperience
          selectedApproach={ApplicationCreateFlowSignInApproach.INBUILT}
          onApproachChange={mockOnApproachChange}
          userTypes={mockUserTypes}
          onUserTypesChange={mockOnUserTypesChange}
        />,
      );

      expect(
        screen.getByText('applications:onboarding.configure.experience.access.userTypes.title'),
      ).toBeInTheDocument();
    });

    it('should not render user types selection when userTypes prop is undefined', () => {
      render(
        <ConfigureExperience
          selectedApproach={ApplicationCreateFlowSignInApproach.INBUILT}
          onApproachChange={mockOnApproachChange}
        />,
      );

      expect(
        screen.queryByText('applications:onboarding.configure.experience.access.userTypes.title'),
      ).not.toBeInTheDocument();
    });

    it('should not render user types selection when only one user type exists', () => {
      const singleUserType = [{id: '1', name: 'Internal', ouId: 'INTERNAL', allowSelfRegistration: true}];

      render(
        <ConfigureExperience
          selectedApproach={ApplicationCreateFlowSignInApproach.INBUILT}
          onApproachChange={mockOnApproachChange}
          userTypes={singleUserType}
          onUserTypesChange={mockOnUserTypesChange}
        />,
      );

      expect(
        screen.queryByText('applications:onboarding.configure.experience.access.userTypes.title'),
      ).not.toBeInTheDocument();
    });

    it('should render user type cards and allow selection', async () => {
      const user = userEvent.setup();
      render(
        <ConfigureExperience
          selectedApproach={ApplicationCreateFlowSignInApproach.INBUILT}
          onApproachChange={mockOnApproachChange}
          userTypes={mockUserTypes}
          selectedUserTypes={['Internal']}
          onUserTypesChange={mockOnUserTypesChange}
        />,
      );

      // Should render user type cards
      expect(screen.getByText('Internal')).toBeInTheDocument();
      expect(screen.getByText('External')).toBeInTheDocument();

      // Click External card to add it to selection
      const externalCard = screen.getByText('External').closest('[class*="MuiCard"]');
      expect(externalCard).toBeInTheDocument();
      await user.click(externalCard!);

      expect(mockOnUserTypesChange).toHaveBeenCalledWith(['Internal', 'External']);
    });

    it('should remove user type from selection when clicking selected card', async () => {
      const user = userEvent.setup();
      render(
        <ConfigureExperience
          selectedApproach={ApplicationCreateFlowSignInApproach.INBUILT}
          onApproachChange={mockOnApproachChange}
          userTypes={mockUserTypes}
          selectedUserTypes={['Internal', 'External']}
          onUserTypesChange={mockOnUserTypesChange}
        />,
      );

      // Click Internal card to remove it from selection
      const internalCard = screen.getByText('Internal').closest('[class*="MuiCard"]');
      await user.click(internalCard!);

      expect(mockOnUserTypesChange).toHaveBeenCalledWith(['External']);
    });

    it('should auto-select first user type when none selected', () => {
      render(
        <ConfigureExperience
          selectedApproach={ApplicationCreateFlowSignInApproach.INBUILT}
          onApproachChange={mockOnApproachChange}
          userTypes={mockUserTypes}
          selectedUserTypes={[]}
          onUserTypesChange={mockOnUserTypesChange}
        />,
      );

      expect(mockOnUserTypesChange).toHaveBeenCalledWith(['Internal']);
    });

    it('should call onReadyChange with false when no user types selected (with multiple available)', () => {
      const mockOnReadyChangeLocal = vi.fn();
      render(
        <ConfigureExperience
          selectedApproach={ApplicationCreateFlowSignInApproach.INBUILT}
          onApproachChange={mockOnApproachChange}
          onReadyChange={mockOnReadyChangeLocal}
          userTypes={mockUserTypes}
          selectedUserTypes={[]}
          onUserTypesChange={mockOnUserTypesChange}
        />,
      );

      // Initially false because no user types selected
      expect(mockOnReadyChangeLocal).toHaveBeenCalledWith(false);
    });

    it('should call onReadyChange with true when user types are selected', () => {
      const mockOnReadyChangeLocal = vi.fn();
      render(
        <ConfigureExperience
          selectedApproach={ApplicationCreateFlowSignInApproach.INBUILT}
          onApproachChange={mockOnApproachChange}
          onReadyChange={mockOnReadyChangeLocal}
          userTypes={mockUserTypes}
          selectedUserTypes={['Internal']}
          onUserTypesChange={mockOnUserTypesChange}
        />,
      );

      expect(mockOnReadyChangeLocal).toHaveBeenCalledWith(true);
    });
  });

  describe('User Types Autocomplete (5+ user types)', () => {
    const manyUserTypes = [
      {id: '1', name: 'Type1', ouId: 'TYPE1', allowSelfRegistration: true},
      {id: '2', name: 'Type2', ouId: 'TYPE2', allowSelfRegistration: false},
      {id: '3', name: 'Type3', ouId: 'TYPE3', allowSelfRegistration: true},
      {id: '4', name: 'Type4', ouId: 'TYPE4', allowSelfRegistration: false},
      {id: '5', name: 'Type5', ouId: 'TYPE5', allowSelfRegistration: true},
    ];
    const mockOnUserTypesChange = vi.fn();

    beforeEach(() => {
      mockOnUserTypesChange.mockClear();
    });

    it('should render autocomplete when 5 or more user types exist', () => {
      render(
        <ConfigureExperience
          selectedApproach={ApplicationCreateFlowSignInApproach.INBUILT}
          onApproachChange={mockOnApproachChange}
          userTypes={manyUserTypes}
          selectedUserTypes={['Type1']}
          onUserTypesChange={mockOnUserTypesChange}
        />,
      );

      // Should have an autocomplete input
      expect(screen.getByRole('combobox')).toBeInTheDocument();
    });

    it('should allow selecting user types from autocomplete', async () => {
      const user = userEvent.setup();
      render(
        <ConfigureExperience
          selectedApproach={ApplicationCreateFlowSignInApproach.INBUILT}
          onApproachChange={mockOnApproachChange}
          userTypes={manyUserTypes}
          selectedUserTypes={['Type1']}
          onUserTypesChange={mockOnUserTypesChange}
        />,
      );

      const autocomplete = screen.getByRole('combobox');
      await user.click(autocomplete);

      // Should show options
      const type2Option = await screen.findByText('Type2');
      await user.click(type2Option);

      expect(mockOnUserTypesChange).toHaveBeenCalledWith(['Type1', 'Type2']);
    });

    it('should show error state when no user types selected with autocomplete', () => {
      render(
        <ConfigureExperience
          selectedApproach={ApplicationCreateFlowSignInApproach.INBUILT}
          onApproachChange={mockOnApproachChange}
          userTypes={manyUserTypes}
          selectedUserTypes={[]}
          onUserTypesChange={mockOnUserTypesChange}
        />,
      );

      // The TextField should have error state
      const textField = screen.getByRole('combobox');
      expect(textField.closest('.MuiAutocomplete-root')?.querySelector('.Mui-error')).toBeInTheDocument();
    });
  });

  describe('Card Click Handlers', () => {
    it('should call onApproachChange when clicking INBUILT card', async () => {
      const user = userEvent.setup();
      render(
        <ConfigureExperience
          selectedApproach={ApplicationCreateFlowSignInApproach.EMBEDDED}
          onApproachChange={mockOnApproachChange}
        />,
      );

      // Find the INBUILT card by its title text and click the card itself
      const inbuiltTitle = screen.getByText('applications:onboarding.configure.approach.inbuilt.title');
      const inbuiltCard = inbuiltTitle.closest('[class*="MuiCard"]');
      await user.click(inbuiltCard!);

      expect(mockOnApproachChange).toHaveBeenCalledWith(ApplicationCreateFlowSignInApproach.INBUILT);
    });

    it('should call onApproachChange when clicking EMBEDDED card', async () => {
      const user = userEvent.setup();
      render(
        <ConfigureExperience
          selectedApproach={ApplicationCreateFlowSignInApproach.INBUILT}
          onApproachChange={mockOnApproachChange}
        />,
      );

      // Find the EMBEDDED card by its title text and click the card itself
      const embeddedTitle = screen.getByText('applications:onboarding.configure.approach.native.title');
      const embeddedCard = embeddedTitle.closest('[class*="MuiCard"]');
      await user.click(embeddedCard!);

      expect(mockOnApproachChange).toHaveBeenCalledWith(ApplicationCreateFlowSignInApproach.EMBEDDED);
    });
  });
});
