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

import {render, screen, fireEvent, waitFor, act} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import FlowCreatePage from '../FlowCreatePage';

// Mock @thunderid/logger/react
vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({
    debug: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    error: vi.fn(),
  }),
}));

// Mock react-i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (_key: string, defaultValue: string) => defaultValue,
  }),
}));

// Mock useNavigate
const mockNavigate = vi.fn().mockResolvedValue(undefined);

vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

// Mock useCreateFlow
const mockMutate = vi.fn();
const mockCreateFlow = {
  mutate: mockMutate,
  isPending: false,
};

vi.mock('../../api/useCreateFlow', () => ({
  default: () => mockCreateFlow,
}));

// Track callbacks from child components
let capturedTypeProps: Record<string, unknown> = {};
let capturedTemplateProps: Record<string, unknown> = {};
let capturedConfigureProps: Record<string, unknown> = {};

// Mock child components
vi.mock('../../components/create-flow/SelectFlowType', () => ({
  default: (props: Record<string, unknown>) => {
    capturedTypeProps = props;
    return <div data-testid="select-flow-type">SelectFlowType</div>;
  },
}));

vi.mock('../../components/create-flow/SelectFlowTemplate', () => ({
  default: (props: Record<string, unknown>) => {
    capturedTemplateProps = props;
    return <div data-testid="select-flow-template">SelectFlowTemplate</div>;
  },
}));

vi.mock('../../components/create-flow/ConfigureFlowName', () => ({
  default: (props: Record<string, unknown>) => {
    capturedConfigureProps = props;
    return <div data-testid="configure-flow-name">ConfigureFlowName</div>;
  },
}));

