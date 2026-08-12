// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {IdentityProviderTypes} from '@thunderid/configure-connections';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import ExecutionExtendedProperties from '../ExecutionExtendedProperties';
import type {Resource} from '@/features/flows/models/resources';
import {ExecutionTypes, type StepData} from '@/features/flows/models/steps';

// Mock react-i18next
vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => {
      const translations: Record<string, string> = {
        'common:status.loading': 'Loading...',
        'flows:core.executions.smsOtp.description': 'Configure SMS OTP settings',
        'flows:core.executions.smsOtp.mode.label': 'Mode',
        'flows:core.executions.smsOtp.mode.placeholder': 'Select mode',
        'flows:core.executions.smsOtp.mode.send': 'Send SMS OTP',
        'flows:core.executions.smsOtp.mode.verify': 'Verify SMS OTP',
        'flows:core.executions.smsOtp.sender.label': 'Sender',
        'flows:core.executions.smsOtp.sender.placeholder': 'Select sender',
        'flows:core.executions.smsOtp.sender.required': 'Sender is required',
        'flows:core.executions.smsOtp.sender.noSenders': 'No SMS senders configured',
        'flows:core.executions.passkey.description': 'Configure Passkey settings',
        'flows:core.executions.passkey.mode.label': 'Mode',
        'flows:core.executions.passkey.mode.placeholder': 'Select mode',
        'flows:core.executions.passkey.mode.challenge': 'Passkey Challenge',
        'flows:core.executions.passkey.mode.verify': 'Passkey Verify',
        'flows:core.executions.passkey.mode.registerStart': 'Passkey Register Start',
        'flows:core.executions.passkey.mode.registerFinish': 'Passkey Register Finish',
        'flows:core.executions.passkey.relyingPartyId.label': 'Relying Party ID',
        'flows:core.executions.passkey.relyingPartyId.placeholder': 'Enter relying party ID',
        'flows:core.executions.passkey.relyingPartyId.hint': 'Relying party identifier hint',
        'flows:core.executions.passkey.relyingPartyName.label': 'Relying Party Name',
        'flows:core.executions.passkey.relyingPartyName.placeholder': 'Enter relying party name',
        'flows:core.executions.passkey.relyingPartyName.hint': 'Relying party name hint',
        'flows:core.executions.templateScenarios.userInvite': 'User Invite',
        'flows:core.executions.templateScenarios.magicLink': 'Magic Link',
        'flows:core.executions.templateScenarios.selfRegistration': 'Self Registration',
        'flows:core.executions.templateScenarios.otp': 'OTP Verification',
        'flows:core.executions.templateScenarios.passwordRecovery': 'Password Recovery',
        'flows:core.executions.templateScenarios.cibaNotification': 'CIBA Notification',
        'flows:core.executions.consent.description': 'Configure the consent executor settings.',
        'flows:core.executions.consent.timeout.label': 'Consent Timeout (seconds)',
        'flows:core.executions.consent.timeout.placeholder': '0',
        'flows:core.executions.consent.timeout.hint':
          'Time in seconds before the consent request expires. Use 0 for no timeout.',
        'flows:core.executions.federation.connection.description':
          'Select a connection from the following list to link it with the login flow.',
        'flows:core.executions.federation.connection.label': 'Connection',
        'flows:core.executions.federation.connection.placeholder': 'Select a connection',
        'flows:core.executions.federation.connection.required': 'Connection is required and must be selected.',
        'flows:core.executions.federation.connection.noConnections':
          'No connections available. Please create a connection to link with the login flow.',
        'flows:core.executions.identifying.description': 'Configure the identifying executor mode.',
        'flows:core.executions.identifying.mode.label': 'Mode',
        'flows:core.executions.identifying.mode.placeholder': 'Select a mode',
        'flows:core.executions.identifying.mode.identify': 'Identify',
        'flows:core.executions.identifying.mode.resolve': 'Resolve (Disambiguation)',
      };
      return translations[key] || key;
    },
  }),
}));

// Mock useValidationStatus
const mockSelectedNotification = {
  hasResourceFieldNotification: vi.fn(() => false),
  getResourceFieldNotification: vi.fn(() => ''),
};

vi.mock('@/features/flows/hooks/useValidationStatus', () => ({
  default: () => ({
    selectedNotification: mockSelectedNotification,
  }),
}));

// Mock useIdentityProviders + useSMSProviders
const mockIdentityProviders = vi.fn<() => {data: unknown[]; isLoading: boolean}>();
const mockSMSProviders = vi.fn<() => {data: unknown[]; isLoading: boolean}>();
vi.mock('@thunderid/configure-connections', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-connections')>()),
  useIdentityProviders: () => mockIdentityProviders(),
  useSMSProviders: () => mockSMSProviders(),
}));

