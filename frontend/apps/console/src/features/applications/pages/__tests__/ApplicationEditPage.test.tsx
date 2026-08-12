// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {UseQueryResult, UseMutationResult} from '@tanstack/react-query';
import userEvent from '@testing-library/user-event';
import {useGetApplication} from '@thunderid/configure-applications';
import type {Application} from '@thunderid/configure-applications';
import {render, screen, waitFor, fireEvent, within} from '@thunderid/test-utils';
import {useState} from 'react';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import useUpdateApplication from '../../api/useUpdateApplication';
import EditAccessSettings from '../../components/edit-application/access/EditAccessSettings';
import EditAdvancedSettings from '../../components/edit-application/advanced-settings/EditAdvancedSettings';
import EditCustomizationSettings from '../../components/edit-application/customization-settings/EditCustomizationSettings';
import McpConnectTab from '../../components/edit-application/mcp/McpConnectTab';
import EditTokenSettings from '../../components/edit-application/token-settings/EditTokenSettings';
import EditTokenSettingsTabs from '../../components/edit-application/token-settings/EditTokenSettingsTabs';
import ApplicationConstants from '../../constants/application-constants';
import {getIntegrationGuideForTemplate} from '../../utils/getIntegrationGuidesForTemplate';
import getTemplateMetadata from '../../utils/getTemplateMetadata';
import ApplicationEditPage from '../ApplicationEditPage';

// Mock dependencies
vi.mock('react-router', async () => {
  const actual = await vi.importActual('react-router');
  return {
    ...actual,
    useNavigate: vi.fn(() => vi.fn()),
    useParams: vi.fn(() => ({applicationId: 'test-app-id'})),
    useLocation: vi.fn(() => ({state: null})),
  };
});

const APPLICATIONS_TRANSLATIONS: Record<string, string> = {
  'applications:edit.page.back': 'Back to Applications',
  'applications:edit.page.logoUpdate.label': 'Update Logo',
  'applications:edit.page.loading': 'Loading application...',
  'applications:edit.page.notFound.title': 'Application Not Found',
  'applications:edit.page.notFound.description': 'The application you are looking for does not exist.',
  'applications:edit.page.description.placeholder': 'Add a description',
  'applications:edit.page.description.empty': 'No description provided',
  'applications:edit.page.tabs.overview': 'Overview',
  'applications:edit.page.tabs.general': 'General',
  'applications:edit.page.tabs.flows': 'Flows',
  'applications:edit.page.tabs.customization': 'Customization',
  'applications:edit.page.tabs.token': 'Token',
  'applications:edit.page.tabs.advanced': 'Advanced',
  'applications:edit.page.unsavedChanges': 'You have unsaved changes',
  'applications:edit.page.reset': 'Reset',
  'applications:edit.page.save': 'Save Changes',
  'applications:edit.page.saving': 'Saving...',
  'applications:update.error': 'Failed to update application. Please try again.',
  'applications:errors.APP-1020': 'An application with this name already exists. Choose a different name.',
};

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string, fallbackOrOptions?: string | {defaultValue?: string}) => {
      if (APPLICATIONS_TRANSLATIONS[key] !== undefined) return APPLICATIONS_TRANSLATIONS[key];
      if (typeof fallbackOrOptions === 'string') return fallbackOrOptions;
      if (fallbackOrOptions && 'defaultValue' in fallbackOrOptions) return fallbackOrOptions.defaultValue ?? key;
      return key;
    },
  }),
}));

vi.mock('@thunderid/configure-applications', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-applications')>()),
  useGetApplication: vi.fn(),
}));

vi.mock('../../api/useUpdateApplication', () => ({
  default: vi.fn(),
}));

vi.mock('../../utils/getTemplateMetadata', () => ({
  default: vi.fn(),
}));

vi.mock('../../utils/getIntegrationGuidesForTemplate', () => ({
  default: vi.fn(),
  getIntegrationGuideForTemplate: vi.fn(),
}));

// Mock child components
vi.mock('../../components/edit-application/access/EditAccessSettings', () => ({
  default: vi.fn(() => <div data-testid="edit-access-settings">Access Settings</div>),
}));

vi.mock('../../components/edit-application/credentials/EditCredentialsSettings', () => ({
  default: vi.fn(() => <div data-testid="edit-credentials-settings">Credentials Settings</div>),
}));

vi.mock('../../components/edit-application/flows-settings/EditFlowsSettings', () => ({
  default: vi.fn(() => <div data-testid="edit-flows-settings">Flows Settings</div>),
}));

vi.mock('../../components/edit-application/customization-settings/EditCustomizationSettings', () => ({
  default: vi.fn(() => <div data-testid="edit-customization-settings">Customization Settings</div>),
}));

vi.mock('../../components/edit-application/token-settings/EditTokenSettings', () => ({
  default: vi.fn(function MockEditTokenSettings() {
    // Mimics EditTokenSettings' real react-hook-form-backed Token Validity fields
    const [clicks, setClicks] = useState(0);
    return (
      <div data-testid="edit-token-settings">
        Token Settings, Clicks: {clicks}
        <button type="button" data-testid="edit-token-settings-bump" onClick={() => setClicks((c) => c + 1)}>
          Bump
        </button>
      </div>
    );
  }),
}));

vi.mock('../../components/edit-application/token-settings/EditTokenSettingsTabs', () => ({
  default: vi.fn(() => <div data-testid="edit-token-settings">Token Settings</div>),
}));

vi.mock('../../components/edit-application/advanced-settings/EditAdvancedSettings', () => ({
  default: vi.fn(() => <div data-testid="edit-advanced-settings">Advanced</div>),
}));

vi.mock('../../components/edit-application/integration-guides/IntegrationGuides', () => ({
  default: vi.fn(() => <div data-testid="integration-guides">Integration Guides</div>),
}));

vi.mock('../../components/edit-application/mcp/McpConnectTab', () => ({
  default: vi.fn(({onValidationChange}: {onValidationChange?: (hasErrors: boolean) => void}) => (
    <div data-testid="mcp-connect-tab">
      Connect Tab
      <button type="button" data-testid="mcp-connect-tab-report-invalid" onClick={() => onValidationChange?.(true)}>
        Report invalid
      </button>
    </div>
  )),
}));

vi.mock('@thunderid/components', async (importOriginal) => {
  const React = await import('react');
  const actual = await importOriginal<typeof import('@thunderid/components')>();
  return {
    ...actual,
    AppleIcon: vi.fn(() => null),
    AndroidLogo: vi.fn(() => null),
    FlutterLogo: vi.fn(() => null),
    CopyableId: vi.fn(() => null),
    EmojiPicker: vi.fn(() => null),
    PageLoadingAnimation: vi.fn(() => <div data-testid="page-loading-animation" />),
    UnsavedChangesBar: vi.fn(
      ({
        message,
        resetLabel,
        saveLabel,
        savingLabel,
        isSaving,
        saveDisabled,
        error,
        onReset,
        onSave,
      }: {
        message: string;
        resetLabel: string;
        saveLabel: string;
        savingLabel: string;
        isSaving: boolean;
        saveDisabled?: boolean;
        error?: string;
        onReset: () => void;
        onSave: () => void;
      }) => (
        <div data-testid="unsaved-changes-bar">
          {error && <div role="alert">{error}</div>}
          <span>{message}</span>
          <button type="button" onClick={onReset}>
            {resetLabel}
          </button>
          <button type="button" onClick={onSave} disabled={isSaving || saveDisabled}>
            {isSaving ? savingLabel : saveLabel}
          </button>
        </div>
      ),
    ),
    ResourceAvatar: vi.fn(function MockResourceAvatar({
      value,
      onSelect,
      editAriaLabel,
    }: {
      value?: string;
      onSelect?: (v: string) => void;
      editAriaLabel?: string;
    }) {
      const [open, setOpen] = React.useState(false);
      const [imgError, setImgError] = React.useState(false);
      const isUrl = typeof value === 'string' && (value.startsWith('http://') || value.startsWith('https://'));
      const displayValue =
        typeof value === 'string' && value.startsWith('emoji:') ? value.slice('emoji:'.length) : value;
      return (
        <>
          {isUrl && (
            <button type="button" onClick={() => setOpen(true)}>
              <img src={value} alt="logo" onError={() => setImgError(true)} style={imgError ? {display: 'none'} : {}} />
            </button>
          )}
          {!isUrl && displayValue && <span>{displayValue}</span>}
          {editAriaLabel && onSelect && (
            <button type="button" aria-label={editAriaLabel} onClick={() => setOpen(true)} />
          )}
          <div data-testid="emoji-picker" style={{display: open ? 'block' : 'none'}}>
            <button
              type="button"
              onClick={() => {
                onSelect?.('emoji:🚀');
                setOpen(false);
              }}
            >
              Select Icon
            </button>
            <button type="button" onClick={() => setOpen(false)}>
              Close
            </button>
          </div>
        </>
      );
    }),
  };
});

