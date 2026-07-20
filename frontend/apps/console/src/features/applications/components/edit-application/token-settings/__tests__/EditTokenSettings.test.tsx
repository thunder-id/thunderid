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

import {render, screen, waitFor, fireEvent} from '@thunderid/test-utils';
import {useState} from 'react';
import {Controller, type Control} from 'react-hook-form';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Application} from '../../../../models/application';
import type {OAuth2Config} from '../../../../models/oauth';
import EditTokenSettings from '../EditTokenSettings';

// Stable mock references — must be created via vi.hoisted so they are available
// inside the hoisted vi.mock factory functions below. Without stable references,
// useThunderID/useConfig/useLogger return new object identities on every render,
// causing fetchSchemas' useEffect to re-fire every render → infinite loop → OOM.
const {mockHttp, mockGetServerUrl, mockLogger} = vi.hoisted(() => {
  const hoistedMockHttp = {
    request: vi.fn().mockResolvedValue({
      data: {
        totalResults: 1,
        startIndex: 0,
        count: 1,
        types: [
          {
            id: 'schema-1',
            name: 'default',
          },
        ],
      },
    }),
  };
  const hoistedMockGetServerUrl = vi.fn().mockReturnValue('https://api.example.com');
  const hoistedMockLogger = {
    error: vi.fn(),
    info: vi.fn(),
    debug: vi.fn(),
  };
  return {mockHttp: hoistedMockHttp, mockGetServerUrl: hoistedMockGetServerUrl, mockLogger: hoistedMockLogger};
});

// Mock child components.
// TokenUserAttributesSection receives accessTokenAttributes/idTokenAttributes in OAuth mode
// and sharedAttributes in native mode — there is no "tokenType" prop on the real component.
// The mock renders separate testids for access and id sections (matching test expectations)
// and exposes the UserInfo inherit checkbox so the User Info tests can verify state.
vi.mock('../TokenUserAttributesSection', () => ({
  default: ({
    accessTokenAttributes,
    idTokenAttributes,
    isUserInfoCustomAttributes,
    onToggleUserInfo,
    userAttributes,
  }: {
    accessTokenAttributes?: string[];
    idTokenAttributes?: string[];
    isUserInfoCustomAttributes?: boolean;
    onToggleUserInfo?: (checked: boolean) => void;
    userAttributes?: string[];
  }) => {
    const isOAuthMode = accessTokenAttributes !== undefined || idTokenAttributes !== undefined;
    if (isOAuthMode) {
      return (
        <div>
          <div data-testid="token-user-attributes-section-access">Access Token Attributes</div>
          <div data-testid="token-user-attributes-section-id">ID Token Attributes</div>
          {userAttributes && <div data-testid="user-attributes-list">{userAttributes.join(',')}</div>}
          <label>
            <input
              type="checkbox"
              checked={!isUserInfoCustomAttributes}
              onChange={(e: React.ChangeEvent<HTMLInputElement>) => onToggleUserInfo?.(!e.target.checked)}
              readOnly={onToggleUserInfo === undefined}
            />
            Use same attributes as ID Token
          </label>
        </div>
      );
    }
    return (
      <div data-testid="token-user-attributes-section-shared">
        Shared Token Attributes
        {userAttributes && <div data-testid="user-attributes-list">{userAttributes.join(',')}</div>}
      </div>
    );
  },
}));

vi.mock('../ScopeSection', () => ({
  default: ({scopes, disabled}: {scopes: string[]; disabled?: boolean}) => (
    <div data-testid="scope-section">
      Scopes: {scopes.join(', ')}
      {disabled && <span data-testid="scope-section-disabled" />}
    </div>
  ),
}));

