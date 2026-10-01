// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import userEvent from '@testing-library/user-event';
import {render, screen} from '@thunderid/test-utils';
import {describe, expect, it, vi, beforeEach} from 'vitest';
import TranslationsEditPage from '@/pages/TranslationsEditPage';

const mockNavigate = vi.fn();
vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
    useParams: () => ({language: 'fr-FR'}),
  };
});

vi.mock('@wso2/oxygen-ui', async () => {
  const actual = await vi.importActual<typeof import('@wso2/oxygen-ui')>('@wso2/oxygen-ui');
  return {
    ...actual,
    useColorScheme: () => ({mode: 'light', systemMode: 'light'}),
  };
});

const {mockMutateAsync, mockUseGetTranslations, mockUseUpdateTranslation} = vi.hoisted(() => ({
  mockMutateAsync: vi.fn(),
  mockUseGetTranslations: vi.fn(),
  mockUseUpdateTranslation: vi.fn(),
}));

vi.mock('@thunderid/i18n', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/i18n')>()),
  useGetTranslations: mockUseGetTranslations,
  useUpdateTranslation: mockUseUpdateTranslation,
  // UnsavedChangesBar is imported from the `@thunderid/components` barrel, which also re-exports
  // components that statically import useGetLanguages from this module — an ESM named export must
  // exist here even though nothing in this test exercises it.
  useGetLanguages: vi.fn(),
  NamespaceConstants: {
    CUSTOM_NAMESPACE: 'custom',
    COMMON: 'common',
    AUTH: 'auth',
    LOGIN_FLOW: 'loginFlow',
  },
  I18nDefaultConstants: {
    FALLBACK_LANGUAGE: 'en-US',
  },
}));

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({error: vi.fn(), info: vi.fn(), warn: vi.fn(), debug: vi.fn()}),
}));

// Stub child components to decouple from their internals
const mockTranslationEditorHeader = vi.fn();
const mockNamespaceSelector = vi.fn();
const mockTranslationEditorCard = vi.fn();

vi.mock('@/components/edit-translation/TranslationEditorHeader', () => ({
  default: (props: {
    onBack: () => void;
    onResetToDefault: () => void;
    isSaving: boolean;
    selectedLanguage: string | null;
    isFallbackLanguage: boolean;
    hasNamespace: boolean;
    error?: string;
  }) => {
    mockTranslationEditorHeader(props);
    return (
      <div data-testid="editor-header">
        <button type="button" onClick={props.onBack}>
          back
        </button>
        <button type="button" onClick={props.onResetToDefault} disabled={!props.hasNamespace || props.isSaving}>
          reset
        </button>
        <span data-testid="header-language">{props.selectedLanguage}</span>
        <span data-testid="header-is-english">{String(props.isFallbackLanguage)}</span>
        {props.error && <span data-testid="header-error">{props.error}</span>}
      </div>
    );
  },
}));

vi.mock('@/components/edit-translation/NamespaceSelector', () => ({
  default: (props: {value: string | null; onChange: (v: string) => void; loading: boolean}) => {
    mockNamespaceSelector(props);
    return (
      <div data-testid="namespace-selector">
        <button type="button" onClick={() => props.onChange('auth')}>
          select namespace
        </button>
        <span data-testid="ns-value">{props.value ?? ''}</span>
        <span data-testid="ns-loading">{String(props.loading)}</span>
      </div>
    );
  },
}));

vi.mock('@/components/edit-translation/TranslationEditorCard', () => ({
  default: (props: {
    selectedLanguage: string | null;
    isLoading: boolean;
    currentValues: Record<string, string>;
    onFieldChange: (key: string, value: string) => void;
    onResetField: (key: string) => void;
    onJsonChange: (changes: Record<string, string>) => void;
  }) => {
    mockTranslationEditorCard(props);
    return (
      <div data-testid="editor-card">
        <button type="button" onClick={() => props.onFieldChange('actions.save', 'Sauvegarder')}>
          change field
        </button>
        <button type="button" onClick={() => props.onResetField('actions.save')}>
          reset field
        </button>
        <span data-testid="card-language">{props.selectedLanguage}</span>
        <span data-testid="card-loading">{String(props.isLoading)}</span>
      </div>
    );
  },
}));

