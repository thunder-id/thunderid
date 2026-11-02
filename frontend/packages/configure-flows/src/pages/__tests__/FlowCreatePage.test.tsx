// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, fallback?: string | {defaultValue?: string}) => {
      if (typeof fallback === 'string') return fallback || key;
      if (fallback && typeof fallback === 'object') return fallback.defaultValue ?? key;
      return key;
    },
  }),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useConfig: () => ({config: {brand: {product_name: 'ThunderID'}}}),
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
const mockReset = vi.fn();
const mockCreateFlow = {
  mutate: mockMutate,
  reset: mockReset,
  isPending: false,
  isError: false,
};

vi.mock('../../api/useCreateFlow', () => ({
  default: () => mockCreateFlow,
}));

// Mock the organization unit hook and the OU step so each test controls how many OUs there are
const ouState: {hasMultipleOUs: boolean; isLoading: boolean; ouList: {id: string}[]} = {
  hasMultipleOUs: false,
  isLoading: false,
  ouList: [{id: 'ou-root'}],
};

vi.mock('@thunderid/configure-organization-units', () => ({
  useHasMultipleOUs: () => ouState,
}));

vi.mock('../../components/create-flow/FlowOrganizationUnitStep', () => ({
  default: ({
    isLoading,
    value,
    onChange,
    onBack,
    onContinue,
  }: {
    isLoading: boolean;
    value: string;
    onChange: (ouId: string) => void;
    onBack: () => void;
    onContinue: () => void;
  }) => (
    <div data-testid="ou-step" data-loading={String(isLoading)} data-value={value}>
      <button type="button" data-testid="ou-pick-b" onClick={() => onChange('ou-b')}>
        pick b
      </button>
      <button type="button" data-testid="ou-continue" onClick={onContinue}>
        continue
      </button>
      <button type="button" data-testid="ou-back" onClick={onBack}>
        back
      </button>
    </div>
  ),
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
    mockCreateFlow.isError = false;
    capturedTypeProps = {};
    capturedTemplateProps = {};
    capturedConfigureProps = {};
    ouState.hasMultipleOUs = false;
    ouState.isLoading = false;
    ouState.ouList = [{id: 'ou-root'}];
  });

  describe('Initial Rendering', () => {
    it('should render the SelectFlowType step initially', () => {
      render(<FlowCreatePage />);

      expect(screen.getByTestId('select-flow-type')).toBeInTheDocument();
    });

    it('should render the breadcrumb with Flow Type label', () => {
      render(<FlowCreatePage />);

      expect(screen.getByText('Flow type')).toBeInTheDocument();
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
      (capturedTypeProps['onTypeChange'] as (type: string) => void)('AUTHENTICATION');
      (capturedTypeProps['onReadyChange'] as (ready: boolean) => void)(true);
    });
    fireEvent.click(screen.getByText('Continue'));
  };

  // Helper to navigate to the Configure step
  const navigateToConfigureStep = (): void => {
    navigateToTemplateStep();
    act(() => {
      (capturedTemplateProps['onTemplateChange'] as (t: unknown) => void)({
        type: 'CREDENTIALS_AUTH',
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

      expect(screen.getByText('Flow type')).toBeInTheDocument();
      expect(screen.getByText('Template')).toBeInTheDocument();
    });

    it('navigates back to Type step when Flow Type breadcrumb is clicked on Template step', () => {
      render(<FlowCreatePage />);

      navigateToTemplateStep();

      // "Flow type" is the first (non-last) breadcrumb — clicking it calls setCurrentStep
      fireEvent.click(screen.getByRole('button', {name: 'Flow type'}));

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
        (capturedConfigureProps['onChange'] as (v: {name: string; handle: string}) => void)({
          name: 'My Flow',
          handle: 'my-flow',
        });
        (capturedConfigureProps['onReadyChange'] as (ready: boolean) => void)(true);
      });

      fireEvent.click(screen.getByText('Create'));

      expect(mockMutate).toHaveBeenCalledWith(
        expect.objectContaining({
          name: 'My Flow',
          handle: 'my-flow',
          flowType: 'AUTHENTICATION',
          ouId: 'ou-root',
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
        (capturedConfigureProps['onChange'] as (v: {name: string; handle: string}) => void)({
          name: 'Test',
          handle: 'test',
        });
        (capturedConfigureProps['onReadyChange'] as (ready: boolean) => void)(true);
      });

      fireEvent.click(screen.getByText('Create'));

      await waitFor(() => {
        expect(mockNavigate).toHaveBeenCalledWith('/flows/flow-123');
      });
    });

    it('should display a resolved error message when creation fails, never the raw server text', async () => {
      mockMutate.mockImplementation((_req: unknown, options: {onError: (err: Error) => void}) => {
        options.onError(new Error('Creation failed'));
      });

      render(<FlowCreatePage />);

      navigateToConfigureStep();

      act(() => {
        (capturedConfigureProps['onChange'] as (v: {name: string; handle: string}) => void)({
          name: 'Test',
          handle: 'test',
        });
        (capturedConfigureProps['onReadyChange'] as (ready: boolean) => void)(true);
      });

      fireEvent.click(screen.getByText('Create'));

      await waitFor(() => {
        expect(screen.getByText('Failed to create flow. Please try again.')).toBeInTheDocument();
        expect(screen.queryByText('Creation failed')).not.toBeInTheDocument();
      });
    });

    it('should clear a stale create error and reset the errored mutation as soon as the flow type changes', async () => {
      mockMutate.mockImplementation((_req: unknown, options: {onError: (err: Error) => void}) => {
        options.onError(new Error('Creation failed'));
      });

      render(<FlowCreatePage />);

      navigateToConfigureStep();

      act(() => {
        (capturedConfigureProps['onChange'] as (v: {name: string; handle: string}) => void)({
          name: 'Test',
          handle: 'test',
        });
        (capturedConfigureProps['onReadyChange'] as (ready: boolean) => void)(true);
      });

      fireEvent.click(screen.getByText('Create'));

      await waitFor(() => {
        expect(screen.getByText('Failed to create flow. Please try again.')).toBeInTheDocument();
      });

      // Simulate the mutation now being in an errored state, as it would be after onError fires.
      mockCreateFlow.isError = true;

      // Navigate back to the Type step and change the type: this feeds the create request, so it
      // should clear the stale error and reset the errored mutation.
      fireEvent.click(screen.getByRole('button', {name: 'Flow type'}));
      act(() => {
        (capturedTypeProps['onTypeChange'] as (type: string) => void)('REGISTRATION');
      });

      expect(screen.queryByText('Failed to create flow. Please try again.')).not.toBeInTheDocument();
      expect(mockReset).toHaveBeenCalledTimes(1);
    });

    it('should not reset the create mutation on a field change while a create is still pending', () => {
      mockCreateFlow.isPending = true;
      mockCreateFlow.isError = false;

      render(<FlowCreatePage />);

      navigateToConfigureStep();

      act(() => {
        (capturedConfigureProps['onChange'] as (v: {name: string; handle: string}) => void)({
          name: 'Test',
          handle: 'test',
        });
      });

      // A real mutation can't be isPending and isError at once, so clearCreateError's isError
      // check alone is sufficient to guard reset() from firing mid-flight.
      expect(mockReset).not.toHaveBeenCalled();
    });
  });
  describe('Organization Unit', () => {
    const fillConfigureStep = (): void => {
      navigateToConfigureStep();
      act(() => {
        (capturedConfigureProps['onChange'] as (v: {name: string; handle: string}) => void)({
          name: 'My Flow',
          handle: 'my-flow',
        });
        (capturedConfigureProps['onReadyChange'] as (ready: boolean) => void)(true);
      });
    };

    it('skips the organization unit step and uses the only organization unit', () => {
      render(<FlowCreatePage />);

      expect(screen.queryByTestId('ou-step')).not.toBeInTheDocument();
      expect(screen.getByTestId('select-flow-type')).toBeInTheDocument();
    });

    it('shows a loading step while the organization units load', () => {
      ouState.isLoading = true;
      render(<FlowCreatePage />);

      expect(screen.getByTestId('ou-step')).toHaveAttribute('data-loading', 'true');
      expect(screen.queryByTestId('select-flow-type')).not.toBeInTheDocument();
    });

    it('asks which organization unit owns the flow when there are several, and sends the choice', () => {
      ouState.hasMultipleOUs = true;
      ouState.ouList = [{id: 'ou-root'}, {id: 'ou-b'}];
      render(<FlowCreatePage />);

      expect(screen.getByTestId('ou-step')).toBeInTheDocument();
      expect(screen.queryByTestId('select-flow-type')).not.toBeInTheDocument();

      fireEvent.click(screen.getByTestId('ou-pick-b'));
      fireEvent.click(screen.getByTestId('ou-continue'));

      expect(screen.getByTestId('select-flow-type')).toBeInTheDocument();

      fillConfigureStep();
      fireEvent.click(screen.getByText('Create'));

      expect(mockMutate).toHaveBeenCalledWith(expect.objectContaining({ouId: 'ou-b'}), expect.any(Object));
    });

    it('returns to the organization unit step from its breadcrumb', () => {
      ouState.hasMultipleOUs = true;
      ouState.ouList = [{id: 'ou-root'}, {id: 'ou-b'}];
      render(<FlowCreatePage />);

      fireEvent.click(screen.getByTestId('ou-pick-b'));
      fireEvent.click(screen.getByTestId('ou-continue'));
      fireEvent.click(screen.getByText('Organization unit'));

      expect(screen.getByTestId('ou-step')).toHaveAttribute('data-value', 'ou-b');
    });

    it('goes back to the organization unit step from the first wizard step', () => {
      ouState.hasMultipleOUs = true;
      ouState.ouList = [{id: 'ou-root'}, {id: 'ou-b'}];
      render(<FlowCreatePage />);

      fireEvent.click(screen.getByTestId('ou-pick-b'));
      fireEvent.click(screen.getByTestId('ou-continue'));
      fireEvent.click(screen.getByText('Back'));

      expect(screen.getByTestId('ou-step')).toHaveAttribute('data-value', 'ou-b');
    });

    it('has no back button on the first step of a single-unit deployment', () => {
      render(<FlowCreatePage />);

      expect(screen.getByTestId('select-flow-type')).toBeInTheDocument();
      expect(screen.queryByText('Back')).not.toBeInTheDocument();
    });

    it('closes the wizard from the organization unit step', () => {
      ouState.hasMultipleOUs = true;
      ouState.ouList = [{id: 'ou-root'}, {id: 'ou-b'}];
      render(<FlowCreatePage />);

      fireEvent.click(screen.getByTestId('ou-back'));

      expect(mockNavigate).toHaveBeenCalledWith('/flows');
    });

    it('refuses to create a flow when no organization unit could be resolved', () => {
      ouState.ouList = [];
      render(<FlowCreatePage />);

      fillConfigureStep();
      fireEvent.click(screen.getByText('Create'));

      expect(mockMutate).not.toHaveBeenCalled();
      expect(screen.getByText('Organization unit is required')).toBeInTheDocument();
    });
  });
});
