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
import type {Node} from '@xyflow/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {FlowSimulation} from '../../../hooks/useFlowSimulation';
import {SimulationOptionKinds} from '../../../utils/getSimulationOptions';
import SimulationStepPreview from '../SimulationStepPreview';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'flows:core.simulation.preview.title': 'End-user preview',
        'flows:core.simulation.preview.noScreen': 'No screen is shown for this step',
        'flows:core.simulation.preview.noScreenHint': 'This step runs in the background',
        'flows:core.simulation.preview.devices.mobile': 'Mobile',
        'flows:core.simulation.preview.devices.tablet': 'Tablet',
        'flows:core.simulation.preview.devices.desktop': 'Desktop',
        'flows:core.simulation.preview.darkMode': 'Switch to dark preview',
        'flows:core.simulation.preview.lightMode': 'Switch to light preview',
        'flows:core.simulation.back': 'Go back one step',
        'flows:core.simulation.staticView': 'Switch to a static canvas view',
        'flows:core.simulation.followSteps': 'Follow steps on the canvas',
        'flows:core.simulation.restart': 'Restart preview',
        'flows:core.simulation.chooseNext': 'Choose how the user proceeds from this step',
        'flows:core.simulation.screenHint': 'Select an option on the preview screen to continue',
        'flows:core.simulation.screenHintOr': 'or select an option on the preview screen',
        'flows:core.simulation.complete': 'Flow complete — no outgoing transitions',
        'flows:core.simulation.kinds.action': 'Continue',
        'flows:core.simulation.kinds.success': 'On success',
        'flows:core.simulation.preview.dynamicFieldsHint': 'Input fields resolved at runtime',
      };
      return translations[key] || key;
    },
  }),
}));

let mockColorSchemeMode: 'light' | 'dark' | 'system' = 'light';
let mockDesignResolve: {data?: {theme: object; layout: object}; isError: boolean} = {
  data: {theme: {palette: {}}, layout: {}},
  isError: false,
};
let mockApplications: {id: string; name: string}[] = [{id: 'app-1', name: 'My App'}];

vi.mock('@wso2/oxygen-ui', async () => {
  const actual = await vi.importActual<typeof import('@wso2/oxygen-ui')>('@wso2/oxygen-ui');
  return {
    ...actual,
    useColorScheme: () => ({mode: mockColorSchemeMode}),
  };
});

vi.mock('@thunderid/hooks', () => ({
  useTemplateLiteralResolver: () => ({
    resolve: (value: string) =>
      value === '{{ t(signin:forms.otp.description) }}' ? 'Enter the verification code' : value,
    resolveAll: (value: string) =>
      value.replaceAll('{{ t(signin:forms.otp.description) }}', 'Enter the verification code'),
  }),
}));

vi.mock('@thunderid/design', () => ({
  DefaultTheme: {palette: {primary: {main: '#4285f4'}}},
  DesignResolveType: {APP: 'APP', OU: 'OU'},
  useGetDesignResolve: ({id}: {id: string}) => (id ? mockDesignResolve : {data: undefined, isError: false}),
}));

vi.mock('@/features/applications/api/useGetApplications', () => ({
  default: () => ({
    data: {applications: mockApplications},
  }),
}));

vi.mock('@/features/applications/api/useGetApplication', () => ({
  default: (applicationId: string) => ({
    data: applicationId
      ? {id: applicationId, name: 'My App', themeId: 'theme-1', logoUrl: 'https://myapp.example/logo.png'}
      : undefined,
  }),
}));

vi.mock('@/components/GatePreview/GatePreview', () => ({
  default: ({
    mock,
    onSubmit,
    onComponentHover,
    toolbarStart,
    toolbarEnd,
    colorScheme,
    theme,
    additionalData,
  }: {
    mock?: {id?: string}[];
    onSubmit?: (component: {id?: string}) => void;
    onComponentHover?: (component: {id?: string} | null) => void;
    toolbarStart?: React.ReactNode;
    toolbarEnd?: React.ReactNode;
    colorScheme?: string;
    theme?: object | null;
    additionalData?: Record<string, unknown>;
  }) => (
    <div
      data-testid="gate-preview"
      data-mock={JSON.stringify(mock)}
      data-color-scheme={colorScheme}
      data-theme={JSON.stringify(theme)}
      data-additional-data={JSON.stringify(additionalData)}
    >
      {toolbarStart}
      {toolbarEnd}
      <button type="button" onClick={() => onSubmit?.({id: 'action_001'})}>
        gate-submit
      </button>
      <button
        type="button"
        onMouseEnter={() => onComponentHover?.(mock?.[1] ?? null)}
        onMouseLeave={() => onComponentHover?.(null)}
      >
        gate-hover
      </button>
    </div>
  ),
}));