const sampleTranslations = {
  translations: {
    common: {'actions.save': 'Save', 'actions.cancel': 'Cancel'},
    auth: {'login.title': 'Login'},
  },
};

describe('TranslationsEditPage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseGetTranslations.mockReturnValue({
      data: sampleTranslations,
      isLoading: false,
    });
    mockUseUpdateTranslation.mockReturnValue({
      mutateAsync: mockMutateAsync.mockResolvedValue(undefined),
    });
  });

  describe('Rendering', () => {
    it('renders the editor header', () => {
      render(<TranslationsEditPage />);

      expect(screen.getByTestId('editor-header')).toBeInTheDocument();
    });

    it('renders the namespace selector', () => {
      render(<TranslationsEditPage />);

      expect(screen.getByTestId('namespace-selector')).toBeInTheDocument();
    });

    it('renders the editor card', () => {
      render(<TranslationsEditPage />);

      expect(screen.getByTestId('editor-card')).toBeInTheDocument();
    });

    it('passes the language from URL params to the editor header', () => {
      render(<TranslationsEditPage />);

      expect(screen.getByTestId('header-language')).toHaveTextContent('fr-FR');
    });

    it('passes the language from URL params to the editor card', () => {
      render(<TranslationsEditPage />);

      expect(screen.getByTestId('card-language')).toHaveTextContent('fr-FR');
    });

    it('initializes with the first namespace from the translation data', () => {
      render(<TranslationsEditPage />);

      expect(screen.getByTestId('ns-value')).toHaveTextContent('common');
    });

    it('switches from no namespace to the first loaded namespace', () => {
      let isLoaded = false;
      mockUseGetTranslations.mockImplementation(({language}: {language: string}) => {
        if (language === 'fr-FR') {
          return {
            data: isLoaded ? sampleTranslations : undefined,
            isLoading: !isLoaded,
          };
        }

        return {
          data: undefined,
          isLoading: false,
        };
      });

      const {rerender} = render(<TranslationsEditPage />);

      expect(screen.getByTestId('ns-value')).toHaveTextContent('');

      isLoaded = true;
      rerender(<TranslationsEditPage />);

      expect(screen.getByTestId('ns-value')).toHaveTextContent('common');
    });

    it('passes loading=true to the editor card while translations are loading', () => {
      mockUseGetTranslations.mockReturnValue({data: undefined, isLoading: true});

      render(<TranslationsEditPage />);

      expect(screen.getByTestId('card-loading')).toHaveTextContent('true');
    });

    it('passes loading=false to the editor card once translations have loaded', () => {
      render(<TranslationsEditPage />);

      expect(screen.getByTestId('card-loading')).toHaveTextContent('false');
    });

    it('sets isEnglish=false for a non-English language', () => {
      render(<TranslationsEditPage />);

      expect(screen.getByTestId('header-is-english')).toHaveTextContent('false');
    });
  });

  describe('Dirty change tracking', () => {
    it('starts with no dirty changes', () => {
      render(<TranslationsEditPage />);

      expect(screen.queryByText('Save Changes')).not.toBeInTheDocument();
    });

    it('shows the unsaved-changes bar after a field is changed', async () => {
      const user = userEvent.setup();
      render(<TranslationsEditPage />);

      await user.click(screen.getByText('change field'));

      expect(screen.getByText('Save Changes')).toBeInTheDocument();
    });

    it('resets dirty changes after Discard is clicked', async () => {
      const user = userEvent.setup();
      render(<TranslationsEditPage />);

      await user.click(screen.getByText('change field'));
      expect(screen.getByText('Save Changes')).toBeInTheDocument();

      await user.click(screen.getByText('Discard Changes'));

      expect(screen.queryByText('Save Changes')).not.toBeInTheDocument();
    });

    it('removes a single dirty key when reset field is called', async () => {
      const user = userEvent.setup();
      render(<TranslationsEditPage />);

      await user.click(screen.getByText('change field'));
      expect(screen.getByText('Save Changes')).toBeInTheDocument();

      await user.click(screen.getByText('reset field'));

      expect(screen.queryByText('Save Changes')).not.toBeInTheDocument();
    });
  });

  describe('Save', () => {
    it('calls updateTranslation.mutateAsync for each dirty key when Save is clicked', async () => {
      const user = userEvent.setup();
      render(<TranslationsEditPage />);

      await user.click(screen.getByText('change field'));
      await user.click(screen.getByText('Save Changes'));

      expect(mockMutateAsync).toHaveBeenCalledWith({
        language: 'fr-FR',
        namespace: 'common',
        key: 'actions.save',
        value: 'Sauvegarder',
      });
    });

    it('shows a success toast after a successful save', async () => {
      const user = userEvent.setup();
      render(<TranslationsEditPage />);

      await user.click(screen.getByText('change field'));
      await user.click(screen.getByText('Save Changes'));

      expect(screen.getByText('All translations saved.')).toBeInTheDocument();
    });

    it('clears dirty changes after a successful save', async () => {
      const user = userEvent.setup();
      render(<TranslationsEditPage />);

      await user.click(screen.getByText('change field'));
      await user.click(screen.getByText('Save Changes'));

      expect(screen.queryByText('Save Changes')).not.toBeInTheDocument();
    });

    it('shows the resolved error inline on the page when at least one save request fails', async () => {
      mockMutateAsync.mockRejectedValueOnce(new Error('Network error'));
      const user = userEvent.setup();
      render(<TranslationsEditPage />);

      await user.click(screen.getByText('change field'));
      await user.click(screen.getByText('Save Changes'));

      expect(screen.getByRole('alert')).toHaveTextContent('Failed to save some translations.');
    });
  });

  describe('Namespace selection', () => {
    it('updates the selected namespace when a new one is chosen', async () => {
      const user = userEvent.setup();
      render(<TranslationsEditPage />);

      await user.click(screen.getByText('select namespace'));

      expect(screen.getByTestId('ns-value')).toHaveTextContent('auth');
    });

    it('resets dirty changes when the namespace changes', async () => {
      const user = userEvent.setup();
      render(<TranslationsEditPage />);

      await user.click(screen.getByText('change field'));
      expect(screen.getByText('Save Changes')).toBeInTheDocument();

      await user.click(screen.getByText('select namespace'));

      expect(screen.queryByText('Save Changes')).not.toBeInTheDocument();
    });
  });

  describe('Navigation', () => {
    it('navigates to /translations when the back button is clicked', async () => {
      const user = userEvent.setup();
      render(<TranslationsEditPage />);

      await user.click(screen.getByText('back'));

      expect(mockNavigate).toHaveBeenCalledWith('/translations');
    });
  });

  describe('Reset to default', () => {
    it('calls updateTranslation.mutateAsync for each default key when Reset is clicked', async () => {
      // Provide default en translations
      mockUseGetTranslations.mockImplementation(({language}: {language: string}) => {
        if (language === 'fr-FR') {
          return {
            data: sampleTranslations,
            isLoading: false,
          };
        }
        // en default translations
        return {
          data: {
            translations: {
              common: {'actions.save': 'Save', 'actions.cancel': 'Cancel'},
            },
          },
          isLoading: false,
        };
      });

      const user = userEvent.setup();
      render(<TranslationsEditPage />);

      await user.click(screen.getByText('reset'));

      expect(mockMutateAsync).toHaveBeenCalledWith({
        language: 'fr-FR',
        namespace: 'common',
        key: 'actions.save',
        value: 'Save',
      });
      expect(mockMutateAsync).toHaveBeenCalledWith({
        language: 'fr-FR',
        namespace: 'common',
        key: 'actions.cancel',
        value: 'Cancel',
      });
    });
  });
});