// TokenValidationSection is called with tokenType="oauth" in OAuth mode and
// tokenType="shared" in native mode. The mock splits "oauth" into separate
// access, id, and refresh testids to match test expectations.
//
// The mock also carries its own local "selected tab" state (mirroring activeValidationTab)
// and react-hook-form Controller bound to the refreshTokenValidity/validityPeriod
// field it's given. Reset-behavior tests prove the field value reverts on a sectionResetKey
// bump, but the selected tab state survives because the component isn't remounted.
vi.mock('../TokenValidationSection', () => ({
  default: function MockTokenValidationSection({
    control,
    tokenType,
  }: {
    control: Control<{
      validityPeriod: number;
      accessTokenValidity: number;
      idTokenValidity: number;
      refreshTokenValidity: number;
    }>;
    tokenType: string;
  }) {
    const [activeValidationTab, setActiveValidationTab] = useState<'access' | 'id' | 'refresh'>('access');
    const fieldName = tokenType === 'oauth' ? 'refreshTokenValidity' : 'validityPeriod';

    const renderInput = () => (
      <Controller
        name={fieldName}
        control={control}
        render={({field}) => (
          <input
            data-testid={`${fieldName}-input`}
            value={field.value}
            onChange={(e) => field.onChange(parseInt(e.target.value, 10))}
          />
        )}
      />
    );

    if (tokenType === 'oauth') {
      return (
        <>
          <div data-testid="token-validation-section-access">Access Token Validation</div>
          <div data-testid="token-validation-section-id">ID Token Validation</div>
          <div data-testid="token-validation-section-refresh">
            Refresh Token Validation
            <button type="button" data-testid="select-refresh-tab" onClick={() => setActiveValidationTab('refresh')}>
              Select Refresh Tab
            </button>
            <div data-testid="active-validation-tab">{activeValidationTab}</div>
            {renderInput()}
          </div>
        </>
      );
    }
    return (
      <div data-testid={`token-validation-section-${tokenType}`}>
        Token Validation Section - {tokenType}
        {renderInput()}
      </div>
    );
  },
}));

// Mock useThunderID — stable mockHttp reference prevents fetchSchemas effect from
// re-firing on every render (http is in the effect's dependency array).
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({
    http: mockHttp,
  }),
}));

// Mock useConfig — stable mockGetServerUrl reference (also in fetchSchemas deps).
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({
      getServerUrl: mockGetServerUrl,
    }),
  };
});

// Mock useLogger — stable mockLogger reference (also in fetchSchemas deps).
vi.mock('@thunderid/logger', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/logger')>();
  return {
    ...actual,
    useLogger: () => mockLogger,
  };
});

