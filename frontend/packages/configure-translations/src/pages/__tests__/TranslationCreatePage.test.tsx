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
import type {TranslationCreateContextType} from '@/contexts/TranslationCreate/TranslationCreateContext';
import {TranslationCreateFlowStep} from '@/models/translation-create-flow';
import TranslationCreatePage from '@/pages/TranslationCreatePage';

const mockNavigate = vi.fn();
vi.mock('react-router', async () => {
  const actual = await vi.importActual<typeof import('react-router')>('react-router');
  return {
    ...actual,
    useNavigate: () => mockNavigate,
  };
});

const {mockRefetch, mockCreateTranslationsMutateAsync} = vi.hoisted(() => ({
  mockRefetch: vi.fn(),
  mockCreateTranslationsMutateAsync: vi.fn().mockResolvedValue({}),
}));

vi.mock('@thunderid/i18n', () => ({
  useGetTranslations: vi.fn().mockReturnValue({data: undefined, isLoading: false, refetch: mockRefetch}),
  useCreateTranslations: vi.fn().mockReturnValue({mutateAsync: mockCreateTranslationsMutateAsync}),
  I18nDefaultConstants: {
    FALLBACK_LANGUAGE: 'en-US',
  },
  enUS: {},
}));

vi.mock('@thunderid/logger/react', () => ({
  useLogger: () => ({error: vi.fn(), info: vi.fn(), warn: vi.fn(), debug: vi.fn()}),
}));

// Stub step components so we can control onReadyChange
const mockSelectCountry = vi.fn();
const mockSelectLanguage = vi.fn();
const mockReviewLocaleCode = vi.fn();
const mockInitializeLanguage = vi.fn();

vi.mock('@/components/create-translation/SelectCountry', () => ({
  default: (props: {onReadyChange?: (v: boolean) => void}) => {
    mockSelectCountry(props);
    return (
      <div data-testid="select-country">
        <button type="button" onClick={() => props.onReadyChange?.(true)}>
          ready
        </button>
      </div>
    );
  },
}));

vi.mock('@/components/create-translation/SelectLanguage', () => ({
  default: (props: {onReadyChange?: (v: boolean) => void}) => {
    mockSelectLanguage(props);
    return (
      <div data-testid="select-language">
        <button type="button" onClick={() => props.onReadyChange?.(true)}>
          ready
        </button>
      </div>
    );
  },
}));

vi.mock('@/components/create-translation/ReviewLocaleCode', () => ({
  default: (props: {onReadyChange?: (v: boolean) => void}) => {
    mockReviewLocaleCode(props);
    return <div data-testid="review-locale-code" />;
  },
}));

vi.mock('@/components/create-translation/InitializeLanguage', () => ({
  default: (props: unknown) => {
    mockInitializeLanguage(props);
    return <div data-testid="initialize-language" />;
  },
}));

// Base context state – tests override individual fields as needed
const baseContext: TranslationCreateContextType = {
  currentStep: TranslationCreateFlowStep.COUNTRY,
  setCurrentStep: vi.fn(),
  selectedCountry: null,
  setSelectedCountry: vi.fn(),
  selectedLocale: null,
  setSelectedLocale: vi.fn(),
  localeCodeOverride: '',
  setLocaleCodeOverride: vi.fn(),
  localeCode: '',
  populateFromEnglish: true,
  setPopulateFromEnglish: vi.fn(),
  isCreating: false,
  setIsCreating: vi.fn(),
  progress: 0,
  setProgress: vi.fn(),
  error: null,
  setError: vi.fn(),
  reset: vi.fn(),
};

const mockUseTranslationCreate = vi.fn<() => TranslationCreateContextType>();

vi.mock('@/contexts/TranslationCreate/useTranslationCreate', () => ({
  default: () => mockUseTranslationCreate(),
}));

