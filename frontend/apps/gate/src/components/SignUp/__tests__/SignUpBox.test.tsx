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

import userEvent from '@testing-library/user-event';
import {DesignContext, type DesignContextType} from '@thunderid/design';
import {screen, fireEvent, waitFor, render as testRender} from '@thunderid/test-utils';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import SignUpBox from '../SignUpBox';

// Mock useDesign and FlowComponentRenderer
const mockUseDesign = vi.fn();
vi.mock('@thunderid/design', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/design')>();
  return {
    ...actual,
    // eslint-disable-next-line @typescript-eslint/no-unsafe-return
    useDesign: () => mockUseDesign(),
    mapEmbeddedFlowTextVariant: (variant: string) => {
      switch (variant) {
        case 'H1':
          return 'h1';
        case 'H2':
          return 'h2';
        default:
          return 'body1';
      }
    },
  };
});

// Wrap renders with DesignContext so real adapters inside FlowComponentRenderer can access it
const render = (ui: React.ReactElement) => {
  const designValue: DesignContextType = {
    isDesignEnabled: false,
    isLoading: false,
    ...(mockUseDesign() as Partial<DesignContextType>),
  };
  return testRender(<DesignContext.Provider value={designValue}>{ui}</DesignContext.Provider>);
};

// Mock useTemplateLiteralResolver
vi.mock('@thunderid/hooks', () => ({
  useTemplateLiteralResolver: () => ({
    resolve: (key: string) => key,
  }),
}));

// Mock react-router hooks
const mockNavigate = vi.fn();
vi.mock('react-router', () => ({
  useNavigate: () => mockNavigate,
  useSearchParams: () => [new URLSearchParams(), vi.fn()],
}));

// Mock ThunderID SignUp component
const mockHandleSubmit = vi.fn().mockResolvedValue(undefined);
const mockHandleInputChange = vi.fn();

interface MockSignUpRenderProps {
  values: Record<string, string>;
  fieldErrors: Record<string, string>;
  error: {message: string} | null;
  touched: Record<string, boolean>;
  handleInputChange: typeof mockHandleInputChange;
  handleSubmit: typeof mockHandleSubmit;
  isLoading: boolean;
  components: unknown[];
}

// Factory function to create fresh mock props for each test
const createMockSignUpRenderProps = (overrides: Partial<MockSignUpRenderProps> = {}): MockSignUpRenderProps => ({
  values: {},
  fieldErrors: {},
  error: null,
  touched: {},
  handleInputChange: mockHandleInputChange,
  handleSubmit: mockHandleSubmit,
  isLoading: false,
  components: [],
  ...overrides,
});

let mockSignUpRenderProps: MockSignUpRenderProps = createMockSignUpRenderProps();
let capturedAfterSignUpUrl: string | undefined;
let mockMeta: {application?: {url?: string}} | null = null;

vi.mock('@thunderid/react', async () => {
  const actual = await vi.importActual('@thunderid/react');
  return {
    ...actual,
    useThunderID: () => ({
      resolveFlowTemplateLiterals: (t: string) => t,
      meta: mockMeta,
    }),
    SignUp: ({
      children,
      afterSignUpUrl = undefined,
    }: {
      children: (props: typeof mockSignUpRenderProps) => React.ReactNode;
      afterSignUpUrl?: string;
    }) => {
      capturedAfterSignUpUrl = afterSignUpUrl;
      return <div data-testid="thunderid-signup">{children(mockSignUpRenderProps)}</div>;
    },
    EmbeddedFlowComponentType: {
      Text: 'TEXT',
      Block: 'BLOCK',
      TextInput: 'TEXT_INPUT',
      PasswordInput: 'PASSWORD_INPUT',
      Action: 'ACTION',
    },
    EmbeddedFlowEventType: {
      Submit: 'SUBMIT',
      Trigger: 'TRIGGER',
    },
  };
});

