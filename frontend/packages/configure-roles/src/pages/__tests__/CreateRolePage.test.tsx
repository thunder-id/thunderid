// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent} from '@testing-library/react';
import {render, screen} from '@thunderid/test-utils';
import type {NavigateFunction} from 'react-router';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import {RoleCreateFlowStep} from '../../models/role-create-flow';
import CreateRolePage from '../CreateRolePage';

// Mock dependencies
vi.mock('../../api/useCreateRole');
vi.mock('../../contexts/RoleCreate/useRoleCreate');
vi.mock('@thunderid/configure-organization-units', () => ({
  useHasMultipleOUs: vi.fn(),
  useGetOrganizationUnit: (id?: string) => ({
    data: id ? {id, name: 'Test Organization Unit'} : undefined,
    isLoading: false,
  }),
  OrganizationUnitPickerScreen: ({onBack, onContinue}: {onBack: () => void; onContinue: () => void}) => (
    <div data-testid="organization-unit-picker-screen">
      <button type="button" onClick={onBack}>
        Back
      </button>
      <button type="button" onClick={onContinue}>
        Continue
      </button>
    </div>
  ),
}));

vi.mock('../../components/create-role/ConfigureBasicInfo', () => ({
  default: () => <div data-testid="configure-basic-info">Configure Basic Info</div>,
}));

vi.mock('../../components/create-role/ConfigurePermissions', () => ({
  default: ({
    onPermissionsChange,
  }: {
    onPermissionsChange: (permissions: {resourceServerId: string; permissions: string[]}[]) => void;
  }) => (
    <div data-testid="configure-permissions">
      <button
        type="button"
        data-testid="stage-permissions"
        onClick={() => onPermissionsChange([{resourceServerId: 'rs-1', permissions: ['bookings']}])}
      >
        Stage
      </button>
    </div>
  ),
}));

vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: vi.fn(),
  };
});

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({
    error: vi.fn(),
    info: vi.fn(),
    warn: vi.fn(),
    debug: vi.fn(),
  }),
}));

const {default: useCreateRole} = await import('../../api/useCreateRole');
const {default: useRoleCreate} = await import('../../contexts/RoleCreate/useRoleCreate');
const {useHasMultipleOUs} = await import('@thunderid/configure-organization-units');
const {useNavigate} = await import('react-router');

