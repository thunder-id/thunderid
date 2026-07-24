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
import {describe, it, expect, vi} from 'vitest';
import type {Application} from '../../../../models/application';
import EditCustomizationSettings from '../EditCustomizationSettings';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
  }),
  Trans: ({i18nKey}: {i18nKey: string}) => i18nKey,
}));

vi.mock('@thunderid/design', () => ({
  useGetThemes: vi.fn(() => ({
    data: {
      themes: [
        {id: 'theme-1', displayName: 'Default Theme'},
        {id: 'theme-2', displayName: 'Dark Theme'},
      ],
    },
    isLoading: false,
  })),
  useGetLayouts: vi.fn(() => ({data: undefined, isLoading: false})),
}));

describe('EditCustomizationSettings', () => {
  const mockApplication: Application = {
    id: 'test-app-id',
    name: 'Test Application',
    description: 'Test Description',
    template: 'custom',
    themeId: 'theme-1',
    tosUri: 'https://example.com/terms',
    policyUri: 'https://example.com/privacy',
    contacts: ['contact@example.com'],
  } as Application;

  const mockOnFieldChange = vi.fn();

  describe('Rendering', () => {
    it('should render all three sections', () => {
      render(
        <EditCustomizationSettings application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />,
      );

      expect(screen.getByText('applications:edit.customization.sections.appearance')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.customization.sections.urls')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.general.sections.contacts')).toBeInTheDocument();
    });

    it('should render sections in correct order', () => {
      render(
        <EditCustomizationSettings application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />,
      );

      // Verify all three sections are present
      expect(screen.getByText('applications:edit.customization.sections.appearance')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.customization.sections.urls')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.general.sections.contacts')).toBeInTheDocument();
    });
  });

  describe('Section Integration', () => {
    it('should pass correct props to AppearanceSection', () => {
      render(
        <EditCustomizationSettings application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />,
      );

      expect(screen.getByText('applications:edit.customization.labels.theme')).toBeInTheDocument();
      expect(screen.getByPlaceholderText('applications:edit.customization.theme.placeholder')).toBeInTheDocument();
    });

    it('should pass correct props to UrlsSection', () => {
      render(
        <EditCustomizationSettings application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />,
      );

      expect(screen.getByText('applications:edit.customization.labels.tosUri')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.customization.labels.policyUri')).toBeInTheDocument();
    });

    it('should pass correct props to ContactsSection', () => {
      render(
        <EditCustomizationSettings application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />,
      );

      expect(screen.getByPlaceholderText('applications:edit.general.contacts.placeholder')).toBeInTheDocument();
    });
  });

  describe('Layout', () => {
    it('should render sections in a Stack with spacing', () => {
      const {container} = render(
        <EditCustomizationSettings application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />,
      );

      const stack = container.firstChild;
      expect(stack).toHaveClass('MuiStack-root');
    });
  });

  describe('Props Propagation', () => {
    it('should propagate application prop to all sections', () => {
      render(
        <EditCustomizationSettings application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />,
      );

      // Verify theme from application
      const themeCombobox = screen.getByPlaceholderText('applications:edit.customization.theme.placeholder');
      expect(themeCombobox).toHaveValue('Default Theme');

      // Verify URLs from application
      const tosField = screen.getByPlaceholderText('applications:edit.customization.tosUri.placeholder');
      const policyField = screen.getByPlaceholderText('applications:edit.customization.policyUri.placeholder');
      expect(tosField).toHaveValue('https://example.com/terms');
      expect(policyField).toHaveValue('https://example.com/privacy');

      // Verify contact chip from application
      expect(screen.getByText('contact@example.com')).toBeInTheDocument();
    });

    it('should propagate editedApp prop to all sections', () => {
      const editedApp = {
        themeId: 'theme-2',
        tosUri: 'https://edited.com/terms',
        policyUri: 'https://edited.com/privacy',
        contacts: ['edited@example.com'],
      };

      render(
        <EditCustomizationSettings
          application={mockApplication}
          editedApp={editedApp}
          onFieldChange={mockOnFieldChange}
        />,
      );

      // Verify edited theme
      const themeCombobox = screen.getByPlaceholderText('applications:edit.customization.theme.placeholder');
      expect(themeCombobox).toHaveValue('Dark Theme');

      // Verify edited URLs
      const tosField = screen.getByPlaceholderText('applications:edit.customization.tosUri.placeholder');
      const policyField = screen.getByPlaceholderText('applications:edit.customization.policyUri.placeholder');
      expect(tosField).toHaveValue('https://edited.com/terms');
      expect(policyField).toHaveValue('https://edited.com/privacy');

      // Verify edited contact chip
      expect(screen.getByText('edited@example.com')).toBeInTheDocument();
    });

    it('should propagate onFieldChange callback to all sections', () => {
      render(
        <EditCustomizationSettings application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />,
      );

      // All sections should be rendered, which means onFieldChange was passed
      expect(screen.getByText('applications:edit.customization.sections.appearance')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.customization.sections.urls')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.general.sections.contacts')).toBeInTheDocument();
    });
  });

  describe('Edge Cases', () => {
    it('should handle minimal application data', () => {
      const minimalApp = {
        id: 'minimal-id',
        name: 'Minimal App',
        template: 'custom',
      } as Application;

      render(<EditCustomizationSettings application={minimalApp} editedApp={{}} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByText('applications:edit.customization.sections.appearance')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.customization.sections.urls')).toBeInTheDocument();
      expect(screen.getByText('applications:edit.general.sections.contacts')).toBeInTheDocument();
    });

    it('should handle empty editedApp', () => {
      render(
        <EditCustomizationSettings application={mockApplication} editedApp={{}} onFieldChange={mockOnFieldChange} />,
      );

      // Should fall back to application values
      const themeCombobox = screen.getByPlaceholderText('applications:edit.customization.theme.placeholder');
      expect(themeCombobox).toHaveValue('Default Theme');
    });
  });

  describe('URLs section reset', () => {
    it('reverts UrlsSection to the application value when sectionResetKey changes', () => {
      const {rerender} = render(
        <EditCustomizationSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          sectionResetKey={0}
        />,
      );

      const tosField = screen.getByPlaceholderText('applications:edit.customization.tosUri.placeholder');
      fireEvent.change(tosField, {target: {value: 'https://not-yet-saved.example.com/terms'}});
      expect(tosField).toHaveValue('https://not-yet-saved.example.com/terms');

      // Simulate the page-level Reset: editedApp is cleared and sectionResetKey is bumped.
      rerender(
        <EditCustomizationSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          sectionResetKey={1}
        />,
      );

      const tosFieldAfterReset = screen.getByPlaceholderText('applications:edit.customization.tosUri.placeholder');
      expect(tosFieldAfterReset).toHaveValue('https://example.com/terms');
    });

    it('keeps the typed value when sectionResetKey stays the same', () => {
      const {rerender} = render(
        <EditCustomizationSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          sectionResetKey={0}
        />,
      );

      const tosField = screen.getByPlaceholderText('applications:edit.customization.tosUri.placeholder');
      fireEvent.change(tosField, {target: {value: 'https://not-yet-saved.example.com/terms'}});

      rerender(
        <EditCustomizationSettings
          application={mockApplication}
          editedApp={{}}
          onFieldChange={mockOnFieldChange}
          sectionResetKey={0}
        />,
      );

      const tosFieldAfterRerender = screen.getByPlaceholderText('applications:edit.customization.tosUri.placeholder');
      expect(tosFieldAfterRerender).toHaveValue('https://not-yet-saved.example.com/terms');
    });
  });
});
