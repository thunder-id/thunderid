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
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Resources} from '../../../models/resources';
import ResourcePanel from '../ResourcePanel';

// Mock react-router
const mockNavigate = vi.fn();
vi.mock('react-router', () => ({
  useNavigate: () => mockNavigate,
}));

// Mock react-i18next
const translations: Record<string, string> = {
  'flows:core.resourcePanel.showResources': 'Show Resources',
  'flows:core.resourcePanel.hideResources': 'Hide Resources',
  'flows:core.headerPanel.goBack': 'Back',
  'flows:core.headerPanel.editTitle': 'Edit Title',
  'flows:core.headerPanel.saveTitle': 'Save Title',
  'flows:core.headerPanel.cancelEdit': 'Cancel',
  'flows:core.resourcePanel.starterTemplates.title': 'Starter Templates',
  'flows:core.resourcePanel.starterTemplates.description': 'Quick start templates for your flow',
  'flows:core.resourcePanel.search.placeholder': 'Search (e.g. MFA, social, consent)',
  'flows:core.resourcePanel.search.clear': 'Clear search',
  'flows:core.resourcePanel.search.noResults': 'No matching resources',
  'flows:core.resourcePanel.search.noResultsHint': 'Try a different keyword',
  'flows:core.resourcePanel.widgets.title': 'Widgets',
  'flows:core.resourcePanel.widgets.description': 'Configurable widgets',
  'flows:core.resourcePanel.steps.title': 'Steps',
  'flows:core.resourcePanel.steps.description': 'Flow step types',
  'flows:core.resourcePanel.components.title': 'Components',
  'flows:core.resourcePanel.components.description': 'UI components',
  'flows:core.resourcePanel.executors.title': 'Executors',
  'flows:core.resourcePanel.executors.description': 'Execution handlers',
};

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => translations[key] || key,
  }),
}));

// Mock useUIPanelState
const mockSetIsResourcePanelOpen = vi.fn();
vi.mock('../../../hooks/useUIPanelState', () => ({
  default: () => ({
    setIsResourcePanelOpen: mockSetIsResourcePanelOpen,
  }),
}));

// Mock ResourcePanelStatic
vi.mock('../ResourcePanelStatic', () => ({
  default: ({
    resource,
    onAdd,
    disabled,
  }: {
    resource: {display?: {label?: string}};
    onAdd: () => void;
    disabled: boolean;
  }) => (
    <div data-testid="resource-panel-static" data-disabled={disabled}>
      <span>{resource?.display?.label}</span>
      <button type="button" onClick={() => onAdd()}>
        Add Static
      </button>
    </div>
  ),
}));

// Mock ResourcePanelDraggable
vi.mock('../ResourcePanelDraggable', () => ({
  default: ({
    resource,
    onAdd,
    disabled,
  }: {
    resource: {display?: {label?: string}};
    onAdd: () => void;
    disabled: boolean;
  }) => (
    <div data-testid="resource-panel-draggable" data-disabled={disabled}>
      <span>{resource?.display?.label}</span>
      <button type="button" onClick={() => onAdd()}>
        Add Draggable
      </button>
    </div>
  ),
}));

const createMockResources = (overrides: Partial<Resources> = {}): Resources =>
  ({
    templates: [
      {
        type: 'BASIC_TEMPLATE',
        resourceType: 'TEMPLATE',
        display: {label: 'Basic Template', showOnResourcePanel: true},
      },
    ],
    widgets: [
      {
        type: 'LOGIN_WIDGET',
        resourceType: 'WIDGET',
        display: {label: 'Login Widget', showOnResourcePanel: true},
      },
    ],
    steps: [
      {
        type: 'VIEW_STEP',
        resourceType: 'STEP',
        display: {label: 'View Step', showOnResourcePanel: true},
      },
    ],
    elements: [
      {
        type: 'TEXT_INPUT',
        resourceType: 'ELEMENT',
        category: 'INPUT',
        display: {label: 'Text Input', showOnResourcePanel: true},
      },
    ],
    executors: [
      {
        type: 'TASK_EXECUTION',
        resourceType: 'STEP',
        category: 'EXECUTOR',
        display: {label: 'Google', showOnResourcePanel: true},
      },
    ],
    ...overrides,
  }) as Resources;

