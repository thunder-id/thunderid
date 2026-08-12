// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import FlowsListView, {type FlowsListViewProps} from '../FlowsListView';
import {type BasicFlowDefinition} from '@/features/flows/models/responses';

// Mock react-i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'common:or': 'or',
        'applications:onboarding.configure.SignInOptions.preConfiguredFlows.searchFlows': 'Search flows...',
        'applications:onboarding.configure.SignInOptions.preConfiguredFlows.toggleLabel':
          'Use a pre-configured flow instead',
      };
      return translations[key] || key;
    },
  }),
}));

describe('FlowsListView', () => {
  const mockOnFlowSelect = vi.fn();
  const mockOnClearSelection = vi.fn();

  const mockFlows: BasicFlowDefinition[] = [
    {
      id: 'flow-1',
      name: 'Basic Authentication Flow',
      activeVersion: 1,
      handle: 'basic-auth-flow',
      flowType: 'AUTHENTICATION',
      createdAt: '',
      updatedAt: '',
    },
    {
      id: 'flow-2',
      name: 'Google OAuth Flow',
      activeVersion: 1,
      handle: 'google-oauth-flow',
      flowType: 'AUTHENTICATION',
      createdAt: '',
      updatedAt: '',
    },
    {
      id: 'flow-3',
      name: 'Multi-Factor Auth Flow',
      activeVersion: 1,
      handle: 'mfa-flow',
      flowType: 'AUTHENTICATION',
      createdAt: '',
      updatedAt: '',
    },
  ];

  const defaultProps: FlowsListViewProps = {
    availableFlows: mockFlows,
    selectedAuthFlow: null,
    onFlowSelect: mockOnFlowSelect,
    onClearSelection: mockOnClearSelection,
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  const renderComponent = (props: Partial<FlowsListViewProps> = {}) =>
    render(<FlowsListView {...defaultProps} {...props} />);

  // The card is collapsed by default; expand it by clicking its header row so tests can reach
  // the autocomplete inside.
  const renderExpanded = (props: Partial<FlowsListViewProps> = {}) => {
    const result = renderComponent(props);
    fireEvent.click(screen.getByRole('button', {name: /pre-configured flow instead/i}));
    return result;
  };

  describe('rendering', () => {
    it('should return null when no selectable flows available', () => {
      const {container} = renderComponent({
        availableFlows: [],
      });

      expect(container.firstChild).toBeNull();
    });

    it('should return null when all flows are console-app flows', () => {
      const consoleAppFlows: BasicFlowDefinition[] = [
        {
          id: 'flow-1',
          name: 'Console App Flow',
          activeVersion: 1,
          handle: 'console-app-login',
          flowType: 'AUTHENTICATION',
          createdAt: '',
          updatedAt: '',
        },
      ];

      const {container} = renderComponent({
        availableFlows: consoleAppFlows,
      });

      expect(container.firstChild).toBeNull();
    });

    it('should return null when all flows are default flows', () => {
      const defaultFlows: BasicFlowDefinition[] = [
        {
          id: 'flow-1',
          name: 'Default Login Flow',
          activeVersion: 1,
          handle: 'default-login',
          flowType: 'AUTHENTICATION',
          createdAt: '',
          updatedAt: '',
        },
      ];

      const {container} = renderComponent({
        availableFlows: defaultFlows,
      });

      expect(container.firstChild).toBeNull();
    });

    it('should render divider with "or" text', () => {
      renderComponent();

      expect(screen.getByText('or')).toBeInTheDocument();
    });

    it('should render the pre-configured flow card title', () => {
      renderComponent();

      expect(screen.getByText('Use a pre-configured flow instead')).toBeInTheDocument();
    });

    it('should be collapsed by default, with the autocomplete not shown', () => {
      renderComponent();

      expect(screen.queryByRole('combobox')).not.toBeInTheDocument();
    });

    it('should reveal the autocomplete once expanded', () => {
      renderExpanded();

      expect(screen.getByRole('combobox')).toBeInTheDocument();
    });
  });

  describe('flow filtering', () => {
    it('should filter out console-app flows', () => {
      const mixedFlows: BasicFlowDefinition[] = [
        ...mockFlows,
        {
          id: 'console-flow',
          name: 'Console App Flow',
          activeVersion: 1,
          handle: 'console-app-flow',
          flowType: 'AUTHENTICATION',
          createdAt: '',
          updatedAt: '',
        },
      ];

      renderExpanded({availableFlows: mixedFlows});

      // The component should render since there are selectable flows
      expect(screen.getByRole('combobox')).toBeInTheDocument();
    });

    it('should filter out default flows', () => {
      const mixedFlows: BasicFlowDefinition[] = [
        ...mockFlows,
        {
          id: 'default-flow',
          name: 'Default Flow',
          activeVersion: 1,
          handle: 'default-auth-flow',
          flowType: 'AUTHENTICATION',
          createdAt: '',
          updatedAt: '',
        },
      ];

      renderExpanded({availableFlows: mixedFlows});

      expect(screen.getByRole('combobox')).toBeInTheDocument();
    });
  });

  describe('autocomplete interaction', () => {
    it('should call onFlowSelect when a flow is selected', async () => {
      const user = userEvent.setup();
      renderExpanded();

      const autocomplete = screen.getByRole('combobox');
      await user.click(autocomplete);

      const flowOption = screen.getByText('Basic Authentication Flow');
      await user.click(flowOption);

      expect(mockOnFlowSelect).toHaveBeenCalledWith('flow-1');
    });

    it('should call onClearSelection when selection is cleared', async () => {
      const user = userEvent.setup();
      renderExpanded({
        selectedAuthFlow: mockFlows[0],
      });

      const autocomplete = screen.getByRole('combobox');
      await user.click(autocomplete);

      // Clear the selection by clicking outside or selecting null
      await user.clear(autocomplete);
      await user.tab(); // blur to trigger onChange with null

      expect(mockOnClearSelection).toHaveBeenCalled();
    });

    it('should show selected flow value in autocomplete', () => {
      renderComponent({
        selectedAuthFlow: mockFlows[1],
      });

      expect(screen.getByRole('combobox')).toHaveValue('Google OAuth Flow');
    });

    it('should display flow options when opened', async () => {
      const user = userEvent.setup();
      renderExpanded();

      const autocomplete = screen.getByRole('combobox');
      await user.click(autocomplete);

      // Check that flow options are displayed
      expect(screen.getByText('Basic Authentication Flow')).toBeInTheDocument();
      expect(screen.getByText('Google OAuth Flow')).toBeInTheDocument();
      expect(screen.getByText('Multi-Factor Auth Flow')).toBeInTheDocument();
    });
  });

  describe('disabled state', () => {
    it('should disable autocomplete when disabled prop is true', () => {
      renderExpanded({disabled: true});

      const autocomplete = screen.getByRole('combobox');
      expect(autocomplete).toBeDisabled();
    });

    it('should enable autocomplete when disabled prop is false', () => {
      renderExpanded({disabled: false});

      const autocomplete = screen.getByRole('combobox');
      expect(autocomplete).not.toBeDisabled();
    });
  });

  describe('edge cases', () => {
    it('should handle flows with special characters in names', async () => {
      const user = userEvent.setup();
      const specialFlows: BasicFlowDefinition[] = [
        {
          id: 'special-flow',
          name: 'OAuth 2 & OIDC Flow',
          activeVersion: 1,
          handle: 'oauth-oidc-flow',
          flowType: 'AUTHENTICATION',
          createdAt: '',
          updatedAt: '',
        },
      ];

      renderExpanded({availableFlows: specialFlows});

      const autocomplete = screen.getByRole('combobox');
      await user.click(autocomplete);

      expect(screen.getByText('OAuth 2 & OIDC Flow')).toBeInTheDocument();
    });

    it('should handle flows with very long names', async () => {
      const user = userEvent.setup();
      const longNameFlows: BasicFlowDefinition[] = [
        {
          id: 'long-flow',
          name: 'This is a very long flow name that should still be displayed properly without breaking the layout',
          activeVersion: 1,
          handle: 'long-name-flow',
          flowType: 'AUTHENTICATION',
          createdAt: '',
          updatedAt: '',
        },
      ];

      renderExpanded({availableFlows: longNameFlows});

      const autocomplete = screen.getByRole('combobox');
      await user.click(autocomplete);

      expect(screen.getByText(/This is a very long flow name/)).toBeInTheDocument();
    });

    it('should handle when selectedAuthFlow is not in available flows', () => {
      const unknownFlow: BasicFlowDefinition = {
        id: 'unknown-flow',
        name: 'Unknown Flow',
        activeVersion: 1,
        handle: 'unknown-flow',
        flowType: 'AUTHENTICATION',
        createdAt: '',
        updatedAt: '',
      };

      renderExpanded({
        selectedAuthFlow: unknownFlow,
      });

      // Should not crash and should still render
      expect(screen.getByRole('combobox')).toBeInTheDocument();
    });
  });

  describe('accessibility', () => {
    it('should have proper ARIA attributes for autocomplete', () => {
      renderExpanded();

      const combobox = screen.getByRole('combobox');
      expect(combobox).toHaveAttribute('aria-autocomplete', 'list');
    });

    it('should be keyboard navigable', async () => {
      const user = userEvent.setup();
      renderExpanded();

      const combobox = screen.getByRole('combobox');
      await user.click(combobox);
      expect(combobox).toHaveFocus();
    });

    it('should expand dropdown on Enter key', async () => {
      const user = userEvent.setup();
      renderExpanded();

      const combobox = screen.getByRole('combobox');
      await user.click(combobox);
      await user.keyboard('{ArrowDown}');

      // Dropdown should be expanded
      expect(combobox).toHaveAttribute('aria-expanded', 'true');
    });
  });
});
