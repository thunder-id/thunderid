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

import {render, screen, act} from '@testing-library/react';
import {useContext} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import {PreviewScreenType} from '../../models/custom-text-preference';
import type {Resource} from '../../models/resources';
import {ResourceTypes} from '../../models/resources';
import {EdgeStyleTypes} from '../../models/steps';
import FlowBuilderCoreProvider from '../FlowBuilderCoreProvider';
import FlowConfigContext from '../FlowConfigContext';
import I18nContext from '../I18nContext';
import InteractionContext from '../InteractionContext';
import UIPanelContext from '../UIPanelContext';

// Mock @xyflow/react
vi.mock('@xyflow/react', () => ({
  ReactFlowProvider: ({children}: {children: React.ReactNode}) => (
    <div data-testid="react-flow-provider">{children}</div>
  ),
}));

// Mock ValidationProvider
vi.mock('../ValidationProvider', () => ({
  default: ({children}: {children: React.ReactNode}) => <div data-testid="validation-provider">{children}</div>,
}));

// Mock Oxygen UI
vi.mock('@wso2/oxygen-ui', () => ({
  Stack: ({children}: {children: React.ReactNode}) => <div>{children}</div>,
  Typography: ({children}: {children: React.ReactNode}) => <span>{children}</span>,
}));

vi.mock('@wso2/oxygen-ui-icons-react', () => ({
  CogIcon: () => <span data-testid="settings-icon" />,
}));

const mockCloseValidationCallback = vi.fn();

// Test components
function MockElementFactory() {
  return <div data-testid="element-factory">Element Factory</div>;
}
function MockResourceProperties() {
  return <div data-testid="resource-properties">Resource Properties</div>;
}

// Test consumer component
function TestConsumer() {
  const uiPanel = useContext(UIPanelContext)!;
  const interaction = useContext(InteractionContext)!;
  const flowConfig = useContext(FlowConfigContext)!;
  const i18n = useContext(I18nContext)!;
  return (
    <div>
      <span data-testid="is-resource-panel-open">{uiPanel.isResourcePanelOpen?.toString()}</span>
      <span data-testid="is-resource-properties-panel-open">{uiPanel.isResourcePropertiesPanelOpen?.toString()}</span>
      <span data-testid="is-version-history-panel-open">{uiPanel.isVersionHistoryPanelOpen?.toString()}</span>
      <span data-testid="is-verbose-mode">{flowConfig.isVerboseMode?.toString()}</span>
      <span data-testid="edge-style">{flowConfig.edgeStyle}</span>
      <span data-testid="primary-i18n-screen">{i18n.primaryI18nScreen}</span>
      <button
        type="button"
        data-testid="set-resource-panel-open"
        onClick={() => uiPanel.setIsResourcePanelOpen?.(false)}
      >
        Close Resource Panel
      </button>
      <button
        type="button"
        data-testid="set-resource-properties-panel-open"
        onClick={() => uiPanel.setIsOpenResourcePropertiesPanel?.(true)}
      >
        Open Properties Panel
      </button>
      <button
        type="button"
        data-testid="set-version-history-panel-open"
        onClick={() => uiPanel.setIsVersionHistoryPanelOpen?.(true)}
      >
        Open Version History
      </button>
      <button type="button" data-testid="set-verbose-mode" onClick={() => flowConfig.setIsVerboseMode?.(false)}>
        Toggle Verbose Mode
      </button>
      <button
        type="button"
        data-testid="set-edge-style"
        onClick={() => flowConfig.setEdgeStyle?.(EdgeStyleTypes.Bezier)}
      >
        Change Edge Style
      </button>
      <button
        type="button"
        data-testid="set-last-interacted-resource"
        onClick={() => {
          const resource: Resource = {
            id: 'test-resource',
            type: 'TEXT_INPUT',
            category: 'INPUT',
          } as Resource;
          interaction.setLastInteractedResource?.(resource);
        }}
      >
        Set Last Resource
      </button>
      <button
        type="button"
        data-testid="set-last-interacted-resource-no-panel"
        onClick={() => {
          const resource: Resource = {
            id: 'test-resource-2',
            type: 'TEXT_INPUT',
            category: 'INPUT',
          } as Resource;
          interaction.setLastInteractedResource?.(resource, false);
        }}
      >
        Set Last Resource Without Panel
      </button>
      <button
        type="button"
        data-testid="set-last-interacted-step-id"
        onClick={() => interaction.setLastInteractedStepId?.('step-123')}
      >
        Set Step ID
      </button>
      <button
        type="button"
        data-testid="on-resource-drop"
        onClick={() => {
          const resource: Resource = {
            id: 'dropped-resource',
            type: 'BUTTON',
            category: 'ACTION',
          } as Resource;
          interaction.onResourceDropOnCanvas?.(resource, 'step-456');
        }}
      >
        Drop Resource
      </button>
      <button
        type="button"
        data-testid="set-template-resource"
        onClick={() => {
          const resource: Resource = {
            id: 'template-resource',
            type: 'BASIC_LOGIN',
            category: 'ELEMENT',
            resourceType: ResourceTypes.Template,
          } as Resource;
          interaction.setLastInteractedResource?.(resource);
        }}
      >
        Set Template Resource
      </button>
      <button
        type="button"
        data-testid="set-widget-resource"
        onClick={() => {
          const resource: Resource = {
            id: 'widget-resource',
            type: 'SOCIAL_LOGIN',
            category: 'ELEMENT',
            resourceType: ResourceTypes.Widget,
          } as Resource;
          interaction.setLastInteractedResource?.(resource);
        }}
      >
        Set Widget Resource
      </button>
      <button
        type="button"
        data-testid="register-close-validation"
        onClick={() => {
          uiPanel.registerCloseValidationPanel?.(mockCloseValidationCallback);
        }}
      >
        Register Close Validation
      </button>
      <button
        type="button"
        data-testid="open-properties-panel-mutual-exclusion"
        onClick={() => {
          uiPanel.setIsOpenResourcePropertiesPanel?.(true);
        }}
      >
        Open Properties Panel Mutual Exclusion
      </button>
    </div>
  );
}