describe('EditTokenSettings', () => {
  const mockOnFieldChange = vi.fn();
  const mockApplication: Application = {
    id: 'app-123',
    name: 'Test App',
    allowedUserTypes: ['default'],
    token: {
      validityPeriod: 3600,
      userAttributes: ['email'],
    },
  } as Application;

  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Native Mode (No OAuth2 Config)', () => {
    it('should render without crashing', () => {
      const {container} = render(<EditTokenSettings application={mockApplication} onFieldChange={mockOnFieldChange} />);

      expect(container).toBeTruthy();
    });

    it('should render shared token user attributes section', () => {
      render(<EditTokenSettings application={mockApplication} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByTestId('token-user-attributes-section-shared')).toBeInTheDocument();
    });

    it('should render shared token validation section', () => {
      render(<EditTokenSettings application={mockApplication} onFieldChange={mockOnFieldChange} />);

      expect(screen.getByTestId('token-validation-section-shared')).toBeInTheDocument();
    });

    it('should not render access token sections in native mode', () => {
      render(<EditTokenSettings application={mockApplication} onFieldChange={mockOnFieldChange} />);

      expect(screen.queryByTestId('token-user-attributes-section-access')).not.toBeInTheDocument();
      expect(screen.queryByTestId('token-validation-section-access')).not.toBeInTheDocument();
    });

    it('should not render ID token sections in native mode', () => {
      render(<EditTokenSettings application={mockApplication} onFieldChange={mockOnFieldChange} />);

      expect(screen.queryByTestId('token-user-attributes-section-id')).not.toBeInTheDocument();
      expect(screen.queryByTestId('token-validation-section-id')).not.toBeInTheDocument();
    });

    it('should not render scope section in native mode', () => {
      render(<EditTokenSettings application={mockApplication} onFieldChange={mockOnFieldChange} />);

      expect(screen.queryByTestId('scope-section')).not.toBeInTheDocument();
    });
  });

  describe('OAuth2/OIDC Mode', () => {
    const mockOAuth2Config: OAuth2Config = {
      token: {
        accessToken: {
          userConfig: {
            validityPeriod: 1800,
            attributes: ['sub', 'email'],
          },
        },
        idToken: {
          validityPeriod: 3600,
          userAttributes: ['sub', 'name', 'email'],
        },
        refreshToken: {
          validityPeriod: 86400,
        },
      },
    } as OAuth2Config;

    it('should render access token user attributes section', () => {
      render(
        <EditTokenSettings
          application={mockApplication}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByTestId('token-user-attributes-section-access')).toBeInTheDocument();
    });

    it('should render ID token user attributes section', () => {
      render(
        <EditTokenSettings
          application={mockApplication}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByTestId('token-user-attributes-section-id')).toBeInTheDocument();
    });

    it('should render access token validation section', () => {
      render(
        <EditTokenSettings
          application={mockApplication}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByTestId('token-validation-section-access')).toBeInTheDocument();
    });

    it('should render ID token validation section', () => {
      render(
        <EditTokenSettings
          application={mockApplication}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByTestId('token-validation-section-id')).toBeInTheDocument();
    });

    it('should render scope section in OAuth mode', () => {
      render(
        <EditTokenSettings
          application={mockApplication}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.getByTestId('scope-section')).toBeInTheDocument();
    });

    it('should not render shared token sections in OAuth mode', () => {
      render(
        <EditTokenSettings
          application={mockApplication}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(screen.queryByTestId('token-user-attributes-section-shared')).not.toBeInTheDocument();
      expect(screen.queryByTestId('token-validation-section-shared')).not.toBeInTheDocument();
    });
  });

  describe('Props Validation', () => {
    it('should handle undefined oauth2Config gracefully', () => {
      const {container} = render(
        <EditTokenSettings application={mockApplication} onFieldChange={mockOnFieldChange} oauth2Config={undefined} />,
      );

      expect(container).toBeTruthy();
      expect(screen.getByTestId('token-user-attributes-section-shared')).toBeInTheDocument();
    });

    it('should handle application without token config', () => {
      const appWithoutToken = {
        ...mockApplication,
        token: undefined,
      };

      const {container} = render(<EditTokenSettings application={appWithoutToken} onFieldChange={mockOnFieldChange} />);

      expect(container).toBeTruthy();
    });

    it('should handle empty allowedUserTypes array', () => {
      const appWithoutUserTypes = {
        ...mockApplication,
        allowedUserTypes: [],
      };

      const {container} = render(
        <EditTokenSettings application={appWithoutUserTypes} onFieldChange={mockOnFieldChange} />,
      );

      expect(container).toBeTruthy();
    });
  });

  describe('Section Rendering Order', () => {
    it('should render all sections for OAuth mode', () => {
      const mockOAuth2Config: OAuth2Config = {
        token: {
          accessToken: {userConfig: {validityPeriod: 1800, attributes: []}},
          idToken: {validityPeriod: 3600, userAttributes: []},
          refreshToken: {validityPeriod: 86400},
        },
      } as unknown as OAuth2Config;

      const {container} = render(
        <EditTokenSettings
          application={mockApplication}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
        />,
      );

      expect(container).toBeTruthy();
      expect(screen.getByTestId('token-user-attributes-section-access')).toBeInTheDocument();
      expect(screen.getByTestId('token-validation-section-access')).toBeInTheDocument();
      expect(screen.getByTestId('token-user-attributes-section-id')).toBeInTheDocument();
      expect(screen.getByTestId('token-validation-section-id')).toBeInTheDocument();
      expect(screen.getByTestId('token-validation-section-refresh')).toBeInTheDocument();
    });

    it('should render all sections for native mode', () => {
      const {container} = render(<EditTokenSettings application={mockApplication} onFieldChange={mockOnFieldChange} />);

      expect(container).toBeTruthy();
      expect(screen.getByTestId('token-user-attributes-section-shared')).toBeInTheDocument();
      expect(screen.getByTestId('token-validation-section-shared')).toBeInTheDocument();
    });
  });

  describe('User Info Configuration Logic', () => {
    const idTokenAttrs = ['sub', 'email'];
    const mockApp = {...mockApplication};

    it('should render User Info section with Inherit checkbox checked by default (No UserInfo Config)', () => {
      const mockConfig = {
        token: {
          idToken: {userAttributes: idTokenAttrs},
        },
      } as OAuth2Config;

      render(<EditTokenSettings application={mockApp} oauth2Config={mockConfig} onFieldChange={mockOnFieldChange} />);

      // Check for the checkbox presence
      const checkbox = screen.getByRole('checkbox', {name: /Use same attributes as ID Token/i});
      expect(checkbox).toBeInTheDocument();
      expect(checkbox).toBeChecked();
    });

    it('should verify "Inherited" state (Checked) when explicit UserInfo attributes MATCH ID Token attributes', () => {
      const mockConfig = {
        token: {
          idToken: {userAttributes: idTokenAttrs},
        },
        userInfo: {
          userAttributes: ['sub', 'email'], // Explicit but Match
        },
      } as OAuth2Config;

      render(<EditTokenSettings application={mockApp} oauth2Config={mockConfig} onFieldChange={mockOnFieldChange} />);

      const checkbox = screen.getByRole('checkbox', {name: /Use same attributes as ID Token/i});
      expect(checkbox).toBeChecked(); // Should be inherited because attributes are identical
    });

    it('should verify "Custom" state (Unchecked) when UserInfo attributes DIFFER from ID Token attributes', () => {
      const mockConfig = {
        token: {
          idToken: {userAttributes: idTokenAttrs},
        },
        userInfo: {
          userAttributes: ['sub', 'email', 'phone'], // Different
        },
      } as OAuth2Config;

      render(<EditTokenSettings application={mockApp} oauth2Config={mockConfig} onFieldChange={mockOnFieldChange} />);

      const checkbox = screen.getByRole('checkbox', {name: /Use same attributes as ID Token/i});
      expect(checkbox).not.toBeChecked();
    });
  });

  describe('Credential Attribute Filtering', () => {
    const mockSchemaRequest = (schema: Record<string, unknown>) => {
      mockHttp.request.mockImplementation(({url}: {url: string}) => {
        if (url.includes('/user-types/schema-1')) {
          return Promise.resolve({
            data: {
              id: 'schema-1',
              name: 'default',
              ouId: 'org-1',
              allowSelfRegistration: false,
              schema,
            },
          });
        }

        return Promise.resolve({
          data: {totalResults: 1, startIndex: 0, count: 1, types: [{id: 'schema-1', name: 'default'}]},
        });
      });
    };

    it.each([
      {
        name: 'top-level credential attributes (e.g., password)',
        schema: {
          email: {type: 'string', required: true, unique: true},
          password: {type: 'string', required: true, credential: true},
          username: {type: 'string', required: false},
          pin: {type: 'number', credential: true},
          age: {type: 'number', required: false},
        },
        included: ['email', 'username', 'age'],
        excluded: ['password', 'pin'],
      },
      {
        name: 'nested credential attributes inside objects',
        schema: {
          email: {type: 'string', required: true},
          security: {
            type: 'object',
            properties: {
              secret: {type: 'string', credential: true},
              question: {type: 'string'},
            },
          },
        },
        included: ['email', 'security.question'],
        excluded: ['security.secret'],
      },
    ])('should exclude $name', async ({schema, included, excluded}) => {
      mockSchemaRequest(schema);

      render(<EditTokenSettings application={mockApplication} onFieldChange={mockOnFieldChange} />);

      const el = await screen.findByTestId('user-attributes-list');
      await waitFor(() => expect(el.textContent).not.toBe(''));
      const attributesList = el.textContent;

      excluded.forEach((attr) => expect(attributesList).not.toContain(attr));
      included.forEach((attr) => expect(attributesList).toContain(attr));
    });
  });

  describe('Token Validity reset', () => {
    const mockOAuth2Config: OAuth2Config = {
      token: {
        accessToken: {userConfig: {validityPeriod: 1800, attributes: []}},
        idToken: {validityPeriod: 3600, userAttributes: []},
        refreshToken: {validityPeriod: 86400},
      },
    } as unknown as OAuth2Config;

    it('reverts the Refresh Token validity value but keeps the selected sub-tab when sectionResetKey changes', async () => {
      const {rerender} = render(
        <EditTokenSettings
          application={mockApplication}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
          sectionResetKey={0}
        />,
      );

      fireEvent.click(screen.getByTestId('select-refresh-tab'));
      expect(screen.getByTestId('active-validation-tab')).toHaveTextContent('refresh');

      const input = screen.getByTestId('refreshTokenValidity-input');
      fireEvent.change(input, {target: {value: '999'}});
      expect(input).toHaveValue('999');

      rerender(
        <EditTokenSettings
          application={mockApplication}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
          sectionResetKey={1}
        />,
      );

      await waitFor(() => {
        expect(screen.getByTestId('refreshTokenValidity-input')).toHaveValue('86400');
      });
      // The sub-tab selection must survive the reset — TokenValidationSection isn't remounted.
      expect(screen.getByTestId('active-validation-tab')).toHaveTextContent('refresh');
    });

    it('keeps the typed value when sectionResetKey stays the same', () => {
      const {rerender} = render(
        <EditTokenSettings
          application={mockApplication}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
          sectionResetKey={0}
        />,
      );

      const input = screen.getByTestId('refreshTokenValidity-input');
      fireEvent.change(input, {target: {value: '999'}});

      rerender(
        <EditTokenSettings
          application={mockApplication}
          oauth2Config={mockOAuth2Config}
          onFieldChange={mockOnFieldChange}
          sectionResetKey={0}
        />,
      );

      expect(screen.getByTestId('refreshTokenValidity-input')).toHaveValue('999');
    });

    it('does not re-dirty the assertion field after a reset in native (non-OAuth) mode', async () => {
      const nativeApplication: Application = {
        ...mockApplication,
        assertion: {validityPeriod: 3600, userAttributes: []},
      } as Application;

      const {rerender} = render(
        <EditTokenSettings application={nativeApplication} onFieldChange={mockOnFieldChange} sectionResetKey={0} />,
      );

      const input = screen.getByTestId('validityPeriod-input');
      fireEvent.change(input, {target: {value: '999'}});

      await waitFor(() => {
        expect(mockOnFieldChange).toHaveBeenCalledWith('assertion', expect.objectContaining({validityPeriod: 999}));
      });
      mockOnFieldChange.mockClear();

      rerender(
        <EditTokenSettings application={nativeApplication} onFieldChange={mockOnFieldChange} sectionResetKey={1} />,
      );

      await waitFor(() => {
        expect(screen.getByTestId('validityPeriod-input')).toHaveValue('3600');
      });

      // Give the (guarded) commit effect a chance to fire and confirm it stayed a no-op.
      await new Promise((resolve) => setTimeout(resolve, 0));
      expect(mockOnFieldChange).not.toHaveBeenCalled();
    });
  });
});
