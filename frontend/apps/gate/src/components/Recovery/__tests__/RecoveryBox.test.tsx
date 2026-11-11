// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act} from '@testing-library/react';
import {render as testRender, screen} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import RecoveryBox from '../RecoveryBox';

const {mockLogger} = vi.hoisted(() => ({
  mockLogger: {
    error: vi.fn(),
    warn: vi.fn(),
    info: vi.fn(),
    debug: vi.fn(),
  },
}));

vi.mock('@thunderid/logger/react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/logger/react')>();
  return {
    ...actual,
    useLogger: () => mockLogger,
  };
});

// Mock useDesign
const mockUseDesign = vi.fn();
vi.mock('@thunderid/design', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/design')>();
  return {
    ...actual,
    // eslint-disable-next-line @typescript-eslint/no-unsafe-return
    useDesign: () => mockUseDesign(),
    AuthCardLayout: ({children}: {children: React.ReactNode}) => <div data-testid="auth-card-layout">{children}</div>,
    FlowComponentRenderer: ({component}: {component: {id?: string; type: string}}) => (
      <div data-testid={`flow-component-${component.id ?? component.type}`}>{component.id ?? component.type}</div>
    ),
  };
});

// Mock useTemplateLiteralResolver
vi.mock('@thunderid/hooks', () => ({
  useTemplateLiteralResolver: () => ({
    resolveAll: (template: string) => template,
  }),
}));

// Mock react-router hooks
let mockSearchParams: URLSearchParams = new URLSearchParams();
vi.mock('react-router', () => ({
  useSearchParams: () => [mockSearchParams, vi.fn()],
}));

// Render props interface
interface MockRecoveryRenderProps {
  fieldErrors: Record<string, string>;
  error: {message?: string} | null;
  touched: Record<string, boolean>;
  isLoading: boolean;
  components: unknown[];
  values: Record<string, string>;
  handleInputChange: () => void;
  handleSubmit: (action: unknown, inputs: unknown, isTrigger: boolean) => Promise<void>;
  isFlowExpired?: boolean;
  isFlowRestarted?: boolean;
  flowExpiryRedirect?: null;
  meta: Record<string, unknown>;
}

const createMockRecoveryRenderProps = (overrides: Partial<MockRecoveryRenderProps> = {}): MockRecoveryRenderProps => ({
  fieldErrors: {},
  error: null,
  touched: {},
  isLoading: false,
  components: [],
  values: {},
  handleInputChange: vi.fn(),
  handleSubmit: vi.fn().mockResolvedValue(undefined),
  meta: {},
  ...overrides,
});

let mockRecoveryRenderProps: MockRecoveryRenderProps = createMockRecoveryRenderProps();

// Track props passed to Recovery
let capturedOnFlowChange: ((response: unknown) => void) | undefined;
let capturedOnError: ((error: Error) => void) | undefined;
let capturedAfterRecoveryUrl: string | undefined;
let capturedAfterFlowExpiredUrl: string | undefined;
let mockMeta: {application?: {name?: string; url?: string}} | null = null;

vi.mock('@thunderid/react', async () => {
  const actual = await vi.importActual('@thunderid/react');
  return {
    ...actual,
    useThunderID: () => ({meta: mockMeta}),
    Recovery: ({
      children,
      afterRecoveryUrl = undefined,
      afterFlowExpiredUrl = undefined,
      onError = undefined,
      onFlowChange = undefined,
    }: {
      children: (props: typeof mockRecoveryRenderProps) => React.ReactNode;
      afterRecoveryUrl?: string;
      afterFlowExpiredUrl?: string;
      onError?: (error: Error) => void;
      onFlowChange?: (response: unknown) => void;
    }) => {
      capturedOnFlowChange = onFlowChange;
      capturedOnError = onError;
      capturedAfterRecoveryUrl = afterRecoveryUrl;
      capturedAfterFlowExpiredUrl = afterFlowExpiredUrl;
      return <div data-testid="thunderid-recovery">{children(mockRecoveryRenderProps)}</div>;
    },
    EmbeddedFlowEventType: {
      Submit: 'SUBMIT',
      Trigger: 'TRIGGER',
    },
  };
});

// Wrap renders with DesignContext consistent with the mock
const render = (ui: React.ReactElement) => testRender(ui);