const mockUseGetApplication = useGetApplication as ReturnType<typeof vi.fn>;
const mockUseUpdateApplication = useUpdateApplication as ReturnType<typeof vi.fn>;
const mockGetTemplateMetadata = getTemplateMetadata as ReturnType<typeof vi.fn>;
const mockGetIntegrationGuidesForTemplate = getIntegrationGuideForTemplate as ReturnType<typeof vi.fn>;

// Edits the inline name/description field displaying `currentValue` to `newValue` and commits with Enter.
async function editInlineField(user: ReturnType<typeof userEvent.setup>, currentValue: string, newValue: string) {
  const section = screen.getByText(currentValue).closest('div');
  const editButton = section?.querySelector('button');
  await user.click(editButton!);

  const input = screen.getByRole('textbox');
  await user.clear(input);
  await user.type(input, `${newValue}{Enter}`);
}

async function mountSectionTab(user: ReturnType<typeof userEvent.setup>, tabName: string, testId: string) {
  await user.click(screen.getByRole('tab', {name: tabName}));
  await waitFor(() => {
    expect(screen.getByTestId(testId)).toBeInTheDocument();
  });
}

// Edits a field, clicks reset, and returns the sectionResetKey `mockedComponent` was called with
// before and after the click. The caller is responsible for asserting on the returned keys.
async function editFieldAndResetSection<P>(
  user: ReturnType<typeof userEvent.setup>,
  mockedComponent: (props: P) => unknown,
  currentName: string,
  newName: string,
): Promise<{initialKey: number | undefined; keyAfterReset: number | undefined}> {
  const readSectionResetKey = () =>
    (vi.mocked(mockedComponent).mock.calls.at(-1)?.[0] as {sectionResetKey?: number} | undefined)?.sectionResetKey;

  const initialKey = readSectionResetKey();

  await editInlineField(user, currentName, newName);

  await waitFor(() => {
    expect(screen.getByRole('button', {name: /reset/i})).toBeInTheDocument();
  });
  await user.click(screen.getByRole('button', {name: /reset/i}));

  let keyAfterReset: number | undefined;
  await waitFor(() => {
    keyAfterReset = readSectionResetKey();
    expect(keyAfterReset).toBe((initialKey ?? 0) + 1);
  });

  return {initialKey, keyAfterReset};
}

