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

import {fireEvent, render, screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {OAuth2Config} from '../../../../models/oauth';
import IdentityAssertionsSection from '../IdentityAssertionsSection';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback: string) => fallback ?? key,
  }),
}));

describe('IdentityAssertionsSection', () => {
  const mockOnTokenConfigChange = vi.fn();

  const baseConfig: OAuth2Config = {
    grantTypes: ['authorization_code'],
    responseTypes: ['code'],
    pkceRequired: false,
    publicClient: false,
  };

  beforeEach(() => {
    mockOnTokenConfigChange.mockClear();
  });

  describe('Rendering', () => {
    it('should return null when oauth2Config is not provided', () => {
      const {container} = render(<IdentityAssertionsSection onTokenConfigChange={mockOnTokenConfigChange} />);

      expect(container.firstChild).toBeNull();
    });

    it('should render the card title and description', () => {
      render(<IdentityAssertionsSection oauth2Config={baseConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      expect(screen.getByText('Identity Assertions (ID-JAG)')).toBeInTheDocument();
      expect(
        screen.getByText(
          "Issue signed assertions of the signed-in user's identity that external services accept for token issuance.",
        ),
      ).toBeInTheDocument();
    });

    it('should render with the switch off and body hidden when idJag is undefined', () => {
      render(<IdentityAssertionsSection oauth2Config={baseConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      const toggle = screen.getByLabelText('Identity Assertions (ID-JAG)');
      expect(toggle).not.toBeChecked();
      expect(screen.queryByText('Allowed audiences *')).not.toBeInTheDocument();
    });

    it('should render the body when idJag is enabled', () => {
      const oauth2Config: OAuth2Config = {
        ...baseConfig,
        token: {
          accessToken: {} as never,
          idToken: {} as never,
          idJag: {enabled: true, allowedAudiences: ['https://api.example.com'], validityPeriod: 300},
        },
      };

      render(<IdentityAssertionsSection oauth2Config={oauth2Config} onTokenConfigChange={mockOnTokenConfigChange} />);

      const toggle = screen.getByLabelText('Identity Assertions (ID-JAG)');
      expect(toggle).toBeChecked();
      expect(screen.getByText('Allowed audiences *')).toBeInTheDocument();
      expect(screen.getByText('https://api.example.com')).toBeInTheDocument();
    });
  });

  describe('Toggle Behavior', () => {
    it('should call onTokenConfigChange with idJag enabled true and the added grant type in a single combined call when toggled on', async () => {
      const user = userEvent.setup();

      render(<IdentityAssertionsSection oauth2Config={baseConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      await user.click(screen.getByLabelText('Identity Assertions (ID-JAG)'));

      expect(mockOnTokenConfigChange).toHaveBeenCalledTimes(1);
      expect(mockOnTokenConfigChange).toHaveBeenCalledWith(
        {idJag: {enabled: true, allowedAudiences: [], validityPeriod: 300}},
        expect.objectContaining({
          grantTypes: expect.arrayContaining([
            'authorization_code',
            'urn:ietf:params:oauth:grant-type:token-exchange',
          ]) as unknown,
        }),
      );
    });

    it('should call onTokenConfigChange with only the token update when toggled on and the grant type is already present', async () => {
      const user = userEvent.setup();
      const oauth2Config: OAuth2Config = {
        ...baseConfig,
        grantTypes: ['authorization_code', 'urn:ietf:params:oauth:grant-type:token-exchange'],
      };

      render(<IdentityAssertionsSection oauth2Config={oauth2Config} onTokenConfigChange={mockOnTokenConfigChange} />);

      await user.click(screen.getByLabelText('Identity Assertions (ID-JAG)'));

      expect(mockOnTokenConfigChange).toHaveBeenCalledTimes(1);
      expect(mockOnTokenConfigChange).toHaveBeenCalledWith({
        idJag: {enabled: true, allowedAudiences: [], validityPeriod: 300},
      });
    });

    it('should keep the token exchange grant type when toggled off', async () => {
      const user = userEvent.setup();
      const oauth2Config: OAuth2Config = {
        ...baseConfig,
        grantTypes: ['authorization_code', 'urn:ietf:params:oauth:grant-type:token-exchange'],
        token: {
          accessToken: {} as never,
          idToken: {} as never,
          idJag: {enabled: true, allowedAudiences: ['https://api.example.com'], validityPeriod: 300},
        },
      };

      render(<IdentityAssertionsSection oauth2Config={oauth2Config} onTokenConfigChange={mockOnTokenConfigChange} />);

      await user.click(screen.getByLabelText('Identity Assertions (ID-JAG)'));

      expect(mockOnTokenConfigChange).toHaveBeenCalledTimes(1);
      expect(mockOnTokenConfigChange).toHaveBeenCalledWith({
        idJag: {enabled: false, allowedAudiences: ['https://api.example.com'], validityPeriod: 300},
      });
    });
  });

  describe('Allowed Audiences', () => {
    const enabledConfig: OAuth2Config = {
      ...baseConfig,
      token: {
        accessToken: {} as never,
        idToken: {} as never,
        idJag: {enabled: true, allowedAudiences: ['https://api.example.com'], validityPeriod: 300},
      },
    };

    it('should add a new audience when typed and confirmed', async () => {
      const user = userEvent.setup({delay: null});

      render(<IdentityAssertionsSection oauth2Config={enabledConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      const input = screen.getByPlaceholderText('Type an audience and press Enter');
      await user.type(input, 'https://other.example.com');
      await user.keyboard('{Enter}');

      await waitFor(() => {
        expect(mockOnTokenConfigChange).toHaveBeenCalledWith({
          idJag: {
            enabled: true,
            allowedAudiences: ['https://api.example.com', 'https://other.example.com'],
            validityPeriod: 300,
          },
        });
      });
    });

    it('should remove an audience when its chip delete icon is clicked', async () => {
      const user = userEvent.setup();

      render(<IdentityAssertionsSection oauth2Config={enabledConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      const chip = screen.getByText('https://api.example.com').closest('.MuiChip-root')!;
      const deleteIcon = chip.querySelector('.MuiChip-deleteIcon')!;
      await user.click(deleteIcon);

      expect(mockOnTokenConfigChange).toHaveBeenCalledWith({
        idJag: {enabled: true, allowedAudiences: [], validityPeriod: 300},
      });
    });

    it('should show an error when idJag is enabled and audiences is empty', () => {
      const oauth2Config: OAuth2Config = {
        ...baseConfig,
        token: {
          accessToken: {} as never,
          idToken: {} as never,
          idJag: {enabled: true, allowedAudiences: [], validityPeriod: 300},
        },
      };

      render(<IdentityAssertionsSection oauth2Config={oauth2Config} onTokenConfigChange={mockOnTokenConfigChange} />);

      expect(screen.getByText('Add at least one audience.')).toBeInTheDocument();
    });

    it('should not show an error when idJag is disabled and audiences is empty', () => {
      render(<IdentityAssertionsSection oauth2Config={baseConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      expect(screen.queryByText('Add at least one audience.')).not.toBeInTheDocument();
    });
  });

  describe('Validity Period', () => {
    const enabledConfig: OAuth2Config = {
      ...baseConfig,
      token: {
        accessToken: {} as never,
        idToken: {} as never,
        idJag: {enabled: true, allowedAudiences: ['https://api.example.com'], validityPeriod: 300},
      },
    };

    it('should default the validity period to 300 when unset', () => {
      const oauth2Config: OAuth2Config = {
        ...baseConfig,
        token: {
          accessToken: {} as never,
          idToken: {} as never,
          idJag: {enabled: true, allowedAudiences: ['https://api.example.com']},
        },
      };

      render(<IdentityAssertionsSection oauth2Config={oauth2Config} onTokenConfigChange={mockOnTokenConfigChange} />);

      expect(screen.getByDisplayValue('300')).toBeInTheDocument();
    });

    it('should call onTokenConfigChange with the updated validity period', () => {
      render(<IdentityAssertionsSection oauth2Config={enabledConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      const input = screen.getByDisplayValue('300');
      fireEvent.change(input, {target: {value: '600'}});

      expect(mockOnTokenConfigChange).toHaveBeenCalledWith({
        idJag: {enabled: true, allowedAudiences: ['https://api.example.com'], validityPeriod: 600},
      });
    });

    it('should clear the validity period when the field is emptied', () => {
      render(<IdentityAssertionsSection oauth2Config={enabledConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      const input = screen.getByDisplayValue('300');
      fireEvent.change(input, {target: {value: ''}});

      expect(mockOnTokenConfigChange).toHaveBeenCalledWith({
        idJag: {enabled: true, allowedAudiences: ['https://api.example.com'], validityPeriod: undefined},
      });
      expect(screen.queryByText('Enter a value of at least 1 second.')).not.toBeInTheDocument();
    });

    it('should show an inline error and not call onTokenConfigChange when the value is 0', () => {
      render(<IdentityAssertionsSection oauth2Config={enabledConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      const input = screen.getByDisplayValue('300');
      fireEvent.change(input, {target: {value: '0'}});

      expect(screen.getByText('Enter a value of at least 1 second.')).toBeInTheDocument();
      expect(screen.getByDisplayValue('0')).toBeInTheDocument();
      expect(mockOnTokenConfigChange).not.toHaveBeenCalled();
    });

    it('should show an inline error and not call onTokenConfigChange when the value is negative', () => {
      render(<IdentityAssertionsSection oauth2Config={enabledConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      const input = screen.getByDisplayValue('300');
      fireEvent.change(input, {target: {value: '-5'}});

      expect(screen.getByText('Enter a value of at least 1 second.')).toBeInTheDocument();
      expect(mockOnTokenConfigChange).not.toHaveBeenCalled();
    });

    it('should clear the error and call onTokenConfigChange once a valid value is entered', () => {
      render(<IdentityAssertionsSection oauth2Config={enabledConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      const input = screen.getByDisplayValue('300');
      fireEvent.change(input, {target: {value: '0'}});
      expect(screen.getByText('Enter a value of at least 1 second.')).toBeInTheDocument();

      fireEvent.change(screen.getByDisplayValue('0'), {target: {value: '120'}});

      expect(screen.queryByText('Enter a value of at least 1 second.')).not.toBeInTheDocument();
      expect(mockOnTokenConfigChange).toHaveBeenCalledWith({
        idJag: {enabled: true, allowedAudiences: ['https://api.example.com'], validityPeriod: 120},
      });
    });

    it('should clear a pending invalid input and show the external value when the config is reset externally (e.g. discard)', () => {
      const {rerender} = render(
        <IdentityAssertionsSection oauth2Config={enabledConfig} onTokenConfigChange={mockOnTokenConfigChange} />,
      );

      const input = screen.getByDisplayValue('300');
      fireEvent.change(input, {target: {value: '0'}});
      expect(screen.getByText('Enter a value of at least 1 second.')).toBeInTheDocument();

      const resetConfig: OAuth2Config = {
        ...baseConfig,
        token: {
          accessToken: {} as never,
          idToken: {} as never,
          idJag: {enabled: true, allowedAudiences: ['https://api.example.com'], validityPeriod: 300},
        },
      };

      rerender(<IdentityAssertionsSection oauth2Config={resetConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      expect(screen.queryByText('Enter a value of at least 1 second.')).not.toBeInTheDocument();
      expect(screen.getByDisplayValue('300')).toBeInTheDocument();
    });
  });

  describe('Public Client Guard', () => {
    it('should disable the toggle when publicClient is true', () => {
      const oauth2Config: OAuth2Config = {...baseConfig, publicClient: true};

      render(<IdentityAssertionsSection oauth2Config={oauth2Config} onTokenConfigChange={mockOnTokenConfigChange} />);

      expect(screen.getByLabelText('Identity Assertions (ID-JAG)')).toBeDisabled();
    });

    it('should disable the toggle when tokenEndpointAuthMethod is none', () => {
      const oauth2Config: OAuth2Config = {...baseConfig, tokenEndpointAuthMethod: 'none'};

      render(<IdentityAssertionsSection oauth2Config={oauth2Config} onTokenConfigChange={mockOnTokenConfigChange} />);

      expect(screen.getByLabelText('Identity Assertions (ID-JAG)')).toBeDisabled();
    });

    it('should show a tooltip explaining why the toggle is disabled', async () => {
      const user = userEvent.setup();
      const oauth2Config: OAuth2Config = {...baseConfig, publicClient: true};

      render(<IdentityAssertionsSection oauth2Config={oauth2Config} onTokenConfigChange={mockOnTokenConfigChange} />);

      const toggle = screen.getByLabelText('Identity Assertions (ID-JAG)');
      const tooltipTrigger = toggle.closest('.MuiSwitch-root')?.parentElement;
      await user.hover(tooltipTrigger!);

      expect(
        await screen.findByText('Identity assertions require a confidential client. Turn off Public Client to enable.'),
      ).toBeInTheDocument();
    });

    it('should not disable the toggle when the client is confidential', () => {
      render(<IdentityAssertionsSection oauth2Config={baseConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      expect(screen.getByLabelText('Identity Assertions (ID-JAG)')).not.toBeDisabled();
    });

    it('should disable the toggle when the disabled prop is set', () => {
      render(
        <IdentityAssertionsSection oauth2Config={baseConfig} onTokenConfigChange={mockOnTokenConfigChange} disabled />,
      );

      expect(screen.getByLabelText('Identity Assertions (ID-JAG)')).toBeDisabled();
    });
  });

  describe('Validation reporting', () => {
    const enabledEmptyAudiencesConfig: OAuth2Config = {
      ...baseConfig,
      token: {
        accessToken: {} as never,
        idToken: {} as never,
        idJag: {enabled: true, allowedAudiences: [], validityPeriod: 300},
      },
    };

    it('should report hasErrors=true when enabled with empty audiences', () => {
      const onValidationChange = vi.fn();

      render(
        <IdentityAssertionsSection
          oauth2Config={enabledEmptyAudiencesConfig}
          onTokenConfigChange={mockOnTokenConfigChange}
          onValidationChange={onValidationChange}
        />,
      );

      expect(onValidationChange).toHaveBeenLastCalledWith(true);
    });

    it('should report hasErrors=false once an audience is added', () => {
      const onValidationChange = vi.fn();

      const {rerender} = render(
        <IdentityAssertionsSection
          oauth2Config={enabledEmptyAudiencesConfig}
          onTokenConfigChange={mockOnTokenConfigChange}
          onValidationChange={onValidationChange}
        />,
      );
      expect(onValidationChange).toHaveBeenLastCalledWith(true);

      const withAudienceConfig: OAuth2Config = {
        ...baseConfig,
        token: {
          accessToken: {} as never,
          idToken: {} as never,
          idJag: {enabled: true, allowedAudiences: ['https://api.example.com'], validityPeriod: 300},
        },
      };
      rerender(
        <IdentityAssertionsSection
          oauth2Config={withAudienceConfig}
          onTokenConfigChange={mockOnTokenConfigChange}
          onValidationChange={onValidationChange}
        />,
      );

      expect(onValidationChange).toHaveBeenLastCalledWith(false);
    });

    it('should report hasErrors=false when toggled off', () => {
      const onValidationChange = vi.fn();

      const {rerender} = render(
        <IdentityAssertionsSection
          oauth2Config={enabledEmptyAudiencesConfig}
          onTokenConfigChange={mockOnTokenConfigChange}
          onValidationChange={onValidationChange}
        />,
      );
      expect(onValidationChange).toHaveBeenLastCalledWith(true);

      const disabledConfig: OAuth2Config = {
        ...baseConfig,
        token: {
          accessToken: {} as never,
          idToken: {} as never,
          idJag: {enabled: false, allowedAudiences: [], validityPeriod: 300},
        },
      };
      rerender(
        <IdentityAssertionsSection
          oauth2Config={disabledConfig}
          onTokenConfigChange={mockOnTokenConfigChange}
          onValidationChange={onValidationChange}
        />,
      );

      expect(onValidationChange).toHaveBeenLastCalledWith(false);
    });

    it('should report hasErrors=true when the validity period input is invalid (0)', () => {
      const onValidationChange = vi.fn();
      const enabledWithAudienceConfig: OAuth2Config = {
        ...baseConfig,
        token: {
          accessToken: {} as never,
          idToken: {} as never,
          idJag: {enabled: true, allowedAudiences: ['https://api.example.com'], validityPeriod: 300},
        },
      };

      render(
        <IdentityAssertionsSection
          oauth2Config={enabledWithAudienceConfig}
          onTokenConfigChange={mockOnTokenConfigChange}
          onValidationChange={onValidationChange}
        />,
      );
      expect(onValidationChange).toHaveBeenLastCalledWith(false);

      const input = screen.getByDisplayValue('300');
      fireEvent.change(input, {target: {value: '0'}});

      expect(onValidationChange).toHaveBeenLastCalledWith(true);
    });
  });
});