describe('CreateRolePage', () => {
  let mockNavigate: ReturnType<typeof vi.fn>;
  let mockSetCurrentStep: ReturnType<typeof vi.fn>;
  let mockSetName: ReturnType<typeof vi.fn>;
  let mockSetOuId: ReturnType<typeof vi.fn>;
  let mockSetError: ReturnType<typeof vi.fn>;
  let mockSetPermissions: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockNavigate = vi.fn();
    mockSetCurrentStep = vi.fn();
    mockSetName = vi.fn();
    mockSetOuId = vi.fn();
    mockSetError = vi.fn();
    mockSetPermissions = vi.fn();

    vi.mocked(useNavigate).mockReturnValue(mockNavigate as unknown as NavigateFunction);

    vi.mocked(useRoleCreate).mockReturnValue({
      currentStep: RoleCreateFlowStep.BASIC_INFO,
      setCurrentStep: mockSetCurrentStep,
      name: 'Test Role',
      setName: mockSetName,
      ouId: '',
      setOuId: mockSetOuId,
      error: null,
      setError: mockSetError,
      permissions: [],
      setPermissions: mockSetPermissions,
      reset: vi.fn(),
    } as unknown as ReturnType<typeof useRoleCreate>);

    vi.mocked(useCreateRole).mockReturnValue({
      mutate: vi.fn(),
      mutateAsync: vi.fn(),
      isPending: false,
      isError: false,
      isSuccess: false,
      error: null,
      data: undefined,
      reset: vi.fn(),
      context: undefined,
      failureCount: 0,
      failureReason: null,
      isIdle: true,
      isPaused: false,
      status: 'idle',
      submittedAt: 0,
      variables: undefined,
    } as unknown as ReturnType<typeof useCreateRole>);

    vi.mocked(useHasMultipleOUs).mockReturnValue({
      hasMultipleOUs: false,
      isLoading: false,
      ouList: [{id: 'ou-1', handle: 'default-ou', name: 'Default OU'}],
    });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should render LinearProgress bar', () => {
    render(<CreateRolePage />);

    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('should render close button', () => {
    render(<CreateRolePage />);

    expect(screen.getByRole('button', {name: /close/i})).toBeInTheDocument();
  });

  it('should render ConfigureBasicInfo on first step', () => {
    render(<CreateRolePage />);

    expect(screen.getByTestId('configure-basic-info')).toBeInTheDocument();
  });

  it('should render Continue button', () => {
    render(<CreateRolePage />);

    expect(screen.getByRole('button', {name: /continue/i})).toBeInTheDocument();
  });

  it('should not render Back button on first step', () => {
    render(<CreateRolePage />);

    expect(screen.queryByRole('button', {name: /back/i})).not.toBeInTheDocument();
  });

  it('should render OrganizationUnitPickerScreen on the organization unit step', () => {
    vi.mocked(useRoleCreate).mockReturnValue({
      currentStep: RoleCreateFlowStep.ORGANIZATION_UNIT,
      setCurrentStep: mockSetCurrentStep,
      name: 'Test Role',
      setName: mockSetName,
      ouId: '',
      setOuId: mockSetOuId,
      error: null,
      setError: mockSetError,
      permissions: [],
      setPermissions: mockSetPermissions,
      reset: vi.fn(),
    } as unknown as ReturnType<typeof useRoleCreate>);

    vi.mocked(useHasMultipleOUs).mockReturnValue({
      hasMultipleOUs: true,
      isLoading: false,
      ouList: [
        {id: 'ou-1', handle: 'ou-1', name: 'OU 1'},
        {id: 'ou-2', handle: 'ou-2', name: 'OU 2'},
      ],
    });

    render(<CreateRolePage />);

    expect(screen.getByTestId('organization-unit-picker-screen')).toBeInTheDocument();
  });

  it('should render Back button on the OU picker', () => {
    vi.mocked(useRoleCreate).mockReturnValue({
      currentStep: RoleCreateFlowStep.ORGANIZATION_UNIT,
      setCurrentStep: mockSetCurrentStep,
      name: 'Test Role',
      setName: mockSetName,
      ouId: '',
      setOuId: mockSetOuId,
      error: null,
      setError: mockSetError,
      permissions: [],
      setPermissions: mockSetPermissions,
      reset: vi.fn(),
    } as unknown as ReturnType<typeof useRoleCreate>);

    vi.mocked(useHasMultipleOUs).mockReturnValue({
      hasMultipleOUs: true,
      isLoading: false,
      ouList: [
        {id: 'ou-1', handle: 'ou-1', name: 'OU 1'},
        {id: 'ou-2', handle: 'ou-2', name: 'OU 2'},
      ],
    });

    render(<CreateRolePage />);

    expect(screen.getByRole('button', {name: /back/i})).toBeInTheDocument();
  });

  it('should render ConfigurePermissions on permissions step', () => {
    vi.mocked(useRoleCreate).mockReturnValue({
      currentStep: RoleCreateFlowStep.PERMISSIONS,
      setCurrentStep: mockSetCurrentStep,
      name: 'Test Role',
      setName: mockSetName,
      ouId: '',
      setOuId: mockSetOuId,
      error: null,
      setError: mockSetError,
      permissions: [],
      setPermissions: mockSetPermissions,
      reset: vi.fn(),
    } as unknown as ReturnType<typeof useRoleCreate>);

    render(<CreateRolePage />);

    expect(screen.getByTestId('configure-permissions')).toBeInTheDocument();
  });

  it('should render Back button on permissions step', () => {
    vi.mocked(useRoleCreate).mockReturnValue({
      currentStep: RoleCreateFlowStep.PERMISSIONS,
      setCurrentStep: mockSetCurrentStep,
      name: 'Test Role',
      setName: mockSetName,
      ouId: '',
      setOuId: mockSetOuId,
      error: null,
      setError: mockSetError,
      permissions: [],
      setPermissions: mockSetPermissions,
      reset: vi.fn(),
    } as unknown as ReturnType<typeof useRoleCreate>);

    render(<CreateRolePage />);

    expect(screen.getByRole('button', {name: /back/i})).toBeInTheDocument();
  });

  it('should render context error alert when error is set', () => {
    vi.mocked(useRoleCreate).mockReturnValue({
      currentStep: RoleCreateFlowStep.BASIC_INFO,
      setCurrentStep: mockSetCurrentStep,
      name: 'Test Role',
      setName: mockSetName,
      ouId: '',
      setOuId: mockSetOuId,
      error: 'Something went wrong',
      setError: mockSetError,
      permissions: [],
      setPermissions: mockSetPermissions,
      reset: vi.fn(),
    } as unknown as ReturnType<typeof useRoleCreate>);

    render(<CreateRolePage />);

    expect(screen.getByText('Something went wrong')).toBeInTheDocument();
  });

  it('should render the provider error inline', () => {
    vi.mocked(useRoleCreate).mockReturnValue({
      currentStep: RoleCreateFlowStep.BASIC_INFO,
      setCurrentStep: mockSetCurrentStep,
      name: 'Test Role',
      setName: mockSetName,
      ouId: '',
      setOuId: mockSetOuId,
      error: 'Failed to create role. Please try again.',
      setError: mockSetError,
      permissions: [],
      setPermissions: mockSetPermissions,
      reset: vi.fn(),
    } as unknown as ReturnType<typeof useRoleCreate>);

    render(<CreateRolePage />);

    expect(screen.getByText('Failed to create role. Please try again.')).toBeInTheDocument();
  });

  it('should resolve and store the mapped error message when the role name conflicts on submit', async () => {
    const error = new Error('Request failed') as Error & {response?: {data?: {code: string}}};
    error.response = {data: {code: 'ROL-1004'}};
    const mockMutateAsync = vi.fn().mockRejectedValue(error);

    vi.mocked(useCreateRole).mockReturnValue({
      mutate: vi.fn(),
      mutateAsync: mockMutateAsync,
      isPending: false,
      isError: false,
      isSuccess: false,
      error: null,
      data: undefined,
      reset: vi.fn(),
      context: undefined,
      failureCount: 0,
      failureReason: null,
      isIdle: true,
      isPaused: false,
      status: 'idle',
      submittedAt: 0,
      variables: undefined,
    } as unknown as ReturnType<typeof useCreateRole>);

    vi.mocked(useRoleCreate).mockReturnValue({
      currentStep: RoleCreateFlowStep.PERMISSIONS,
      setCurrentStep: mockSetCurrentStep,
      name: 'Test Role',
      setName: mockSetName,
      ouId: '',
      setOuId: mockSetOuId,
      error: null,
      setError: mockSetError,
      permissions: [],
      setPermissions: mockSetPermissions,
      reset: vi.fn(),
    } as unknown as ReturnType<typeof useRoleCreate>);

    render(<CreateRolePage />);

    fireEvent.click(screen.getByRole('button', {name: /continue/i}));

    await vi.waitFor(() => {
      expect(mockSetError).toHaveBeenCalledWith(
        'A role with this name already exists in this organization unit. Choose a different name.',
      );
    });
  });

  it('should disable Continue button while mutation is pending', () => {
    vi.mocked(useCreateRole).mockReturnValue({
      mutate: vi.fn(),
      mutateAsync: vi.fn(),
      isPending: true,
      isError: false,
      isSuccess: false,
      error: null,
      data: undefined,
      reset: vi.fn(),
      context: undefined,
      failureCount: 0,
      failureReason: null,
      isIdle: false,
      isPaused: false,
      status: 'pending',
      submittedAt: 0,
      variables: undefined,
    } as unknown as ReturnType<typeof useCreateRole>);

    render(<CreateRolePage />);

    expect(screen.getByRole('button', {name: /saving/i})).toBeDisabled();
  });

  it('should submit with permissions in POST payload when permissions are selected on the permissions step', async () => {
    const mockMutateAsync = vi.fn().mockResolvedValue({});
    vi.mocked(useCreateRole).mockReturnValue({
      mutate: vi.fn(),
      mutateAsync: mockMutateAsync,
      isPending: false,
      isError: false,
      isSuccess: false,
      error: null,
      data: undefined,
      reset: vi.fn(),
      context: undefined,
      failureCount: 0,
      failureReason: null,
      isIdle: true,
      isPaused: false,
      status: 'idle',
      submittedAt: 0,
      variables: undefined,
    } as unknown as ReturnType<typeof useCreateRole>);

    vi.mocked(useRoleCreate).mockReturnValue({
      currentStep: RoleCreateFlowStep.PERMISSIONS,
      setCurrentStep: mockSetCurrentStep,
      name: 'Test Role',
      setName: mockSetName,
      ouId: '',
      setOuId: mockSetOuId,
      error: null,
      setError: mockSetError,
      permissions: [{resourceServerId: 'rs-1', permissions: ['bookings']}],
      setPermissions: mockSetPermissions,
      reset: vi.fn(),
    } as unknown as ReturnType<typeof useRoleCreate>);

    render(<CreateRolePage />);

    fireEvent.click(screen.getByRole('button', {name: /continue/i}));

    await vi.waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith(
        expect.objectContaining({
          permissions: [{resourceServerId: 'rs-1', permissions: ['bookings']}],
        }),
      );
    });
  });

  it('should submit without a permissions field when no permissions are selected on the permissions step', async () => {
    const mockMutateAsync = vi.fn().mockResolvedValue({});
    vi.mocked(useCreateRole).mockReturnValue({
      mutate: vi.fn(),
      mutateAsync: mockMutateAsync,
      isPending: false,
      isError: false,
      isSuccess: false,
      error: null,
      data: undefined,
      reset: vi.fn(),
      context: undefined,
      failureCount: 0,
      failureReason: null,
      isIdle: true,
      isPaused: false,
      status: 'idle',
      submittedAt: 0,
      variables: undefined,
    } as unknown as ReturnType<typeof useCreateRole>);

    vi.mocked(useRoleCreate).mockReturnValue({
      currentStep: RoleCreateFlowStep.PERMISSIONS,
      setCurrentStep: mockSetCurrentStep,
      name: 'Test Role',
      setName: mockSetName,
      ouId: '',
      setOuId: mockSetOuId,
      error: null,
      setError: mockSetError,
      permissions: [],
      setPermissions: mockSetPermissions,
      reset: vi.fn(),
    } as unknown as ReturnType<typeof useRoleCreate>);

    render(<CreateRolePage />);

    fireEvent.click(screen.getByRole('button', {name: /continue/i}));

    await vi.waitFor(() => {
      expect(mockMutateAsync).toHaveBeenCalledWith({
        name: 'Test Role',
        ouId: 'ou-1',
      });
    });
  });

  describe('submit guards', () => {
    const submitWithName = (name: string): ReturnType<typeof vi.fn> => {
      const mockMutateAsync = vi.fn().mockResolvedValue({});
      vi.mocked(useCreateRole).mockReturnValue({
        mutate: vi.fn(),
        mutateAsync: mockMutateAsync,
        isPending: false,
        isError: false,
        isSuccess: false,
        error: null,
        data: undefined,
        reset: vi.fn(),
        context: undefined,
        failureCount: 0,
        failureReason: null,
        isIdle: true,
        isPaused: false,
        status: 'idle',
        submittedAt: 0,
        variables: undefined,
      } as unknown as ReturnType<typeof useCreateRole>);

      vi.mocked(useRoleCreate).mockReturnValue({
        currentStep: RoleCreateFlowStep.PERMISSIONS,
        setCurrentStep: mockSetCurrentStep,
        name,
        setName: mockSetName,
        ouId: '',
        setOuId: mockSetOuId,
        error: null,
        setError: mockSetError,
        permissions: [],
        setPermissions: mockSetPermissions,
        reset: vi.fn(),
      } as unknown as ReturnType<typeof useRoleCreate>);

      render(<CreateRolePage />);

      fireEvent.click(screen.getByRole('button', {name: /continue/i}));

      return mockMutateAsync;
    };

    it('should refuse a name that is only whitespace', async () => {
      const mockMutateAsync = submitWithName('   ');

      await vi.waitFor(() => {
        expect(mockSetError).toHaveBeenCalledWith('Role name is required');
      });
      expect(mockMutateAsync).not.toHaveBeenCalled();
    });

    it('should refuse a name longer than the maximum length', async () => {
      const mockMutateAsync = submitWithName('a'.repeat(101));

      await vi.waitFor(() => {
        expect(mockSetError).toHaveBeenCalledWith('Role name cannot exceed 100 characters');
      });
      expect(mockMutateAsync).not.toHaveBeenCalled();
    });
  });
});
