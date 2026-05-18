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

vi.mock('@thunderid/i18n', () => ({
  useGetTranslations: mockUseGetTranslations,
  useUpdateTranslation: mockUseUpdateTranslation,
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
    onSave: () => void;
    onDiscard: () => void;
    onResetToDefault: () => void;
    hasDirtyChanges: boolean;
    isSaving: boolean;
    selectedLanguage: string | null;
    isFallbackLanguage: boolean;
    hasNamespace: boolean;
    dirtyCount: number;
  }) => {
    mockTranslationEditorHeader(props);
    return (
      <div data-testid="editor-header">
        <button type="button" onClick={props.onBack}>
          back
        </button>
        <button type="button" onClick={props.onSave} disabled={!props.hasDirtyChanges || props.isSaving}>
          save
        </button>
        <button type="button" onClick={props.onDiscard} disabled={!props.hasDirtyChanges}>
          discard
        </button>
        <button type="button" onClick={props.onResetToDefault} disabled={!props.hasNamespace || props.isSaving}>
          reset
        </button>
        <span data-testid="header-language">{props.selectedLanguage}</span>
        <span data-testid="header-dirty-count">{props.dirtyCount}</span>
        <span data-testid="header-is-english">{String(props.isFallbackLanguage)}</span>
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

      expect(screen.getByTestId('header-dirty-count')).toHaveTextContent('0');
    });

    it('increments the dirty count after a field is changed', async () => {
      const user = userEvent.setup();
      render(<TranslationsEditPage />);

      await user.click(screen.getByText('change field'));

      expect(screen.getByTestId('header-dirty-count')).toHaveTextContent('1');
    });

    it('resets dirty changes after Discard is clicked', async () => {
      const user = userEvent.setup();
      render(<TranslationsEditPage />);

      await user.click(screen.getByText('change field'));
      expect(screen.getByTestId('header-dirty-count')).toHaveTextContent('1');

      await user.click(screen.getByText('discard'));

      expect(screen.getByTestId('header-dirty-count')).toHaveTextContent('0');
    });

    it('removes a single dirty key when reset field is called', async () => {
      const user = userEvent.setup();
      render(<TranslationsEditPage />);

      await user.click(screen.getByText('change field'));
      expect(screen.getByTestId('header-dirty-count')).toHaveTextContent('1');

      await user.click(screen.getByText('reset field'));

      expect(screen.getByTestId('header-dirty-count')).toHaveTextContent('0');
    });
  });

  describe('Save', () => {
    it('calls updateTranslation.mutateAsync for each dirty key when Save is clicked', async () => {
      const user = userEvent.setup();
      render(<TranslationsEditPage />);

      await user.click(screen.getByText('change field'));
      await user.click(screen.getByText('save'));

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
      await user.click(screen.getByText('save'));

      expect(screen.getByText('All translations saved.')).toBeInTheDocument();
    });

    it('clears dirty changes after a successful save', async () => {
      const user = userEvent.setup();
      render(<TranslationsEditPage />);

      await user.click(screen.getByText('change field'));
      await user.click(screen.getByText('save'));

      expect(screen.getByTestId('header-dirty-count')).toHaveTextContent('0');
    });

    it('shows an error toast when at least one save request fails', async () => {
      mockMutateAsync.mockRejectedValueOnce(new Error('Network error'));
      const user = userEvent.setup();
      render(<TranslationsEditPage />);

      await user.click(screen.getByText('change field'));
      await user.click(screen.getByText('save'));

      expect(screen.getByText('Failed to save some translations.')).toBeInTheDocument();
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
      expect(screen.getByTestId('header-dirty-count')).toHaveTextContent('1');

      await user.click(screen.getByText('select namespace'));

      expect(screen.getByTestId('header-dirty-count')).toHaveTextContent('0');
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