describe('ApplicationEditPage', () => {
  const mockApplication: Application = {
    id: 'test-app-id',
    name: 'Test Application',
    description: 'Test application description',
    template: 'react',
    logoUrl: 'https://example.com/logo.png',
    url: 'https://example.com',
    allowedUserTypes: ['user'],
    inboundAuthConfig: [
      {
        type: 'oauth2',
        config: {
          responseTypes: ['code'],
          clientId: 'test-client-id',
          clientSecret: 'test-client-secret',
          grantTypes: ['authorization_code'],
          redirectUris: ['https://example.com/callback'],
          pkceRequired: true,
          publicClient: false,
          tokenEndpointAuthMethod: 'client_secret_basic',
        },
      },
    ],
  };

  const mockUpdateApplicationMutate = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();

    // Default mock implementations
    mockGetTemplateMetadata.mockReturnValue({
      displayName: 'React',
      icon: <div>React Icon</div>,
    });

    // Return null by default to indicate no integration guides
    mockGetIntegrationGuidesForTemplate.mockReturnValue(null);

    mockUseGetApplication.mockReturnValue({
      data: mockApplication,
      isLoading: false,
      isError: false,
      error: null,
    } as UseQueryResult<Application>);

    mockUseUpdateApplication.mockReturnValue({
      mutate: mockUpdateApplicationMutate,
      mutateAsync: vi.fn().mockResolvedValue(mockApplication),
      isPending: false,
      isError: false,
      error: null,
      reset: vi.fn(),
    } as unknown as UseMutationResult<Application, Error, Partial<Application>>);
  });

  const renderComponent = () => render(<ApplicationEditPage />);

  describe('Loading State', () => {
    it('should display loading state while fetching application', () => {
      mockUseGetApplication.mockReturnValue({
        data: undefined,
        isLoading: true,
        isError: false,
        error: null,
      } as UseQueryResult<Application>);

      renderComponent();

      // When loading, the component shows a loading indicator
      expect(screen.getByTestId('page-loading-animation')).toBeInTheDocument();
    });
  });

  describe('Error State', () => {
    it('should display error state when application is not found', () => {
      mockUseGetApplication.mockReturnValue({
        data: undefined,
        isLoading: false,
        isError: true,
        error: new Error('Not found'),
      } as unknown as UseQueryResult<Application>);

      renderComponent();

      // Check for error UI elements
      expect(screen.getByRole('button', {name: /back to applications/i})).toBeInTheDocument();
    });

    it('should display back button in error state', () => {
      mockUseGetApplication.mockReturnValue({
        data: undefined,
        isLoading: false,
        isError: true,
        error: new Error('Not found'),
      } as unknown as UseQueryResult<Application>);

      renderComponent();

      expect(screen.getByRole('button', {name: /back to applications/i})).toBeInTheDocument();
    });

    it('should navigate back when back button is clicked in error state', () => {
      mockUseGetApplication.mockReturnValue({
        data: undefined,
        isLoading: false,
        isError: true,
        error: new Error('Not found'),
      } as unknown as UseQueryResult<Application>);

      renderComponent();

      fireEvent.click(screen.getByRole('button', {name: /back to applications/i}));

      // Button should still be present after click (navigation is async)
      expect(screen.getByRole('button', {name: /back to applications/i})).toBeInTheDocument();
    });
  });

  describe('Successful Load', () => {
    it('should render application details correctly', () => {
      renderComponent();

      expect(screen.getByText('Test Application')).toBeInTheDocument();
      expect(screen.getByText('Test application description')).toBeInTheDocument();
    });

    it('should display application logo', () => {
      renderComponent();

      const logo = screen.getByRole('img');
      expect(logo).toHaveAttribute('src', 'https://example.com/logo.png');
    });

    it('should display template chip when template metadata is available', () => {
      renderComponent();

      expect(screen.getByText('React')).toBeInTheDocument();
    });

    it('should display back button', () => {
      renderComponent();

      expect(screen.getByRole('button', {name: /back to applications/i})).toBeInTheDocument();
    });

    it('should handle empty description', () => {
      mockUseGetApplication.mockReturnValue({
        data: {...mockApplication, description: undefined},
        isLoading: false,
        isError: false,
        error: null,
      } as UseQueryResult<Application>);

      renderComponent();

      expect(screen.getByText('No description provided')).toBeInTheDocument();
    });
  });

  describe('Tab Navigation', () => {
    it('should render all tabs without integration guides', () => {
      renderComponent();

      expect(screen.getByRole('tab', {name: /access/i})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: /credentials/i})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: /flows/i})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: /customization/i})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: /token/i})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: /advanced/i})).toBeInTheDocument();
    });

    it('should render overview tab when integration guides are available', () => {
      mockGetIntegrationGuidesForTemplate.mockReturnValue(['react-vite']);

      renderComponent();

      expect(screen.getByRole('tab', {name: /overview/i})).toBeInTheDocument();
    });

    it('should display the overview tab by default even when there are no integration guides', () => {
      // Mock returns null by default (no integration guides) — the Overview tab still shows,
      // since it also surfaces application identifiers and endpoints regardless of guides.
      renderComponent();

      const overviewTab = screen.getByRole('tab', {name: /overview/i});
      expect(overviewTab).toHaveAttribute('aria-selected', 'true');
    });

    it('should display overview tab by default when integration guides are available', async () => {
      mockGetIntegrationGuidesForTemplate.mockReturnValue(['react-vite']);

      renderComponent();

      await waitFor(() => {
        expect(screen.getByTestId('integration-guides')).toBeInTheDocument();
      });
    });

    it('should switch to access tab when clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      const accessTab = screen.getByRole('tab', {name: /access/i});
      await user.click(accessTab);

      await waitFor(() => {
        expect(screen.getByTestId('edit-access-settings')).toBeInTheDocument();
      });
    });

    it('should switch to credentials tab when clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      const credentialsTab = screen.getByRole('tab', {name: /credentials/i});
      await user.click(credentialsTab);

      await waitFor(() => {
        expect(screen.getByTestId('edit-credentials-settings')).toBeInTheDocument();
      });
    });

    it('should switch to flows tab when clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      const flowsTab = screen.getByRole('tab', {name: /flows/i});
      await user.click(flowsTab);

      await waitFor(() => {
        expect(screen.getByTestId('edit-flows-settings')).toBeInTheDocument();
      });
    });

    it('should switch to customization tab when clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      const customizationTab = screen.getByRole('tab', {name: /customization/i});
      await user.click(customizationTab);

      await waitFor(() => {
        expect(screen.getByTestId('edit-customization-settings')).toBeInTheDocument();
      });
    });

    it('should switch to token tab when clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      const tokenTab = screen.getByRole('tab', {name: /token/i});
      await user.click(tokenTab);

      await waitFor(() => {
        expect(screen.getByTestId('edit-token-settings')).toBeInTheDocument();
      });
    });

    it('should switch to advanced tab when clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      const advancedTab = screen.getByRole('tab', {name: /advanced/i});
      await user.click(advancedTab);

      await waitFor(() => {
        expect(screen.getByTestId('edit-advanced-settings')).toBeInTheDocument();
      });
    });

    it('should not pass allowedGrantTypes to EditAdvancedSettings for non-mcp-client templates', async () => {
      const user = userEvent.setup();
      renderComponent();

      const advancedTab = screen.getByRole('tab', {name: /advanced/i});
      await user.click(advancedTab);

      await waitFor(() => {
        expect(screen.getByTestId('edit-advanced-settings')).toBeInTheDocument();
      });

      const lastCallProps = vi.mocked(EditAdvancedSettings).mock.calls.at(-1)?.[0];
      expect(lastCallProps?.allowedGrantTypes).toBeUndefined();
    });
  });

  describe('Inline Editing', () => {
    it('should enable name editing when edit icon is clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      const nameSection = screen.getByText('Test Application').closest('div');
      const editButton = nameSection?.querySelector('button');
      expect(editButton).toBeInTheDocument();

      await user.click(editButton!);

      const nameInput = screen.getByRole('textbox');
      expect(nameInput).toHaveValue('Test Application');
    });

    it('should save name changes on blur', async () => {
      const user = userEvent.setup();
      renderComponent();

      // Click edit button
      const nameSection = screen.getByText('Test Application').closest('div');
      const editButton = nameSection?.querySelector('button');
      await user.click(editButton!);

      // Change name
      const nameInput = screen.getByRole('textbox');
      await user.clear(nameInput);
      await user.type(nameInput, 'Updated Application');

      // Blur to save
      await user.tab();

      await waitFor(() => {
        expect(screen.getByText('Updated Application')).toBeInTheDocument();
      });
    });

    it('should save name changes on Enter key', async () => {
      const user = userEvent.setup();
      renderComponent();

      // Click edit button
      const nameSection = screen.getByText('Test Application').closest('div');
      const editButton = nameSection?.querySelector('button');
      await user.click(editButton!);

      // Change name and press Enter
      const nameInput = screen.getByRole('textbox');
      await user.clear(nameInput);
      await user.type(nameInput, 'Updated Application{Enter}');

      await waitFor(() => {
        expect(screen.getByText('Updated Application')).toBeInTheDocument();
      });
    });

    it('should cancel name editing on Escape key', async () => {
      const user = userEvent.setup();
      renderComponent();

      // Click edit button
      const nameSection = screen.getByText('Test Application').closest('div');
      const editButton = nameSection?.querySelector('button');
      await user.click(editButton!);

      // Change name and press Escape
      const nameInput = screen.getByRole('textbox');
      await user.clear(nameInput);
      await user.type(nameInput, 'Updated Application{Escape}');

      await waitFor(() => {
        expect(screen.getByText('Test Application')).toBeInTheDocument();
        expect(screen.queryByDisplayValue('Updated Application')).not.toBeInTheDocument();
      });
    });

    it('should enable description editing when edit icon is clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      const descriptionSection = screen.getByText('Test application description').closest('div');
      const editButton = descriptionSection?.querySelector('button');
      expect(editButton).toBeInTheDocument();

      await user.click(editButton!);

      const descriptionInput = screen.getByPlaceholderText('Add a description');
      expect(descriptionInput).toHaveValue('Test application description');
    });

    it('should save description changes on blur', async () => {
      const user = userEvent.setup();
      renderComponent();

      // Click edit button
      const descriptionSection = screen.getByText('Test application description').closest('div');
      const editButton = descriptionSection?.querySelector('button');
      await user.click(editButton!);

      // Change description
      const descriptionInput = screen.getByPlaceholderText('Add a description');
      await user.clear(descriptionInput);
      await user.type(descriptionInput, 'Updated description');

      // Blur to save
      fireEvent.blur(descriptionInput);

      await waitFor(() => {
        expect(screen.getByText('Updated description')).toBeInTheDocument();
      });
    });

    it('should save description changes on Ctrl+Enter', async () => {
      const user = userEvent.setup();
      renderComponent();

      // Click edit button
      const descriptionSection = screen.getByText('Test application description').closest('div');
      const editButton = descriptionSection?.querySelector('button');
      await user.click(editButton!);

      // Change description
      const descriptionInput = screen.getByPlaceholderText('Add a description');
      await user.clear(descriptionInput);
      await user.type(descriptionInput, 'Description via Ctrl+Enter');

      // Press Ctrl+Enter to save
      await user.keyboard('{Control>}{Enter}{/Control}');

      await waitFor(() => {
        expect(screen.getByText('Description via Ctrl+Enter')).toBeInTheDocument();
      });
    });

    it('should cancel description editing on Escape key', async () => {
      const user = userEvent.setup();
      renderComponent();

      // Click edit button
      const descriptionSection = screen.getByText('Test application description').closest('div');
      const editButton = descriptionSection?.querySelector('button');
      await user.click(editButton!);

      // Change description and press Escape
      const descriptionInput = screen.getByPlaceholderText('Add a description');
      await user.clear(descriptionInput);
      await user.type(descriptionInput, 'Changed description');
      await user.keyboard('{Escape}');

      await waitFor(() => {
        expect(screen.getByText('Test application description')).toBeInTheDocument();
        expect(screen.queryByDisplayValue('Changed description')).not.toBeInTheDocument();
      });
    });

    it('should not save empty name on blur', async () => {
      const user = userEvent.setup();
      renderComponent();

      // Click edit button
      const nameSection = screen.getByText('Test Application').closest('div');
      const editButton = nameSection?.querySelector('button');
      await user.click(editButton!);

      // Clear name
      const nameInput = screen.getByRole('textbox');
      await user.clear(nameInput);

      // Blur without entering text
      await user.tab();

      await waitFor(() => {
        expect(screen.getByText('Test Application')).toBeInTheDocument();
      });
    });
  });

  describe('Logo Update', () => {
    it('should render logo update modal', () => {
      renderComponent();

      // Modal should be in the DOM (hidden by default)
      expect(screen.getByTestId('emoji-picker')).toBeInTheDocument();
    });

    it('should open logo modal when avatar is clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      // Click on the avatar to open the modal
      const avatar = screen.getByRole('img');
      await user.click(avatar);

      await waitFor(() => {
        const modal = screen.getByTestId('emoji-picker');
        expect(modal).toHaveStyle({display: 'block'});
      });
    });

    it('should update logo and close modal when logo is updated', async () => {
      const user = userEvent.setup();
      renderComponent();

      // Open the modal
      const avatar = screen.getByRole('img');
      await user.click(avatar);

      // Click update logo button in modal
      const modal = screen.getByTestId('emoji-picker');
      const selectIconButton = within(modal).getByRole('button', {name: /select icon/i});
      await user.click(selectIconButton);

      await waitFor(() => {
        // Should show unsaved changes since logo was updated
        expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
      });

      // Modal should be closed
      await waitFor(() => {
        const closedModal = screen.getByTestId('emoji-picker');
        expect(closedModal).toHaveStyle({display: 'none'});
      });
    });

    it('should close logo modal when close button is clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      // Open the modal
      const avatar = screen.getByRole('img');
      await user.click(avatar);

      // Click close button
      const closeButton = screen.getByRole('button', {name: /close/i});
      await user.click(closeButton);

      await waitFor(() => {
        const modal = screen.getByTestId('emoji-picker');
        expect(modal).toHaveStyle({display: 'none'});
      });
    });
  });

  describe('Save Functionality', () => {
    it('should show floating action bar when changes are made', async () => {
      const user = userEvent.setup();
      renderComponent();

      // Make a change to name
      const nameSection = screen.getByText('Test Application').closest('div');
      const editButton = nameSection?.querySelector('button');
      await user.click(editButton!);

      const nameInput = screen.getByRole('textbox');
      await user.clear(nameInput);
      await user.type(nameInput, 'Updated Application{Enter}');

      await waitFor(() => {
        expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
      });
    });

    it('keeps save enabled for a user-facing client with a redirect URI but no allowed user types', async () => {
      const user = userEvent.setup();
      mockUseGetApplication.mockReturnValue({
        data: {...mockApplication, allowedUserTypes: []},
        isLoading: false,
        isError: false,
        error: null,
      } as unknown as UseQueryResult<Application>);
      renderComponent();

      const nameSection = screen.getByText('Test Application').closest('div');
      const editButton = nameSection?.querySelector('button');
      await user.click(editButton!);
      const nameInput = screen.getByRole('textbox');
      await user.clear(nameInput);
      await user.type(nameInput, 'Updated Application{Enter}');

      await waitFor(() => {
        expect(screen.getByRole('button', {name: /save changes/i})).toBeEnabled();
      });
    });

    it('disables save with a named issue when authorization_code has no redirect URI', async () => {
      const user = userEvent.setup();
      mockUseGetApplication.mockReturnValue({
        data: {
          ...mockApplication,
          inboundAuthConfig: [
            {type: 'oauth2', config: {grantTypes: ['authorization_code'], responseTypes: ['code'], redirectUris: []}},
          ],
        },
        isLoading: false,
        isError: false,
        error: null,
      } as unknown as UseQueryResult<Application>);
      renderComponent();

      const nameSection = screen.getByText('Test Application').closest('div');
      const editButton = nameSection?.querySelector('button');
      await user.click(editButton!);
      const nameInput = screen.getByRole('textbox');
      await user.clear(nameInput);
      await user.type(nameInput, 'Updated Application{Enter}');

      await waitFor(() => {
        expect(screen.getByText(/A redirect URI is required\./)).toBeInTheDocument();
      });
      expect(screen.getByRole('button', {name: /save changes/i})).toBeDisabled();
    });

    it('freezes the flows tab when no user-facing grant is granted', async () => {
      const user = userEvent.setup();
      mockUseGetApplication.mockReturnValue({
        data: {
          ...mockApplication,
          inboundAuthConfig: [{type: 'oauth2', config: {grantTypes: ['client_credentials'], responseTypes: []}}],
        },
        isLoading: false,
        isError: false,
        error: null,
      } as unknown as UseQueryResult<Application>);
      renderComponent();

      await user.click(screen.getByRole('tab', {name: /flows/i}));

      expect(
        screen.getByText(
          'These settings apply only to user-facing flows. Enable a user-facing grant (e.g. authorization code) in the Advanced tab to configure them.',
        ),
      ).toBeInTheDocument();
    });

    it('should display reset and save buttons in action bar', async () => {
      const user = userEvent.setup();
      renderComponent();

      // Make a change
      const nameSection = screen.getByText('Test Application').closest('div');
      const editButton = nameSection?.querySelector('button');
      await user.click(editButton!);

      const nameInput = screen.getByRole('textbox');
      await user.clear(nameInput);
      await user.type(nameInput, 'Updated Application{Enter}');

      await waitFor(() => {
        expect(screen.getByRole('button', {name: /reset/i})).toBeInTheDocument();
        expect(screen.getByRole('button', {name: /save changes/i})).toBeInTheDocument();
      });
    });

    it('should hide the action bar when a field is manually retyped back to its original value', async () => {
      const user = userEvent.setup();
      renderComponent();

      await editInlineField(user, 'Test Application', 'Updated Application');

      await waitFor(() => {
        expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
      });

      // Retype the exact original value
      await editInlineField(user, 'Updated Application', 'Test Application');

      await waitFor(() => {
        expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
      });
    });

    it('should discard a rename that exceeds the maximum length', async () => {
      const user = userEvent.setup();
      renderComponent();

      const section = screen.getByText('Test Application').closest('div');
      await user.click(section!.querySelector('button')!);
      const input = screen.getByRole('textbox');
      fireEvent.change(input, {target: {value: 'a'.repeat(ApplicationConstants.NAME_MAX_LENGTH + 1)}});
      fireEvent.keyDown(input, {key: 'Enter'});

      await waitFor(() => {
        expect(screen.getByText('Test Application')).toBeInTheDocument();
      });
      expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
    });

    it('should keep the action bar visible if only one of two edited fields is reverted', async () => {
      const user = userEvent.setup();
      renderComponent();

      // Edit name
      await editInlineField(user, 'Test Application', 'Updated Application');

      // Edit description
      await editInlineField(user, 'Test application description', 'Updated description');

      await waitFor(() => {
        expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
      });

      // Revert only the name
      await editInlineField(user, 'Updated Application', 'Test Application');

      // Description is still changed, so the bar must stay visible
      await waitFor(() => {
        expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
      });
    });

    it('should reset changes when reset button is clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      // Make a change
      const nameSection = screen.getByText('Test Application').closest('div');
      const editButton = nameSection?.querySelector('button');
      await user.click(editButton!);

      const nameInput = screen.getByRole('textbox');
      await user.clear(nameInput);
      await user.type(nameInput, 'Updated Application{Enter}');

      // Click reset
      await waitFor(() => {
        expect(screen.getByRole('button', {name: /reset/i})).toBeInTheDocument();
      });

      const resetButton = screen.getByRole('button', {name: /reset/i});
      await user.click(resetButton);

      await waitFor(() => {
        expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
      });
    });

    it('should bump sectionResetKey passed to EditAccessSettings when reset is clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      // EditAccessSettings only mounts once its TabPanel is active
      await mountSectionTab(user, 'Access', 'edit-access-settings');

      const {initialKey, keyAfterReset} = await editFieldAndResetSection(
        user,
        EditAccessSettings,
        'Test Application',
        'Updated Application',
      );
      expect(keyAfterReset).toBe((initialKey ?? 0) + 1);
    });

    it('should bump sectionResetKey passed to EditCustomizationSettings when reset is clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      // EditCustomizationSettings only mounts once its TabPanel is active
      await mountSectionTab(user, 'Customization', 'edit-customization-settings');

      const {initialKey, keyAfterReset} = await editFieldAndResetSection(
        user,
        EditCustomizationSettings,
        'Test Application',
        'Updated Application',
      );
      expect(keyAfterReset).toBe((initialKey ?? 0) + 1);
    });

    it('should bump sectionResetKey passed to EditTokenSettingsTabs when reset is clicked', async () => {
      const user = userEvent.setup();
      renderComponent();

      // EditTokenSettingsTabs only mounts once its TabPanel is active
      await mountSectionTab(user, 'Token', 'edit-token-settings');

      const {initialKey, keyAfterReset} = await editFieldAndResetSection(
        user,
        EditTokenSettingsTabs,
        'Test Application',
        'Updated Application',
      );
      expect(keyAfterReset).toBe((initialKey ?? 0) + 1);
    });

    it('should save changes when save button is clicked', async () => {
      const user = userEvent.setup();
      const mockMutateAsync = vi.fn().mockResolvedValue(mockApplication);

      mockUseUpdateApplication.mockReturnValue({
        mutate: mockUpdateApplicationMutate,
        mutateAsync: mockMutateAsync,
        isPending: false,
        isError: false,
        error: null,
        reset: vi.fn(),
      } as unknown as UseMutationResult<Application, Error, Partial<Application>>);

      renderComponent();

      // Make a change
      const nameSection = screen.getByText('Test Application').closest('div');
      const editButton = nameSection?.querySelector('button');
      await user.click(editButton!);

      const nameInput = screen.getByRole('textbox');
      await user.clear(nameInput);
      await user.type(nameInput, 'Updated Application{Enter}');

      // Click save
      await waitFor(() => {
        expect(screen.getByRole('button', {name: /save changes/i})).toBeInTheDocument();
      });

      const saveButton = screen.getByRole('button', {name: /save changes/i});
      await user.click(saveButton);

      await waitFor(() => {
        expect(mockMutateAsync).toHaveBeenCalled();
        const callArgs = mockMutateAsync.mock.calls[0][0] as {applicationId: string; data: Partial<Application>};
        expect(callArgs).toHaveProperty('applicationId', 'test-app-id');
        expect(callArgs).toHaveProperty('data');
        expect(callArgs.data).toHaveProperty('name', 'Updated Application');
      });
    });

    it('should bump sectionResetKey passed to EditAccessSettings when save succeeds', async () => {
      const user = userEvent.setup();
      mockUseUpdateApplication.mockReturnValue({
        mutate: mockUpdateApplicationMutate,
        mutateAsync: vi.fn().mockResolvedValue(mockApplication),
        isPending: false,
        isError: false,
        error: null,
        reset: vi.fn(),
      } as unknown as UseMutationResult<Application, Error, Partial<Application>>);

      // The bump only fires after refetch() resolves
      mockUseGetApplication.mockReturnValue({
        data: mockApplication,
        isLoading: false,
        isError: false,
        error: null,
        refetch: vi.fn().mockResolvedValue({data: mockApplication}),
      } as unknown as UseQueryResult<Application>);

      renderComponent();

      // EditAccessSettings only mounts once its TabPanel is active
      await mountSectionTab(user, 'Access', 'edit-access-settings');

      const initialKey = vi.mocked(EditAccessSettings).mock.calls.at(-1)?.[0].sectionResetKey;

      await editInlineField(user, 'Test Application', 'Updated Application');

      await waitFor(() => {
        expect(screen.getByRole('button', {name: /save changes/i})).toBeInTheDocument();
      });

      await user.click(screen.getByRole('button', {name: /save changes/i}));

      await waitFor(() => {
        const keyAfterSave = vi.mocked(EditAccessSettings).mock.calls.at(-1)?.[0].sectionResetKey;
        expect(keyAfterSave).toBe((initialKey ?? 0) + 1);
      });
    });

    it('should disable save button while saving', async () => {
      const user = userEvent.setup();

      mockUseUpdateApplication.mockReturnValue({
        mutate: mockUpdateApplicationMutate,
        mutateAsync: vi.fn().mockResolvedValue(mockApplication),
        isPending: true,
        isError: false,
        error: null,
        reset: vi.fn(),
      } as unknown as UseMutationResult<Application, Error, Partial<Application>>);

      renderComponent();

      // Make a change first to show the action bar
      const nameSection = screen.getByText('Test Application').closest('div');
      const editButton = nameSection?.querySelector('button');
      await user.click(editButton!);

      const nameInput = screen.getByRole('textbox');
      await user.clear(nameInput);
      await user.type(nameInput, 'Updated Application{Enter}');

      await waitFor(() => {
        const saveButton = screen.getByRole('button', {name: /saving/i});
        expect(saveButton).toBeDisabled();
      });
    });

    it('should hide action bar after successful save', async () => {
      const user = userEvent.setup();
      const mockMutateAsync = vi.fn().mockResolvedValue({...mockApplication, name: 'Updated Application'});

      mockUseUpdateApplication.mockReturnValue({
        mutate: mockUpdateApplicationMutate,
        mutateAsync: mockMutateAsync,
        isPending: false,
        isError: false,
        error: null,
        reset: vi.fn(),
      } as unknown as UseMutationResult<Application, Error, Partial<Application>>);

      renderComponent();

      // Make a change
      const nameSection = screen.getByText('Test Application').closest('div');
      const editButton = nameSection?.querySelector('button');
      await user.click(editButton!);

      const nameInput = screen.getByRole('textbox');
      await user.clear(nameInput);
      await user.type(nameInput, 'Updated Application{Enter}');

      // Wait for action bar to appear
      await waitFor(() => {
        expect(screen.getByRole('button', {name: /save changes/i})).toBeInTheDocument();
      });

      // Click save
      const saveButton = screen.getByRole('button', {name: /save changes/i});
      await user.click(saveButton);

      await waitFor(
        () => {
          expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
        },
        {timeout: 10000},
      );
    });
  });

  describe('Accessibility', () => {
    it('should have proper ARIA labels for tabs', () => {
      renderComponent();

      const accessTab = screen.getByRole('tab', {name: /access/i});
      expect(accessTab).toHaveAttribute('id');
      expect(accessTab).toHaveAttribute('aria-controls');
    });

    it('should show editable input during inline editing', async () => {
      const user = userEvent.setup();
      renderComponent();

      const nameSection = screen.getByText('Test Application').closest('div');
      const editButton = nameSection?.querySelector('button');
      await user.click(editButton!);

      expect(screen.getByRole('textbox')).toBeInTheDocument();
    });
  });

  describe('Application Not Found', () => {
    it('should display warning when application is null', () => {
      mockUseGetApplication.mockReturnValue({
        data: null,
        isLoading: false,
        isError: false,
        error: null,
      } as unknown as UseQueryResult<Application>);

      renderComponent();

      expect(screen.getByText('applications:edit.page.notFound')).toBeInTheDocument();
      expect(screen.getByRole('button', {name: /back to applications/i})).toBeInTheDocument();
    });

    it('should navigate back when back button is clicked in not found state', () => {
      mockUseGetApplication.mockReturnValue({
        data: null,
        isLoading: false,
        isError: false,
        error: null,
      } as unknown as UseQueryResult<Application>);

      renderComponent();

      fireEvent.click(screen.getByRole('button', {name: /back to applications/i}));

      // Button should still be present after click (navigation is async)
      expect(screen.getByRole('button', {name: /back to applications/i})).toBeInTheDocument();
    });
  });

  describe('Error Handling', () => {
    it('should resolve a mapped error code instead of raw server text', () => {
      mockUseGetApplication.mockReturnValue({
        data: undefined,
        isLoading: false,
        isError: true,
        error: {message: 'raw server text', response: {data: {code: 'APP-1020'}}},
      } as unknown as UseQueryResult<Application>);

      renderComponent();

      expect(
        screen.getByText('An application with this name already exists. Choose a different name.'),
      ).toBeInTheDocument();
      expect(screen.queryByText('raw server text')).not.toBeInTheDocument();
    });

    it('should display a generic fallback message when the error has no mapped code', () => {
      mockUseGetApplication.mockReturnValue({
        data: undefined,
        isLoading: false,
        isError: true,
        error: {},
      } as unknown as UseQueryResult<Application>);

      renderComponent();

      expect(screen.getByText('Something went wrong')).toBeInTheDocument();
    });

    it('should handle save failure gracefully', async () => {
      const user = userEvent.setup();
      const mockMutateAsync = vi.fn().mockRejectedValue(new Error('Save failed'));

      mockUseUpdateApplication.mockReturnValue({
        mutate: mockUpdateApplicationMutate,
        mutateAsync: mockMutateAsync,
        isPending: false,
        isError: false,
        error: null,
        reset: vi.fn(),
      } as unknown as UseMutationResult<Application, Error, Partial<Application>>);

      renderComponent();

      // Make a change
      const nameSection = screen.getByText('Test Application').closest('div');
      const editButton = nameSection?.querySelector('button');
      await user.click(editButton!);

      const nameInput = screen.getByRole('textbox');
      await user.clear(nameInput);
      await user.type(nameInput, 'Updated Application{Enter}');

      // Click save
      await waitFor(() => {
        expect(screen.getByRole('button', {name: /save changes/i})).toBeInTheDocument();
      });

      const saveButton = screen.getByRole('button', {name: /save changes/i});
      await user.click(saveButton);

      // Should have called mutateAsync (even if it failed)
      await waitFor(() => {
        expect(mockMutateAsync).toHaveBeenCalled();
      });
    });

    it('should not save when application or applicationId is missing', async () => {
      const user = userEvent.setup();

      // Mock useParams to return undefined applicationId
      const {useParams} = await import('react-router');
      (useParams as ReturnType<typeof vi.fn>).mockReturnValue({applicationId: undefined});

      const mockMutateAsync = vi.fn().mockResolvedValue(mockApplication);
      mockUseUpdateApplication.mockReturnValue({
        mutate: mockUpdateApplicationMutate,
        mutateAsync: mockMutateAsync,
        isPending: false,
        isError: false,
        error: null,
        reset: vi.fn(),
      } as unknown as UseMutationResult<Application, Error, Partial<Application>>);

      renderComponent();

      // Make a change to trigger the floating save bar
      const nameSection = screen.getByText('Test Application').closest('div');
      const editButton = nameSection?.querySelector('button');
      await user.click(editButton!);

      const nameInput = screen.getByRole('textbox');
      await user.clear(nameInput);
      await user.type(nameInput, 'Updated Application{Enter}');

      // Click save — handleSave should return early due to missing applicationId
      await waitFor(() => {
        expect(screen.getByRole('button', {name: /save changes/i})).toBeInTheDocument();
      });

      const saveButton = screen.getByRole('button', {name: /save changes/i});
      await user.click(saveButton);

      // mutateAsync should not have been called since applicationId is missing
      expect(mockMutateAsync).not.toHaveBeenCalled();

      // Restore original mock
      (useParams as ReturnType<typeof vi.fn>).mockReturnValue({applicationId: 'test-app-id'});
    });

    it('should display an update error inline in the save bar, not a toast', async () => {
      const user = userEvent.setup();
      const mockReset = vi.fn();

      mockUseUpdateApplication.mockReturnValue({
        mutate: mockUpdateApplicationMutate,
        mutateAsync: vi.fn(),
        isPending: false,
        isError: true,
        error: Object.assign(new Error('Request failed'), {response: {data: {code: 'APP-1020'}}}),
        reset: mockReset,
      } as unknown as UseMutationResult<Application, Error, Partial<Application>>);

      renderComponent();

      // Make a change to trigger the floating save bar
      const nameSection = screen.getByText('Test Application').closest('div');
      const editButton = nameSection?.querySelector('button');
      await user.click(editButton!);

      const nameInput = screen.getByRole('textbox');
      await user.clear(nameInput);
      await user.type(nameInput, 'Updated Application{Enter}');

      await waitFor(() => {
        expect(
          screen.getByText('An application with this name already exists. Choose a different name.'),
        ).toBeInTheDocument();
      });
    });

    it('should clear the update error when Reset is clicked', async () => {
      const user = userEvent.setup();
      const mockReset = vi.fn();

      mockUseUpdateApplication.mockReturnValue({
        mutate: mockUpdateApplicationMutate,
        mutateAsync: vi.fn(),
        isPending: false,
        isError: true,
        error: new Error('Request failed'),
        reset: mockReset,
      } as unknown as UseMutationResult<Application, Error, Partial<Application>>);

      renderComponent();

      const nameSection = screen.getByText('Test Application').closest('div');
      const editButton = nameSection?.querySelector('button');
      await user.click(editButton!);

      const nameInput = screen.getByRole('textbox');
      await user.clear(nameInput);
      await user.type(nameInput, 'Updated Application{Enter}');

      await waitFor(() => {
        expect(screen.getByRole('button', {name: /reset/i})).toBeInTheDocument();
      });

      const resetButton = screen.getByRole('button', {name: /reset/i});
      await user.click(resetButton);

      expect(mockReset).toHaveBeenCalled();
    });

    it('should clear a stale update error as soon as another field changes', async () => {
      const user = userEvent.setup();
      const mockReset = vi.fn();

      mockUseUpdateApplication.mockReturnValue({
        mutate: mockUpdateApplicationMutate,
        mutateAsync: vi.fn(),
        isPending: false,
        isError: true,
        error: new Error('Request failed'),
        reset: mockReset,
      } as unknown as UseMutationResult<Application, Error, Partial<Application>>);

      renderComponent();

      const nameSection = screen.getByText('Test Application').closest('div');
      const editButton = nameSection?.querySelector('button');
      await user.click(editButton!);

      const nameInput = screen.getByRole('textbox');
      await user.clear(nameInput);
      await user.type(nameInput, 'Updated Application{Enter}');

      expect(mockReset).toHaveBeenCalled();
    });

    it('should clear a stale update error when the logo changes', async () => {
      const user = userEvent.setup();
      const mockReset = vi.fn();

      mockUseUpdateApplication.mockReturnValue({
        mutate: mockUpdateApplicationMutate,
        mutateAsync: vi.fn(),
        isPending: false,
        isError: true,
        error: new Error('Request failed'),
        reset: mockReset,
      } as unknown as UseMutationResult<Application, Error, Partial<Application>>);

      renderComponent();

      // The logo goes through ResourceAvatar.onSelect, which sets editedApp directly rather than
      // going through handleFieldChange, so it needs its own reset.
      await user.click(screen.getByRole('img'));
      const modal = screen.getByTestId('emoji-picker');
      await user.click(within(modal).getByRole('button', {name: /select icon/i}));

      expect(mockReset).toHaveBeenCalled();
    });
  });

  describe('Logo Image Error Handling', () => {
    it('should handle logo image loading error', () => {
      renderComponent();

      const logo = screen.getByRole('img');

      // Simulate image load error
      logo.dispatchEvent(new Event('error'));

      // The component should still be functional
      expect(screen.getByText('Test Application')).toBeInTheDocument();
    });
  });

  describe('Template Metadata', () => {
    it('should not display template chip when template metadata is null', () => {
      mockGetTemplateMetadata.mockReturnValue(null);

      renderComponent();

      expect(screen.queryByText('React')).not.toBeInTheDocument();
    });

    it('should handle application without template', () => {
      mockUseGetApplication.mockReturnValue({
        data: {...mockApplication, template: undefined},
        isLoading: false,
        isError: false,
        error: null,
      } as unknown as UseQueryResult<Application>);

      renderComponent();

      // Should render without crashing
      expect(screen.getByText('Test Application')).toBeInTheDocument();
    });
  });

  describe('OAuth2 Config', () => {
    it('should handle application without inboundAuthConfig', () => {
      mockUseGetApplication.mockReturnValue({
        data: {...mockApplication, inboundAuthConfig: undefined},
        isLoading: false,
        isError: false,
        error: null,
      } as unknown as UseQueryResult<Application>);

      renderComponent();

      // Should render without crashing
      expect(screen.getByText('Test Application')).toBeInTheDocument();
    });

    it('should handle application with non-oauth2 inboundAuthConfig', () => {
      mockUseGetApplication.mockReturnValue({
        data: {
          ...mockApplication,
          inboundAuthConfig: [{type: 'saml', config: {issuer: 'test'}}],
        },
        isLoading: false,
        isError: false,
        error: null,
      } as unknown as UseQueryResult<Application>);

      renderComponent();

      // Should render without crashing
      expect(screen.getByText('Test Application')).toBeInTheDocument();
    });
  });

  describe('Name and Description Editing Edge Cases', () => {
    it('should not save empty name on Enter', async () => {
      const user = userEvent.setup();
      renderComponent();

      // Click edit button
      const nameSection = screen.getByText('Test Application').closest('div');
      const editButton = nameSection?.querySelector('button');
      await user.click(editButton!);

      // Clear and press Enter
      const nameInput = screen.getByRole('textbox');
      await user.clear(nameInput);
      await user.keyboard('{Enter}');

      await waitFor(() => {
        // Original name should be preserved
        expect(screen.getByText('Test Application')).toBeInTheDocument();
      });
    });

    it('should save empty description when cleared', async () => {
      const user = userEvent.setup();
      renderComponent();

      // Click edit button for description
      const descriptionSection = screen.getByText('Test application description').closest('div');
      const editButton = descriptionSection?.querySelector('button');
      await user.click(editButton!);

      // Clear description and blur
      const descriptionInput = screen.getByPlaceholderText('Add a description');
      await user.clear(descriptionInput);
      await user.tab();

      await waitFor(() => {
        // Should show unsaved changes indicator
        expect(screen.getByText('You have unsaved changes')).toBeInTheDocument();
      });
    });

    it('does not raise an unsaved-changes diff when description editor is opened and closed without changes', async () => {
      const user = userEvent.setup();
      renderComponent();

      // Open the description editor (description is non-empty: 'Test application description')
      const descriptionSection = screen.getByText('Test application description').closest('div');
      const editButton = descriptionSection?.querySelector('button');
      await user.click(editButton!);

      // Blur immediately without typing — value is unchanged.
      const descriptionInput = screen.getByPlaceholderText('Add a description');
      fireEvent.blur(descriptionInput);

      // No diff should be created → no unsaved-changes bar.
      expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
    });

    it('should handle description when original is empty and new is empty', async () => {
      mockUseGetApplication.mockReturnValue({
        data: {...mockApplication, description: undefined},
        isLoading: false,
        isError: false,
        error: null,
      } as UseQueryResult<Application>);

      const user = userEvent.setup();
      renderComponent();

      // Click edit button for description - when description is undefined, the component shows 'No description provided'
      const descriptionSection = screen.getByText('No description provided').closest('div');
      const editButton = descriptionSection?.querySelector('button');
      await user.click(editButton!);

      // Just blur without typing
      const descriptionInput = screen.getByPlaceholderText('Add a description');
      await user.click(descriptionInput);
      await user.tab();

      // Should not show unsaved changes since nothing changed
      expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
    });
  });

  describe('Edit Icon Click for Logo', () => {
    it('should open logo modal when edit icon button is clicked', async () => {
      renderComponent();

      fireEvent.click(screen.getByRole('button', {name: 'Update Logo'}));

      await waitFor(() => {
        expect(screen.getByTestId('emoji-picker')).toHaveStyle({display: 'block'});
      });
    });
  });

  describe('Avatar Image Error', () => {
    it('should hide avatar image when image fails to load', () => {
      renderComponent();

      const avatar = screen.getByRole('img');

      // Simulate image load error via the onError handler
      fireEvent.error(avatar);

      // The image should be hidden
      expect(avatar).toHaveStyle({display: 'none'});
    });
  });

  describe('Edited App Fallbacks', () => {
    it('should display edited name when editedApp has name', async () => {
      const user = userEvent.setup();
      renderComponent();

      // Edit the name
      const nameSection = screen.getByText('Test Application').closest('div');
      const editButton = nameSection?.querySelector('button');
      await user.click(editButton!);

      const nameInput = screen.getByRole('textbox');
      await user.clear(nameInput);
      await user.type(nameInput, 'New App Name{Enter}');

      await waitFor(() => {
        expect(screen.getByText('New App Name')).toBeInTheDocument();
      });

      // Now click edit again - tempName should be set from editedApp.name
      const updatedNameSection = screen.getByText('New App Name').closest('div');
      const editButtonAgain = updatedNameSection?.querySelector('button');
      await user.click(editButtonAgain!);

      const nameInputAgain = screen.getByRole('textbox');
      expect(nameInputAgain).toHaveValue('New App Name');
    });

    it('should display edited description when editedApp has description', async () => {
      const user = userEvent.setup();
      renderComponent();

      // Edit description
      const descriptionSection = screen.getByText('Test application description').closest('div');
      const editButton = descriptionSection?.querySelector('button');
      await user.click(editButton!);

      const descriptionInput = screen.getByPlaceholderText('Add a description');
      await user.clear(descriptionInput);
      await user.type(descriptionInput, 'New description');
      fireEvent.blur(descriptionInput);

      await waitFor(() => {
        expect(screen.getByText('New description')).toBeInTheDocument();
      });

      // Click edit again - tempDescription should use editedApp.description
      const updatedSection = screen.getByText('New description').closest('div');
      const editButtonAgain = updatedSection?.querySelector('button');
      await user.click(editButtonAgain!);

      const descriptionInputAgain = screen.getByPlaceholderText('Add a description');
      expect(descriptionInputAgain).toHaveValue('New description');
    });

    it('should display edited logoUrl in avatar when editedApp has logoUrl', async () => {
      const user = userEvent.setup();
      renderComponent();

      // Open modal and update logo
      const avatar = screen.getByRole('img');
      await user.click(avatar);

      const logoModal = screen.getByTestId('emoji-picker');
      const selectIconButton = within(logoModal).getByRole('button', {name: /select icon/i});
      await user.click(selectIconButton);

      await waitFor(() => {
        // Avatar now displays the selected emoji '🚀' (from mock)
        expect(screen.getByText('🚀')).toBeInTheDocument();
      });
    });

    it('should handle application with no logoUrl', () => {
      mockUseGetApplication.mockReturnValue({
        data: {...mockApplication, logoUrl: undefined},
        isLoading: false,
        isError: false,
        error: null,
      } as UseQueryResult<Application>);

      renderComponent();

      // Should render without crashing - avatar will use fallback icon
      expect(screen.getByText('Test Application')).toBeInTheDocument();
    });
  });

  describe('Tab Navigation with Integration Guides', () => {
    it('should switch to access tab when overview is first tab', async () => {
      const user = userEvent.setup();
      mockGetIntegrationGuidesForTemplate.mockReturnValue(['react-vite']);

      renderComponent();

      // Click Access tab (second tab when integration guides are present)
      const accessTab = screen.getByRole('tab', {name: /access/i});
      await user.click(accessTab);

      await waitFor(() => {
        expect(screen.getByTestId('edit-access-settings')).toBeInTheDocument();
      });
    });

    it('should switch to flows tab when integration guides exist', async () => {
      const user = userEvent.setup();
      mockGetIntegrationGuidesForTemplate.mockReturnValue(['react-vite']);

      renderComponent();

      const flowsTab = screen.getByRole('tab', {name: /flows/i});
      await user.click(flowsTab);

      await waitFor(() => {
        expect(screen.getByTestId('edit-flows-settings')).toBeInTheDocument();
      });
    });
  });

  describe('Description Escape with Edited Value', () => {
    it('should restore edited description on Escape key when editedApp has description', async () => {
      const user = userEvent.setup();
      renderComponent();

      // First, edit the description to set editedApp.description
      const descriptionSection = screen.getByText('Test application description').closest('div');
      const editButton = descriptionSection?.querySelector('button');
      await user.click(editButton!);

      const descriptionInput = screen.getByPlaceholderText('Add a description');
      await user.clear(descriptionInput);
      await user.type(descriptionInput, 'Edited description');
      fireEvent.blur(descriptionInput);

      await waitFor(() => {
        expect(screen.getByText('Edited description')).toBeInTheDocument();
      });

      // Now edit again and press Escape - should restore the editedApp.description
      const updatedSection = screen.getByText('Edited description').closest('div');
      const editButtonAgain = updatedSection?.querySelector('button');
      await user.click(editButtonAgain!);

      const descriptionInputAgain = screen.getByPlaceholderText('Add a description');
      await user.clear(descriptionInputAgain);
      await user.type(descriptionInputAgain, 'Something else');
      await user.keyboard('{Escape}');

      await waitFor(() => {
        // Should revert to the editedApp.description value
        expect(screen.getByText('Edited description')).toBeInTheDocument();
      });
    });
  });

  describe('Name Editing with Edited Value', () => {
    it('should restore edited name on Escape when editedApp has name', async () => {
      const user = userEvent.setup();
      renderComponent();

      // First, edit name to set editedApp.name
      const nameSection = screen.getByText('Test Application').closest('div');
      const editButton = nameSection?.querySelector('button');
      await user.click(editButton!);

      const nameInput = screen.getByRole('textbox');
      await user.clear(nameInput);
      await user.type(nameInput, 'Edited Name{Enter}');

      await waitFor(() => {
        expect(screen.getByText('Edited Name')).toBeInTheDocument();
      });

      // Edit again and press Escape - should restore editedApp.name
      const updatedNameSection = screen.getByText('Edited Name').closest('div');
      const editButtonAgain = updatedNameSection?.querySelector('button');
      await user.click(editButtonAgain!);

      const nameInputAgain = screen.getByRole('textbox');
      await user.clear(nameInputAgain);
      await user.type(nameInputAgain, 'Something else{Escape}');

      await waitFor(() => {
        expect(screen.getByText('Edited Name')).toBeInTheDocument();
      });
    });
  });

  describe('MCP Tab Set', () => {
    const mockMcpApplication: Application = {
      id: 'test-mcp-app-id',
      name: 'Test MCP Client',
      description: 'Test MCP client description',
      template: 'mcp-client',
      inboundAuthConfig: [
        {
          type: 'oauth2',
          config: {
            responseTypes: ['code'],
            clientId: 'test-mcp-client-id',
            grantTypes: ['authorization_code', 'refresh_token'],
            redirectUris: ['http://127.0.0.1:8080/callback'],
            pkceRequired: true,
            publicClient: true,
            tokenEndpointAuthMethod: 'none',
          },
        },
      ],
    };

    const mockMcpM2mApplication: Application = {
      ...mockMcpApplication,
      inboundAuthConfig: [
        {
          type: 'oauth2',
          config: {
            responseTypes: [],
            clientId: 'test-mcp-m2m-client-id',
            grantTypes: ['client_credentials'],
            tokenEndpointAuthMethod: 'client_secret_basic',
            publicClient: false,
          },
        },
      ],
    };

    it('renders the MCP tab set for an mcp-client template application', () => {
      mockUseGetApplication.mockReturnValue({
        data: mockMcpApplication,
        isLoading: false,
        isError: false,
        error: null,
      } as UseQueryResult<Application>);

      renderComponent();

      expect(screen.getByRole('tab', {name: 'General'})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: 'Flows'})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: 'Customization'})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: 'Token'})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: 'Advanced'})).toBeInTheDocument();
    });

    it('renders the Overview tab panel by default, and the General tab panel once selected', async () => {
      mockUseGetApplication.mockReturnValue({
        data: mockMcpApplication,
        isLoading: false,
        isError: false,
        error: null,
      } as UseQueryResult<Application>);

      const user = userEvent.setup();
      renderComponent();

      expect(screen.getByTestId('integration-guides')).toBeInTheDocument();
      expect(screen.queryByTestId('mcp-connect-tab')).not.toBeInTheDocument();

      await user.click(screen.getByRole('tab', {name: 'General'}));

      expect(screen.getByTestId('mcp-connect-tab')).toBeInTheDocument();
    });

    it('hides the Flows and Customization tabs for a machine-to-machine client', () => {
      mockUseGetApplication.mockReturnValue({
        data: mockMcpM2mApplication,
        isLoading: false,
        isError: false,
        error: null,
      } as UseQueryResult<Application>);

      renderComponent();

      expect(screen.getByRole('tab', {name: 'General'})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: 'Token'})).toBeInTheDocument();
      expect(screen.getByRole('tab', {name: 'Advanced'})).toBeInTheDocument();
      expect(screen.queryByRole('tab', {name: 'Flows'})).not.toBeInTheDocument();
      expect(screen.queryByRole('tab', {name: 'Customization'})).not.toBeInTheDocument();
    });

    it('passes allowedGrantTypes to EditAdvancedSettings for the mcp-client Advanced tab', async () => {
      mockUseGetApplication.mockReturnValue({
        data: mockMcpApplication,
        isLoading: false,
        isError: false,
        error: null,
      } as UseQueryResult<Application>);

      const user = userEvent.setup();
      renderComponent();

      await user.click(screen.getByRole('tab', {name: 'Advanced'}));

      await waitFor(() => {
        expect(screen.getByTestId('edit-advanced-settings')).toBeInTheDocument();
      });

      const lastCallProps = vi.mocked(EditAdvancedSettings).mock.calls.at(-1)?.[0];
      expect(lastCallProps?.allowedGrantTypes).toEqual(['authorization_code', 'refresh_token', 'client_credentials']);
    });

    it('should bump sectionResetKey passed to McpConnectTab when reset is clicked', async () => {
      mockUseGetApplication.mockReturnValue({
        data: mockMcpApplication,
        isLoading: false,
        isError: false,
        error: null,
      } as UseQueryResult<Application>);

      const user = userEvent.setup();
      renderComponent();

      // McpConnectTab only mounts once its General TabPanel is active
      await mountSectionTab(user, 'General', 'mcp-connect-tab');

      // Make a change via the page header, which is shared across MCP and non-MCP layouts
      const {initialKey, keyAfterReset} = await editFieldAndResetSection(
        user,
        McpConnectTab,
        'Test MCP Client',
        'Updated MCP Client',
      );
      expect(keyAfterReset).toBe((initialKey ?? 0) + 1);
    });

    it('should bump sectionResetKey passed to EditCustomizationSettings when reset is clicked in an mcp-client', async () => {
      mockUseGetApplication.mockReturnValue({
        data: mockMcpApplication,
        isLoading: false,
        isError: false,
        error: null,
      } as UseQueryResult<Application>);

      const user = userEvent.setup();
      renderComponent();

      // EditCustomizationSettings only mounts once its TabPanel is active
      await mountSectionTab(user, 'Customization', 'edit-customization-settings');

      // Make a change via the page header, which is shared across MCP and non-MCP layouts
      const {initialKey, keyAfterReset} = await editFieldAndResetSection(
        user,
        EditCustomizationSettings,
        'Test MCP Client',
        'Updated MCP Client',
      );
      expect(keyAfterReset).toBe((initialKey ?? 0) + 1);
    });

    it('does not remount EditTokenSettings, but bumps sectionResetKey, when reset is clicked in an mcp-client', async () => {
      mockUseGetApplication.mockReturnValue({
        data: mockMcpApplication,
        isLoading: false,
        isError: false,
        error: null,
      } as UseQueryResult<Application>);

      const user = userEvent.setup();
      renderComponent();

      // EditTokenSettings only mounts once its TabPanel is active
      await mountSectionTab(user, 'Token', 'edit-token-settings');

      await user.click(screen.getByTestId('edit-token-settings-bump'));
      expect(screen.getByTestId('edit-token-settings')).toHaveTextContent('Clicks: 1');

      // Make a change via the page header, which is shared across MCP and non-MCP layouts
      const {initialKey, keyAfterReset} = await editFieldAndResetSection(
        user,
        EditTokenSettings,
        'Test MCP Client',
        'Updated MCP Client',
      );
      expect(keyAfterReset).toBe((initialKey ?? 0) + 1);

      expect(screen.getByTestId('edit-token-settings')).toHaveTextContent('Clicks: 1');
    });

    it('locks the PKCE constraint for a user-delegated mcp-client Advanced tab', async () => {
      mockUseGetApplication.mockReturnValue({
        data: mockMcpApplication,
        isLoading: false,
        isError: false,
        error: null,
      } as UseQueryResult<Application>);

      const user = userEvent.setup();
      renderComponent();

      await user.click(screen.getByRole('tab', {name: 'Advanced'}));

      await waitFor(() => {
        expect(screen.getByTestId('edit-advanced-settings')).toBeInTheDocument();
      });

      const lastCallProps = vi.mocked(EditAdvancedSettings).mock.calls.at(-1)?.[0];
      expect(lastCallProps?.oauth2Constraints?.pkceRequired).toEqual({readOnly: true, value: true});
    });

    it('does not lock the PKCE constraint for an M2M-only mcp-client Advanced tab', async () => {
      mockUseGetApplication.mockReturnValue({
        data: mockMcpM2mApplication,
        isLoading: false,
        isError: false,
        error: null,
      } as UseQueryResult<Application>);

      const user = userEvent.setup();
      renderComponent();

      await user.click(screen.getByRole('tab', {name: 'Advanced'}));

      await waitFor(() => {
        expect(screen.getByTestId('edit-advanced-settings')).toBeInTheDocument();
      });

      const lastCallProps = vi.mocked(EditAdvancedSettings).mock.calls.at(-1)?.[0];
      expect(lastCallProps?.oauth2Constraints).toBeUndefined();
    });

    it('keeps the generic tab set unchanged for non-mcp-client templates', () => {
      // mockApplication (template: 'react') is the default mock from the outer describe block.
      renderComponent();

      expect(screen.getByRole('tab', {name: /access/i})).toBeInTheDocument();
      expect(screen.queryByTestId('mcp-connect-tab')).not.toBeInTheDocument();
    });

    it('disables the Save button when the MCP access section reports invalid', async () => {
      mockUseGetApplication.mockReturnValue({
        data: mockMcpApplication,
        isLoading: false,
        isError: false,
        error: null,
      } as UseQueryResult<Application>);

      const user = userEvent.setup();
      renderComponent();

      // McpConnectTab only mounts once its General TabPanel is active
      await mountSectionTab(user, 'General', 'mcp-connect-tab');

      await user.click(screen.getByTestId('mcp-connect-tab-report-invalid'));

      // Trigger a change so the Save bar is rendered.
      const nameSection = screen.getByText('Test MCP Client').closest('div');
      const editButton = nameSection?.querySelector('button');
      await user.click(editButton!);
      const nameInput = screen.getByRole('textbox');
      await user.clear(nameInput);
      await user.type(nameInput, 'Edited MCP Client{Enter}');

      await waitFor(() => {
        expect(screen.getByTestId('unsaved-changes-bar')).toBeInTheDocument();
      });

      expect(screen.getByRole('button', {name: 'Save Changes'})).toBeDisabled();
    });
  });

  describe('Client Secret Popup (just created)', () => {
    afterEach(async () => {
      const {useLocation} = await import('react-router');
      (useLocation as ReturnType<typeof vi.fn>).mockReturnValue({state: null});
    });

    it('should not render the secret dialog when there is no justCreatedSecret navigation state', () => {
      renderComponent();

      expect(screen.queryByTestId('application-show-client-secret')).not.toBeInTheDocument();
    });

    it('should render the secret dialog when justCreatedSecret is present in location state', async () => {
      const {useLocation} = await import('react-router');
      (useLocation as ReturnType<typeof vi.fn>).mockReturnValue({
        state: {
          justCreatedSecret: {
            appName: 'My New App',
            clientId: 'new-client-id',
            clientSecret: 'brand-new-secret',
          },
        },
      });

      renderComponent();

      expect(screen.getByTestId('application-show-client-secret')).toBeInTheDocument();
      expect(screen.getByDisplayValue('brand-new-secret')).toBeInTheDocument();
    });

    it('should close the secret dialog when Continue is clicked', async () => {
      const {useLocation} = await import('react-router');
      (useLocation as ReturnType<typeof vi.fn>).mockReturnValue({
        state: {
          justCreatedSecret: {
            appName: 'My New App',
            clientSecret: 'brand-new-secret',
          },
        },
      });

      renderComponent();

      const user = userEvent.setup();
      await user.click(screen.getByTestId('application-client-secret-continue'));

      await waitFor(() => {
        expect(screen.queryByTestId('application-show-client-secret')).not.toBeInTheDocument();
      });
    });
  });
});
