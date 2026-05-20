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
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import ExecutionExtendedProperties from '../ExecutionExtendedProperties';
import type {Resource} from '@/features/flows/models/resources';
import {ExecutionTypes} from '@/features/flows/models/steps';
import {IdentityProviderTypes} from '@/features/integrations/models/identity-provider';

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

// Mock useIdentityProviders
const mockIdentityProviders = vi.fn<() => {data: unknown[]; isLoading: boolean}>();
vi.mock('@/features/integrations/api/useIdentityProviders', () => ({
  default: () => mockIdentityProviders(),
}));

// Mock useNotificationSenders
const mockNotificationSenders = vi.fn<() => {data: unknown[]; isLoading: boolean}>();
vi.mock('@/features/notification-senders/api/useNotificationSenders', () => ({
  default: () => mockNotificationSenders(),
}));

describe('ExecutionExtendedProperties', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockIdentityProviders.mockReturnValue({
      data: [],
      isLoading: false,
    });
    mockNotificationSenders.mockReturnValue({
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
      expect(
        screen.getByText('Select a connection from the following list to link it with the login flow.'),
      ).toBeInTheDocument();
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

  describe('SMS OTP Executor', () => {
    const smsOtpResource = {
      id: 'sms-otp-executor-1',
      data: {
        action: {
          executor: {
            name: ExecutionTypes.SMSOTPAuth,
            mode: '',
          },
        },
        properties: {
          senderId: '',
        },
        display: {
          label: 'SMS OTP',
        },
      },
    } as unknown as Resource;

    it('should render SMS OTP configuration UI', () => {
      mockNotificationSenders.mockReturnValue({
        data: [{id: 'sender-1', name: 'Twilio Sender'}],
        isLoading: false,
      });

      render(<ExecutionExtendedProperties resource={smsOtpResource} onChange={mockOnChange} />);

      expect(screen.getByText('Configure SMS OTP settings')).toBeInTheDocument();
      expect(screen.getByText('Mode')).toBeInTheDocument();
      expect(screen.getByText('Sender')).toBeInTheDocument();
    });

    it('should show mode options', async () => {
      const user = userEvent.setup();
      mockNotificationSenders.mockReturnValue({
        data: [{id: 'sender-1', name: 'Twilio Sender'}],
        isLoading: false,
      });

      render(<ExecutionExtendedProperties resource={smsOtpResource} onChange={mockOnChange} />);

      const comboboxes = screen.getAllByRole('combobox');
      const modeSelect = comboboxes[0];
      await user.click(modeSelect);

      expect(screen.getByText('Send SMS OTP')).toBeInTheDocument();
      expect(screen.getByText('Verify SMS OTP')).toBeInTheDocument();
    });

    it('should call onChange with updated data when mode is selected', async () => {
      const user = userEvent.setup();
      mockNotificationSenders.mockReturnValue({
        data: [{id: 'sender-1', name: 'Twilio Sender'}],
        isLoading: false,
      });

      render(<ExecutionExtendedProperties resource={smsOtpResource} onChange={mockOnChange} />);

      const comboboxes = screen.getAllByRole('combobox');
      const modeSelect = comboboxes[0];
      await user.click(modeSelect);
      await user.click(screen.getByText('Send SMS OTP'));

      expect(mockOnChange).toHaveBeenCalledWith(
        'data',
        expect.objectContaining({
          action: expect.objectContaining({
            executor: expect.objectContaining({
              mode: 'send',
            }) as unknown,
          }) as unknown,
          display: expect.objectContaining({
            label: 'Send SMS OTP',
          }) as unknown,
        }),
        smsOtpResource,
      );
    });

    it('should show sender options', async () => {
      const user = userEvent.setup();
      mockSelectedNotification.hasResourceFieldNotification.mockReturnValue(false);
      mockNotificationSenders.mockReturnValue({
        data: [
          {id: 'sender-1', name: 'Twilio Sender'},
          {id: 'sender-2', name: 'Vonage Sender'},
        ],
        isLoading: false,
      });

      render(<ExecutionExtendedProperties resource={smsOtpResource} onChange={mockOnChange} />);

      const comboboxes = screen.getAllByRole('combobox');
      const senderSelect = comboboxes[1]; // Second combobox is sender
      await user.click(senderSelect);

      expect(screen.getByText('Twilio Sender')).toBeInTheDocument();
      expect(screen.getByText('Vonage Sender')).toBeInTheDocument();
    });

    it('should call onChange when sender is selected', async () => {
      const user = userEvent.setup();
      mockSelectedNotification.hasResourceFieldNotification.mockReturnValue(false);
      mockNotificationSenders.mockReturnValue({
        data: [{id: 'sender-1', name: 'Twilio Sender'}],
        isLoading: false,
      });

      render(<ExecutionExtendedProperties resource={smsOtpResource} onChange={mockOnChange} />);

      const comboboxes = screen.getAllByRole('combobox');
      const senderSelect = comboboxes[1];
      await user.click(senderSelect);
      await user.click(screen.getByText('Twilio Sender'));

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.senderId', 'sender-1', smsOtpResource);
    });

    it('should show error when sender is placeholder', () => {
      mockSelectedNotification.hasResourceFieldNotification.mockReturnValue(false);
      mockNotificationSenders.mockReturnValue({
        data: [{id: 'sender-1', name: 'Twilio Sender'}],
        isLoading: false,
      });

      const resourceWithPlaceholder = {
        ...smsOtpResource,
        data: {
          ...(smsOtpResource as unknown as {data: object}).data,
          properties: {senderId: '{{SENDER_ID}}'},
        },
      } as unknown as Resource;

      render(<ExecutionExtendedProperties resource={resourceWithPlaceholder} onChange={mockOnChange} />);

      expect(screen.getByText('Sender is required')).toBeInTheDocument();
    });

    it('should show warning when no senders are configured', () => {
      mockSelectedNotification.hasResourceFieldNotification.mockReturnValue(false);
      mockNotificationSenders.mockReturnValue({
        data: [],
        isLoading: false,
      });

      render(<ExecutionExtendedProperties resource={smsOtpResource} onChange={mockOnChange} />);

      expect(screen.getByText('No SMS senders configured')).toBeInTheDocument();
    });

    it('should disable sender dropdown while loading', () => {
      mockSelectedNotification.hasResourceFieldNotification.mockReturnValue(false);
      mockNotificationSenders.mockReturnValue({
        data: [],
        isLoading: true,
      });

      render(<ExecutionExtendedProperties resource={smsOtpResource} onChange={mockOnChange} />);

      const comboboxes = screen.getAllByRole('combobox');
      const senderSelect = comboboxes[1];
      expect(senderSelect).toHaveAttribute('aria-disabled', 'true');
    });

    it('should disable sender dropdown when no senders available', () => {
      mockSelectedNotification.hasResourceFieldNotification.mockReturnValue(false);
      mockNotificationSenders.mockReturnValue({
        data: [],
        isLoading: false,
      });

      render(<ExecutionExtendedProperties resource={smsOtpResource} onChange={mockOnChange} />);

      const comboboxes = screen.getAllByRole('combobox');
      const senderSelect = comboboxes[1];
      expect(senderSelect).toHaveAttribute('aria-disabled', 'true');
    });

    it('should show selected sender value', () => {
      mockSelectedNotification.hasResourceFieldNotification.mockReturnValue(false);
      mockNotificationSenders.mockReturnValue({
        data: [{id: 'sender-1', name: 'Twilio Sender'}],
        isLoading: false,
      });

      const resourceWithSender = {
        ...smsOtpResource,
        data: {
          ...(smsOtpResource as unknown as {data: object}).data,
          properties: {senderId: 'sender-1'},
        },
      } as unknown as Resource;

      render(<ExecutionExtendedProperties resource={resourceWithSender} onChange={mockOnChange} />);

      const comboboxes = screen.getAllByRole('combobox');
      const senderSelect = comboboxes[1];
      expect(senderSelect).toHaveTextContent('Twilio Sender');
    });

    it('should show selected mode value', () => {
      mockSelectedNotification.hasResourceFieldNotification.mockReturnValue(false);
      mockNotificationSenders.mockReturnValue({
        data: [{id: 'sender-1', name: 'Twilio Sender'}],
        isLoading: false,
      });

      const resourceWithMode = {
        ...smsOtpResource,
        data: {
          ...(smsOtpResource as unknown as {data: object}).data,
          action: {
            executor: {
              name: ExecutionTypes.SMSOTPAuth,
              mode: 'verify',
            },
          },
        },
      } as unknown as Resource;

      render(<ExecutionExtendedProperties resource={resourceWithMode} onChange={mockOnChange} />);

      const comboboxes = screen.getAllByRole('combobox');
      const modeSelect = comboboxes[0];
      expect(modeSelect).toHaveTextContent('Verify SMS OTP');
    });

    it('should update display label when mode changes to verify', async () => {
      const user = userEvent.setup();
      mockNotificationSenders.mockReturnValue({
        data: [{id: 'sender-1', name: 'Twilio Sender'}],
        isLoading: false,
      });

      render(<ExecutionExtendedProperties resource={smsOtpResource} onChange={mockOnChange} />);

      const comboboxes = screen.getAllByRole('combobox');
      const modeSelect = comboboxes[0];
      await user.click(modeSelect);
      await user.click(screen.getByText('Verify SMS OTP'));

      expect(mockOnChange).toHaveBeenCalledWith(
        'data',
        expect.objectContaining({
          display: expect.objectContaining({
            label: 'Verify SMS OTP',
          }) as unknown,
        }),
        smsOtpResource,
      );
    });

    it('should preserve existing data properties when mode changes', async () => {
      const user = userEvent.setup();
      mockNotificationSenders.mockReturnValue({
        data: [{id: 'sender-1', name: 'Twilio Sender'}],
        isLoading: false,
      });

      const resourceWithExistingData = {
        ...smsOtpResource,
        data: {
          ...(smsOtpResource as unknown as {data: object}).data,
          properties: {senderId: 'sender-1', someOtherProp: 'value'},
          display: {label: 'Old Label', icon: 'icon.png'},
        },
      } as unknown as Resource;

      render(<ExecutionExtendedProperties resource={resourceWithExistingData} onChange={mockOnChange} />);

      const comboboxes = screen.getAllByRole('combobox');
      const modeSelect = comboboxes[0];
      await user.click(modeSelect);
      await user.click(screen.getByText('Send SMS OTP'));

      expect(mockOnChange).toHaveBeenCalledWith(
        'data',
        expect.objectContaining({
          properties: expect.objectContaining({
            senderId: 'sender-1',
            someOtherProp: 'value',
          }) as unknown,
          display: expect.objectContaining({
            label: 'Send SMS OTP',
            icon: 'icon.png',
          }) as unknown,
        }),
        resourceWithExistingData,
      );
    });

    it('should preserve display properties when mode changes', async () => {
      const user = userEvent.setup();
      mockNotificationSenders.mockReturnValue({
        data: [{id: 'sender-1', name: 'Twilio Sender'}],
        isLoading: false,
      });

      const resourceWithDisplay = {
        ...smsOtpResource,
        data: {
          ...(smsOtpResource as unknown as {data: object}).data,
          display: {icon: 'sms-icon.png'},
        },
      } as unknown as Resource;

      render(<ExecutionExtendedProperties resource={resourceWithDisplay} onChange={mockOnChange} />);

      const comboboxes = screen.getAllByRole('combobox');
      const modeSelect = comboboxes[0];
      await user.click(modeSelect);
      await user.click(screen.getByText('Send SMS OTP'));

      // Should preserve existing display properties while updating label
      expect(mockOnChange).toHaveBeenCalledWith(
        'data',
        expect.objectContaining({
          display: expect.objectContaining({
            label: 'Send SMS OTP',
            icon: 'sms-icon.png',
          }) as unknown,
        }),
        resourceWithDisplay,
      );
    });

    it('should show validation error for sender field', () => {
      (mockSelectedNotification.hasResourceFieldNotification as unknown as ReturnType<typeof vi.fn>).mockImplementation(
        (key: string) => key === 'sms-otp-executor-1_data.properties.senderId',
      );
      (mockSelectedNotification.getResourceFieldNotification as unknown as ReturnType<typeof vi.fn>).mockImplementation(
        (key: string) => (key === 'sms-otp-executor-1_data.properties.senderId' ? 'Sender ID is invalid' : ''),
      );
      mockNotificationSenders.mockReturnValue({
        data: [{id: 'sender-1', name: 'Twilio Sender'}],
        isLoading: false,
      });

      render(<ExecutionExtendedProperties resource={smsOtpResource} onChange={mockOnChange} />);

      expect(screen.getByText('Sender ID is invalid')).toBeInTheDocument();
    });

    it('should not show warning when senders are still loading', () => {
      mockSelectedNotification.hasResourceFieldNotification.mockReturnValue(false);
      mockNotificationSenders.mockReturnValue({
        data: [],
        isLoading: true,
      });

      render(<ExecutionExtendedProperties resource={smsOtpResource} onChange={mockOnChange} />);

      expect(screen.queryByText('No SMS senders configured')).not.toBeInTheDocument();
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

      fireEvent.change(screen.getByLabelText('Relying Party ID'), {
        target: {value: 'localhost'},
      });
      fireEvent.change(screen.getByLabelText('Relying Party Name'), {
        target: {value: 'ThunderID'},
      });

      expect(mockOnChange).toHaveBeenCalledWith(
        'data.properties.relyingPartyId',
        'localhost',
        resourceWithChallengeMode,
        true,
      );
      expect(mockOnChange).toHaveBeenCalledWith(
        'data.properties.relyingPartyName',
        'ThunderID',
        resourceWithChallengeMode,
        true,
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

    it('should call onChange when timeout changes', () => {
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

      expect(mockOnChange).toHaveBeenLastCalledWith('data.properties.timeout', '45', consentResourceWithTimeout, true);
    });

    it('should normalize empty timeout to 0', () => {
      render(<ExecutionExtendedProperties resource={consentResource} onChange={mockOnChange} />);

      const timeoutInput = screen.getByLabelText('Consent Timeout (seconds)');
      fireEvent.change(timeoutInput, {target: {value: ''}});

      expect(mockOnChange).toHaveBeenLastCalledWith('data.properties.timeout', '0', consentResource, true);
    });

    it('should clamp negative timeout to 0', () => {
      render(<ExecutionExtendedProperties resource={consentResource} onChange={mockOnChange} />);

      const timeoutInput = screen.getByLabelText('Consent Timeout (seconds)');
      fireEvent.change(timeoutInput, {target: {value: '-5'}});

      expect(mockOnChange).toHaveBeenLastCalledWith('data.properties.timeout', '0', consentResource, true);
    });

    it('should floor decimal timeout to integer', () => {
      render(<ExecutionExtendedProperties resource={consentResource} onChange={mockOnChange} />);

      const timeoutInput = screen.getByLabelText('Consent Timeout (seconds)');
      fireEvent.change(timeoutInput, {target: {value: '3.7'}});

      expect(mockOnChange).toHaveBeenLastCalledWith('data.properties.timeout', '3', consentResource, true);
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

    it('should call onChange with debounce when email template changes', () => {
      render(<ExecutionExtendedProperties resource={emailResource} onChange={mockOnChange} />);

      fireEvent.change(screen.getByLabelText('flows:core.executions.email.emailTemplate.label'), {
        target: {value: 'welcome-email'},
      });

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.emailTemplate', 'welcome-email', emailResource, true);
    });

    it('should display existing email template value', () => {
      const resourceWithTemplate = {
        ...emailResource,
        data: {
          ...(emailResource as unknown as {data: object}).data,
          properties: {emailTemplate: 'reset-password'},
        },
      } as unknown as Resource;

      render(<ExecutionExtendedProperties resource={resourceWithTemplate} onChange={mockOnChange} />);

      expect(screen.getByLabelText('flows:core.executions.email.emailTemplate.label')).toHaveValue('reset-password');
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
      mockNotificationSenders.mockReturnValue({
        data: [{id: 'sender-1', name: 'Twilio'}],
        isLoading: false,
      });

      render(<ExecutionExtendedProperties resource={smsResource} onChange={mockOnChange} />);

      expect(screen.getByText('flows:core.executions.sms.description')).toBeInTheDocument();
      expect(screen.getByLabelText('flows:core.executions.sms.smsTemplate.label')).toBeInTheDocument();
      expect(screen.getByText('Sender')).toBeInTheDocument();
    });

    it('should call onChange with debounce when SMS template changes', () => {
      mockNotificationSenders.mockReturnValue({
        data: [],
        isLoading: false,
      });

      render(<ExecutionExtendedProperties resource={smsResource} onChange={mockOnChange} />);

      fireEvent.change(screen.getByLabelText('flows:core.executions.sms.smsTemplate.label'), {
        target: {value: 'otp-message'},
      });

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.smsTemplate', 'otp-message', smsResource, true);
    });

    it('should show warning when no senders are available', () => {
      mockNotificationSenders.mockReturnValue({
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
          assignGroup: '',
          assignRole: '',
        },
      },
    } as unknown as Resource;

    it('should render provisioning configuration', () => {
      render(<ExecutionExtendedProperties resource={provisioningResource} onChange={mockOnChange} />);

      expect(screen.getByText('flows:core.executions.provisioning.description')).toBeInTheDocument();
      expect(screen.getByText('flows:core.executions.federation.allowCrossOUProvisioning.label')).toBeInTheDocument();
      expect(
        screen.getByText('flows:core.executions.provisioning.includeOptionalCredentials.label'),
      ).toBeInTheDocument();
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

    it('should call onChange without debounce when includeOptionalCredentials checkbox is toggled', () => {
      render(<ExecutionExtendedProperties resource={provisioningResource} onChange={mockOnChange} />);

      const checkboxes = screen.getAllByRole('checkbox');
      const includeOptionalCredentialsCheckbox = checkboxes[1];
      fireEvent.click(includeOptionalCredentialsCheckbox);

      expect(mockOnChange).toHaveBeenCalledWith(
        'data.properties.includeOptionalCredentials',
        true,
        provisioningResource,
      );
    });

    it('should call onChange with debounce when assignGroup changes', () => {
      render(<ExecutionExtendedProperties resource={provisioningResource} onChange={mockOnChange} />);

      fireEvent.change(screen.getByLabelText('flows:core.executions.provisioning.assignGroup.label'), {
        target: {value: 'admin-group'},
      });

      expect(mockOnChange).toHaveBeenCalledWith(
        'data.properties.assignGroup',
        'admin-group',
        provisioningResource,
        true,
      );
    });

    it('should call onChange with debounce when assignRole changes', () => {
      render(<ExecutionExtendedProperties resource={provisioningResource} onChange={mockOnChange} />);

      fireEvent.change(screen.getByLabelText('flows:core.executions.provisioning.assignRole.label'), {
        target: {value: 'editor-role'},
      });

      expect(mockOnChange).toHaveBeenCalledWith(
        'data.properties.assignRole',
        'editor-role',
        provisioningResource,
        true,
      );
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

    it('should call onChange with debounce when parentOuId changes', () => {
      render(<ExecutionExtendedProperties resource={ouResource} onChange={mockOnChange} />);

      fireEvent.change(screen.getByLabelText('flows:core.executions.ouExecutor.parentOuId.label'), {
        target: {value: 'ou-123'},
      });

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.parentOuId', 'ou-123', ouResource, true);
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

    it('should call onChange with debounce when URL changes', () => {
      render(<ExecutionExtendedProperties resource={httpResource} onChange={mockOnChange} />);

      fireEvent.change(screen.getByLabelText('flows:core.executions.httpRequest.url.label'), {
        target: {value: 'https://api.example.com'},
      });

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.url', 'https://api.example.com', httpResource, true);
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

    it('should call onChange with debounce when timeout changes', () => {
      render(<ExecutionExtendedProperties resource={httpResource} onChange={mockOnChange} />);

      fireEvent.change(screen.getByLabelText('flows:core.executions.httpRequest.timeout.label'), {
        target: {value: '15'},
      });

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.timeout', 15, httpResource, true);
    });

    it('should clamp timeout to max 20', () => {
      render(<ExecutionExtendedProperties resource={httpResource} onChange={mockOnChange} />);

      fireEvent.change(screen.getByLabelText('flows:core.executions.httpRequest.timeout.label'), {
        target: {value: '99'},
      });

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.timeout', 20, httpResource, true);
    });

    it('should call onChange with debounce when body changes', () => {
      render(<ExecutionExtendedProperties resource={httpResource} onChange={mockOnChange} />);

      fireEvent.change(screen.getByLabelText('flows:core.executions.httpRequest.body.label'), {
        target: {value: 'raw body text'},
      });

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.body', 'raw body text', httpResource, true);
    });

    it('should parse valid JSON body', () => {
      render(<ExecutionExtendedProperties resource={httpResource} onChange={mockOnChange} />);

      fireEvent.change(screen.getByLabelText('flows:core.executions.httpRequest.body.label'), {
        target: {value: '{"key":"value"}'},
      });

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.body', {key: 'value'}, httpResource, true);
    });

    it('should call onChange with debounce when retryCount changes', () => {
      render(<ExecutionExtendedProperties resource={httpResource} onChange={mockOnChange} />);

      fireEvent.change(screen.getByLabelText('flows:core.executions.httpRequest.errorHandling.retryCount.label'), {
        target: {value: '3'},
      });

      expect(mockOnChange).toHaveBeenCalledWith(
        'data.properties.errorHandling',
        expect.objectContaining({retryCount: 3}),
        httpResource,
        true,
      );
    });

    it('should call onChange with debounce when retryDelay changes', () => {
      render(<ExecutionExtendedProperties resource={httpResource} onChange={mockOnChange} />);

      fireEvent.change(screen.getByLabelText('flows:core.executions.httpRequest.errorHandling.retryDelay.label'), {
        target: {value: '1000'},
      });

      expect(mockOnChange).toHaveBeenCalledWith(
        'data.properties.errorHandling',
        expect.objectContaining({retryDelay: 1000}),
        httpResource,
        true,
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