describe('ResourcePanel', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Panel Open/Close States', () => {
    it('should render expand button when panel is closed', () => {
      render(<ResourcePanel resources={createMockResources()} onAdd={vi.fn()} open={false} />);

      // When closed, the expand button should be visible
      const buttons = screen.getAllByRole('button');
      expect(buttons.length).toBeGreaterThan(0);
    });

    it('should render drawer when panel is open', () => {
      const {container} = render(<ResourcePanel resources={createMockResources()} onAdd={vi.fn()} open />);

      // Drawer should be in the document when open
      const drawer = container.querySelector('.MuiDrawer-root');
      expect(drawer).toBeInTheDocument();
    });

    it('should call setIsResourcePanelOpen when toggle button is clicked', () => {
      render(<ResourcePanel resources={createMockResources()} onAdd={vi.fn()} open />);

      // Find all buttons and check for collapse functionality
      const buttons = screen.getAllByRole('button');
      // Try to find a collapse button with aria-label or specific icon
      const collapseButton = buttons.find(
        (btn) =>
          btn.getAttribute('aria-label')?.toLowerCase().includes('hide') ??
          btn.getAttribute('aria-label')?.toLowerCase().includes('collapse') ??
          btn.getAttribute('aria-label')?.toLowerCase().includes('close'),
      );

      expect(buttons.length).toBeGreaterThan(0);
      expect(collapseButton).toBeDefined();
      fireEvent.click(collapseButton!);
      expect(mockSetIsResourcePanelOpen).toHaveBeenCalled();
    });
  });

  describe('Flow Title', () => {
    it('should display flow title when provided', () => {
      render(<ResourcePanel resources={createMockResources()} onAdd={vi.fn()} open flowTitle="My Test Flow" />);

      expect(screen.getByText('My Test Flow')).toBeInTheDocument();
    });

    it('should display flow handle when provided', () => {
      render(
        <ResourcePanel
          resources={createMockResources()}
          onAdd={vi.fn()}
          open
          flowTitle="My Test Flow"
          flowHandle="my-test-flow"
        />,
      );

      expect(screen.getByText('my-test-flow')).toBeInTheDocument();
    });

    it('should not show edit button when onFlowTitleChange is not provided', () => {
      render(<ResourcePanel resources={createMockResources()} onAdd={vi.fn()} open flowTitle="My Test Flow" />);

      // Edit button should not be present when onFlowTitleChange is not provided
      const editButtons = screen.queryAllByRole('button').filter((btn) => {
        const svg = btn.querySelector('svg');
        return svg && btn.closest('[data-testid]')?.getAttribute('data-testid')?.includes('edit');
      });
      expect(editButtons).toHaveLength(0);
    });
  });

  describe('Title Editing', () => {
    it('should show text field when edit mode is active', () => {
      const onFlowTitleChange = vi.fn();
      render(
        <ResourcePanel
          resources={createMockResources()}
          onAdd={vi.fn()}
          open
          flowTitle="My Test Flow"
          onFlowTitleChange={onFlowTitleChange}
        />,
      );

      // Find and click the edit button by its aria-label
      const editButton = screen.getByRole('button', {name: /edit title/i});
      fireEvent.click(editButton);
      expect(screen.getByRole('textbox')).toBeInTheDocument();
    });

    it('should call onFlowTitleChange when title is saved', () => {
      const onFlowTitleChange = vi.fn();
      render(
        <ResourcePanel
          resources={createMockResources()}
          onAdd={vi.fn()}
          open
          flowTitle="My Test Flow"
          onFlowTitleChange={onFlowTitleChange}
        />,
      );

      // Find all buttons and click the edit button
      const buttons = screen.getAllByRole('button');
      // Click the first small button which should be edit
      buttons.forEach((btn) => {
        if (btn.querySelector('svg')) {
          fireEvent.click(btn);
        }
      });

      const textField = screen.queryByRole('textbox');
      expect(textField).toBeTruthy();
      fireEvent.change(textField!, {target: {value: 'New Title'}});
      fireEvent.keyDown(textField!, {key: 'Enter'});
      expect(onFlowTitleChange).toHaveBeenCalledWith('New Title');
    });

    it('should cancel editing when Escape is pressed', () => {
      const onFlowTitleChange = vi.fn();
      render(
        <ResourcePanel
          resources={createMockResources()}
          onAdd={vi.fn()}
          open
          flowTitle="My Test Flow"
          onFlowTitleChange={onFlowTitleChange}
        />,
      );

      // Enter edit mode by clicking edit button
      const buttons = screen.getAllByRole('button');
      buttons.forEach((btn) => {
        if (btn.querySelector('svg')) {
          fireEvent.click(btn);
        }
      });

      const textField = screen.queryByRole('textbox');
      expect(textField).toBeTruthy();
      fireEvent.change(textField!, {target: {value: 'Changed Title'}});
      fireEvent.keyDown(textField!, {key: 'Escape'});
      // Title should not be changed
      expect(onFlowTitleChange).not.toHaveBeenCalled();
    });
  });

  describe('Navigation', () => {
    it('should not render back button in panel header (moved to top bar)', () => {
      render(<ResourcePanel resources={createMockResources()} onAdd={vi.fn()} open flowTitle="Test" />);

      expect(screen.queryByText('Back')).not.toBeInTheDocument();
    });
  });

  describe('Resource Sections', () => {
    it('should render Widgets, Steps, Components, and Executors accordions', () => {
      render(<ResourcePanel resources={createMockResources()} onAdd={vi.fn()} open />);

      expect(screen.getByText('Widgets')).toBeInTheDocument();
      expect(screen.getByText('Steps')).toBeInTheDocument();
      expect(screen.getByText('Components')).toBeInTheDocument();
      expect(screen.getByText('Executors')).toBeInTheDocument();
    });

    it('should show section descriptions in the collapsed accordion summary', () => {
      render(<ResourcePanel resources={createMockResources()} onAdd={vi.fn()} open />);

      expect(screen.getByText('Configurable widgets').closest('.MuiAccordionSummary-root')).not.toBeNull();
      expect(screen.getByText('Execution handlers').closest('.MuiAccordionSummary-root')).not.toBeNull();
    });
  });

  describe('Search', () => {
    it('should filter resources by label and hide sections without matches', () => {
      render(<ResourcePanel resources={createMockResources()} onAdd={vi.fn()} open />);

      const searchInput = screen.getByPlaceholderText('Search (e.g. MFA, social, consent)');
      fireEvent.change(searchInput, {target: {value: 'Text Input'}});

      expect(screen.getByText('Components')).toBeInTheDocument();
      expect(screen.getByText('Text Input')).toBeInTheDocument();
      expect(screen.queryByText('Widgets')).not.toBeInTheDocument();
      expect(screen.queryByText('Executors')).not.toBeInTheDocument();
    });

    it('should match resources via capability synonyms', () => {
      render(<ResourcePanel resources={createMockResources()} onAdd={vi.fn()} open />);

      const searchInput = screen.getByPlaceholderText('Search (e.g. MFA, social, consent)');
      fireEvent.change(searchInput, {target: {value: 'social'}});

      expect(screen.getByText('Executors')).toBeInTheDocument();
      expect(screen.getByText('Google')).toBeInTheDocument();
      expect(screen.queryByText('Components')).not.toBeInTheDocument();
    });

    it('should show an empty state when nothing matches', () => {
      render(<ResourcePanel resources={createMockResources()} onAdd={vi.fn()} open />);

      const searchInput = screen.getByPlaceholderText('Search (e.g. MFA, social, consent)');
      fireEvent.change(searchInput, {target: {value: 'zzz-no-match'}});

      expect(screen.getByText('No matching resources')).toBeInTheDocument();
    });

    it('should restore all sections when the search is cleared', () => {
      render(<ResourcePanel resources={createMockResources()} onAdd={vi.fn()} open />);

      const searchInput = screen.getByPlaceholderText('Search (e.g. MFA, social, consent)');
      fireEvent.change(searchInput, {target: {value: 'zzz-no-match'}});
      fireEvent.click(screen.getByRole('button', {name: 'Clear search'}));

      expect(screen.getByText('Widgets')).toBeInTheDocument();
      expect(screen.getByText('Components')).toBeInTheDocument();
    });
  });

  describe('Resource Items', () => {
    it('should render draggable items for widgets, steps, elements, and executors', () => {
      render(<ResourcePanel resources={createMockResources()} onAdd={vi.fn()} open />);

      const draggableItems = screen.getAllByTestId('resource-panel-draggable');
      expect(draggableItems.length).toBeGreaterThan(0);
    });
  });

  describe('Resource Filtering', () => {
    it('should filter out resources with showOnResourcePanel=false', () => {
      const resources = createMockResources({
        steps: [
          {
            type: 'VISIBLE_STEP',
            resourceType: 'STEP',
            display: {label: 'Visible Step', showOnResourcePanel: true},
          },
          {
            type: 'HIDDEN_STEP',
            resourceType: 'STEP',
            display: {label: 'Hidden Step', showOnResourcePanel: false},
          },
        ],
      } as Partial<Resources>);

      render(<ResourcePanel resources={resources} onAdd={vi.fn()} open />);

      expect(screen.getByText('Visible Step')).toBeInTheDocument();
      expect(screen.queryByText('Hidden Step')).not.toBeInTheDocument();
    });
  });

  describe('Disabled State', () => {
    it('should pass disabled prop to resource items', () => {
      render(<ResourcePanel resources={createMockResources()} onAdd={vi.fn()} open disabled />);

      const draggableItems = screen.getAllByTestId('resource-panel-draggable');
      draggableItems.forEach((item) => {
        expect(item).toHaveAttribute('data-disabled', 'true');
      });
    });
  });

  describe('Children Rendering', () => {
    it('should render children in main content area', () => {
      render(
        <ResourcePanel resources={createMockResources()} onAdd={vi.fn()} open>
          <div data-testid="canvas-content">Canvas Content</div>
        </ResourcePanel>,
      );

      expect(screen.getByTestId('canvas-content')).toBeInTheDocument();
    });
  });

  describe('onAdd Callback', () => {
    it('should call onAdd when draggable resource add button is clicked', () => {
      const onAdd = vi.fn();
      render(<ResourcePanel resources={createMockResources()} onAdd={onAdd} open />);

      const addButtons = screen.getAllByText('Add Draggable');
      fireEvent.click(addButtons[0]);

      expect(onAdd).toHaveBeenCalled();
    });
  });

  describe('Toggle Panel', () => {
    it('should toggle panel state when expand button is clicked when closed', () => {
      render(<ResourcePanel resources={createMockResources()} onAdd={vi.fn()} open={false} />);

      // Find the expand button (ChevronRightIcon button)
      const buttons = screen.getAllByRole('button');
      const expandButton = buttons[0]; // First button should be expand when closed

      fireEvent.click(expandButton);

      expect(mockSetIsResourcePanelOpen).toHaveBeenCalled();
    });

    it('should toggle panel state when collapse button is clicked when open', () => {
      render(<ResourcePanel resources={createMockResources()} onAdd={vi.fn()} open />);

      // Find buttons and click the collapse (ChevronLeftIcon) button
      const buttons = screen.getAllByRole('button');
      // Find button that contains the collapse icon (look for one near the top of panel)
      let toggleFound = false;
      buttons.forEach((btn) => {
        if (!toggleFound) {
          fireEvent.click(btn);
          if (mockSetIsResourcePanelOpen.mock.calls.length > 0) {
            toggleFound = true;
          }
        }
      });

      // Verify at least one call to toggle happened
      expect(mockSetIsResourcePanelOpen).toHaveBeenCalled();
    });

    it('should call setIsResourcePanelOpen with toggle function', () => {
      render(<ResourcePanel resources={createMockResources()} onAdd={vi.fn()} open />);

      // Trigger the toggle
      const buttons = screen.getAllByRole('button');
      buttons.forEach((btn) => {
        fireEvent.click(btn);
      });

      expect(mockSetIsResourcePanelOpen.mock.calls.length).toBeGreaterThan(0);
      // Verify it was called with a function that toggles the value
      const toggleFn = mockSetIsResourcePanelOpen.mock.calls[0][0] as ((prev: boolean) => boolean) | undefined;
      expect(typeof toggleFn).toBe('function');
      expect((toggleFn as (prev: boolean) => boolean)(true)).toBe(false);
      expect((toggleFn as (prev: boolean) => boolean)(false)).toBe(true);
    });
  });
});
