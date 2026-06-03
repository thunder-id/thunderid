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
import TranslationCreateProvider from '@/contexts/TranslationCreate/TranslationCreateProvider';
import useTranslationCreate from '@/contexts/TranslationCreate/useTranslationCreate';
import {TranslationCreateFlowStep} from '@/models/translation-create-flow';

// Test component to consume the context
function TestConsumer() {
  const context = useTranslationCreate();

  return (
    <div>
      <div data-testid="current-step">{context.currentStep}</div>
      <div data-testid="selected-country">{context.selectedCountry?.name ?? 'null'}</div>
      <div data-testid="selected-locale">{context.selectedLocale?.code ?? 'null'}</div>
      <div data-testid="locale-code-override">{context.localeCodeOverride}</div>
      <div data-testid="locale-code">{context.localeCode}</div>
      <div data-testid="populate-from-english">{String(context.populateFromEnglish)}</div>
      <div data-testid="is-creating">{String(context.isCreating)}</div>
      <div data-testid="progress">{String(context.progress)}</div>
      <div data-testid="error">{context.error ?? 'null'}</div>

      <button type="button" onClick={() => context.setCurrentStep(TranslationCreateFlowStep.LANGUAGE)}>
        Set Language Step
      </button>
      <button type="button" onClick={() => context.setSelectedCountry({name: 'France', regionCode: 'FR', flag: '🇫🇷'})}>
        Set Country
      </button>
      <button
        type="button"
        onClick={() => context.setSelectedLocale({code: 'fr-FR', displayName: 'French (France)', flag: '🇫🇷'})}
      >
        Set Locale
      </button>
      <button type="button" onClick={() => context.setLocaleCodeOverride('fr-CA')}>
        Set Locale Code Override
      </button>
      <button type="button" onClick={() => context.setPopulateFromEnglish(false)}>
        Set Populate False
      </button>
      <button type="button" onClick={() => context.setIsCreating(true)}>
        Set Creating
      </button>
      <button type="button" onClick={() => context.setProgress(50)}>
        Set Progress 50
      </button>
      <button type="button" onClick={() => context.setError('Something went wrong')}>
        Set Error
      </button>
      <button type="button" onClick={() => context.reset()}>
        Reset
      </button>
    </div>
  );
}