describe('TranslationCreatePage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockUseTranslationCreate.mockReturnValue({...baseContext});
  });

  describe('Rendering', () => {
    it('renders a linear progress bar', () => {
      render(<TranslationCreatePage />);

      expect(screen.getByRole('progressbar')).toBeInTheDocument();
    });

    it('renders the current step breadcrumb label', () => {
      render(<TranslationCreatePage />);

      expect(screen.getByText('Country')).toBeInTheDocument();
    });

    it('renders the SelectCountry step on mount', () => {
      render(<TranslationCreatePage />);

      expect(screen.getByTestId('select-country')).toBeInTheDocument();
    });

    it('does not render the Back button on the first step', () => {
      render(<TranslationCreatePage />);

      expect(screen.queryByText('Back')).not.toBeInTheDocument();
    });

    it('renders the Continue button on non-final steps', () => {
      render(<TranslationCreatePage />);

      expect(screen.getByText('Continue')).toBeInTheDocument();
    });

    it('renders the Create button on the final step', () => {
      mockUseTranslationCreate.mockReturnValue({
        ...baseContext,
        currentStep: TranslationCreateFlowStep.INITIALIZE,
      });

      render(<TranslationCreatePage />);

      expect(screen.getByText('Create Language')).toBeInTheDocument();
    });

    it('renders the SelectLanguage step when currentStep is LANGUAGE', () => {
      mockUseTranslationCreate.mockReturnValue({
        ...baseContext,
        currentStep: TranslationCreateFlowStep.LANGUAGE,
        selectedCountry: {name: 'France', regionCode: 'FR', flag: '🇫🇷'},
      });

      render(<TranslationCreatePage />);

      expect(screen.getByTestId('select-language')).toBeInTheDocument();
    });

    it('renders the ReviewLocaleCode step when currentStep is LOCALE_CODE', () => {
      mockUseTranslationCreate.mockReturnValue({
        ...baseContext,
        currentStep: TranslationCreateFlowStep.LOCALE_CODE,
        selectedLocale: {code: 'fr-FR', displayName: 'French (France)', flag: '🇫🇷'},
      });

      render(<TranslationCreatePage />);

      expect(screen.getByTestId('review-locale-code')).toBeInTheDocument();
    });

    it('renders the InitializeLanguage step when currentStep is INITIALIZE', () => {
      mockUseTranslationCreate.mockReturnValue({
        ...baseContext,
        currentStep: TranslationCreateFlowStep.INITIALIZE,
      });

      render(<TranslationCreatePage />);

      expect(screen.getByTestId('initialize-language')).toBeInTheDocument();
    });

    it('renders an error alert when error is set', () => {
      mockUseTranslationCreate.mockReturnValue({
        ...baseContext,
        error: 'Something went wrong',
      });

      render(<TranslationCreatePage />);

      expect(screen.getByText('Something went wrong')).toBeInTheDocument();
    });
  });

  describe('Step readiness', () => {
    it('disables Continue when the current step is not ready', () => {
      render(<TranslationCreatePage />);

      expect(screen.getByText('Continue').closest('button')).toBeDisabled();
    });

    it('enables Continue after the step reports ready', async () => {
      const user = userEvent.setup();
      render(<TranslationCreatePage />);

      await user.click(screen.getByText('ready'));

      expect(screen.getByText('Continue').closest('button')).not.toBeDisabled();
    });
  });

  describe('Navigation', () => {
    it('calls setCurrentStep with the next step when Continue is clicked', async () => {
      const setCurrentStep = vi.fn();
      mockUseTranslationCreate.mockReturnValue({
        ...baseContext,
        setCurrentStep,
      });
      const user = userEvent.setup();
      render(<TranslationCreatePage />);

      // Mark step ready then advance
      await user.click(screen.getByText('ready'));
      await user.click(screen.getByText('Continue'));

      expect(setCurrentStep).toHaveBeenCalledWith(TranslationCreateFlowStep.LANGUAGE);
    });

    it('calls setCurrentStep with the previous step when Back is clicked', async () => {
      const setCurrentStep = vi.fn();
      mockUseTranslationCreate.mockReturnValue({
        ...baseContext,
        currentStep: TranslationCreateFlowStep.LANGUAGE,
        selectedCountry: {name: 'France', regionCode: 'FR', flag: '🇫🇷'},
        setCurrentStep,
      });
      const user = userEvent.setup();
      render(<TranslationCreatePage />);

      await user.click(screen.getByText('Back'));

      expect(setCurrentStep).toHaveBeenCalledWith(TranslationCreateFlowStep.COUNTRY);
    });

    it('navigates to /translations when the close button is clicked', async () => {
      const user = userEvent.setup();
      render(<TranslationCreatePage />);

      // The close button renders an X icon; it's the only icon button in the header
      const closeButton = screen.getAllByRole('button')[0];
      await user.click(closeButton);

      expect(mockNavigate).toHaveBeenCalledWith('/translations');
    });
  });

  describe('Creating state', () => {
    it('disables Continue while isCreating is true', () => {
      mockUseTranslationCreate.mockReturnValue({
        ...baseContext,
        isCreating: true,
      });

      render(<TranslationCreatePage />);

      expect(screen.getByText('Continue').closest('button')).toBeDisabled();
    });

    it('disables the close button while isCreating is true', () => {
      mockUseTranslationCreate.mockReturnValue({
        ...baseContext,
        isCreating: true,
      });

      render(<TranslationCreatePage />);

      const closeButton = screen.getAllByRole('button')[0];
      expect(closeButton).toBeDisabled();
    });
  });

  describe('Create flow', () => {
    it('calls setLocaleCodeOverride when advancing from LANGUAGE step', async () => {
      const setCurrentStep = vi.fn();
      const setLocaleCodeOverride = vi.fn();
      mockUseTranslationCreate.mockReturnValue({
        ...baseContext,
        currentStep: TranslationCreateFlowStep.LANGUAGE,
        selectedCountry: {name: 'France', regionCode: 'FR', flag: '🇫🇷'},
        selectedLocale: {code: 'fr-FR', displayName: 'French (France)', flag: '🇫🇷'},
        setCurrentStep,
        setLocaleCodeOverride,
      });
      const user = userEvent.setup();
      render(<TranslationCreatePage />);

      // Mark step ready then advance
      await user.click(screen.getByText('ready'));
      await user.click(screen.getByText('Continue'));

      expect(setLocaleCodeOverride).toHaveBeenCalledWith('fr-FR');
      expect(setCurrentStep).toHaveBeenCalledWith(TranslationCreateFlowStep.LOCALE_CODE);
    });

    it('creates translations when Create is clicked on the final step', async () => {
      const setIsCreating = vi.fn();
      const setProgress = vi.fn();
      const setError = vi.fn();

      mockRefetch.mockResolvedValue({
        data: {
          translations: {
            common: {'actions.save': 'Save'},
          },
        },
        error: null,
      });

      mockUseTranslationCreate.mockReturnValue({
        ...baseContext,
        currentStep: TranslationCreateFlowStep.INITIALIZE,
        localeCode: 'fr-FR',
        populateFromEnglish: true,
        setIsCreating,
        setProgress,
        setError,
      });

      const user = userEvent.setup();
      render(<TranslationCreatePage />);

      await user.click(screen.getByText('Create Language'));

      expect(setIsCreating).toHaveBeenCalledWith(true);
      expect(mockRefetch).toHaveBeenCalled();

      // Wait for async create to complete
      await vi.waitFor(() => {
        expect(mockCreateTranslationsMutateAsync).toHaveBeenCalledWith({
          language: 'fr-FR',
          translations: {
            common: {'actions.save': 'Save'},
          },
        });
      });
    });

    it('sets error when fetching en-US translations fails during create', async () => {
      const setError = vi.fn();
      const setIsCreating = vi.fn();

      mockRefetch.mockResolvedValue({
        data: null,
        error: new Error('Fetch failed'),
      });

      mockUseTranslationCreate.mockReturnValue({
        ...baseContext,
        currentStep: TranslationCreateFlowStep.INITIALIZE,
        localeCode: 'fr-FR',
        setError,
        setIsCreating,
        setProgress: vi.fn(),
      });

      const user = userEvent.setup();
      render(<TranslationCreatePage />);

      await user.click(screen.getByText('Create Language'));

      await vi.waitFor(() => {
        expect(setError).toHaveBeenCalledWith('Failed to add language. Please try again.');
        expect(setIsCreating).toHaveBeenCalledWith(false);
      });
    });

    it('sets error when creating translations fails after fetching defaults', async () => {
      const setError = vi.fn();
      const setIsCreating = vi.fn();

      mockRefetch.mockResolvedValue({
        data: {
          translations: {
            common: {'actions.save': 'Save'},
          },
        },
        error: null,
      });
      mockCreateTranslationsMutateAsync.mockRejectedValueOnce(new Error('Create failed'));

      mockUseTranslationCreate.mockReturnValue({
        ...baseContext,
        currentStep: TranslationCreateFlowStep.INITIALIZE,
        localeCode: 'fr-FR',
        setError,
        setIsCreating,
        setProgress: vi.fn(),
      });

      const user = userEvent.setup();
      render(<TranslationCreatePage />);

      await user.click(screen.getByText('Create Language'));

      await vi.waitFor(() => {
        expect(setError).toHaveBeenCalledWith('Failed to add language. Please try again.');
        expect(setIsCreating).toHaveBeenCalledWith(false);
      });
    });

    it('does not start creation when localeCode is empty', async () => {
      mockUseTranslationCreate.mockReturnValue({
        ...baseContext,
        currentStep: TranslationCreateFlowStep.INITIALIZE,
        localeCode: '',
      });

      const user = userEvent.setup();
      render(<TranslationCreatePage />);

      await user.click(screen.getByText('Create Language'));

      expect(mockRefetch).not.toHaveBeenCalled();
    });
  });

  describe('Breadcrumb navigation', () => {
    it('navigates to a previous step when a breadcrumb is clicked', async () => {
      const setCurrentStep = vi.fn();
      mockUseTranslationCreate.mockReturnValue({
        ...baseContext,
        currentStep: TranslationCreateFlowStep.LOCALE_CODE,
        selectedCountry: {name: 'France', regionCode: 'FR', flag: '🇫🇷'},
        selectedLocale: {code: 'fr-FR', displayName: 'French (France)', flag: '🇫🇷'},
        setCurrentStep,
      });
      const user = userEvent.setup();
      render(<TranslationCreatePage />);

      // Click on the first breadcrumb (COUNTRY)
      await user.click(screen.getByText('Country'));

      expect(setCurrentStep).toHaveBeenCalledWith(TranslationCreateFlowStep.COUNTRY);
    });
  });
});