const createSimulation = (overrides: Partial<FlowSimulation> = {}): FlowSimulation => ({
  isSimulating: true,
  pathNodeIds: ['view-1'],
  pathEdges: [],
  currentNodeId: 'view-1',
  options: [
    {
      edgeId: 'e1',
      targetNodeId: 'executor-1',
      kind: SimulationOptionKinds.Action,
      actionLabel: 'Sign In',
      sourceComponentId: 'action_001',
    },
  ],
  previewedOption: null,
  followCamera: true,
  toggleFollowCamera: vi.fn(),
  start: vi.fn(),
  choose: vi.fn(),
  back: vi.fn(),
  preview: vi.fn(),
  stop: vi.fn(),
  ...overrides,
});

const viewNode = {
  id: 'view-1',
  type: 'VIEW',
  position: {x: 0, y: 0},
  data: {
    components: [
      {id: 'text_001', type: 'TEXT', label: 'Sign In', variant: 'HEADING_1'},
      {
        id: 'block_001',
        type: 'BLOCK',
        components: [
          {id: 'input_001', type: 'TEXT_INPUT', label: 'Username', placeholder: 'Enter your username'},
          {id: 'action_001', type: 'ACTION', label: 'Sign In', variant: 'PRIMARY'},
          {id: 'action_unwired', type: 'ACTION', label: 'Forgot Password', variant: 'TEXT'},
        ],
      },
    ],
  },
} as unknown as Node;

const executorNode = {
  id: 'executor-1',
  type: 'TASK_EXECUTION',
  position: {x: 0, y: 0},
  data: {},
} as unknown as Node;

const dynamicInputNode = {
  id: 'view-1',
  type: 'VIEW',
  position: {x: 0, y: 0},
  data: {
    components: [
      {
        id: 'block_001',
        type: 'BLOCK',
        components: [
          {
            id: 'dynamic_001',
            type: 'DYNAMIC_INPUT_PLACEHOLDER',
            placeholder: 'Dynamic Input',
            hint: 'Resolves input fields passed from runtime.',
          },
          {id: 'action_001', type: 'ACTION', label: 'Continue', variant: 'PRIMARY'},
        ],
      },
    ],
  },
} as unknown as Node;

const consentNode = {
  id: 'view-1',
  type: 'VIEW',
  position: {x: 0, y: 0},
  data: {
    components: [
      {
        id: 'consent_block',
        type: 'BLOCK',
        components: [
          {id: 'consent_input', type: 'CONSENT_INPUT', required: true},
          {id: 'consent_action_allow', type: 'ACTION', label: 'Allow', variant: 'PRIMARY'},
        ],
      },
    ],
  },
} as unknown as Node;