describe('TranslationCreateProvider', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('provides initial state values', () => {
    render(
      <TranslationCreateProvider>
        <TestConsumer />
      </TranslationCreateProvider>,
    );

    expect(screen.getByTestId('current-step')).toHaveTextContent(TranslationCreateFlowStep.COUNTRY);
    expect(screen.getByTestId('selected-country')).toHaveTextContent('null');
    expect(screen.getByTestId('selected-locale')).toHaveTextContent('null');
    expect(screen.getByTestId('locale-code-override')).toHaveTextContent('');
    expect(screen.getByTestId('locale-code')).toHaveTextContent('');
    expect(screen.getByTestId('populate-from-english')).toHaveTextContent('true');
    expect(screen.getByTestId('is-creating')).toHaveTextContent('false');
    expect(screen.getByTestId('progress')).toHaveTextContent('0');
    expect(screen.getByTestId('error')).toHaveTextContent('null');
  });

  it('updates current step when setCurrentStep is called', async () => {
    const user = userEvent.setup();

    render(
      <TranslationCreateProvider>
        <TestConsumer />
      </TranslationCreateProvider>,
    );

    await user.click(screen.getByText('Set Language Step'));

    expect(screen.getByTestId('current-step')).toHaveTextContent(TranslationCreateFlowStep.LANGUAGE);
  });

  it('updates selected country when setSelectedCountry is called', async () => {
    const user = userEvent.setup();

    render(
      <TranslationCreateProvider>
        <TestConsumer />
      </TranslationCreateProvider>,
    );

    await user.click(screen.getByText('Set Country'));

    expect(screen.getByTestId('selected-country')).toHaveTextContent('France');
  });

  it('updates selected locale when setSelectedLocale is called', async () => {
    const user = userEvent.setup();

    render(
      <TranslationCreateProvider>
        <TestConsumer />
      </TranslationCreateProvider>,
    );

    await user.click(screen.getByText('Set Locale'));

    expect(screen.getByTestId('selected-locale')).toHaveTextContent('fr-FR');
  });

  it('derives localeCode from localeCodeOverride when set', async () => {
    const user = userEvent.setup();

    render(
      <TranslationCreateProvider>
        <TestConsumer />
      </TranslationCreateProvider>,
    );

    await user.click(screen.getByText('Set Locale Code Override'));

    expect(screen.getByTestId('locale-code-override')).toHaveTextContent('fr-CA');
    expect(screen.getByTestId('locale-code')).toHaveTextContent('fr-CA');
  });

  it('derives localeCode from selectedLocale when no override is set', async () => {
    const user = userEvent.setup();

    render(
      <TranslationCreateProvider>
        <TestConsumer />
      </TranslationCreateProvider>,
    );

    await user.click(screen.getByText('Set Locale'));

    expect(screen.getByTestId('locale-code-override')).toHaveTextContent('');
    expect(screen.getByTestId('locale-code')).toHaveTextContent('fr-FR');
  });

  it('override takes precedence over selectedLocale for localeCode', async () => {
    const user = userEvent.setup();

    render(
      <TranslationCreateProvider>
        <TestConsumer />
      </TranslationCreateProvider>,
    );

    await user.click(screen.getByText('Set Locale'));
    await user.click(screen.getByText('Set Locale Code Override'));

    expect(screen.getByTestId('locale-code')).toHaveTextContent('fr-CA');
  });

  it('updates populateFromEnglish when setPopulateFromEnglish is called', async () => {
    const user = userEvent.setup();

    render(
      <TranslationCreateProvider>
        <TestConsumer />
      </TranslationCreateProvider>,
    );

    expect(screen.getByTestId('populate-from-english')).toHaveTextContent('true');

    await user.click(screen.getByText('Set Populate False'));

    expect(screen.getByTestId('populate-from-english')).toHaveTextContent('false');
  });

  it('updates isCreating when setIsCreating is called', async () => {
    const user = userEvent.setup();

    render(
      <TranslationCreateProvider>
        <TestConsumer />
      </TranslationCreateProvider>,
    );

    await user.click(screen.getByText('Set Creating'));

    expect(screen.getByTestId('is-creating')).toHaveTextContent('true');
  });

  it('updates progress when setProgress is called', async () => {
    const user = userEvent.setup();

    render(
      <TranslationCreateProvider>
        <TestConsumer />
      </TranslationCreateProvider>,
    );

    await user.click(screen.getByText('Set Progress 50'));

    expect(screen.getByTestId('progress')).toHaveTextContent('50');
  });

  it('updates error when setError is called', async () => {
    const user = userEvent.setup();

    render(
      <TranslationCreateProvider>
        <TestConsumer />
      </TranslationCreateProvider>,
    );

    await user.click(screen.getByText('Set Error'));

    expect(screen.getByTestId('error')).toHaveTextContent('Something went wrong');
  });

  it('resets all state when reset is called', async () => {
    const user = userEvent.setup();

    render(
      <TranslationCreateProvider>
        <TestConsumer />
      </TranslationCreateProvider>,
    );

    // Set several values
    await user.click(screen.getByText('Set Language Step'));
    await user.click(screen.getByText('Set Country'));
    await user.click(screen.getByText('Set Locale Code Override'));
    await user.click(screen.getByText('Set Error'));

    // Verify values are set
    expect(screen.getByTestId('current-step')).toHaveTextContent(TranslationCreateFlowStep.LANGUAGE);
    expect(screen.getByTestId('selected-country')).toHaveTextContent('France');
    expect(screen.getByTestId('locale-code-override')).toHaveTextContent('fr-CA');
    expect(screen.getByTestId('error')).toHaveTextContent('Something went wrong');

    // Reset
    await user.click(screen.getByText('Reset'));

    // Verify back to initial state
    expect(screen.getByTestId('current-step')).toHaveTextContent(TranslationCreateFlowStep.COUNTRY);
    expect(screen.getByTestId('selected-country')).toHaveTextContent('null');
    expect(screen.getByTestId('locale-code-override')).toHaveTextContent('');
    expect(screen.getByTestId('locale-code')).toHaveTextContent('');
    expect(screen.getByTestId('populate-from-english')).toHaveTextContent('true');
    expect(screen.getByTestId('is-creating')).toHaveTextContent('false');
    expect(screen.getByTestId('progress')).toHaveTextContent('0');
    expect(screen.getByTestId('error')).toHaveTextContent('null');
  });

  it('memoizes context value to prevent unnecessary re-renders', () => {
    const renderSpy = vi.fn();

    function TestRenderer() {
      renderSpy();
      return <TestConsumer />;
    }

    const {rerender} = render(
      <TranslationCreateProvider>
        <TestRenderer />
      </TranslationCreateProvider>,
    );

    expect(renderSpy).toHaveBeenCalledTimes(1);

    rerender(
      <TranslationCreateProvider>
        <TestRenderer />
      </TranslationCreateProvider>,
    );

    expect(renderSpy).toHaveBeenCalledTimes(2);
  });
});
