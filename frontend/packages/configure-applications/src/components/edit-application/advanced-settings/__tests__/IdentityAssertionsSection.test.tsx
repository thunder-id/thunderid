// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {useState} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {OAuth2Config} from '../../../../models/oauth';
import IdentityAssertionsSection from '../IdentityAssertionsSection';

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallback: string) => fallback ?? key,
  }),
}));

const {mockUseAuthorizationServerMetadata} = vi.hoisted(() => ({mockUseAuthorizationServerMetadata: vi.fn()}));

vi.mock('@thunderid/configure-connections', () => ({
  useAuthorizationServerMetadata: mockUseAuthorizationServerMetadata,
}));

const TOKEN_EXCHANGE = 'urn:ietf:params:oauth:grant-type:token-exchange';
const TOKEN_EXCHANGE_UNAVAILABLE_HINT =
  'This server does not allow the token exchange grant, so identity assertions cannot be issued.';
const GRANT_TYPE_HINT = 'The token exchange grant type is enabled together with this feature.';

// Stands in for the runtime's authorization server metadata query (data undefined while loading or failed).
const mockServerMetadata = (data: {grant_types_supported?: string[]} | undefined, isError = false): void => {
  mockUseAuthorizationServerMetadata.mockReturnValue({data, isError});
};

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
    mockServerMetadata(undefined);
  });

  describe('Rendering', () => {
    it('should return null when oauth2Config is not provided', () => {
      const {container} = render(<IdentityAssertionsSection onTokenConfigChange={mockOnTokenConfigChange} />);

      expect(container.firstChild).toBeNull();
    });

    it('should render the card title and description', () => {
      render(<IdentityAssertionsSection oauth2Config={baseConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      expect(screen.getByText('Identity assertions (ID-JAG)')).toBeInTheDocument();
      expect(
        screen.getByText(
          "Issue signed assertions of the signed-in user's identity that external services accept for token issuance.",
        ),
      ).toBeInTheDocument();
    });

    it('should render with the switch off and body hidden when idJag is undefined', () => {
      render(<IdentityAssertionsSection oauth2Config={baseConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      const toggle = screen.getByLabelText('Identity assertions (ID-JAG)');
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

      const toggle = screen.getByLabelText('Identity assertions (ID-JAG)');
      expect(toggle).toBeChecked();
      expect(screen.getByText('Allowed audiences *')).toBeInTheDocument();
      expect(screen.getByText('https://api.example.com')).toBeInTheDocument();
    });
  });

  describe('Toggle Behavior', () => {
    it('should call onTokenConfigChange with idJag enabled true and the added grant type in a single combined call when toggled on', async () => {
      const user = userEvent.setup();

      render(<IdentityAssertionsSection oauth2Config={baseConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      await user.click(screen.getByLabelText('Identity assertions (ID-JAG)'));

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

      await user.click(screen.getByLabelText('Identity assertions (ID-JAG)'));

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

      await user.click(screen.getByLabelText('Identity assertions (ID-JAG)'));

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

    it('should keep the field empty, show an inline error and not call onTokenConfigChange when the field is emptied', () => {
      const onValidationChange = vi.fn();

      render(
        <IdentityAssertionsSection
          oauth2Config={enabledConfig}
          onTokenConfigChange={mockOnTokenConfigChange}
          onValidationChange={onValidationChange}
        />,
      );

      const input = screen.getByDisplayValue('300');
      fireEvent.change(input, {target: {value: ''}});

      expect(input).toHaveValue(null);
      expect(screen.getByText('Enter a value of at least 1 second.')).toBeInTheDocument();
      expect(onValidationChange).toHaveBeenLastCalledWith(true);
      expect(mockOnTokenConfigChange).not.toHaveBeenCalled();
    });

    // Applies each token change to its own config, the way EditAdvancedSettings does, so the card
    // re-renders with what it pushed.
    function Harness({onValidationChange = undefined}: {onValidationChange?: (hasErrors: boolean) => void}) {
      const [config, setConfig] = useState<OAuth2Config>(enabledConfig);
      return (
        <IdentityAssertionsSection
          oauth2Config={config}
          onTokenConfigChange={(tokenUpdates) => {
            mockOnTokenConfigChange(tokenUpdates);
            setConfig((prev) => ({...prev, token: {...prev.token, ...tokenUpdates} as OAuth2Config['token']}));
          }}
          onValidationChange={onValidationChange}
        />
      );
    }

    it('should let a cleared field take a new value when the parent applies each change', () => {
      render(<Harness />);

      const input = screen.getByDisplayValue('300');
      fireEvent.change(input, {target: {value: ''}});
      expect(input).toHaveValue(null);

      fireEvent.change(input, {target: {value: '600'}});
      expect(input).toHaveValue(600);
      expect(mockOnTokenConfigChange).toHaveBeenLastCalledWith({
        idJag: {enabled: true, allowedAudiences: ['https://api.example.com'], validityPeriod: 600},
      });
    });

    it('should drop a cleared field and its error when ID-JAG is turned off and on again', () => {
      const onValidationChange = vi.fn();
      render(<Harness onValidationChange={onValidationChange} />);

      fireEvent.change(screen.getByDisplayValue('300'), {target: {value: ''}});
      expect(onValidationChange).toHaveBeenLastCalledWith(true);

      const toggle = screen.getByLabelText('Identity assertions (ID-JAG)');
      fireEvent.click(toggle);
      fireEvent.click(toggle);

      expect(screen.getByDisplayValue('300')).toBeInTheDocument();
      expect(screen.queryByText('Enter a value of at least 1 second.')).not.toBeInTheDocument();
      expect(onValidationChange).toHaveBeenLastCalledWith(false);
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

  describe('Public client Guard', () => {
    it('should disable the toggle when publicClient is true', () => {
      const oauth2Config: OAuth2Config = {...baseConfig, publicClient: true};

      render(<IdentityAssertionsSection oauth2Config={oauth2Config} onTokenConfigChange={mockOnTokenConfigChange} />);

      expect(screen.getByLabelText('Identity assertions (ID-JAG)')).toBeDisabled();
    });

    it('should disable the toggle when tokenEndpointAuthMethod is none', () => {
      const oauth2Config: OAuth2Config = {...baseConfig, tokenEndpointAuthMethod: 'none'};

      render(<IdentityAssertionsSection oauth2Config={oauth2Config} onTokenConfigChange={mockOnTokenConfigChange} />);

      expect(screen.getByLabelText('Identity assertions (ID-JAG)')).toBeDisabled();
    });

    it('should show a tooltip explaining why the toggle is disabled', async () => {
      const user = userEvent.setup();
      const oauth2Config: OAuth2Config = {...baseConfig, publicClient: true};

      render(<IdentityAssertionsSection oauth2Config={oauth2Config} onTokenConfigChange={mockOnTokenConfigChange} />);

      const toggle = screen.getByLabelText('Identity assertions (ID-JAG)');
      const tooltipTrigger = toggle.closest('.MuiSwitch-root')?.parentElement;
      await user.hover(tooltipTrigger!);

      expect(
        await screen.findByText('Identity assertions require a confidential client. Turn off public client to enable.'),
      ).toBeInTheDocument();
    });

    it('should explain that a public metadata document client cannot be made confidential here', async () => {
      const user = userEvent.setup();
      const oauth2Config: OAuth2Config = {
        ...baseConfig,
        clientIdMetadataDocument: true,
        publicClient: true,
        tokenEndpointAuthMethod: 'none',
      };

      render(<IdentityAssertionsSection oauth2Config={oauth2Config} onTokenConfigChange={mockOnTokenConfigChange} />);

      const toggle = screen.getByLabelText('Identity assertions (ID-JAG)');
      expect(toggle).toBeDisabled();
      await user.hover(toggle.closest('.MuiSwitch-root')!.parentElement!);

      expect(
        await screen.findByText(
          "Identity assertions require a confidential client. This client's metadata document declares it as a public client.",
        ),
      ).toBeInTheDocument();
    });

    it('should add the token exchange grant when a private key JWT metadata document client turns it on', async () => {
      const user = userEvent.setup();
      const oauth2Config: OAuth2Config = {
        ...baseConfig,
        clientIdMetadataDocument: true,
        pkceRequired: true,
        tokenEndpointAuthMethod: 'private_key_jwt',
      };

      render(<IdentityAssertionsSection oauth2Config={oauth2Config} onTokenConfigChange={mockOnTokenConfigChange} />);
      await user.click(screen.getByLabelText('Identity assertions (ID-JAG)'));

      expect(mockOnTokenConfigChange).toHaveBeenCalledWith(
        {idJag: {enabled: true, allowedAudiences: [], validityPeriod: 300}},
        {grantTypes: ['authorization_code', 'urn:ietf:params:oauth:grant-type:token-exchange']},
      );
    });

    it('should remove the token exchange grant when a metadata document client turns it off', async () => {
      const user = userEvent.setup();
      const oauth2Config = {
        ...baseConfig,
        clientIdMetadataDocument: true,
        pkceRequired: true,
        tokenEndpointAuthMethod: 'private_key_jwt',
        grantTypes: ['authorization_code', 'urn:ietf:params:oauth:grant-type:token-exchange'],
        token: {idJag: {enabled: true, allowedAudiences: ['https://as.example.com'], validityPeriod: 300}},
      } as unknown as OAuth2Config;

      render(<IdentityAssertionsSection oauth2Config={oauth2Config} onTokenConfigChange={mockOnTokenConfigChange} />);
      await user.click(screen.getByLabelText('Identity assertions (ID-JAG)'));

      expect(mockOnTokenConfigChange).toHaveBeenCalledWith(
        {idJag: {enabled: false, allowedAudiences: ['https://as.example.com'], validityPeriod: 300}},
        {grantTypes: ['authorization_code']},
      );
    });

    it('should not disable the toggle when the client is confidential', () => {
      render(<IdentityAssertionsSection oauth2Config={baseConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      expect(screen.getByLabelText('Identity assertions (ID-JAG)')).not.toBeDisabled();
    });

    it('should disable the toggle when the disabled prop is set', () => {
      render(
        <IdentityAssertionsSection oauth2Config={baseConfig} onTokenConfigChange={mockOnTokenConfigChange} disabled />,
      );

      expect(screen.getByLabelText('Identity assertions (ID-JAG)')).toBeDisabled();
    });
  });

  describe('Token exchange availability', () => {
    const enabledConfig: OAuth2Config = {
      ...baseConfig,
      grantTypes: ['authorization_code', TOKEN_EXCHANGE],
      token: {
        accessToken: {} as never,
        idToken: {} as never,
        idJag: {enabled: true, allowedAudiences: ['https://api.example.com'], validityPeriod: 300},
      },
    };

    it('should disable turning ID-JAG on when the runtime metadata does not list the token exchange grant', () => {
      mockServerMetadata({grant_types_supported: ['authorization_code', 'client_credentials']});

      render(<IdentityAssertionsSection oauth2Config={baseConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      const toggle = screen.getByLabelText('Identity assertions (ID-JAG)');
      expect(toggle).not.toBeChecked();
      expect(toggle).toBeDisabled();
    });

    it('should explain in a tooltip that the server does not allow the token exchange grant', async () => {
      const user = userEvent.setup();
      mockServerMetadata({grant_types_supported: ['authorization_code']});

      render(<IdentityAssertionsSection oauth2Config={baseConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      const toggle = screen.getByLabelText('Identity assertions (ID-JAG)');
      await user.hover(toggle.closest('.MuiSwitch-root')!.parentElement!);

      expect(await screen.findByRole('tooltip')).toHaveTextContent(TOKEN_EXCHANGE_UNAVAILABLE_HINT);
    });

    it('should not call onTokenConfigChange when the disabled toggle is clicked', async () => {
      // A disabled switch has pointer-events: none, so the check is skipped to force the click through.
      const user = userEvent.setup({pointerEventsCheck: 0});
      mockServerMetadata({grant_types_supported: ['authorization_code']});

      render(<IdentityAssertionsSection oauth2Config={baseConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      await user.click(screen.getByLabelText('Identity assertions (ID-JAG)'));

      expect(mockOnTokenConfigChange).not.toHaveBeenCalled();
    });

    it('should let a client that already has ID-JAG on turn it off and drop the token exchange grant in the same update', async () => {
      const user = userEvent.setup();
      mockServerMetadata({grant_types_supported: ['authorization_code']});

      render(<IdentityAssertionsSection oauth2Config={enabledConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      const toggle = screen.getByLabelText('Identity assertions (ID-JAG)');
      expect(toggle).toBeChecked();
      expect(toggle).not.toBeDisabled();

      await user.click(toggle);

      expect(mockOnTokenConfigChange).toHaveBeenCalledTimes(1);
      expect(mockOnTokenConfigChange).toHaveBeenCalledWith(
        {idJag: {enabled: false, allowedAudiences: ['https://api.example.com'], validityPeriod: 300}},
        expect.objectContaining({grantTypes: ['authorization_code']}),
      );
    });

    it('should send only the token update when turning ID-JAG off on a client that does not hold the token exchange grant', async () => {
      const user = userEvent.setup();
      mockServerMetadata({grant_types_supported: ['authorization_code']});
      const oauth2Config: OAuth2Config = {...enabledConfig, grantTypes: ['authorization_code']};

      render(<IdentityAssertionsSection oauth2Config={oauth2Config} onTokenConfigChange={mockOnTokenConfigChange} />);

      await user.click(screen.getByLabelText('Identity assertions (ID-JAG)'));

      expect(mockOnTokenConfigChange).toHaveBeenCalledTimes(1);
      expect(mockOnTokenConfigChange).toHaveBeenCalledWith({
        idJag: {enabled: false, allowedAudiences: ['https://api.example.com'], validityPeriod: 300},
      });
    });

    it('should show the hint in place of the grant type caption while a client that has ID-JAG on cannot be issued assertions', () => {
      mockServerMetadata({grant_types_supported: ['authorization_code']});

      render(<IdentityAssertionsSection oauth2Config={enabledConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      expect(screen.getByText(TOKEN_EXCHANGE_UNAVAILABLE_HINT)).toBeInTheDocument();
      expect(screen.queryByText(GRANT_TYPE_HINT)).not.toBeInTheDocument();
      expect(screen.getByText('Allowed audiences *')).toBeInTheDocument();
    });

    it('should show the hint as visible text in the card body while ID-JAG is off and cannot be turned on', () => {
      mockServerMetadata({grant_types_supported: ['authorization_code']});

      render(<IdentityAssertionsSection oauth2Config={baseConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      expect(screen.getByText(TOKEN_EXCHANGE_UNAVAILABLE_HINT)).toBeInTheDocument();
      expect(screen.queryByText('Allowed audiences *')).not.toBeInTheDocument();
      expect(screen.queryByText(GRANT_TYPE_HINT)).not.toBeInTheDocument();
    });

    it('should allow turning ID-JAG on and show no hint when the runtime metadata lists the token exchange grant', async () => {
      const user = userEvent.setup();
      mockServerMetadata({grant_types_supported: ['authorization_code', TOKEN_EXCHANGE]});

      render(<IdentityAssertionsSection oauth2Config={baseConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      const toggle = screen.getByLabelText('Identity assertions (ID-JAG)');
      expect(toggle).not.toBeDisabled();
      await user.click(toggle);

      expect(mockOnTokenConfigChange).toHaveBeenCalledWith(
        {idJag: {enabled: true, allowedAudiences: [], validityPeriod: 300}},
        expect.objectContaining({grantTypes: ['authorization_code', TOKEN_EXCHANGE]}),
      );
      expect(screen.queryByText(TOKEN_EXCHANGE_UNAVAILABLE_HINT)).not.toBeInTheDocument();
    });

    it('should allow turning ID-JAG on while the metadata is loading', async () => {
      const user = userEvent.setup();
      mockServerMetadata(undefined);

      render(<IdentityAssertionsSection oauth2Config={baseConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      const toggle = screen.getByLabelText('Identity assertions (ID-JAG)');
      expect(toggle).not.toBeDisabled();
      await user.click(toggle);

      expect(mockOnTokenConfigChange).toHaveBeenCalledTimes(1);
      expect(screen.queryByText(TOKEN_EXCHANGE_UNAVAILABLE_HINT)).not.toBeInTheDocument();
    });

    it('should allow turning ID-JAG on when the metadata failed to load', () => {
      mockServerMetadata(undefined, true);

      render(<IdentityAssertionsSection oauth2Config={baseConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      expect(screen.getByLabelText('Identity assertions (ID-JAG)')).not.toBeDisabled();
      expect(screen.queryByText(TOKEN_EXCHANGE_UNAVAILABLE_HINT)).not.toBeInTheDocument();
    });

    it('should allow turning ID-JAG on when the metadata has no grant types list', () => {
      mockServerMetadata({});

      render(<IdentityAssertionsSection oauth2Config={baseConfig} onTokenConfigChange={mockOnTokenConfigChange} />);

      expect(screen.getByLabelText('Identity assertions (ID-JAG)')).not.toBeDisabled();
    });

    it('should keep the public client explanation when the client is public and the grant is unavailable', async () => {
      const user = userEvent.setup();
      mockServerMetadata({grant_types_supported: ['authorization_code']});
      const oauth2Config: OAuth2Config = {...baseConfig, publicClient: true};

      render(<IdentityAssertionsSection oauth2Config={oauth2Config} onTokenConfigChange={mockOnTokenConfigChange} />);

      const toggle = screen.getByLabelText('Identity assertions (ID-JAG)');
      await user.hover(toggle.closest('.MuiSwitch-root')!.parentElement!);

      expect(
        await screen.findByText('Identity assertions require a confidential client. Turn off public client to enable.'),
      ).toBeInTheDocument();
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