describe('FlowBuilderCoreProvider', () => {
  const defaultProps = {
    ElementFactory: MockElementFactory,
    ResourceProperties: MockResourceProperties,
    screenTypes: [PreviewScreenType.LOGIN, PreviewScreenType.COMMON],
  };

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Provider Structure', () => {
    it('should wrap children with ReactFlowProvider', () => {
      render(
        <FlowBuilderCoreProvider {...defaultProps}>
          <div data-testid="child">Child Content</div>
        </FlowBuilderCoreProvider>,
      );

      expect(screen.getByTestId('react-flow-provider')).toBeInTheDocument();
    });

    it('should include ValidationProvider', () => {
      render(
        <FlowBuilderCoreProvider {...defaultProps}>
          <div data-testid="child">Child Content</div>
        </FlowBuilderCoreProvider>,
      );

      expect(screen.getByTestId('validation-provider')).toBeInTheDocument();
    });

    it('should render children', () => {
      render(
        <FlowBuilderCoreProvider {...defaultProps}>
          <div data-testid="child">Child Content</div>
        </FlowBuilderCoreProvider>,
      );

      expect(screen.getByTestId('child')).toHaveTextContent('Child Content');
    });
  });

  describe('Initial Context State', () => {
    it('should have resource panel open by default', () => {
      render(
        <FlowBuilderCoreProvider {...defaultProps}>
          <TestConsumer />
        </FlowBuilderCoreProvider>,
      );

      expect(screen.getByTestId('is-resource-panel-open')).toHaveTextContent('true');
    });

    it('should have resource properties panel closed by default', () => {
      render(
        <FlowBuilderCoreProvider {...defaultProps}>
          <TestConsumer />
        </FlowBuilderCoreProvider>,
      );

      expect(screen.getByTestId('is-resource-properties-panel-open')).toHaveTextContent('false');
    });

    it('should have version history panel closed by default', () => {
      render(
        <FlowBuilderCoreProvider {...defaultProps}>
          <TestConsumer />
        </FlowBuilderCoreProvider>,
      );

      expect(screen.getByTestId('is-version-history-panel-open')).toHaveTextContent('false');
    });

    it('should have verbose mode enabled by default', () => {
      render(
        <FlowBuilderCoreProvider {...defaultProps}>
          <TestConsumer />
        </FlowBuilderCoreProvider>,
      );

      expect(screen.getByTestId('is-verbose-mode')).toHaveTextContent('true');
    });

    it('should use SmoothStep edge style by default', () => {
      render(
        <FlowBuilderCoreProvider {...defaultProps}>
          <TestConsumer />
        </FlowBuilderCoreProvider>,
      );

      expect(screen.getByTestId('edge-style')).toHaveTextContent(EdgeStyleTypes.SmoothStep);
    });

    it('should use first screen type as primary i18n screen', () => {
      render(
        <FlowBuilderCoreProvider {...defaultProps}>
          <TestConsumer />
        </FlowBuilderCoreProvider>,
      );

      expect(screen.getByTestId('primary-i18n-screen')).toHaveTextContent(PreviewScreenType.LOGIN);
    });
  });

  describe('State Updates', () => {
    it('should update resource panel state', () => {
      render(
        <FlowBuilderCoreProvider {...defaultProps}>
          <TestConsumer />
        </FlowBuilderCoreProvider>,
      );

      act(() => {
        screen.getByTestId('set-resource-panel-open').click();
      });

      expect(screen.getByTestId('is-resource-panel-open')).toHaveTextContent('false');
    });

    it('should update resource properties panel state', () => {
      render(
        <FlowBuilderCoreProvider {...defaultProps}>
          <TestConsumer />
        </FlowBuilderCoreProvider>,
      );

      act(() => {
        screen.getByTestId('set-resource-properties-panel-open').click();
      });

      expect(screen.getByTestId('is-resource-properties-panel-open')).toHaveTextContent('true');
    });

    it('should update version history panel state', () => {
      render(
        <FlowBuilderCoreProvider {...defaultProps}>
          <TestConsumer />
        </FlowBuilderCoreProvider>,
      );

      act(() => {
        screen.getByTestId('set-version-history-panel-open').click();
      });

      expect(screen.getByTestId('is-version-history-panel-open')).toHaveTextContent('true');
    });

    it('should update verbose mode', () => {
      render(
        <FlowBuilderCoreProvider {...defaultProps}>
          <TestConsumer />
        </FlowBuilderCoreProvider>,
      );

      act(() => {
        screen.getByTestId('set-verbose-mode').click();
      });

      expect(screen.getByTestId('is-verbose-mode')).toHaveTextContent('false');
    });

    it('should update edge style', () => {
      render(
        <FlowBuilderCoreProvider {...defaultProps}>
          <TestConsumer />
        </FlowBuilderCoreProvider>,
      );

      act(() => {
        screen.getByTestId('set-edge-style').click();
      });

      expect(screen.getByTestId('edge-style')).toHaveTextContent(EdgeStyleTypes.Bezier);
    });
  });

  describe('Resource Interaction', () => {
    it('should set last interacted resource and open properties panel', () => {
      render(
        <FlowBuilderCoreProvider {...defaultProps}>
          <TestConsumer />
        </FlowBuilderCoreProvider>,
      );

      act(() => {
        screen.getByTestId('set-last-interacted-resource').click();
      });

      expect(screen.getByTestId('is-resource-properties-panel-open')).toHaveTextContent('true');
    });

    it('should set last interacted resource without opening properties panel when openPanel is false', () => {
      render(
        <FlowBuilderCoreProvider {...defaultProps}>
          <TestConsumer />
        </FlowBuilderCoreProvider>,
      );

      act(() => {
        screen.getByTestId('set-last-interacted-resource-no-panel').click();
      });

      expect(screen.getByTestId('is-resource-properties-panel-open')).toHaveTextContent('false');
    });

    it('should handle onResourceDropOnCanvas without opening properties panel', () => {
      render(
        <FlowBuilderCoreProvider {...defaultProps}>
          <TestConsumer />
        </FlowBuilderCoreProvider>,
      );

      act(() => {
        screen.getByTestId('on-resource-drop').click();
      });

      // When dropping from resource panel, properties panel should not open
      expect(screen.getByTestId('is-resource-properties-panel-open')).toHaveTextContent('false');
    });
  });

  describe('Default Screen Types', () => {
    it('should use COMMON screen type when no screen types provided', () => {
      render(
        <FlowBuilderCoreProvider
          ElementFactory={MockElementFactory}
          ResourceProperties={MockResourceProperties}
          screenTypes={[]}
        >
          <TestConsumer />
        </FlowBuilderCoreProvider>,
      );

      expect(screen.getByTestId('primary-i18n-screen')).toHaveTextContent(PreviewScreenType.COMMON);
    });
  });

  describe('Validation Config', () => {
    it('should pass validation config to ValidationProvider', () => {
      render(
        <FlowBuilderCoreProvider
          {...defaultProps}
          validationConfig={{isOTPValidationEnabled: true, isRecoveryFactorValidationEnabled: false}}
        >
          <TestConsumer />
        </FlowBuilderCoreProvider>,
      );

      expect(screen.getByTestId('validation-provider')).toBeInTheDocument();
    });
  });

  describe('Template and Widget Resource Handling', () => {
    it('should not open properties panel for Template resources', () => {
      render(
        <FlowBuilderCoreProvider {...defaultProps}>
          <TestConsumer />
        </FlowBuilderCoreProvider>,
      );

      act(() => {
        screen.getByTestId('set-template-resource').click();
      });

      expect(screen.getByTestId('is-resource-properties-panel-open')).toHaveTextContent('false');
    });

    it('should not open properties panel for Widget resources', () => {
      render(
        <FlowBuilderCoreProvider {...defaultProps}>
          <TestConsumer />
        </FlowBuilderCoreProvider>,
      );

      act(() => {
        screen.getByTestId('set-widget-resource').click();
      });

      expect(screen.getByTestId('is-resource-properties-panel-open')).toHaveTextContent('false');
    });
  });

  describe('Validation Panel Mutual Exclusion', () => {
    it('should call registered close validation callback when opening properties panel', () => {
      render(
        <FlowBuilderCoreProvider {...defaultProps}>
          <TestConsumer />
        </FlowBuilderCoreProvider>,
      );

      // Register a close validation callback
      act(() => {
        screen.getByTestId('register-close-validation').click();
      });

      // Opening the properties panel should trigger mutual exclusion
      act(() => {
        screen.getByTestId('open-properties-panel-mutual-exclusion').click();
      });

      expect(screen.getByTestId('is-resource-properties-panel-open')).toHaveTextContent('true');
      // The registered close callback should have been invoked for mutual exclusion
      expect(mockCloseValidationCallback).toHaveBeenCalledOnce();
    });
  });
});