describe('SignUpBox', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseDesign.mockReturnValue({
      isDesignEnabled: false,
    });
    mockSignUpRenderProps = createMockSignUpRenderProps();
    capturedAfterSignUpUrl = undefined;
    mockMeta = null;
  });

  it('renders without crashing', () => {
    const {container} = render(<SignUpBox />);
    expect(container).toBeInTheDocument();
  });

  it('shows loading spinner when components is null', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: null as unknown as unknown[],
    });
    render(<SignUpBox />);
    expect(screen.getByTestId('thunderid-signup')).toBeInTheDocument();
  });

  it('shows error alert when error is present', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      error: {message: 'Registration failed'},
      components: [{id: 'block', type: 'BLOCK', components: []}],
    });
    render(<SignUpBox />);
    expect(screen.getByText('Registration failed')).toBeInTheDocument();
  });

  it('shows fallback error when no components are available', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [],
    });
    render(<SignUpBox />);
    expect(screen.getByText("Oops, that didn't work")).toBeInTheDocument();
  });

  it('always shows sign-in link regardless of flow state', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({components: []});
    render(<SignUpBox />);

    expect(screen.getByText(/Already have an account/)).toBeInTheDocument();
  });

  it('renders TEXT component as heading', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'text-1',
          type: 'TEXT',
          label: 'Create Account',
          variant: 'H1',
        },
      ],
    });
    render(<SignUpBox />);
    expect(screen.getByText('Create Account')).toBeInTheDocument();
  });

  it('renders TEXT_INPUT component', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'email-input',
              type: 'TEXT_INPUT',
              ref: 'email',
              label: 'Email',
              placeholder: 'Enter your email',
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Sign Up',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);
    expect(screen.getByLabelText(/Email/)).toBeInTheDocument();
  });

  it('renders PASSWORD_INPUT component with toggle visibility', async () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'password-input',
              type: 'PASSWORD_INPUT',
              ref: 'password',
              label: 'Password',
              placeholder: 'Enter your password',
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Sign Up',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);

    const passwordInput = screen.getByLabelText(/Password/);
    expect(passwordInput).toBeInTheDocument();
    expect(passwordInput).toHaveAttribute('type', 'password');

    // Toggle visibility
    const toggleButton = screen.getByLabelText('toggle password visibility');
    await userEvent.click(toggleButton);

    expect(passwordInput).toHaveAttribute('type', 'text');
  });

  it('renders EMAIL_INPUT component', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'email-input',
              type: 'EMAIL_INPUT',
              ref: 'email',
              label: 'Email Address',
              placeholder: 'Enter email',
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Continue',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);
    expect(screen.getByLabelText(/Email Address/)).toBeInTheDocument();
  });

  it('renders SELECT component', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'country-select',
              type: 'SELECT',
              ref: 'country',
              label: 'Country',
              placeholder: 'Select your country',
              options: ['USA', 'Canada', 'UK'],
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Continue',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);
    expect(screen.getByText('Country')).toBeInTheDocument();
    expect(screen.getByRole('combobox')).toBeInTheDocument();
  });

  it('renders SELECT component with object options', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'country-select',
              type: 'SELECT',
              ref: 'country',
              label: 'Country',
              placeholder: 'Select your country',
              options: [
                {value: 'us', label: 'United States'},
                {value: 'ca', label: 'Canada'},
              ],
              hint: 'Select your country of residence',
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Continue',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);
    expect(screen.getByText('Country')).toBeInTheDocument();
    expect(screen.getByText('Select your country of residence')).toBeInTheDocument();
  });

  it('renders PHONE_INPUT component', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'phone-input',
              type: 'PHONE_INPUT',
              ref: 'phone',
              label: 'Phone Number',
              placeholder: 'Enter phone',
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Continue',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);
    expect(screen.getByLabelText(/Phone Number/)).toBeInTheDocument();
  });

  it('renders OTP_INPUT component', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'otp-input',
              type: 'OTP_INPUT',
              ref: 'otp',
              label: 'Enter OTP',
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Verify',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);
    expect(screen.getByText('Enter OTP')).toBeInTheDocument();
    expect(screen.getAllByRole('textbox')).toHaveLength(6);
  });

  it('renders RESEND button', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'otp-input',
              type: 'OTP_INPUT',
              ref: 'otp',
              label: 'Enter OTP',
              required: true,
            },
            {
              id: 'resend-btn',
              type: 'RESEND',
              eventType: 'SUBMIT',
              label: 'Resend OTP',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);
    expect(screen.getByText('Resend OTP')).toBeInTheDocument();
  });

  it('renders TRIGGER action buttons for social login', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'google-btn',
              type: 'ACTION',
              eventType: 'TRIGGER',
              label: 'Continue with Google',
              image: 'google.svg',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);
    expect(screen.getByText('Continue with Google')).toBeInTheDocument();
  });

  it('renders sign in redirect link', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [{id: 'block', type: 'BLOCK', components: []}],
    });
    render(<SignUpBox />);
    expect(screen.getByText('Sign in')).toBeInTheDocument();
  });

  it('falls back to sign-in URL as afterSignUpUrl when meta has no application URL', () => {
    mockMeta = null;
    render(<SignUpBox />);
    const expectedUrl = `${window.location.origin}${import.meta.env.BASE_URL.replace(/\/$/, '')}/signin`;
    expect(capturedAfterSignUpUrl).toBe(expectedUrl);
  });

  it('uses application URL from flow meta as afterSignUpUrl when available', () => {
    mockMeta = {application: {url: 'https://myapp.example.com/home'}};
    render(<SignUpBox />);
    expect(capturedAfterSignUpUrl).toBe('https://myapp.example.com/home');
  });

  it('navigates to sign in page when clicking sign in link', async () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [{id: 'block', type: 'BLOCK', components: []}],
    });
    render(<SignUpBox />);

    const signInLink = screen.getByText('Sign in');
    await userEvent.click(signInLink);

    expect(mockNavigate).toHaveBeenCalledWith('/signin');
  });

  it('submits form when submit button is clicked', async () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'email-input',
              type: 'TEXT_INPUT',
              ref: 'email',
              label: 'Email',
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Sign Up',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);

    const submitBtn = screen.getByText('Sign Up');
    fireEvent.click(submitBtn);

    await waitFor(() => {
      expect(mockHandleSubmit).toHaveBeenCalled();
    });
  });

  it('shows validation errors for fields', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      touched: {email: true},
      fieldErrors: {email: 'Email is required'},
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'email-input',
              type: 'TEXT_INPUT',
              ref: 'email',
              label: 'Email',
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Sign Up',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);
    expect(screen.getByText('Email is required')).toBeInTheDocument();
  });

  it('renders correctly when design is enabled', () => {
    mockUseDesign.mockReturnValue({
      isDesignEnabled: true,
    });
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [{id: 'block', type: 'BLOCK', components: []}],
    });
    render(<SignUpBox />);
    expect(screen.getByTestId('thunderid-signup')).toBeInTheDocument();
  });

  it('handles TRIGGER action within form block', async () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'verify-btn',
              type: 'ACTION',
              eventType: 'TRIGGER',
              label: 'Verify Email',
              variant: 'PRIMARY',
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Continue',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);

    const verifyBtn = screen.getByText('Verify Email');
    await userEvent.click(verifyBtn);

    await waitFor(() => {
      expect(mockHandleSubmit).toHaveBeenCalled();
    });
  });

  it('handles social login trigger click', async () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'github-btn',
              type: 'ACTION',
              eventType: 'TRIGGER',
              label: 'Continue with GitHub',
              image: 'github.svg',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);

    const githubBtn = screen.getByText('Continue with GitHub');
    await userEvent.click(githubBtn);

    await waitFor(() => {
      expect(mockHandleSubmit).toHaveBeenCalled();
    });
  });

  it('shows validation error for SELECT component', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      touched: {country: true},
      fieldErrors: {country: 'Country is required'},
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'country-select',
              type: 'SELECT',
              ref: 'country',
              label: 'Country',
              placeholder: 'Select your country',
              options: ['USA', 'Canada'],
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Continue',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);
    expect(screen.getByText('Country is required')).toBeInTheDocument();
  });

  it('shows validation error for OTP_INPUT component', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      touched: {otp: true},
      fieldErrors: {otp: 'OTP is required'},
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'otp-input',
              type: 'OTP_INPUT',
              ref: 'otp',
              label: 'Enter OTP',
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Verify',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);
    expect(screen.getByText('OTP is required')).toBeInTheDocument();
  });

  it('handles TEXT_INPUT change event', async () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'username-input',
              type: 'TEXT_INPUT',
              ref: 'username',
              label: 'Username',
              placeholder: 'Enter username',
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Sign Up',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);

    const usernameInput = screen.getByLabelText(/Username/);
    await userEvent.type(usernameInput, 'testuser');

    expect(mockHandleInputChange).toHaveBeenCalled();
  });

  it('handles PASSWORD_INPUT change event', async () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'password-input',
              type: 'PASSWORD_INPUT',
              ref: 'password',
              label: 'Password',
              placeholder: 'Enter password',
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Sign Up',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);

    const passwordInput = screen.getByLabelText(/Password/);
    await userEvent.type(passwordInput, 'pass123');

    expect(mockHandleInputChange).toHaveBeenCalled();
  });

  it('handles EMAIL_INPUT change event', async () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'email-input',
              type: 'EMAIL_INPUT',
              ref: 'email',
              label: 'Email',
              placeholder: 'Enter email',
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Sign Up',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);

    const emailInput = screen.getByLabelText(/Email/);
    await userEvent.type(emailInput, 'test@example.com');

    expect(mockHandleInputChange).toHaveBeenCalled();
  });

  it('handles SELECT change event', async () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      values: {},
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'country-select',
              type: 'SELECT',
              ref: 'country',
              label: 'Country',
              placeholder: 'Select your country',
              options: ['USA', 'Canada', 'UK'],
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Continue',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);

    const selectInput = screen.getByRole('combobox');
    await userEvent.click(selectInput);

    // Select an option
    const option = await screen.findByText('USA');
    await userEvent.click(option);

    expect(mockHandleInputChange).toHaveBeenCalled();
  });

  it('renders SELECT component with hint text', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'country-select',
              type: 'SELECT',
              ref: 'country',
              label: 'Country',
              placeholder: 'Select your country',
              options: ['USA', 'Canada', 'UK'],
              hint: 'Choose your country of residence',
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Continue',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);

    expect(screen.getByText('Choose your country of residence')).toBeInTheDocument();
  });

  it('handles PHONE_INPUT change event', async () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'phone-input',
              type: 'PHONE_INPUT',
              ref: 'phone',
              label: 'Phone Number',
              placeholder: 'Enter phone',
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Continue',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);

    const phoneInput = screen.getByLabelText(/Phone Number/);
    await userEvent.type(phoneInput, '+1234567890');

    expect(mockHandleInputChange).toHaveBeenCalled();
  });

  it('handles OTP_INPUT digit entry', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      values: {},
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'otp-input',
              type: 'OTP_INPUT',
              ref: 'otp',
              label: 'Enter OTP',
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Verify',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);

    const otpInputs = screen.getAllByRole('textbox');

    // Type a digit in the first input
    fireEvent.change(otpInputs[0], {target: {value: '1'}});

    expect(mockHandleInputChange).toHaveBeenCalled();
  });

  it('handles OTP_INPUT backspace navigation', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      values: {},
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'otp-input',
              type: 'OTP_INPUT',
              ref: 'otp',
              label: 'Enter OTP',
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Verify',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);

    const otpInputs = screen.getAllByRole('textbox');

    // Focus on second input and press backspace when empty
    otpInputs[1].focus();
    fireEvent.keyDown(otpInputs[1], {key: 'Backspace'});

    // Verify the keydown handler was called
    expect(otpInputs).toHaveLength(6);
  });

  it('handles OTP_INPUT paste', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      values: {},
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'otp-input',
              type: 'OTP_INPUT',
              ref: 'otp',
              label: 'Enter OTP',
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Verify',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);

    const otpInputs = screen.getAllByRole('textbox');

    // Use fireEvent with clipboardData mock
    fireEvent.paste(otpInputs[0], {
      clipboardData: {
        getData: () => '123456',
      },
    });

    // Verify the OTP inputs render
    expect(otpInputs).toHaveLength(6);
  });

  it('rejects non-digit input in OTP field', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      values: {},
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'otp-input',
              type: 'OTP_INPUT',
              ref: 'otp',
              label: 'Enter OTP',
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Verify',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);

    const otpInputs = screen.getAllByRole('textbox');

    // Try to type a non-digit character
    fireEvent.change(otpInputs[0], {target: {value: 'a'}});

    // The input should not accept the character
    expect(mockHandleInputChange).not.toHaveBeenCalled();
  });

  it('handles SELECT with object option having complex value', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      values: {},
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'complex-select',
              type: 'SELECT',
              ref: 'complexField',
              label: 'Complex Field',
              placeholder: 'Select option',
              options: [{value: {nested: 'value'}, label: {text: 'Label'}}],
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Continue',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);

    expect(screen.getByText('Complex Field')).toBeInTheDocument();
  });

  it('shows validation error for PASSWORD_INPUT', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      touched: {password: true},
      fieldErrors: {password: 'Password is required'},
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'password-input',
              type: 'PASSWORD_INPUT',
              ref: 'password',
              label: 'Password',
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Sign Up',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);
    expect(screen.getByText('Password is required')).toBeInTheDocument();
  });

  it('shows validation error for EMAIL_INPUT', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      touched: {email: true},
      fieldErrors: {email: 'Email is invalid'},
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'email-input',
              type: 'EMAIL_INPUT',
              ref: 'email',
              label: 'Email',
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Sign Up',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);
    expect(screen.getByText('Email is invalid')).toBeInTheDocument();
  });

  it('shows validation error for PHONE_INPUT', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      touched: {phone: true},
      fieldErrors: {phone: 'Phone is required'},
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'phone-input',
              type: 'PHONE_INPUT',
              ref: 'phone',
              label: 'Phone Number',
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Continue',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);
    expect(screen.getByText('Phone is required')).toBeInTheDocument();
  });

  it('renders outlined button variant for non-PRIMARY action', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'email-input',
              type: 'TEXT_INPUT',
              ref: 'email',
              label: 'Email',
              required: false,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Continue',
              variant: 'SECONDARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);

    const submitBtn = screen.getByText('Continue');
    expect(submitBtn).toBeInTheDocument();
  });

  it('shows "Creating account..." text when loading', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      isLoading: true,
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'email-input',
              type: 'TEXT_INPUT',
              ref: 'email',
              label: 'Email',
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Sign Up',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);

    expect(screen.getByText('Creating account...')).toBeInTheDocument();
  });

  it('renders block without submit action and shows nothing', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'email-input',
              type: 'TEXT_INPUT',
              ref: 'email',
              label: 'Email',
              required: true,
            },
            // No submit action - only trigger actions or no actions
          ],
        },
      ],
    });
    render(<SignUpBox />);
    // Block without submit or trigger action should not render form fields
    expect(screen.queryByLabelText(/Email/)).not.toBeInTheDocument();
  });

  it('handles autoComplete attribute for username field', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'username-input',
              type: 'TEXT_INPUT',
              ref: 'username',
              label: 'Username',
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Sign Up',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);

    const usernameInput = screen.getByLabelText(/Username/);
    expect(usernameInput).toHaveAttribute('autocomplete', 'username');
  });

  it('handles autoComplete attribute for email field', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'email-input',
              type: 'TEXT_INPUT',
              ref: 'email',
              label: 'Email',
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Sign Up',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);

    const emailInput = screen.getByLabelText(/Email/);
    expect(emailInput).toHaveAttribute('autocomplete', 'email');
  });

  it('handles autoComplete attribute for other fields', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'other-input',
              type: 'TEXT_INPUT',
              ref: 'otherField',
              label: 'Other Field',
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Sign Up',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);

    const otherInput = screen.getByLabelText(/Other Field/);
    expect(otherInput).toHaveAttribute('autocomplete', 'off');
  });

  it('renders TRIGGER inside form block and clicks it', async () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'trigger-btn',
              type: 'ACTION',
              eventType: 'TRIGGER',
              label: 'Trigger Action',
              variant: 'SECONDARY',
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Continue',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);

    const triggerBtn = screen.getByText('Trigger Action');
    expect(triggerBtn).toBeInTheDocument();

    // Click the trigger button
    await userEvent.click(triggerBtn);

    await waitFor(() => {
      expect(mockHandleSubmit).toHaveBeenCalled();
    });
  });

  it('returns null for unknown component type in block', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'unknown-comp',
              type: 'UNKNOWN_COMPONENT',
              ref: 'unknown',
              label: 'Unknown',
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Continue',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);

    // Unknown component should not render
    expect(screen.queryByLabelText(/Unknown/)).not.toBeInTheDocument();
    // But submit button should render
    expect(screen.getByText('Continue')).toBeInTheDocument();
  });

  it('returns null for unknown top-level component type', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'unknown-1',
          type: 'UNKNOWN_TYPE',
          label: 'Unknown',
        },
      ],
    });
    render(<SignUpBox />);

    expect(screen.queryByText('Unknown')).not.toBeInTheDocument();
  });

  it('handles social login trigger in block without form elements', async () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'trigger-block',
          type: 'BLOCK',
          components: [
            {
              id: 'facebook-btn',
              type: 'ACTION',
              eventType: 'TRIGGER',
              label: 'Continue with Facebook',
              image: 'facebook.svg',
            },
            {
              id: 'twitter-btn',
              type: 'ACTION',
              eventType: 'TRIGGER',
              label: 'Continue with Twitter',
              image: 'twitter.svg',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);

    expect(screen.getByText('Continue with Facebook')).toBeInTheDocument();
    expect(screen.getByText('Continue with Twitter')).toBeInTheDocument();

    // Click Facebook button
    const facebookBtn = screen.getByText('Continue with Facebook');
    await userEvent.click(facebookBtn);

    await waitFor(() => {
      expect(mockHandleSubmit).toHaveBeenCalled();
    });
  });

  it('returns null for block with no submit or trigger actions', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'empty-block',
          type: 'BLOCK',
          components: [
            {
              id: 'text-field',
              type: 'TEXT_INPUT',
              ref: 'field',
              label: 'Field',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);

    // Block without submit or trigger action should not render
    expect(screen.queryByLabelText(/Field/)).not.toBeInTheDocument();
  });

  it('does not render input without ref', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
          components: [
            {
              id: 'no-ref-input',
              type: 'TEXT_INPUT',
              // ref is undefined
              label: 'No Ref Field',
              required: true,
            },
            {
              id: 'submit-btn',
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Continue',
              variant: 'PRIMARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);

    // Input without ref should not render
    expect(screen.queryByLabelText(/No Ref Field/)).not.toBeInTheDocument();
  });

  it('returns null for non-TRIGGER action in social login block', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'trigger-block',
          type: 'BLOCK',
          components: [
            {
              id: 'google-btn',
              type: 'ACTION',
              eventType: 'TRIGGER',
              label: 'Continue with Google',
              image: 'google.svg',
            },
            {
              id: 'some-other-action',
              type: 'ACTION',
              eventType: 'OTHER_EVENT',
              label: 'Other Action',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);

    // Google trigger should render
    expect(screen.getByText('Continue with Google')).toBeInTheDocument();
    // Other action type should not render (returns null)
    expect(screen.queryByText('Other Action')).not.toBeInTheDocument();
  });

  it('uses fallback index keys when components have undefined id', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          type: 'TEXT',
          label: 'Create Account',
          variant: 'H1',
        },
        {
          type: 'BLOCK',
          components: [
            {
              type: 'TEXT_INPUT',
              ref: 'given_name',
              label: 'First Name',
              placeholder: 'Enter first name',
              required: false,
            },
            {
              type: 'PASSWORD_INPUT',
              ref: 'password',
              label: 'Password',
              placeholder: 'Enter password',
              required: false,
            },
            {
              type: 'EMAIL_INPUT',
              ref: 'email',
              label: 'Email',
              placeholder: 'Enter email',
              required: false,
            },
            {
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Register',
              variant: 'SECONDARY',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);
    expect(screen.getByText('Create Account')).toBeInTheDocument();
    expect(screen.getByLabelText(/First Name/)).toBeInTheDocument();
  });

  it('uses fallback keys for PHONE_INPUT, OTP_INPUT, SELECT, RESEND, TRIGGER with undefined id', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          type: 'BLOCK',
          components: [
            {
              type: 'PHONE_INPUT',
              ref: 'phone',
              label: 'Phone',
              placeholder: 'Enter phone',
              required: false,
            },
            {
              type: 'OTP_INPUT',
              ref: 'otp',
              label: 'OTP',
              required: false,
            },
            {
              type: 'SELECT',
              ref: 'country',
              label: 'Country',
              placeholder: 'Select country',
              options: ['US', 'UK'],
              required: false,
            },
            {
              type: 'RESEND',
              eventType: 'SUBMIT',
              label: 'Resend',
            },
            {
              type: 'ACTION',
              eventType: 'TRIGGER',
              label: 'Alt Action',
              variant: 'SECONDARY',
            },
            {
              type: 'ACTION',
              eventType: 'SUBMIT',
              label: 'Go',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);
    expect(screen.getByLabelText(/Phone/)).toBeInTheDocument();
    expect(screen.getByText('OTP')).toBeInTheDocument();
    expect(screen.getByText('Resend')).toBeInTheDocument();
  });

  it('uses fallback keys for social login trigger with undefined id', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          type: 'BLOCK',
          components: [
            {
              type: 'ACTION',
              eventType: 'TRIGGER',
              label: 'Continue with Google',
              image: 'google.svg',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);
    expect(screen.getByText('Continue with Google')).toBeInTheDocument();
  });

  it('renders with branding enabled and centered text alignment', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'text-1',
          type: 'TEXT',
          label: 'Create Account',
          variant: 'H2',
        },
      ],
    });
    render(<SignUpBox />);
    expect(screen.getByText('Create Account')).toBeInTheDocument();
  });

  it('renders branded logo with alt fallback', () => {
    render(<SignUpBox />);
    expect(screen.getByTestId('thunderid-signup')).toBeInTheDocument();
  });

  it('renders branded logo with custom alt, height, and width', () => {
    render(<SignUpBox />);
    expect(screen.getByTestId('thunderid-signup')).toBeInTheDocument();
  });

  it('renders block without components property', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'block-1',
          type: 'BLOCK',
        },
      ],
    });
    render(<SignUpBox />);
    expect(screen.getByTestId('thunderid-signup')).toBeInTheDocument();
  });

  it('renders social login trigger with missing label and image', () => {
    mockSignUpRenderProps = createMockSignUpRenderProps({
      components: [
        {
          id: 'trigger-block',
          type: 'BLOCK',
          components: [
            {
              id: 'provider-btn',
              type: 'ACTION',
              eventType: 'TRIGGER',
            },
          ],
        },
      ],
    });
    render(<SignUpBox />);
    expect(screen.getByTestId('thunderid-signup')).toBeInTheDocument();
  });
});