describe('SimulationStepPreview', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockColorSchemeMode = 'light';
    mockDesignResolve = {data: {theme: {palette: {}}, layout: {}}, isError: false};
    mockApplications = [{id: 'app-1', name: 'My App'}];
  });

  it('should render nothing when not simulating', () => {
    render(<SimulationStepPreview node={viewNode} simulation={createSimulation({isSimulating: false})} />);

    expect(screen.queryByTestId('simulation-step-preview')).not.toBeInTheDocument();
  });

  it('should render the view components as an end-user screen', () => {
    render(<SimulationStepPreview node={viewNode} simulation={createSimulation()} />);

    expect(screen.getByText('End-user preview')).toBeInTheDocument();
    // The sketch renders the exact variant the end user sees (HEADING_1 → h1),
    // scaled to fit the panel via zoom rather than substituted.
    expect(screen.getByText('Sign In', {selector: 'h1'})).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Enter your username')).toBeInTheDocument();
  });

  it('should advance the simulation when a wired action button is clicked', () => {
    const simulation = createSimulation();
    render(<SimulationStepPreview node={viewNode} simulation={simulation} />);

    fireEvent.click(screen.getByRole('button', {name: 'Sign In'}));

    expect(simulation.choose).toHaveBeenCalledWith(simulation.options[0]);
  });

  it('should disable action buttons without an outgoing transition', () => {
    render(<SimulationStepPreview node={viewNode} simulation={createSimulation()} />);

    expect(screen.getByRole('button', {name: 'Forgot Password'})).toBeDisabled();
  });

  it('should preview the transition edge when hovering a wired button', () => {
    const simulation = createSimulation();
    render(<SimulationStepPreview node={viewNode} simulation={simulation} />);

    fireEvent.mouseEnter(screen.getByRole('button', {name: 'Sign In'}));

    expect(simulation.preview).toHaveBeenCalledWith(simulation.options[0]);
  });

  it('should preview the transition edge when a wired button gains keyboard focus', () => {
    const simulation = createSimulation();
    render(<SimulationStepPreview node={viewNode} simulation={simulation} />);

    fireEvent.focus(screen.getByRole('button', {name: 'Sign In'}));
    expect(simulation.preview).toHaveBeenCalledWith(simulation.options[0]);

    fireEvent.blur(screen.getByRole('button', {name: 'Sign In'}));
    expect(simulation.preview).toHaveBeenLastCalledWith(null);
  });

  it('should show a background-step placeholder for nodes without a screen', () => {
    render(<SimulationStepPreview node={executorNode} simulation={createSimulation({currentNodeId: 'executor-1'})} />);

    expect(screen.getByText('No screen is shown for this step')).toBeInTheDocument();
  });

  it('should sketch a consent attributes placeholder for consent inputs', () => {
    render(<SimulationStepPreview node={consentNode} simulation={createSimulation()} />);

    expect(screen.getByTestId('consent-placeholder')).toBeInTheDocument();
  });

  it('should sketch a stand-in field for dynamic input placeholders', () => {
    render(<SimulationStepPreview node={dynamicInputNode} simulation={createSimulation()} />);

    expect(screen.getByTestId('dynamic-fields-placeholder')).toBeInTheDocument();
  });

  it('should sketch template-literal labels with placeholder badges', () => {
    const nodeWithTemplate = {
      ...viewNode,
      data: {
        components: [
          {id: 'text_001', type: 'TEXT', label: '{{ meta(application.name) }} wants access', variant: 'HEADING_1'},
        ],
      },
    } as unknown as Node;

    render(<SimulationStepPreview node={nodeWithTemplate} simulation={createSimulation()} />);

    expect(screen.getByText('application.name')).toBeInTheDocument();
    expect(screen.getByText('wants access')).toBeInTheDocument();
  });

  it('should not request unresolved template literals as action button images', () => {
    const nodeWithTemplatedImage = {
      ...viewNode,
      data: {
        components: [
          {
            id: 'action_001',
            type: 'ACTION',
            label: 'Sign In',
            variant: 'PRIMARY',
            image: '{{ meta(application.logoUrl) }}',
          },
        ],
      },
    } as unknown as Node;

    render(<SimulationStepPreview node={nodeWithTemplatedImage} simulation={createSimulation()} />);

    expect(screen.getByRole('button', {name: 'Sign In'}).querySelector('img')).toBeNull();
  });

  it('should toggle camera following from the panel controls', () => {
    const simulation = createSimulation();
    render(<SimulationStepPreview node={viewNode} simulation={simulation} />);

    const toggle = screen.getByRole('button', {name: 'Switch to a static canvas view'});

    fireEvent.click(toggle);

    expect(simulation.toggleFollowCamera).toHaveBeenCalled();
  });

  it('should label the camera toggle as follow when in static view', () => {
    render(<SimulationStepPreview node={viewNode} simulation={createSimulation({followCamera: false})} />);

    expect(screen.getByRole('button', {name: 'Follow steps on the canvas'})).toBeInTheDocument();
  });

  it('should preserve the application, device, and color scheme selections across steps', () => {
    const {rerender} = render(<SimulationStepPreview node={viewNode} simulation={createSimulation()} />);

    // Select an application, a device, and toggle the preview color scheme.
    fireEvent.mouseDown(screen.getByRole('combobox'));
    fireEvent.click(screen.getByRole('option', {name: 'My App'}));
    fireEvent.click(screen.getByRole('button', {name: 'Tablet'}));
    fireEvent.click(screen.getByRole('button', {name: 'Switch to dark preview'}));

    // Advance to a background step and come back to a view step.
    rerender(
      <SimulationStepPreview node={executorNode} simulation={createSimulation({currentNodeId: 'executor-1'})} />,
    );
    rerender(<SimulationStepPreview node={viewNode} simulation={createSimulation()} />);

    const panel = screen.getByTestId('simulation-step-preview');
    expect(panel).toHaveAttribute('data-device', 'tablet');
    expect(screen.getByTestId('gate-preview')).toHaveAttribute('data-color-scheme', 'dark');
  });

  describe('Application-themed preview', () => {
    const selectApplication = (): void => {
      fireEvent.mouseDown(screen.getByRole('combobox'));
      fireEvent.click(screen.getByRole('option', {name: 'My App'}));
    };

    it('should list applications in the preview selector', () => {
      render(<SimulationStepPreview node={viewNode} simulation={createSimulation()} />);

      fireEvent.mouseDown(screen.getByRole('combobox'));

      expect(screen.getByRole('option', {name: 'My App'})).toBeInTheDocument();
    });

    it('should render the themed gate preview when an application with a theme is selected', () => {
      render(<SimulationStepPreview node={viewNode} simulation={createSimulation()} />);

      selectApplication();

      expect(screen.getByTestId('gate-preview')).toBeInTheDocument();
    });

    it('should default to the Console application when one exists', () => {
      mockApplications = [
        {id: 'console-app', name: 'Console'},
        {id: 'app-1', name: 'My App'},
      ];

      render(<SimulationStepPreview node={viewNode} simulation={createSimulation()} />);

      expect(screen.getByTestId('gate-preview')).toBeInTheDocument();
      expect(screen.getByRole('combobox')).toHaveTextContent('Console');
    });

    it('should derive runtime event types for actions handed to the gate preview', () => {
      const nodeWithBareActions = {
        ...viewNode,
        data: {
          components: [
            {
              id: 'block_001',
              type: 'BLOCK',
              components: [{id: 'action_001', type: 'ACTION', category: 'ACTION', label: 'Create User'}],
            },
          ],
        },
      } as unknown as Node;

      render(<SimulationStepPreview node={nodeWithBareActions} simulation={createSimulation()} />);

      selectApplication();

      // The gate's block renderer drops actions without an eventType.
      expect(screen.getByTestId('gate-preview').getAttribute('data-mock')).toContain('"eventType":"TRIGGER"');
    });

    it('should stand in a fields skeleton for dynamic input placeholders in the gate preview', () => {
      render(<SimulationStepPreview node={dynamicInputNode} simulation={createSimulation()} />);

      selectApplication();

      const mock = screen.getByTestId('gate-preview').getAttribute('data-mock');
      // The skeleton rides on the gate's RICH_TEXT adapter.
      expect(mock).toContain('"type":"RICH_TEXT"');
      expect(mock).toContain('Input fields resolved at runtime');
      expect(mock).not.toContain('DYNAMIC_INPUT_PLACEHOLDER');
      // Builder chrome texts must not leak into the themed preview.
      expect(mock).not.toContain('Dynamic Input');
      expect(mock).not.toContain('Resolves input fields');
    });

    it('should hand the gate preview placeholder consent data for consent steps', () => {
      render(<SimulationStepPreview node={consentNode} simulation={createSimulation()} />);

      selectApplication();

      expect(screen.getByTestId('gate-preview').getAttribute('data-additional-data')).toContain('consentPrompt');
    });

    it('should not hand the gate preview consent data for steps without consent inputs', () => {
      render(<SimulationStepPreview node={viewNode} simulation={createSimulation()} />);

      selectApplication();

      expect(screen.getByTestId('gate-preview')).not.toHaveAttribute('data-additional-data');
    });

    it('should resolve application meta placeholders in the previewed components', () => {
      const nodeWithLogo = {
        ...viewNode,
        data: {
          components: [{id: 'image_001', type: 'IMAGE', src: '{{ meta(application.logoUrl) }}'}],
        },
      } as unknown as Node;

      render(<SimulationStepPreview node={nodeWithLogo} simulation={createSimulation()} />);

      selectApplication();

      expect(screen.getByTestId('gate-preview').getAttribute('data-mock')).toContain('https://myapp.example/logo.png');
    });

    it('should advance the simulation when an action is submitted inside the gate preview', () => {
      const simulation = createSimulation();
      render(<SimulationStepPreview node={viewNode} simulation={simulation} />);

      selectApplication();
      fireEvent.click(screen.getByText('gate-submit'));

      expect(simulation.choose).toHaveBeenCalledWith(simulation.options[0]);
    });

    it('should render the gate default design when the application has no design configured', () => {
      // The design resolve endpoint responds 404 for applications without a design.
      // The gate falls back to an empty theme merged over the renderer defaults —
      // the preview must do the same instead of loading forever.
      mockDesignResolve = {data: undefined, isError: true};
      render(<SimulationStepPreview node={viewNode} simulation={createSimulation()} />);

      selectApplication();

      const gatePreview = screen.getByTestId('gate-preview');
      expect(gatePreview).toBeInTheDocument();
      expect(gatePreview).toHaveAttribute('data-theme', '{}');
    });

    it('should preserve rich text links when resolving mixed-content labels for the gate preview', () => {
      const nodeWithRichText = {
        ...viewNode,
        data: {
          components: [
            {
              id: 'rich_001',
              type: 'RICH_TEXT',
              label: '<p><span>{{ t(signin:forms.otp.description) }} </span><a href="#">Reset</a></p>',
            },
          ],
        },
      } as unknown as Node;

      render(<SimulationStepPreview node={nodeWithRichText} simulation={createSimulation()} />);

      selectApplication();

      const mock = screen.getByTestId('gate-preview').getAttribute('data-mock');
      expect(mock).toContain('Enter the verification code');
      expect(mock).toContain('<a href=');
    });

    it('should preview the wired edge when hovering a component inside the gate preview', () => {
      const simulation = createSimulation();
      render(<SimulationStepPreview node={viewNode} simulation={simulation} />);

      selectApplication();

      // The hover trigger reports the block containing the wired Sign In action.
      fireEvent.mouseEnter(screen.getByText('gate-hover'));
      expect(simulation.preview).toHaveBeenCalledWith(simulation.options[0]);

      fireEvent.mouseLeave(screen.getByText('gate-hover'));
      expect(simulation.preview).toHaveBeenCalledWith(null);
    });

    it('should show the background-step placeholder instead of the themed preview for steps without a screen', () => {
      render(
        <SimulationStepPreview node={executorNode} simulation={createSimulation({currentNodeId: 'executor-1'})} />,
      );

      selectApplication();

      expect(screen.queryByTestId('gate-preview')).not.toBeInTheDocument();
      expect(screen.getByText('No screen is shown for this step')).toBeInTheDocument();
    });

    it('should default the themed preview to the console color scheme', () => {
      mockColorSchemeMode = 'dark';
      render(<SimulationStepPreview node={viewNode} simulation={createSimulation()} />);

      selectApplication();

      expect(screen.getByTestId('gate-preview')).toHaveAttribute('data-color-scheme', 'dark');
    });

    it('should toggle the themed preview between light and dark', () => {
      render(<SimulationStepPreview node={viewNode} simulation={createSimulation()} />);

      selectApplication();
      expect(screen.getByTestId('gate-preview')).toHaveAttribute('data-color-scheme', 'light');

      fireEvent.click(screen.getByRole('button', {name: 'Switch to dark preview'}));
      expect(screen.getByTestId('gate-preview')).toHaveAttribute('data-color-scheme', 'dark');

      fireEvent.click(screen.getByRole('button', {name: 'Switch to light preview'}));
      expect(screen.getByTestId('gate-preview')).toHaveAttribute('data-color-scheme', 'light');
    });

    it('should not show the scheme toggle for the default preview', () => {
      render(<SimulationStepPreview node={viewNode} simulation={createSimulation()} />);

      expect(screen.queryByRole('button', {name: 'Switch to dark preview'})).not.toBeInTheDocument();
    });

    it('should resolve i18n templates before passing components to the gate preview', () => {
      const nodeWithI18n = {
        ...viewNode,
        data: {
          components: [{id: 'text_001', type: 'TEXT', label: '{{ t(signin:forms.otp.description) }}'}],
        },
      } as unknown as Node;

      render(<SimulationStepPreview node={nodeWithI18n} simulation={createSimulation()} />);

      selectApplication();

      expect(screen.getByTestId('gate-preview').getAttribute('data-mock')).toContain('Enter the verification code');
    });
  });

  describe('Simulation controls', () => {
    const successOption = {
      edgeId: 'e2',
      targetNodeId: 'done-1',
      kind: SimulationOptionKinds.Success,
      actionLabel: undefined,
      sourceComponentId: undefined,
    };

    it('should go back one step from the header control', () => {
      const simulation = createSimulation({pathNodeIds: ['view-0', 'view-1']});
      render(<SimulationStepPreview node={viewNode} simulation={simulation} />);

      fireEvent.click(screen.getByRole('button', {name: 'Go back one step'}));

      expect(simulation.back).toHaveBeenCalled();
    });

    it('should disable the back control on the first step', () => {
      render(<SimulationStepPreview node={viewNode} simulation={createSimulation()} />);

      expect(screen.getByRole('button', {name: 'Go back one step'})).toBeDisabled();
    });

    it('should restart the simulation from the header control', () => {
      const simulation = createSimulation();
      render(<SimulationStepPreview node={viewNode} simulation={simulation} />);

      fireEvent.click(screen.getByRole('button', {name: 'Restart preview'}));

      expect(simulation.start).toHaveBeenCalled();
    });

    it('should hint to use the screen when every option is triggerable from it', () => {
      render(<SimulationStepPreview node={viewNode} simulation={createSimulation()} />);

      const footer = screen.getByTestId('simulation-preview-footer');
      expect(footer).toHaveTextContent('Select an option on the preview screen to continue');
      expect(screen.queryByText('Choose how the user proceeds from this step')).not.toBeInTheDocument();
    });

    it('should list only options that cannot be triggered from the screen', () => {
      const simulation = createSimulation({options: [...createSimulation().options, successOption]});
      render(<SimulationStepPreview node={viewNode} simulation={simulation} />);

      const footer = screen.getByTestId('simulation-preview-footer');
      expect(footer).toHaveTextContent('On success');
      expect(screen.queryByRole('button', {name: /Sign In Continue/})).not.toBeInTheDocument();
    });

    it('should append the screen hint after footer options when both exist', () => {
      const simulation = createSimulation({options: [...createSimulation().options, successOption]});
      render(<SimulationStepPreview node={viewNode} simulation={simulation} />);

      expect(screen.getByTestId('simulation-screen-hint')).toHaveTextContent(
        'or select an option on the preview screen',
      );
    });

    it('should not show the screen hint for steps without a screen', () => {
      const simulation = createSimulation({currentNodeId: 'executor-1', options: [successOption]});
      render(<SimulationStepPreview node={executorNode} simulation={simulation} />);

      expect(screen.queryByTestId('simulation-screen-hint')).not.toBeInTheDocument();
    });

    it('should list every option for steps without a screen', () => {
      const simulation = createSimulation({currentNodeId: 'executor-1'});
      render(<SimulationStepPreview node={executorNode} simulation={simulation} />);

      expect(screen.getByTestId('simulation-preview-footer')).toHaveTextContent('Sign In');
    });

    it('should advance the simulation when a footer option is chosen', () => {
      const simulation = createSimulation({options: [successOption]});
      render(<SimulationStepPreview node={viewNode} simulation={simulation} />);

      fireEvent.click(screen.getByRole('button', {name: 'On success'}));

      expect(simulation.choose).toHaveBeenCalledWith(successOption);
    });

    it('should preview the transition edge when hovering a footer option', () => {
      const simulation = createSimulation({options: [successOption]});
      render(<SimulationStepPreview node={viewNode} simulation={simulation} />);

      fireEvent.mouseEnter(screen.getByRole('button', {name: 'On success'}));

      expect(simulation.preview).toHaveBeenCalledWith(successOption);
    });

    it('should preview the transition edge when a footer option gains keyboard focus', () => {
      const simulation = createSimulation({options: [successOption]});
      render(<SimulationStepPreview node={viewNode} simulation={simulation} />);

      fireEvent.focus(screen.getByRole('button', {name: 'On success'}));
      expect(simulation.preview).toHaveBeenCalledWith(successOption);

      fireEvent.blur(screen.getByRole('button', {name: 'On success'}));
      expect(simulation.preview).toHaveBeenLastCalledWith(null);
    });

    it('should show the completion state when there are no outgoing transitions', () => {
      render(<SimulationStepPreview node={viewNode} simulation={createSimulation({options: []})} />);

      expect(screen.getByText('Flow complete — no outgoing transitions')).toBeInTheDocument();
    });
  });

  describe('Panel behavior', () => {
    it('should exit the simulation via the close button', () => {
      const simulation = createSimulation();
      render(<SimulationStepPreview node={viewNode} simulation={simulation} />);

      fireEvent.click(screen.getByTestId('simulation-preview-close'));

      expect(simulation.stop).toHaveBeenCalled();
    });

    it('should switch the preview via the device presets', () => {
      render(<SimulationStepPreview node={viewNode} simulation={createSimulation()} />);

      const panel = screen.getByTestId('simulation-step-preview');
      expect(panel).toHaveAttribute('data-device', 'mobile');

      fireEvent.click(screen.getByRole('button', {name: 'Tablet'}));
      expect(panel).toHaveAttribute('data-device', 'tablet');

      fireEvent.click(screen.getByRole('button', {name: 'Desktop'}));
      expect(panel).toHaveAttribute('data-device', 'desktop');
    });
  });
});