describe('FlowCreatePage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockCreateFlow.isPending = false;
    capturedTypeProps = {};
    capturedTemplateProps = {};
    capturedConfigureProps = {};
  });

  describe('Initial Rendering', () => {
    it('should render the SelectFlowType step initially', () => {
      render(<FlowCreatePage />);

      expect(screen.getByTestId('select-flow-type')).toBeInTheDocument();
    });

    it('should render the breadcrumb with Flow Type label', () => {
      render(<FlowCreatePage />);

      expect(screen.getByText('Flow Type')).toBeInTheDocument();
    });

    it('should render the Continue button', () => {
      render(<FlowCreatePage />);

      expect(screen.getByText('Continue')).toBeInTheDocument();
    });

    it('should not render the Back button on the first step', () => {
      render(<FlowCreatePage />);

      expect(screen.queryByText('Back')).not.toBeInTheDocument();
    });

    it('should render the close button', () => {
      render(<FlowCreatePage />);

      // The close button is an IconButton with X icon
      const buttons = screen.getAllByRole('button');
      // First button should be the close button
      expect(buttons.length).toBeGreaterThan(0);
    });

    it('should have Continue disabled when no type is selected', () => {
      render(<FlowCreatePage />);

      expect(screen.getByText('Continue')).toBeDisabled();
    });
  });

  describe('Close Navigation', () => {
    it('should navigate to /flows when close button is clicked', () => {
      render(<FlowCreatePage />);

      // The close button is the first IconButton in the header
      const buttons = screen.getAllByRole('button');
      // Find the close button (it wraps an X icon, but we mocked the component)
      // Click the first button which should be the close icon button
      fireEvent.click(buttons[0]);

      expect(mockNavigate).toHaveBeenCalledWith('/flows');
    });
  });

  // Helper to navigate to the Template step
  const navigateToTemplateStep = (): void => {
    act(() => {
      (capturedTypeProps.onTypeChange as (type: string) => void)('AUTHENTICATION');
      (capturedTypeProps.onReadyChange as (ready: boolean) => void)(true);
    });
    fireEvent.click(screen.getByText('Continue'));
  };

  // Helper to navigate to the Configure step
  const navigateToConfigureStep = (): void => {
    navigateToTemplateStep();
    act(() => {
      (capturedTemplateProps.onTemplateChange as (t: unknown) => void)({
        type: 'BASIC_AUTH',
        flowType: 'AUTHENTICATION',
        config: {nodes: [{id: 'start'}]},
      });
    });
    fireEvent.click(screen.getByText('Continue'));
  };

  describe('Step Navigation', () => {
    it('should advance to Template step when Continue is clicked after selecting a type', () => {
      render(<FlowCreatePage />);

      navigateToTemplateStep();

      expect(screen.getByTestId('select-flow-template')).toBeInTheDocument();
    });

    it('should show breadcrumbs for visited steps', () => {
      render(<FlowCreatePage />);

      navigateToTemplateStep();

      expect(screen.getByText('Flow Type')).toBeInTheDocument();
      expect(screen.getByText('Template')).toBeInTheDocument();
    });

    it('navigates back to Type step when Flow Type breadcrumb is clicked on Template step', () => {
      render(<FlowCreatePage />);

      navigateToTemplateStep();

      // "Flow Type" is the first (non-last) breadcrumb — clicking it calls setCurrentStep
      fireEvent.click(screen.getByRole('button', {name: 'Flow Type'}));

      expect(screen.getByTestId('select-flow-type')).toBeInTheDocument();
    });

    it('should show Back button on the Template step', () => {
      render(<FlowCreatePage />);

      navigateToTemplateStep();

      expect(screen.getByText('Back')).toBeInTheDocument();
    });

    it('should go back to Type step when Back is clicked on Template step', () => {
      render(<FlowCreatePage />);

      navigateToTemplateStep();
      fireEvent.click(screen.getByText('Back'));

      expect(screen.getByTestId('select-flow-type')).toBeInTheDocument();
    });

    it('should advance to Configure step from Template step', () => {
      render(<FlowCreatePage />);

      navigateToConfigureStep();

      expect(screen.getByTestId('configure-flow-name')).toBeInTheDocument();
    });

    it('should show Create button on the Configure step', () => {
      render(<FlowCreatePage />);

      navigateToConfigureStep();

      expect(screen.getByText('Create')).toBeInTheDocument();
    });
  });

  describe('Flow Creation', () => {
    it('should call createFlow.mutate when Create is clicked', () => {
      render(<FlowCreatePage />);

      navigateToConfigureStep();

      act(() => {
        (capturedConfigureProps.onChange as (v: {name: string; handle: string}) => void)({
          name: 'My Flow',
          handle: 'my-flow',
        });
        (capturedConfigureProps.onReadyChange as (ready: boolean) => void)(true);
      });

      fireEvent.click(screen.getByText('Create'));

      expect(mockMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'My Flow',
          handle: 'my-flow',
          flowType: 'AUTHENTICATION',
        }),
        expect.any(Object),
      );
    });

    it('should navigate to flow builder on successful creation', async () => {
      mockMutate.mockImplementation((_req: unknown, options: {onSuccess: (flow: {id: string}) => void}) => {
        options.onSuccess({id: 'flow-123'});
      });

      render(<FlowCreatePage />);

      navigateToConfigureStep();

      act(() => {
        (capturedConfigureProps.onChange as (v: {name: string; handle: string}) => void)({
          name: 'Test',
          handle: 'test',
        });
        (capturedConfigureProps.onReadyChange as (ready: boolean) => void)(true);
      });

      fireEvent.click(screen.getByText('Create'));

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/flows/signin/flow-123');
      });
    });

    it('should display error alert when creation fails', async () => {
      mockMutate.mockImplementation((_req: unknown, options: {onError: (err: {message: string}) => void}) => {
        options.onError({message: 'Creation failed'});
      });

      render(<FlowCreatePage />);

      navigateToConfigureStep();

      act(() => {
        (capturedConfigureProps.onChange as (v: {name: string; handle: string}) => void)({
          name: 'Test',
          handle: 'test',
        });
        (capturedConfigureProps.onReadyChange as (ready: boolean) => void)(true);
      });

      fireEvent.click(screen.getByText('Create'));

      await waitFor(() => {
        expect(screen.getByText('Creation failed')).toBeInTheDocument();
      });
    });
  });
});