describe('ExecutionExtendedProperties', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockIdentityProviders.mockReturnValue({
      data: [],
      isLoading: false,
    });
    mockSMSProviders.mockReturnValue({
      data: [],
      isLoading: false,
    });
  });

  describe('Google Federation Executor', () => {
    const googleResource = {
      id: 'google-executor-1',
      data: {
        action: {
          executor: {
            name: ExecutionTypes.GoogleFederation,
          },
        },
        properties: {
          idpId: '',
        },
      },
    } as unknown as Resource;

    it('should render connection selector for Google executor', () => {
      mockIdentityProviders.mockReturnValue({
        data: [{id: 'google-idp-1', name: 'Google IDP', type: IdentityProviderTypes.GOOGLE}],
        isLoading: false,
      });

      render(<ExecutionExtendedProperties resource={googleResource} onChange={mockOnChange} />);

      expect(screen.getByText('Connection')).toBeInTheDocument();
      // The verbose intro paragraph was removed to declutter the panel; the
      // label and placeholder carry the context.
      expect(
        screen.queryByText('Select a connection from the following list to link it with the login flow.'),
      ).not.toBeInTheDocument();
    });

    // The panel may only write properties the federated executors declare as supported in their
    // backend ExecutorMeta. Writing any other key makes the whole flow unsavable, and unchecking
    // the box does not recover it because the validator rejects on key presence, not value.
    it('should only write properties the federated executors support', () => {
      mockIdentityProviders.mockReturnValue({
        data: [{id: 'google-idp-1', name: 'Google IDP', type: IdentityProviderTypes.GOOGLE}],
        isLoading: false,
      });

      render(<ExecutionExtendedProperties resource={googleResource} onChange={mockOnChange} />);

      screen.getAllByRole('checkbox').forEach((checkbox) => fireEvent.click(checkbox));

      expect(new Set(mockOnChange.mock.calls.map((call) => call[0] as string))).toEqual(
        new Set([
          'data.properties.allowAuthenticationWithoutLocalUser',
          'data.properties.allowRegistrationWithExistingUser',
        ]),
      );
    });

    it('should show available Google connections in dropdown', async () => {
      const user = userEvent.setup();
      mockIdentityProviders.mockReturnValue({
        data: [
          {id: 'google-idp-1', name: 'My Google IDP', type: IdentityProviderTypes.GOOGLE},
          {id: 'google-idp-2', name: 'Another Google IDP', type: IdentityProviderTypes.GOOGLE},
        ],
        isLoading: false,
      });

      render(<ExecutionExtendedProperties resource={googleResource} onChange={mockOnChange} />);

      const select = screen.getByRole('combobox');
      await user.click(select);

      expect(screen.getByText('My Google IDP')).toBeInTheDocument();
      expect(screen.getByText('Another Google IDP')).toBeInTheDocument();
    });

    it('should call onChange when connection is selected', async () => {
      const user = userEvent.setup();
      mockIdentityProviders.mockReturnValue({
        data: [{id: 'google-idp-1', name: 'My Google IDP', type: IdentityProviderTypes.GOOGLE}],
        isLoading: false,
      });

      render(<ExecutionExtendedProperties resource={googleResource} onChange={mockOnChange} />);

      const select = screen.getByRole('combobox');
      await user.click(select);
      await user.click(screen.getByText('My Google IDP'));

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.idpId', 'google-idp-1', googleResource);
    });

    it('should show error when connection is placeholder', () => {
      mockIdentityProviders.mockReturnValue({
        data: [{id: 'google-idp-1', name: 'My Google IDP', type: IdentityProviderTypes.GOOGLE}],
        isLoading: false,
      });

      const resourceWithPlaceholder = {
        ...googleResource,
        data: {
          ...(googleResource as unknown as {data: object}).data,
          properties: {idpId: '{{IDP_ID}}'},
        },
      } as unknown as Resource;

      render(<ExecutionExtendedProperties resource={resourceWithPlaceholder} onChange={mockOnChange} />);

      expect(screen.getByText('Connection is required and must be selected.')).toBeInTheDocument();
    });

    it('should show validation error from notification', () => {
      mockSelectedNotification.hasResourceFieldNotification.mockReturnValue(true);
      mockSelectedNotification.getResourceFieldNotification.mockReturnValue('Custom validation error');

      mockIdentityProviders.mockReturnValue({
        data: [{id: 'google-idp-1', name: 'My Google IDP', type: IdentityProviderTypes.GOOGLE}],
        isLoading: false,
      });

      render(<ExecutionExtendedProperties resource={googleResource} onChange={mockOnChange} />);

      expect(screen.getByText('Custom validation error')).toBeInTheDocument();
    });

    it('should show warning when no connections are available', () => {
      mockIdentityProviders.mockReturnValue({
        data: [],
        isLoading: false,
      });

      render(<ExecutionExtendedProperties resource={googleResource} onChange={mockOnChange} />);

      expect(
        screen.getByText('No connections available. Please create a connection to link with the login flow.'),
      ).toBeInTheDocument();
    });

    it('should disable dropdown while loading', () => {
      mockIdentityProviders.mockReturnValue({
        data: [],
        isLoading: true,
      });

      render(<ExecutionExtendedProperties resource={googleResource} onChange={mockOnChange} />);

      const select = screen.getByRole('combobox');
      expect(select).toHaveAttribute('aria-disabled', 'true');
    });

    it('should show loading text in dropdown while loading', async () => {
      const user = userEvent.setup();
      mockIdentityProviders.mockReturnValue({
        data: [],
        isLoading: true,
      });

      render(<ExecutionExtendedProperties resource={googleResource} onChange={mockOnChange} />);

      const select = screen.getByRole('combobox');
      await user.click(select);

      expect(screen.getByText('Loading...')).toBeInTheDocument();
    });

    it('should show selected connection value', () => {
      mockIdentityProviders.mockReturnValue({
        data: [{id: 'google-idp-1', name: 'My Google IDP', type: IdentityProviderTypes.GOOGLE}],
        isLoading: false,
      });

      const resourceWithSelection = {
        ...googleResource,
        data: {
          ...(googleResource as unknown as {data: object}).data,
          properties: {idpId: 'google-idp-1'},
        },
      } as unknown as Resource;

      render(<ExecutionExtendedProperties resource={resourceWithSelection} onChange={mockOnChange} />);

      expect(screen.getByRole('combobox')).toHaveTextContent('My Google IDP');
    });
  });

  describe('GitHub Federation Executor', () => {
    const githubResource = {
      id: 'github-executor-1',
      data: {
        action: {
          executor: {
            name: ExecutionTypes.GithubFederation,
          },
        },
        properties: {
          idpId: '',
        },
      },
    } as unknown as Resource;

    it('should render connection selector for GitHub executor', () => {
      mockIdentityProviders.mockReturnValue({
        data: [{id: 'github-idp-1', name: 'GitHub IDP', type: IdentityProviderTypes.GITHUB}],
        isLoading: false,
      });

      render(<ExecutionExtendedProperties resource={githubResource} onChange={mockOnChange} />);

      expect(screen.getByText('Connection')).toBeInTheDocument();
    });

    it('should filter to only show GitHub connections', async () => {
      const user = userEvent.setup();
      mockIdentityProviders.mockReturnValue({
        data: [
          {id: 'google-idp-1', name: 'Google IDP', type: IdentityProviderTypes.GOOGLE},
          {id: 'github-idp-1', name: 'GitHub IDP', type: IdentityProviderTypes.GITHUB},
        ],
        isLoading: false,
      });

      render(<ExecutionExtendedProperties resource={githubResource} onChange={mockOnChange} />);

      const select = screen.getByRole('combobox');
      await user.click(select);

      expect(screen.getByText('GitHub IDP')).toBeInTheDocument();
      expect(screen.queryByText('Google IDP')).not.toBeInTheDocument();
    });
  });

  describe('Passkey Executor', () => {
    const passkeyResource = {
      id: 'passkey-executor-1',
      data: {
        action: {
          executor: {
            name: ExecutionTypes.PasskeyAuth,
            mode: '',
          },
        },
        display: {
          label: 'Passkey',
        },
      },
    } as unknown as Resource;

    it('should render Passkey configuration UI', () => {
      render(<ExecutionExtendedProperties resource={passkeyResource} onChange={mockOnChange} />);

      expect(screen.getByText('Configure Passkey settings')).toBeInTheDocument();
      expect(screen.getByText('Mode')).toBeInTheDocument();
    });

    it('should show mode options', async () => {
      const user = userEvent.setup();

      render(<ExecutionExtendedProperties resource={passkeyResource} onChange={mockOnChange} />);

      const modeSelect = screen.getByRole('combobox');
      await user.click(modeSelect);

      expect(screen.getByText('Passkey Challenge')).toBeInTheDocument();
      expect(screen.getByText('Passkey Verify')).toBeInTheDocument();
      expect(screen.getByText('Passkey Register Start')).toBeInTheDocument();
      expect(screen.getByText('Passkey Register Finish')).toBeInTheDocument();
    });

    it('should call onChange with updated data when mode is selected', async () => {
      const user = userEvent.setup();

      render(<ExecutionExtendedProperties resource={passkeyResource} onChange={mockOnChange} />);

      const modeSelect = screen.getByRole('combobox');
      await user.click(modeSelect);
      await user.click(screen.getByText('Passkey Challenge'));

      expect(mockOnChange).toHaveBeenCalledWith(
        'data',
        expect.objectContaining({
          action: expect.objectContaining({
            executor: expect.objectContaining({
              mode: 'challenge',
            }) as unknown,
          }) as unknown,
          display: expect.objectContaining({
            label: 'Request Passkey',
          }) as unknown,
        }),
        passkeyResource,
      );
    });

    it('should show selected mode value', () => {
      const resourceWithMode = {
        ...passkeyResource,
        data: {
          ...(passkeyResource as unknown as {data: object}).data,
          action: {
            executor: {
              name: ExecutionTypes.PasskeyAuth,
              mode: 'verify',
            },
          },
        },
      } as unknown as Resource;

      render(<ExecutionExtendedProperties resource={resourceWithMode} onChange={mockOnChange} />);

      const modeSelect = screen.getByRole('combobox');
      expect(modeSelect).toHaveTextContent('Passkey Verify');
    });

    it('should update display label when mode changes to verify', async () => {
      const user = userEvent.setup();

      render(<ExecutionExtendedProperties resource={passkeyResource} onChange={mockOnChange} />);

      const modeSelect = screen.getByRole('combobox');
      await user.click(modeSelect);
      await user.click(screen.getByText('Passkey Verify'));

      expect(mockOnChange).toHaveBeenCalledWith(
        'data',
        expect.objectContaining({
          display: expect.objectContaining({
            label: 'Verify Passkey',
          }) as unknown,
        }),
        passkeyResource,
      );
    });

    it('should preserve existing data properties when mode changes', async () => {
      const user = userEvent.setup();

      const resourceWithExistingData = {
        ...passkeyResource,
        data: {
          ...(passkeyResource as unknown as {data: object}).data,
          properties: {relyingPartyId: 'localhost', relyingPartyName: 'ThunderID'},
          display: {label: 'Old Label', icon: 'passkey-icon.png'},
        },
      } as unknown as Resource;

      render(<ExecutionExtendedProperties resource={resourceWithExistingData} onChange={mockOnChange} />);

      const modeSelect = screen.getByRole('combobox');
      await user.click(modeSelect);
      await user.click(screen.getByText('Passkey Challenge'));

      expect(mockOnChange).toHaveBeenCalledWith(
        'data',
        expect.objectContaining({
          properties: expect.objectContaining({
            relyingPartyId: 'localhost',
            relyingPartyName: 'ThunderID',
          }) as unknown,
          display: expect.objectContaining({
            label: 'Request Passkey',
            icon: 'passkey-icon.png',
          }) as unknown,
        }),
        resourceWithExistingData,
      );
    });

    it('should show relying party fields for challenge mode', () => {
      const resourceWithChallengeMode = {
        ...passkeyResource,
        data: {
          ...(passkeyResource as unknown as {data: object}).data,
          action: {
            executor: {
              name: ExecutionTypes.PasskeyAuth,
              mode: 'challenge',
            },
          },
        },
      } as unknown as Resource;

      render(<ExecutionExtendedProperties resource={resourceWithChallengeMode} onChange={mockOnChange} />);

      expect(screen.getByLabelText('Relying Party ID')).toBeInTheDocument();
      expect(screen.getByLabelText('Relying Party Name')).toBeInTheDocument();
    });

    it('should show relying party fields for register_start mode', () => {
      const resourceWithRegisterStartMode = {
        ...passkeyResource,
        data: {
          ...(passkeyResource as unknown as {data: object}).data,
          action: {
            executor: {
              name: ExecutionTypes.PasskeyAuth,
              mode: 'register_start',
            },
          },
        },
      } as unknown as Resource;

      render(<ExecutionExtendedProperties resource={resourceWithRegisterStartMode} onChange={mockOnChange} />);

      expect(screen.getByLabelText('Relying Party ID')).toBeInTheDocument();
      expect(screen.getByLabelText('Relying Party Name')).toBeInTheDocument();
    });

    it('should not show relying party fields for verify mode', () => {
      const resourceWithVerifyMode = {
        ...passkeyResource,
        data: {
          ...(passkeyResource as unknown as {data: object}).data,
          action: {
            executor: {
              name: ExecutionTypes.PasskeyAuth,
              mode: 'verify',
            },
          },
        },
      } as unknown as Resource;

      render(<ExecutionExtendedProperties resource={resourceWithVerifyMode} onChange={mockOnChange} />);

      expect(screen.queryByLabelText('Relying Party ID')).not.toBeInTheDocument();
      expect(screen.queryByLabelText('Relying Party Name')).not.toBeInTheDocument();
    });

    it('should call onChange for relying party fields', () => {
      const resourceWithChallengeMode = {
        ...passkeyResource,
        data: {
          ...(passkeyResource as unknown as {data: object}).data,
          action: {
            executor: {
              name: ExecutionTypes.PasskeyAuth,
              mode: 'challenge',
            },
          },
        },
      } as unknown as Resource;

      render(<ExecutionExtendedProperties resource={resourceWithChallengeMode} onChange={mockOnChange} />);

      const relyingPartyIdInput = screen.getByLabelText('Relying Party ID');
      const relyingPartyNameInput = screen.getByLabelText('Relying Party Name');

      fireEvent.change(relyingPartyIdInput, {target: {value: 'localhost'}});
      fireEvent.blur(relyingPartyIdInput);
      fireEvent.change(relyingPartyNameInput, {target: {value: 'ThunderID'}});
      fireEvent.blur(relyingPartyNameInput);

      expect(mockOnChange).toHaveBeenCalledWith(
        'data.properties.relyingPartyId',
        'localhost',
        resourceWithChallengeMode,
      );
      expect(mockOnChange).toHaveBeenCalledWith(
        'data.properties.relyingPartyName',
        'ThunderID',
        resourceWithChallengeMode,
      );
    });

    it('should update display label when mode changes to register_start', async () => {
      const user = userEvent.setup();

      render(<ExecutionExtendedProperties resource={passkeyResource} onChange={mockOnChange} />);

      const modeSelect = screen.getByRole('combobox');
      await user.click(modeSelect);
      await user.click(screen.getByText('Passkey Register Start'));

      expect(mockOnChange).toHaveBeenCalledWith(
        'data',
        expect.objectContaining({
          action: expect.objectContaining({
            executor: expect.objectContaining({
              mode: 'register_start',
            }) as unknown,
          }) as unknown,
          display: expect.objectContaining({
            label: 'Start Passkey Registration',
          }) as unknown,
        }),
        passkeyResource,
      );
    });

    it('should update display label when mode changes to register_finish', async () => {
      const user = userEvent.setup();

      render(<ExecutionExtendedProperties resource={passkeyResource} onChange={mockOnChange} />);

      const modeSelect = screen.getByRole('combobox');
      await user.click(modeSelect);
      await user.click(screen.getByText('Passkey Register Finish'));

      expect(mockOnChange).toHaveBeenCalledWith(
        'data',
        expect.objectContaining({
          action: expect.objectContaining({
            executor: expect.objectContaining({
              mode: 'register_finish',
            }) as unknown,
          }) as unknown,
          display: expect.objectContaining({
            label: 'Finish Passkey Registration',
          }) as unknown,
        }),
        passkeyResource,
      );
    });
  });

  describe('Consent Executor', () => {
    const consentResource = {
      id: 'consent-executor-1',
      data: {
        action: {
          executor: {
            name: ExecutionTypes.ConsentExecutor,
          },
        },
        properties: {},
      },
    } as unknown as Resource;

    it('should render timeout configuration for consent executor', () => {
      render(<ExecutionExtendedProperties resource={consentResource} onChange={mockOnChange} />);

      expect(screen.getByText('Configure the consent executor settings.')).toBeInTheDocument();
      expect(screen.getByLabelText('Consent Timeout (seconds)')).toBeInTheDocument();
      expect(
        screen.getByText('Time in seconds before the consent request expires. Use 0 for no timeout.'),
      ).toBeInTheDocument();
    });

    it('should default timeout to 0 when value is not set', () => {
      render(<ExecutionExtendedProperties resource={consentResource} onChange={mockOnChange} />);

      expect(screen.getByLabelText('Consent Timeout (seconds)')).toHaveValue(0);
    });

    it('should commit the timeout on blur', () => {
      const consentResourceWithTimeout = {
        ...consentResource,
        data: {
          ...(consentResource as unknown as {data: object}).data,
          properties: {
            timeout: '20',
          },
        },
      } as unknown as Resource;

      render(<ExecutionExtendedProperties resource={consentResourceWithTimeout} onChange={mockOnChange} />);

      const timeoutInput = screen.getByLabelText('Consent Timeout (seconds)');
      fireEvent.change(timeoutInput, {
        target: {value: '45'},
      });
      fireEvent.blur(timeoutInput);

      expect(mockOnChange).toHaveBeenLastCalledWith('data.properties.timeout', '45', consentResourceWithTimeout);
    });

    it('should not commit the timeout while typing', () => {
      render(<ExecutionExtendedProperties resource={consentResource} onChange={mockOnChange} />);

      const timeoutInput = screen.getByLabelText('Consent Timeout (seconds)');
      fireEvent.change(timeoutInput, {target: {value: '45'}});

      expect(mockOnChange).not.toHaveBeenCalled();
      expect(timeoutInput).toHaveValue(45);
    });

    it('should normalize empty timeout to 0 on blur', () => {
      const consentResourceWithTimeout = {
        ...consentResource,
        data: {
          ...(consentResource as unknown as {data: object}).data,
          properties: {timeout: '20'},
        },
      } as unknown as Resource;

      render(<ExecutionExtendedProperties resource={consentResourceWithTimeout} onChange={mockOnChange} />);

      const timeoutInput = screen.getByLabelText('Consent Timeout (seconds)');
      fireEvent.change(timeoutInput, {target: {value: ''}});
      fireEvent.blur(timeoutInput);

      expect(mockOnChange).toHaveBeenLastCalledWith('data.properties.timeout', '0', consentResourceWithTimeout);
    });

    it('should clamp negative timeout to 0 on blur', () => {
      const consentResourceWithTimeout = {
        ...consentResource,
        data: {
          ...(consentResource as unknown as {data: object}).data,
          properties: {timeout: '20'},
        },
      } as unknown as Resource;

      render(<ExecutionExtendedProperties resource={consentResourceWithTimeout} onChange={mockOnChange} />);

      const timeoutInput = screen.getByLabelText('Consent Timeout (seconds)');
      fireEvent.change(timeoutInput, {target: {value: '-5'}});
      fireEvent.blur(timeoutInput);

      expect(mockOnChange).toHaveBeenLastCalledWith('data.properties.timeout', '0', consentResourceWithTimeout);
    });

    it('should floor decimal timeout to integer on blur', () => {
      render(<ExecutionExtendedProperties resource={consentResource} onChange={mockOnChange} />);

      const timeoutInput = screen.getByLabelText('Consent Timeout (seconds)');
      fireEvent.change(timeoutInput, {target: {value: '3.7'}});
      fireEvent.blur(timeoutInput);

      expect(mockOnChange).toHaveBeenLastCalledWith('data.properties.timeout', '3', consentResource);
    });

    it('should commit the timeout on Enter', () => {
      render(<ExecutionExtendedProperties resource={consentResource} onChange={mockOnChange} />);

      const timeoutInput = screen.getByLabelText('Consent Timeout (seconds)');
      fireEvent.change(timeoutInput, {target: {value: '15'}});
      fireEvent.keyDown(timeoutInput, {key: 'Enter'});

      expect(mockOnChange).toHaveBeenLastCalledWith('data.properties.timeout', '15', consentResource);
    });
  });

  describe('OTP Executor', () => {
    const otpResource = {
      id: 'otp-executor-1',
      data: {
        action: {
          executor: {
            name: ExecutionTypes.OTPExecutor,
          },
        },
        properties: {
          otpLength: 6,
          otpValidityPeriodSeconds: 120,
        },
      },
    } as unknown as Resource;

    // The executor resolves these through a numeric conversion that rejects strings, so
    // committing them as text would leave the configured value ignored at runtime.
    it('should commit otpLength as a number', () => {
      render(<ExecutionExtendedProperties resource={otpResource} onChange={mockOnChange} />);

      const otpLengthInput = screen.getByLabelText('flows:core.executions.otp.otpLength.label');
      fireEvent.change(otpLengthInput, {target: {value: '8'}});
      fireEvent.blur(otpLengthInput);

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.otpLength', 8, otpResource);
    });

    it('should commit otpValidityPeriodSeconds as a number', () => {
      render(<ExecutionExtendedProperties resource={otpResource} onChange={mockOnChange} />);

      const validityInput = screen.getByLabelText('flows:core.executions.otp.otpValidityPeriodSeconds.label');
      fireEvent.change(validityInput, {target: {value: '300'}});
      fireEvent.blur(validityInput);

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.otpValidityPeriodSeconds', 300, otpResource);
    });

    it('should commit maxAttempts as a number', () => {
      render(<ExecutionExtendedProperties resource={otpResource} onChange={mockOnChange} />);

      const maxAttemptsInput = screen.getByLabelText('flows:core.executions.otp.maxAttempts.label');
      fireEvent.change(maxAttemptsInput, {target: {value: '5'}});
      fireEvent.blur(maxAttemptsInput);

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.maxAttempts', 5, otpResource);
    });

    it('should not commit otpLength while typing', () => {
      render(<ExecutionExtendedProperties resource={otpResource} onChange={mockOnChange} />);

      const otpLengthInput = screen.getByLabelText('flows:core.executions.otp.otpLength.label');
      fireEvent.change(otpLengthInput, {target: {value: '8'}});

      expect(mockOnChange).not.toHaveBeenCalled();
      expect(otpLengthInput).toHaveValue(8);
    });

    it('should clamp otpLength to the supported range on blur', () => {
      render(<ExecutionExtendedProperties resource={otpResource} onChange={mockOnChange} />);

      const otpLengthInput = screen.getByLabelText('flows:core.executions.otp.otpLength.label');
      fireEvent.change(otpLengthInput, {target: {value: '25'}});
      fireEvent.blur(otpLengthInput);

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.otpLength', 10, otpResource);
    });

    it('should default the numeric-only checkbox to checked when the property is unset', () => {
      render(<ExecutionExtendedProperties resource={otpResource} onChange={mockOnChange} />);

      expect(screen.getByRole('checkbox')).toBeChecked();
    });

    it('should render the numeric-only checkbox unchecked when explicitly set to false', () => {
      const alphanumericOtpResource = {
        ...otpResource,
        data: {
          ...(otpResource as unknown as {data: object}).data,
          properties: {otpLength: 6, otpValidityPeriodSeconds: 120, otpUseNumericOnly: false},
        },
      } as unknown as Resource;

      render(<ExecutionExtendedProperties resource={alphanumericOtpResource} onChange={mockOnChange} />);

      expect(screen.getByRole('checkbox')).not.toBeChecked();
    });

    it('should commit false immediately when the default-checked numeric-only box is unchecked', () => {
      render(<ExecutionExtendedProperties resource={otpResource} onChange={mockOnChange} />);

      fireEvent.click(screen.getByRole('checkbox'));

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.otpUseNumericOnly', false, otpResource);
    });
  });

  describe('Email Executor', () => {
    const emailResource = {
      id: 'email-executor-1',
      data: {
        action: {
          executor: {
            name: ExecutionTypes.EmailExecutor,
            mode: 'send',
          },
        },
        properties: {
          emailTemplate: '',
        },
      },
    } as unknown as Resource;

    it('should render email template configuration', () => {
      render(<ExecutionExtendedProperties resource={emailResource} onChange={mockOnChange} />);

      expect(screen.getByText('flows:core.executions.email.description')).toBeInTheDocument();
      expect(screen.getByLabelText('flows:core.executions.email.emailTemplate.label')).toBeInTheDocument();
    });

    it('should offer the supported template scenarios with readable labels', async () => {
      render(<ExecutionExtendedProperties resource={emailResource} onChange={mockOnChange} />);

      await userEvent.click(screen.getByLabelText('flows:core.executions.email.emailTemplate.label'));

      expect(screen.getByRole('option', {name: 'OTP Verification'})).toBeInTheDocument();
      expect(screen.getByRole('option', {name: 'User Invite'})).toBeInTheDocument();
      expect(screen.getByRole('option', {name: 'Password Recovery'})).toBeInTheDocument();
    });

    it('should commit the raw scenario value for the selected label', async () => {
      render(<ExecutionExtendedProperties resource={emailResource} onChange={mockOnChange} />);

      await userEvent.click(screen.getByLabelText('flows:core.executions.email.emailTemplate.label'));
      await userEvent.click(screen.getByRole('option', {name: 'Magic Link'}));

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.emailTemplate', 'MAGIC_LINK', emailResource);
    });

    it('should find a scenario by searching its readable label', async () => {
      render(<ExecutionExtendedProperties resource={emailResource} onChange={mockOnChange} />);

      await userEvent.type(screen.getByLabelText('flows:core.executions.email.emailTemplate.label'), 'recov');

      expect(screen.getByRole('option', {name: 'Password Recovery'})).toBeInTheDocument();
      expect(screen.queryByRole('option', {name: 'User Invite'})).not.toBeInTheDocument();
    });

    it('should display the label for the existing email template value', () => {
      const resourceWithTemplate = {
        ...emailResource,
        data: {
          ...(emailResource as unknown as {data: object}).data,
          properties: {emailTemplate: 'PASSWORD_RECOVERY'},
        },
      } as unknown as Resource;

      render(<ExecutionExtendedProperties resource={resourceWithTemplate} onChange={mockOnChange} />);

      expect(screen.getByLabelText('flows:core.executions.email.emailTemplate.label')).toHaveValue('Password Recovery');
    });

    it('should preserve a template scenario it does not know about', async () => {
      const resourceWithUnknownTemplate = {
        ...emailResource,
        data: {
          ...(emailResource as unknown as {data: object}).data,
          properties: {emailTemplate: 'CUSTOM_SCENARIO'},
        },
      } as unknown as Resource;

      render(<ExecutionExtendedProperties resource={resourceWithUnknownTemplate} onChange={mockOnChange} />);

      const input = screen.getByLabelText('flows:core.executions.email.emailTemplate.label');
      expect(input).toHaveValue('CUSTOM_SCENARIO');

      await userEvent.click(input);
      expect(screen.getByRole('option', {name: 'CUSTOM_SCENARIO'})).toBeInTheDocument();
    });
  });

  describe('SMS Executor', () => {
    const smsResource = {
      id: 'sms-executor-1',
      data: {
        action: {
          executor: {
            name: ExecutionTypes.SMSExecutor,
            mode: 'send',
          },
        },
        properties: {
          smsTemplate: '',
          senderId: '',
        },
      },
    } as unknown as Resource;

    it('should render SMS template and sender configuration', () => {
      mockSMSProviders.mockReturnValue({
        data: [{id: 'sender-1', name: 'Twilio'}],
        isLoading: false,
      });

      render(<ExecutionExtendedProperties resource={smsResource} onChange={mockOnChange} />);

      expect(screen.getByText('flows:core.executions.sms.description')).toBeInTheDocument();
      expect(screen.getByLabelText('flows:core.executions.sms.smsTemplate.label')).toBeInTheDocument();
      expect(screen.getByText('Sender')).toBeInTheDocument();
    });

    it('should commit the selected SMS template immediately', async () => {
      mockSMSProviders.mockReturnValue({
        data: [],
        isLoading: false,
      });

      render(<ExecutionExtendedProperties resource={smsResource} onChange={mockOnChange} />);

      await userEvent.click(screen.getByLabelText('flows:core.executions.sms.smsTemplate.label'));
      await userEvent.click(screen.getByRole('option', {name: 'OTP Verification'}));

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.smsTemplate', 'OTP', smsResource);
    });

    it('should show warning when no senders are available', () => {
      mockSMSProviders.mockReturnValue({
        data: [],
        isLoading: false,
      });

      render(<ExecutionExtendedProperties resource={smsResource} onChange={mockOnChange} />);

      expect(screen.getByText('No SMS senders configured')).toBeInTheDocument();
    });
  });

  describe('OU Resolver Executor', () => {
    const ouResolverResource = {
      id: 'ou-resolver-1',
      data: {
        action: {
          executor: {
            name: ExecutionTypes.OUResolverExecutor,
          },
        },
        properties: {
          resolveFrom: 'caller',
        },
      },
    } as unknown as Resource;

    it('should render OU resolver configuration', () => {
      render(<ExecutionExtendedProperties resource={ouResolverResource} onChange={mockOnChange} />);

      expect(screen.getByText('flows:core.executions.ouResolver.description')).toBeInTheDocument();
      expect(screen.getByText('flows:core.executions.ouResolver.resolveFrom.label')).toBeInTheDocument();
    });

    it('should show resolve from options', async () => {
      const user = userEvent.setup();

      render(<ExecutionExtendedProperties resource={ouResolverResource} onChange={mockOnChange} />);

      const select = screen.getByRole('combobox');
      await user.click(select);

      // 'caller' appears twice: once in the selected trigger and once in the dropdown menu item
      expect(screen.getAllByText('flows:core.executions.ouResolver.resolveFrom.caller')).toHaveLength(2);
      expect(screen.getByText('flows:core.executions.ouResolver.resolveFrom.prompt')).toBeInTheDocument();
      expect(screen.getByText('flows:core.executions.ouResolver.resolveFrom.promptAll')).toBeInTheDocument();
    });

    it('should call onChange when resolve from is changed', async () => {
      const user = userEvent.setup();

      render(<ExecutionExtendedProperties resource={ouResolverResource} onChange={mockOnChange} />);

      const select = screen.getByRole('combobox');
      await user.click(select);
      await user.click(screen.getByText('flows:core.executions.ouResolver.resolveFrom.prompt'));

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.resolveFrom', 'prompt', ouResolverResource);
    });
  });

  describe('Invite Executor', () => {
    const inviteResource = {
      id: 'invite-executor-1',
      data: {
        action: {
          executor: {
            name: ExecutionTypes.InviteExecutor,
            mode: '',
          },
        },
        display: {
          label: 'Invite',
        },
      },
    } as unknown as Resource;

    it('should render invite mode configuration', () => {
      render(<ExecutionExtendedProperties resource={inviteResource} onChange={mockOnChange} />);

      expect(screen.getByText('flows:core.executions.invite.description')).toBeInTheDocument();
      expect(screen.getByText('flows:core.executions.invite.mode.label')).toBeInTheDocument();
    });

    it('should show mode options', async () => {
      const user = userEvent.setup();

      render(<ExecutionExtendedProperties resource={inviteResource} onChange={mockOnChange} />);

      const select = screen.getByRole('combobox');
      await user.click(select);

      expect(screen.getByText('flows:core.executions.invite.mode.generate')).toBeInTheDocument();
      expect(screen.getByText('flows:core.executions.invite.mode.verify')).toBeInTheDocument();
    });

    it('should call onChange with updated data when mode is selected', async () => {
      const user = userEvent.setup();

      render(<ExecutionExtendedProperties resource={inviteResource} onChange={mockOnChange} />);

      const select = screen.getByRole('combobox');
      await user.click(select);
      await user.click(screen.getByText('flows:core.executions.invite.mode.generate'));

      expect(mockOnChange).toHaveBeenCalledWith(
        'data',
        expect.objectContaining({
          action: expect.objectContaining({
            executor: expect.objectContaining({
              mode: 'generate',
            }) as unknown,
          }) as unknown,
          display: expect.objectContaining({
            label: 'Generate Invite',
          }) as unknown,
        }),
        inviteResource,
      );
    });
  });

  describe('Permission Validator Executor', () => {
    const permissionResource = {
      id: 'permission-validator-1',
      data: {
        action: {
          executor: {
            name: ExecutionTypes.PermissionValidator,
          },
        },
        properties: {
          requiredScopes: [],
        },
      },
    } as unknown as Resource;

    it('should render permission validator configuration', () => {
      render(<ExecutionExtendedProperties resource={permissionResource} onChange={mockOnChange} />);

      expect(screen.getByText('flows:core.executions.permissionValidator.description')).toBeInTheDocument();
      expect(
        screen.getByLabelText('flows:core.executions.permissionValidator.requiredScopes.label'),
      ).toBeInTheDocument();
    });

    it('should commit scopes on blur', () => {
      render(<ExecutionExtendedProperties resource={permissionResource} onChange={mockOnChange} />);

      const input = screen.getByLabelText('flows:core.executions.permissionValidator.requiredScopes.label');
      fireEvent.change(input, {target: {value: 'read, write'}});
      fireEvent.blur(input);

      expect(mockOnChange).toHaveBeenCalledWith(
        'data.properties.requiredScopes',
        ['read', 'write'],
        permissionResource,
      );
    });

    it('should display existing scopes as comma-separated string', () => {
      const resourceWithScopes = {
        ...permissionResource,
        data: {
          ...(permissionResource as unknown as {data: object}).data,
          properties: {requiredScopes: ['openid', 'profile']},
        },
      } as unknown as Resource;

      render(<ExecutionExtendedProperties resource={resourceWithScopes} onChange={mockOnChange} />);

      expect(screen.getByLabelText('flows:core.executions.permissionValidator.requiredScopes.label')).toHaveValue(
        'openid, profile',
      );
    });
  });

  describe('Provisioning Executor', () => {
    const provisioningResource = {
      id: 'provisioning-executor-1',
      data: {
        action: {
          executor: {
            name: ExecutionTypes.ProvisioningExecutor,
          },
        },
        properties: {
          allowCrossOUProvisioning: false,
          includeOptional: false,
          includeOptionalCredentials: false,
          maxPerPrompt: 5,
          assignGroup: '',
          assignRole: '',
        },
      },
    } as unknown as Resource;
    const provisioningStepData = provisioningResource.data as StepData;

    it('should render provisioning configuration', () => {
      render(<ExecutionExtendedProperties resource={provisioningResource} onChange={mockOnChange} />);

      expect(screen.getByText('flows:core.executions.provisioning.description')).toBeInTheDocument();
      expect(screen.getByText('flows:core.executions.federation.allowCrossOUProvisioning.label')).toBeInTheDocument();
      expect(screen.getByText('flows:core.executions.provisioning.includeOptional.label')).toBeInTheDocument();
      expect(
        screen.getByText('flows:core.executions.provisioning.includeOptionalCredentials.label'),
      ).toBeInTheDocument();
      expect(screen.getByLabelText('flows:core.executions.provisioning.maxPerPrompt.label')).toBeInTheDocument();
      expect(screen.getByLabelText('flows:core.executions.provisioning.assignGroup.label')).toBeInTheDocument();
      expect(screen.getByLabelText('flows:core.executions.provisioning.assignRole.label')).toBeInTheDocument();
    });

    it('should call onChange without debounce when allowCrossOUProvisioning checkbox is toggled', () => {
      render(<ExecutionExtendedProperties resource={provisioningResource} onChange={mockOnChange} />);

      const checkboxes = screen.getAllByRole('checkbox');
      const allowCrossOUCheckbox = checkboxes[0];
      fireEvent.click(allowCrossOUCheckbox);

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.allowCrossOUProvisioning', true, provisioningResource);
    });

    it('should call onChange without debounce when includeOptional checkbox is toggled', () => {
      render(<ExecutionExtendedProperties resource={provisioningResource} onChange={mockOnChange} />);

      const checkboxes = screen.getAllByRole('checkbox');
      const includeOptionalCheckbox = checkboxes[1];
      fireEvent.click(includeOptionalCheckbox);

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.includeOptional', true, provisioningResource);
    });

    it('should call onChange without debounce when includeOptionalCredentials checkbox is toggled', () => {
      render(<ExecutionExtendedProperties resource={provisioningResource} onChange={mockOnChange} />);

      const checkboxes = screen.getAllByRole('checkbox');
      const includeOptionalCredentialsCheckbox = checkboxes[2];
      fireEvent.click(includeOptionalCredentialsCheckbox);

      expect(mockOnChange).toHaveBeenCalledWith(
        'data.properties.includeOptionalCredentials',
        true,
        provisioningResource,
      );
    });

    it('should commit maxPerPrompt as a number on blur', () => {
      render(<ExecutionExtendedProperties resource={provisioningResource} onChange={mockOnChange} />);

      const maxPerPromptInput = screen.getByLabelText('flows:core.executions.provisioning.maxPerPrompt.label');
      fireEvent.change(maxPerPromptInput, {target: {value: '3'}});
      fireEvent.blur(maxPerPromptInput);

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.maxPerPrompt', 3, provisioningResource);
    });

    it('should fall back to 0 for malformed persisted maxPerPrompt values', () => {
      const malformedProvisioningResource = {
        ...provisioningResource,
        data: {
          ...provisioningStepData,
          properties: {
            ...provisioningStepData.properties!,
            maxPerPrompt: 'invalid',
          },
        },
      } as unknown as Resource;

      render(<ExecutionExtendedProperties resource={malformedProvisioningResource} onChange={mockOnChange} />);

      expect(screen.getByLabelText('flows:core.executions.provisioning.maxPerPrompt.label')).toHaveValue(0);
    });

    it('should fall back to 0 for non-finite persisted maxPerPrompt values', () => {
      const malformedProvisioningResource = {
        ...provisioningResource,
        data: {
          ...provisioningStepData,
          properties: {
            ...provisioningStepData.properties!,
            maxPerPrompt: 'Infinity',
          },
        },
      } as unknown as Resource;

      render(<ExecutionExtendedProperties resource={malformedProvisioningResource} onChange={mockOnChange} />);

      expect(screen.getByLabelText('flows:core.executions.provisioning.maxPerPrompt.label')).toHaveValue(0);
    });

    it('should commit assignGroup on blur', () => {
      render(<ExecutionExtendedProperties resource={provisioningResource} onChange={mockOnChange} />);

      const assignGroupInput = screen.getByLabelText('flows:core.executions.provisioning.assignGroup.label');
      fireEvent.change(assignGroupInput, {target: {value: 'admin-group'}});
      fireEvent.blur(assignGroupInput);

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.assignGroup', 'admin-group', provisioningResource);
    });

    it('should commit assignRole on blur', () => {
      render(<ExecutionExtendedProperties resource={provisioningResource} onChange={mockOnChange} />);

      const assignRoleInput = screen.getByLabelText('flows:core.executions.provisioning.assignRole.label');
      fireEvent.change(assignRoleInput, {target: {value: 'editor-role'}});
      fireEvent.blur(assignRoleInput);

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.assignRole', 'editor-role', provisioningResource);
    });
  });

  describe('Session Sign Out Executor', () => {
    const signOutResource = {
      id: 'session-sign-out-executor-1',
      data: {
        action: {
          executor: {
            name: ExecutionTypes.SessionSignOut,
          },
        },
        properties: {
          promptOnSignOut: false,
        },
      },
    } as unknown as Resource;

    it('should render session sign out configuration', () => {
      render(<ExecutionExtendedProperties resource={signOutResource} onChange={mockOnChange} />);

      expect(screen.getByText('flows:core.executions.sessionSignOut.description')).toBeInTheDocument();
      expect(screen.getByText('flows:core.executions.sessionSignOut.promptOnSignOut.label')).toBeInTheDocument();
    });

    it('should call onChange without debounce when promptOnSignOut checkbox is toggled', () => {
      render(<ExecutionExtendedProperties resource={signOutResource} onChange={mockOnChange} />);

      const checkbox = screen.getAllByRole('checkbox')[0];
      fireEvent.click(checkbox);

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.promptOnSignOut', true, signOutResource);
    });
  });

  describe('OU Executor', () => {
    const ouResource = {
      id: 'ou-executor-1',
      data: {
        action: {
          executor: {
            name: ExecutionTypes.OUExecutor,
          },
        },
        properties: {
          parentOuId: '',
        },
      },
    } as unknown as Resource;

    it('should render OU executor configuration', () => {
      render(<ExecutionExtendedProperties resource={ouResource} onChange={mockOnChange} />);

      expect(screen.getByText('flows:core.executions.ouExecutor.description')).toBeInTheDocument();
      expect(screen.getByLabelText('flows:core.executions.ouExecutor.parentOuId.label')).toBeInTheDocument();
    });

    it('should commit parentOuId on blur', () => {
      render(<ExecutionExtendedProperties resource={ouResource} onChange={mockOnChange} />);

      const parentOuIdInput = screen.getByLabelText('flows:core.executions.ouExecutor.parentOuId.label');
      fireEvent.change(parentOuIdInput, {target: {value: 'ou-123'}});
      fireEvent.blur(parentOuIdInput);

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.parentOuId', 'ou-123', ouResource);
    });

    it('should display existing parentOuId value', () => {
      const resourceWithOuId = {
        ...ouResource,
        data: {
          ...(ouResource as unknown as {data: object}).data,
          properties: {parentOuId: 'existing-ou'},
        },
      } as unknown as Resource;

      render(<ExecutionExtendedProperties resource={resourceWithOuId} onChange={mockOnChange} />);

      expect(screen.getByLabelText('flows:core.executions.ouExecutor.parentOuId.label')).toHaveValue('existing-ou');
    });
  });

  describe('User Type Resolver Executor', () => {
    const userTypeResource = {
      id: 'user-type-resolver-1',
      data: {
        action: {
          executor: {
            name: ExecutionTypes.UserTypeResolver,
          },
        },
        properties: {
          allowedUserTypes: [],
        },
      },
    } as unknown as Resource;

    it('should render user type resolver configuration', () => {
      render(<ExecutionExtendedProperties resource={userTypeResource} onChange={mockOnChange} />);

      expect(screen.getByText('flows:core.executions.userTypeResolver.description')).toBeInTheDocument();
      expect(
        screen.getByLabelText('flows:core.executions.userTypeResolver.allowedUserTypes.label'),
      ).toBeInTheDocument();
    });

    it('should commit allowed user types on blur', () => {
      render(<ExecutionExtendedProperties resource={userTypeResource} onChange={mockOnChange} />);

      const input = screen.getByLabelText('flows:core.executions.userTypeResolver.allowedUserTypes.label');
      fireEvent.change(input, {target: {value: 'admin, employee'}});
      fireEvent.blur(input);

      expect(mockOnChange).toHaveBeenCalledWith(
        'data.properties.allowedUserTypes',
        ['admin', 'employee'],
        userTypeResource,
      );
    });

    it('should display existing user types as comma-separated string', () => {
      const resourceWithTypes = {
        ...userTypeResource,
        data: {
          ...(userTypeResource as unknown as {data: object}).data,
          properties: {allowedUserTypes: ['customer', 'partner']},
        },
      } as unknown as Resource;

      render(<ExecutionExtendedProperties resource={resourceWithTypes} onChange={mockOnChange} />);

      expect(screen.getByLabelText('flows:core.executions.userTypeResolver.allowedUserTypes.label')).toHaveValue(
        'customer, partner',
      );
    });
  });

  describe('HTTP Request Executor', () => {
    const httpResource = {
      id: 'http-executor-1',
      data: {
        action: {
          executor: {
            name: ExecutionTypes.HTTPRequestExecutor,
          },
        },
        properties: {
          url: '',
          method: 'GET',
          headers: {},
          body: {},
          timeout: 10,
          responseMapping: {},
          errorHandling: {
            failOnError: false,
            retryCount: 0,
            retryDelay: 0,
          },
        },
      },
    } as unknown as Resource;

    it('should render HTTP request configuration', () => {
      render(<ExecutionExtendedProperties resource={httpResource} onChange={mockOnChange} />);

      expect(screen.getByText('flows:core.executions.httpRequest.description')).toBeInTheDocument();
      expect(screen.getByLabelText('flows:core.executions.httpRequest.url.label')).toBeInTheDocument();
      expect(screen.getByText('flows:core.executions.httpRequest.method.label')).toBeInTheDocument();
      expect(screen.getByText('flows:core.executions.httpRequest.headers.label')).toBeInTheDocument();
      expect(screen.getByLabelText('flows:core.executions.httpRequest.body.label')).toBeInTheDocument();
      expect(screen.getByLabelText('flows:core.executions.httpRequest.timeout.label')).toBeInTheDocument();
    });

    it('should commit the URL on blur', () => {
      render(<ExecutionExtendedProperties resource={httpResource} onChange={mockOnChange} />);

      const urlInput = screen.getByLabelText('flows:core.executions.httpRequest.url.label');
      fireEvent.change(urlInput, {target: {value: 'https://api.example.com'}});
      fireEvent.blur(urlInput);

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.url', 'https://api.example.com', httpResource);
    });

    it('should not commit the URL while typing', () => {
      render(<ExecutionExtendedProperties resource={httpResource} onChange={mockOnChange} />);

      const urlInput = screen.getByLabelText('flows:core.executions.httpRequest.url.label');
      fireEvent.change(urlInput, {target: {value: 'https://api.example.com'}});

      expect(mockOnChange).not.toHaveBeenCalled();
      expect(urlInput).toHaveValue('https://api.example.com');
    });

    it('should call onChange without debounce when method changes', async () => {
      const user = userEvent.setup();

      render(<ExecutionExtendedProperties resource={httpResource} onChange={mockOnChange} />);

      const select = screen.getByRole('combobox');
      await user.click(select);
      await user.click(screen.getByText('POST'));

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.method', 'POST', httpResource);
    });

    it('should call onChange when failOnError checkbox is toggled', () => {
      render(<ExecutionExtendedProperties resource={httpResource} onChange={mockOnChange} />);

      const checkbox = screen.getByRole('checkbox');
      fireEvent.click(checkbox);

      expect(mockOnChange).toHaveBeenCalledWith(
        'data.properties.errorHandling',
        expect.objectContaining({failOnError: true}),
        httpResource,
      );
    });

    it('should commit the timeout on blur', () => {
      render(<ExecutionExtendedProperties resource={httpResource} onChange={mockOnChange} />);

      const timeoutInput = screen.getByLabelText('flows:core.executions.httpRequest.timeout.label');
      fireEvent.change(timeoutInput, {target: {value: '15'}});
      fireEvent.blur(timeoutInput);

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.timeout', 15, httpResource);
    });

    it('should clamp timeout to max 20 on blur', () => {
      render(<ExecutionExtendedProperties resource={httpResource} onChange={mockOnChange} />);

      const timeoutInput = screen.getByLabelText('flows:core.executions.httpRequest.timeout.label');
      fireEvent.change(timeoutInput, {target: {value: '99'}});
      fireEvent.blur(timeoutInput);

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.timeout', 20, httpResource);
      expect(timeoutInput).toHaveValue(20);
    });

    it('should commit a raw body on blur', () => {
      render(<ExecutionExtendedProperties resource={httpResource} onChange={mockOnChange} />);

      const bodyInput = screen.getByLabelText('flows:core.executions.httpRequest.body.label');
      fireEvent.change(bodyInput, {target: {value: 'raw body text'}});
      fireEvent.blur(bodyInput);

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.body', 'raw body text', httpResource);
    });

    it('should parse valid JSON body on blur', () => {
      render(<ExecutionExtendedProperties resource={httpResource} onChange={mockOnChange} />);

      const bodyInput = screen.getByLabelText('flows:core.executions.httpRequest.body.label');
      fireEvent.change(bodyInput, {target: {value: '{"key":"value"}'}});
      fireEvent.blur(bodyInput);

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.body', {key: 'value'}, httpResource);
    });

    it('should not store a half-typed body while typing JSON', () => {
      render(<ExecutionExtendedProperties resource={httpResource} onChange={mockOnChange} />);

      const bodyInput = screen.getByLabelText('flows:core.executions.httpRequest.body.label');
      fireEvent.change(bodyInput, {target: {value: '{"key"'}});
      fireEvent.change(bodyInput, {target: {value: '{"key":"value"}'}});

      expect(mockOnChange).not.toHaveBeenCalled();
    });

    it('should keep Enter as a newline in the multiline body', () => {
      render(<ExecutionExtendedProperties resource={httpResource} onChange={mockOnChange} />);

      const bodyInput = screen.getByLabelText('flows:core.executions.httpRequest.body.label');
      fireEvent.change(bodyInput, {target: {value: 'line one'}});
      fireEvent.keyDown(bodyInput, {key: 'Enter'});

      expect(mockOnChange).not.toHaveBeenCalled();
    });

    it('should commit retryCount on blur', () => {
      render(<ExecutionExtendedProperties resource={httpResource} onChange={mockOnChange} />);

      const retryCountInput = screen.getByLabelText('flows:core.executions.httpRequest.errorHandling.retryCount.label');
      fireEvent.change(retryCountInput, {target: {value: '3'}});
      fireEvent.blur(retryCountInput);

      expect(mockOnChange).toHaveBeenCalledWith(
        'data.properties.errorHandling',
        expect.objectContaining({retryCount: 3}),
        httpResource,
      );
    });

    it('should commit retryDelay on blur', () => {
      render(<ExecutionExtendedProperties resource={httpResource} onChange={mockOnChange} />);

      const retryDelayInput = screen.getByLabelText('flows:core.executions.httpRequest.errorHandling.retryDelay.label');
      fireEvent.change(retryDelayInput, {target: {value: '1000'}});
      fireEvent.blur(retryDelayInput);

      expect(mockOnChange).toHaveBeenCalledWith(
        'data.properties.errorHandling',
        expect.objectContaining({retryDelay: 1000}),
        httpResource,
      );
    });
  });

  describe('Credential Setter Executor', () => {
    const credentialSetterResource = {
      id: 'credential-setter-1',
      data: {
        action: {
          executor: {
            name: ExecutionTypes.CredentialSetter,
          },
        },
        properties: {},
      },
    } as unknown as Resource;

    it('should render NoConfigProperties message', () => {
      render(<ExecutionExtendedProperties resource={credentialSetterResource} onChange={mockOnChange} />);

      expect(screen.getByText('flows:core.executions.noConfig.description')).toBeInTheDocument();
    });
  });

  describe('Attribute Uniqueness Validator Executor', () => {
    const attributeUniquenessResource = {
      id: 'attr-uniqueness-1',
      data: {
        action: {
          executor: {
            name: ExecutionTypes.AttributeUniquenessValidator,
          },
        },
        properties: {},
      },
    } as unknown as Resource;

    it('should render NoConfigProperties message', () => {
      render(<ExecutionExtendedProperties resource={attributeUniquenessResource} onChange={mockOnChange} />);

      expect(screen.getByText('flows:core.executions.noConfig.description')).toBeInTheDocument();
    });
  });

  describe('Identifying Executor', () => {
    const identifyingResource = {
      id: 'identifying-executor-1',
      data: {
        action: {
          executor: {
            name: ExecutionTypes.IdentifyingExecutor,
            mode: '',
          },
        },
        display: {
          label: 'Identify User',
        },
      },
    } as unknown as Resource;

    it('should render identifying mode configuration', () => {
      render(<ExecutionExtendedProperties resource={identifyingResource} onChange={mockOnChange} />);

      expect(screen.getByText('Configure the identifying executor mode.')).toBeInTheDocument();
      expect(screen.getByText('Mode')).toBeInTheDocument();
    });

    it('should show mode options with placeholder', async () => {
      const user = userEvent.setup();

      render(<ExecutionExtendedProperties resource={identifyingResource} onChange={mockOnChange} />);

      const select = screen.getByRole('combobox');
      await user.click(select);

      expect(screen.getByText('Identify')).toBeInTheDocument();
      expect(screen.getByText('Resolve (Disambiguation)')).toBeInTheDocument();
    });

    it('should call onChange with updated data when identify mode is selected', async () => {
      const user = userEvent.setup();

      render(<ExecutionExtendedProperties resource={identifyingResource} onChange={mockOnChange} />);

      const select = screen.getByRole('combobox');
      await user.click(select);
      await user.click(screen.getByText('Identify'));

      expect(mockOnChange).toHaveBeenCalledWith(
        'data',
        expect.objectContaining({
          action: expect.objectContaining({
            executor: expect.objectContaining({
              mode: 'identify',
            }) as unknown,
          }) as unknown,
          display: expect.objectContaining({
            label: 'Identify User',
          }) as unknown,
        }),
        identifyingResource,
      );
    });

    it('should call onChange with updated data when resolve mode is selected', async () => {
      const user = userEvent.setup();

      render(<ExecutionExtendedProperties resource={identifyingResource} onChange={mockOnChange} />);

      const select = screen.getByRole('combobox');
      await user.click(select);
      await user.click(screen.getByText('Resolve (Disambiguation)'));

      expect(mockOnChange).toHaveBeenCalledWith(
        'data',
        expect.objectContaining({
          action: expect.objectContaining({
            executor: expect.objectContaining({
              mode: 'resolve',
            }) as unknown,
          }) as unknown,
          display: expect.objectContaining({
            label: 'Resolve User',
          }) as unknown,
        }),
        identifyingResource,
      );
    });
  });

  describe('Edge Cases', () => {
    it('should return null when executor name is not defined', () => {
      const resourceWithoutExecutor = {
        id: 'resource-1',
        data: {},
      } as unknown as Resource;

      const {container} = render(
        <ExecutionExtendedProperties resource={resourceWithoutExecutor} onChange={mockOnChange} />,
      );

      expect(container.firstChild).toBeNull();
    });

    it('should render only the inputs editor when executor type is not mapped', () => {
      const resourceWithUnmappedExecutor = {
        id: 'resource-1',
        data: {
          action: {
            executor: {
              name: 'UnknownExecutor',
            },
          },
        },
      } as unknown as Resource;

      const {container} = render(
        <ExecutionExtendedProperties resource={resourceWithUnmappedExecutor} onChange={mockOnChange} />,
      );

      expect(container.firstChild).not.toBeNull();
      expect(screen.getByText('flows:core.executions.inputs.title')).toBeInTheDocument();
    });

    it('should handle undefined resource gracefully', () => {
      const {container} = render(
        <ExecutionExtendedProperties resource={undefined as unknown as Resource} onChange={mockOnChange} />,
      );

      expect(container.firstChild).toBeNull();
    });

    it('should handle null properties gracefully', () => {
      const resourceWithNullProperties = {
        id: 'google-executor-1',
        data: {
          action: {
            executor: {
              name: ExecutionTypes.GoogleFederation,
            },
          },
          properties: null,
        },
      } as unknown as Resource;

      mockIdentityProviders.mockReturnValue({
        data: [{id: 'google-idp-1', name: 'Google IDP', type: IdentityProviderTypes.GOOGLE}],
        isLoading: false,
      });

      render(<ExecutionExtendedProperties resource={resourceWithNullProperties} onChange={mockOnChange} />);

      expect(screen.getByText('Connection')).toBeInTheDocument();
    });
  });
});
