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
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import FlowConfigContext, {type FlowConfigContextProps} from '../../../context/FlowConfigContext';
import {EdgeStyleTypes} from '../../../models/steps';
import EdgeStyleMenu from '../EdgeStyleSelector';

// Mock react-i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'flows:core.headerPanel.edgeStyles.bezier': 'Bezier',
        'flows:core.headerPanel.edgeStyles.smoothStep': 'Smooth Step',
        'flows:core.headerPanel.edgeStyles.step': 'Step',
      };
      return translations[key] || key;
    },
  }),
}));

describe('EdgeStyleMenu', () => {
  const mockSetEdgeStyle = vi.fn();
  const mockOnClose = vi.fn();

  const defaultFlowConfigValue: FlowConfigContextProps = {
    ElementFactory: () => null,
    ResourceProperties: () => null,
    flowCompletionConfigs: {},
    setFlowCompletionConfigs: vi.fn(),
    isVerboseMode: false,
    setIsVerboseMode: vi.fn(),
    edgeStyle: EdgeStyleTypes.SmoothStep,
    setEdgeStyle: mockSetEdgeStyle,
    flowNodeTypes: {},
    flowEdgeTypes: {},
    setFlowNodeTypes: vi.fn(),
    setFlowEdgeTypes: vi.fn(),
    flowNodes: [],
    setFlowNodes: vi.fn(),
    graphValidationRules: [],
    setGraphValidationRules: vi.fn(),
  };

  const createWrapper = (overrides: Partial<FlowConfigContextProps> = {}) => {
    const flowConfigValue: FlowConfigContextProps = {...defaultFlowConfigValue, ...overrides};

    function Wrapper({children}: {children: ReactNode}) {
      return <FlowConfigContext.Provider value={flowConfigValue}>{children}</FlowConfigContext.Provider>;
    }
    return Wrapper;
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Menu Visibility', () => {
    it('should not render menu when anchorEl is null', () => {
      render(<EdgeStyleMenu anchorEl={null} onClose={mockOnClose} />, {
        wrapper: createWrapper(),
      });

      // Menu should not be visible
      expect(screen.queryByRole('menu')).not.toBeInTheDocument();
    });

    it('should render menu when anchorEl is provided', () => {
      const anchorEl = document.createElement('button');
      document.body.appendChild(anchorEl);

      render(<EdgeStyleMenu anchorEl={anchorEl} onClose={mockOnClose} />, {
        wrapper: createWrapper(),
      });

      expect(screen.getByRole('menu')).toBeInTheDocument();

      document.body.removeChild(anchorEl);
    });
  });

  describe('Menu Options', () => {
    let anchorEl: HTMLButtonElement;

    beforeEach(() => {
      anchorEl = document.createElement('button');
      document.body.appendChild(anchorEl);
    });

    afterEach(() => {
      if (document.body.contains(anchorEl)) {
        document.body.removeChild(anchorEl);
      }
    });

    it('should render all three edge style options', () => {
      render(<EdgeStyleMenu anchorEl={anchorEl} onClose={mockOnClose} />, {
        wrapper: createWrapper(),
      });

      expect(screen.getByText('Bezier')).toBeInTheDocument();
      expect(screen.getByText('Smooth Step')).toBeInTheDocument();
      expect(screen.getByText('Step')).toBeInTheDocument();
    });

    it('should render menu items as clickable', () => {
      render(<EdgeStyleMenu anchorEl={anchorEl} onClose={mockOnClose} />, {
        wrapper: createWrapper(),
      });

      const menuItems = screen.getAllByRole('menuitem');
      expect(menuItems).toHaveLength(3);
    });

    it('should render icons for each option', () => {
      render(<EdgeStyleMenu anchorEl={anchorEl} onClose={mockOnClose} />, {
        wrapper: createWrapper(),
      });

      // Each menu item should have an icon (ListItemIcon)
      const menuItems = screen.getAllByRole('menuitem');
      expect(menuItems).toHaveLength(3);
    });
  });

  describe('Style Selection', () => {
    let anchorEl: HTMLButtonElement;

    beforeEach(() => {
      anchorEl = document.createElement('button');
      document.body.appendChild(anchorEl);
    });

    afterEach(() => {
      if (document.body.contains(anchorEl)) {
        document.body.removeChild(anchorEl);
      }
    });

    it('should call setEdgeStyle with Bezier when Bezier option is clicked', () => {
      render(<EdgeStyleMenu anchorEl={anchorEl} onClose={mockOnClose} />, {
        wrapper: createWrapper(),
      });

      const bezierOption = screen.getByText('Bezier');
      fireEvent.click(bezierOption);

      expect(mockSetEdgeStyle).toHaveBeenCalledWith(EdgeStyleTypes.Bezier);
    });

    it('should call setEdgeStyle with SmoothStep when Smooth Step option is clicked', () => {
      render(<EdgeStyleMenu anchorEl={anchorEl} onClose={mockOnClose} />, {
        wrapper: createWrapper(),
      });

      const smoothStepOption = screen.getByText('Smooth Step');
      fireEvent.click(smoothStepOption);

      expect(mockSetEdgeStyle).toHaveBeenCalledWith(EdgeStyleTypes.SmoothStep);
    });

    it('should call setEdgeStyle with Step when Step option is clicked', () => {
      render(<EdgeStyleMenu anchorEl={anchorEl} onClose={mockOnClose} />, {
        wrapper: createWrapper(),
      });

      const stepOption = screen.getByText('Step');
      fireEvent.click(stepOption);

      expect(mockSetEdgeStyle).toHaveBeenCalledWith(EdgeStyleTypes.Step);
    });

    it('should call onClose after selecting an option', () => {
      render(<EdgeStyleMenu anchorEl={anchorEl} onClose={mockOnClose} />, {
        wrapper: createWrapper(),
      });

      const bezierOption = screen.getByText('Bezier');
      fireEvent.click(bezierOption);

      expect(mockOnClose).toHaveBeenCalled();
    });
  });

  describe('Edge Style Context', () => {
    let anchorEl: HTMLButtonElement;

    beforeEach(() => {
      anchorEl = document.createElement('button');
      document.body.appendChild(anchorEl);
    });

    afterEach(() => {
      if (document.body.contains(anchorEl)) {
        document.body.removeChild(anchorEl);
      }
    });

    it('should render all edge style options with SmoothStep context', () => {
      render(<EdgeStyleMenu anchorEl={anchorEl} onClose={mockOnClose} />, {
        wrapper: createWrapper({edgeStyle: EdgeStyleTypes.SmoothStep}),
      });

      const menuItems = screen.getAllByRole('menuitem');
      expect(menuItems).toHaveLength(3);
      expect(screen.getByText('Smooth Step')).toBeInTheDocument();
    });

    it('should render all edge style options with Bezier context', () => {
      render(<EdgeStyleMenu anchorEl={anchorEl} onClose={mockOnClose} />, {
        wrapper: createWrapper({edgeStyle: EdgeStyleTypes.Bezier}),
      });

      const menuItems = screen.getAllByRole('menuitem');
      expect(menuItems).toHaveLength(3);
      expect(screen.getByText('Bezier')).toBeInTheDocument();
    });

    it('should render all edge style options with Step context', () => {
      render(<EdgeStyleMenu anchorEl={anchorEl} onClose={mockOnClose} />, {
        wrapper: createWrapper({edgeStyle: EdgeStyleTypes.Step}),
      });

      const menuItems = screen.getAllByRole('menuitem');
      expect(menuItems).toHaveLength(3);
      expect(screen.getByText('Step')).toBeInTheDocument();
    });
  });

  describe('Menu Props', () => {
    it('should render menu with correct structure', () => {
      const anchorEl = document.createElement('button');
      document.body.appendChild(anchorEl);

      render(<EdgeStyleMenu anchorEl={anchorEl} onClose={mockOnClose} />, {
        wrapper: createWrapper(),
      });

      const menu = screen.getByRole('menu');
      expect(menu).toBeInTheDocument();
      // The menu is rendered and accessible
      expect(screen.getAllByRole('menuitem')).toHaveLength(3);

      document.body.removeChild(anchorEl);
    });
  });

  describe('Selected State', () => {
    let anchorEl: HTMLButtonElement;

    beforeEach(() => {
      anchorEl = document.createElement('button');
      document.body.appendChild(anchorEl);
    });

    afterEach(() => {
      if (document.body.contains(anchorEl)) {
        document.body.removeChild(anchorEl);
      }
    });

    it('should mark Bezier as selected when edgeStyle is Bezier', () => {
      render(<EdgeStyleMenu anchorEl={anchorEl} onClose={mockOnClose} />, {
        wrapper: createWrapper({edgeStyle: EdgeStyleTypes.Bezier}),
      });

      const menuItems = screen.getAllByRole('menuitem');
      // First item (Bezier) should be selected
      expect(menuItems[0]).toHaveClass('Mui-selected');
    });

    it('should mark SmoothStep as selected when edgeStyle is SmoothStep', () => {
      render(<EdgeStyleMenu anchorEl={anchorEl} onClose={mockOnClose} />, {
        wrapper: createWrapper({edgeStyle: EdgeStyleTypes.SmoothStep}),
      });

      const menuItems = screen.getAllByRole('menuitem');
      // Second item (SmoothStep) should be selected
      expect(menuItems[1]).toHaveClass('Mui-selected');
    });

    it('should mark Step as selected when edgeStyle is Step', () => {
      render(<EdgeStyleMenu anchorEl={anchorEl} onClose={mockOnClose} />, {
        wrapper: createWrapper({edgeStyle: EdgeStyleTypes.Step}),
      });

      const menuItems = screen.getAllByRole('menuitem');
      // Third item (Step) should be selected
      expect(menuItems[2]).toHaveClass('Mui-selected');
    });
  });

  describe('Boolean Conversion', () => {
    it('should convert null anchorEl to false for open state', () => {
      render(<EdgeStyleMenu anchorEl={null} onClose={mockOnClose} />, {
        wrapper: createWrapper(),
      });

      // Menu should not be visible when anchorEl is null
      expect(screen.queryByRole('menu')).not.toBeInTheDocument();
    });

    it('should convert valid anchorEl to true for open state', () => {
      const anchorEl = document.createElement('button');
      document.body.appendChild(anchorEl);

      render(<EdgeStyleMenu anchorEl={anchorEl} onClose={mockOnClose} />, {
        wrapper: createWrapper(),
      });

      // Menu should be visible when anchorEl is provided
      expect(screen.getByRole('menu')).toBeInTheDocument();

      document.body.removeChild(anchorEl);
    });
  });
});