describe('RecoveryBox', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseDesign.mockReturnValue({
      isDesignEnabled: false,
      isLoading: false,
    });
    mockSearchParams = new URLSearchParams();
    mockRecoveryRenderProps = createMockRecoveryRenderProps();
    capturedOnFlowChange = undefined;
    capturedOnError = undefined;
    capturedAfterRecoveryUrl = undefined;
    capturedAfterFlowExpiredUrl = undefined;
    mockMeta = null;
  });

  it('sends the user back to the application once the flow expires', () => {
    mockMeta = {application: {url: 'https://acme.example.com'}};
    render(<RecoveryBox />);

    expect(capturedAfterFlowExpiredUrl).toBe('https://acme.example.com');
  });

  it('renders without crashing', () => {
    const {container} = render(<RecoveryBox />);
    expect(container).toBeInTheDocument();
  });

  it('renders the ThunderID Recovery component', () => {
    render(<RecoveryBox />);
    expect(screen.getByTestId('thunderid-recovery')).toBeInTheDocument();
  });

  it('tells the user when the flow was started again', () => {
    mockRecoveryRenderProps = createMockRecoveryRenderProps({isFlowRestarted: true});
    render(<RecoveryBox />);

    expect(screen.getByText('Your session expired, so we started again.')).toBeInTheDocument();
  });

  it('shows the expired state in place of the flow once it expires', () => {
    mockRecoveryRenderProps = createMockRecoveryRenderProps({
      isFlowExpired: true,
      flowExpiryRedirect: null,
      error: {message: 'Invalid request'},
      meta: {application: {name: 'Acme Portal'}},
    });
    render(<RecoveryBox />);

    expect(screen.getByTestId('flow-expired')).toBeInTheDocument();
    expect(screen.getByText('Please go back to Acme Portal and try again.')).toBeInTheDocument();
    expect(screen.queryByText('Invalid request')).not.toBeInTheDocument();
  });

  it('renders AuthCardLayout', () => {
    render(<RecoveryBox />);
    expect(screen.getByTestId('auth-card-layout')).toBeInTheDocument();
  });

  it('shows loading spinner when isLoading and no components', () => {
    mockRecoveryRenderProps = createMockRecoveryRenderProps({
      isLoading: true,
      components: [],
    });
    render(<RecoveryBox />);
    expect(screen.getByRole('progressbar')).toBeInTheDocument();
  });

  it('does not show loading spinner when not loading', () => {
    mockRecoveryRenderProps = createMockRecoveryRenderProps({
      isLoading: false,
      components: [],
    });
    render(<RecoveryBox />);
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
  });

  it('does not show loading spinner when loading but components exist', () => {
    mockRecoveryRenderProps = createMockRecoveryRenderProps({
      isLoading: true,
      components: [{id: 'comp-1', type: 'TEXT'}],
    });
    render(<RecoveryBox />);
    expect(screen.queryByRole('progressbar')).not.toBeInTheDocument();
  });

  it('shows error alert when SDK error is present', () => {
    mockRecoveryRenderProps = createMockRecoveryRenderProps({
      error: {message: 'Invalid email address'},
      components: [],
    });
    render(<RecoveryBox />);
    expect(screen.getByText('Invalid email address')).toBeInTheDocument();
    expect(screen.getByRole('alert')).toBeInTheDocument();
  });

  it('shows flow error alert from onFlowChange with error', () => {
    render(<RecoveryBox />);

    expect(capturedOnFlowChange).toBeDefined();
    act(() => {
      capturedOnFlowChange?.({
        error: {
          code: 'FEE-60001',
          message: {key: 'flows.errors.invalid_code', defaultValue: 'Invalid recovery code'},
          description: {key: 'flows.errors.invalid_code.desc', defaultValue: 'Invalid recovery code'},
        },
      });
    });

    expect(screen.getByText('Invalid recovery code')).toBeInTheDocument();
    expect(screen.getByRole('alert')).toBeInTheDocument();
  });

  it('clears flow error when onFlowChange has no error', () => {
    render(<RecoveryBox />);

    act(() => {
      capturedOnFlowChange?.({
        error: {
          code: 'FEE-60001',
          message: {key: 'flows.errors.some', defaultValue: 'Some error'},
          description: {key: 'flows.errors.some.desc', defaultValue: 'Some error'},
        },
      });
    });
    expect(screen.getByText('Some error')).toBeInTheDocument();

    act(() => {
      capturedOnFlowChange?.({});
    });
    expect(screen.queryByText('Some error')).not.toBeInTheDocument();
  });

  it('shows default error description when error has no message', () => {
    mockRecoveryRenderProps = createMockRecoveryRenderProps({
      error: {},
      components: [],
    });
    render(<RecoveryBox />);
    expect(screen.getByRole('alert')).toBeInTheDocument();
  });

  it('renders flow components when components array is non-empty', () => {
    mockRecoveryRenderProps = createMockRecoveryRenderProps({
      components: [
        {id: 'comp-1', type: 'TEXT'},
        {id: 'comp-2', type: 'BLOCK'},
      ],
    });
    render(<RecoveryBox />);
    expect(screen.getByTestId('flow-component-comp-1')).toBeInTheDocument();
    expect(screen.getByTestId('flow-component-comp-2')).toBeInTheDocument();
  });

  it('does not render flow components when components array is empty', () => {
    mockRecoveryRenderProps = createMockRecoveryRenderProps({
      components: [],
    });
    render(<RecoveryBox />);
    expect(screen.queryByTestId(/^flow-component-/)).not.toBeInTheDocument();
  });

  it('passes afterRecoveryUrl without applicationId', () => {
    render(<RecoveryBox />);
    expect(capturedAfterRecoveryUrl).toContain('/signin');
    expect(capturedAfterRecoveryUrl).not.toContain('applicationId');
  });

  it('passes afterRecoveryUrl with applicationId from search params', () => {
    mockSearchParams = new URLSearchParams({applicationId: 'app-123'});
    render(<RecoveryBox />);
    expect(capturedAfterRecoveryUrl).toContain('/signin');
    expect(capturedAfterRecoveryUrl).toContain('applicationId=app-123');
  });

  it('calls logger.error via onError callback', () => {
    render(<RecoveryBox />);

    expect(capturedOnError).toBeDefined();
    const testError = new Error('Recovery error');
    capturedOnError?.(testError);

    expect(mockLogger.error).toHaveBeenCalledWith('Recovery error:', testError);
  });

  it('renders with design enabled', () => {
    mockUseDesign.mockReturnValue({
      isDesignEnabled: true,
      isLoading: false,
    });
    render(<RecoveryBox />);
    expect(screen.getByTestId('thunderid-recovery')).toBeInTheDocument();
  });

  it('uses fallback index keys when components have undefined id', () => {
    mockRecoveryRenderProps = createMockRecoveryRenderProps({
      components: [{type: 'TEXT'}, {type: 'BLOCK'}],
    });
    render(<RecoveryBox />);
    expect(screen.getByTestId('flow-component-TEXT')).toBeInTheDocument();
    expect(screen.getByTestId('flow-component-BLOCK')).toBeInTheDocument();
  });

  it('shows flow error with alert title', () => {
    render(<RecoveryBox />);

    act(() => {
      capturedOnFlowChange?.({
        error: {
          code: 'FEE-60001',
          message: {key: 'flows.errors.not_found', defaultValue: 'Account not found'},
          description: {key: 'flows.errors.not_found.desc', defaultValue: 'Account not found'},
        },
      });
    });

    expect(screen.getByText('Account not found')).toBeInTheDocument();
    expect(screen.getByRole('alert')).toBeInTheDocument();
  });

  it('prefers flow error over SDK error in alert', () => {
    mockRecoveryRenderProps = createMockRecoveryRenderProps({
      error: {message: 'SDK connectivity error'},
    });
    render(<RecoveryBox />);

    act(() => {
      capturedOnFlowChange?.({
        error: {
          code: 'FEE-60001',
          message: {key: 'flows.errors.validation', defaultValue: 'Flow validation error'},
          description: {key: 'flows.errors.validation.desc', defaultValue: 'Flow validation error'},
        },
      });
    });

    expect(screen.getByText('Flow validation error')).toBeInTheDocument();
    expect(screen.queryByText('SDK connectivity error')).not.toBeInTheDocument();
  });
});
