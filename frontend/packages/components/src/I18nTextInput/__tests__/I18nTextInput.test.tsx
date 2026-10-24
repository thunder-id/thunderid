// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import I18nTextInput from '../I18nTextInput';

const {mockResolve, mockMutate, mockAddResourceBundle, mockOnTranslationCreated, mockLanguages, mockTranslations} =
  vi.hoisted(() => ({
    mockResolve: vi.fn<(value: string) => string | null>(),
    mockMutate: vi.fn(),
    mockAddResourceBundle: vi.fn(),
    mockOnTranslationCreated: vi.fn(),
    mockLanguages: {value: {languages: ['en-US', 'fr-FR']} as {languages: string[]} | undefined},
    mockTranslations: {
      value: {
        translations: {
          custom: {
            display_name: 'Display name',
            first_name: 'First name',
          },
        },
      },
    },
  }));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({
    t: (key: string) => key,
    i18n: {
      addResourceBundle: mockAddResourceBundle,
    },
  }),
}));

vi.mock('@thunderid/hooks', () => ({
  useTemplateLiteralResolver: () => ({
    resolve: mockResolve,
  }),
}));

vi.mock('@thunderid/i18n', () => ({
  NamespaceConstants: {
    CUSTOM_NAMESPACE: 'custom',
  },
  I18nDefaultConstants: {
    FALLBACK_LANGUAGE: 'en-US',
  },
  useGetLanguages: () => ({
    data: mockLanguages.value,
  }),
  useGetTranslations: () => ({
    data: mockTranslations.value,
    isLoading: false,
  }),
  useUpdateTranslation: ({onMutationSuccess}: {onMutationSuccess?: () => void} = {}) => ({
    mutate: (payload: unknown, callbacks?: {onSuccess?: () => void; onError?: (err: Error) => void}) => {
      mockMutate(payload, callbacks);
      onMutationSuccess?.();
    },
    isPending: false,
  }),
}));

describe('I18nTextInput', () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockResolve.mockReturnValue('Resolved Display');
    mockLanguages.value = {languages: ['en-US', 'fr-FR']};
    mockTranslations.value = {
      translations: {
        custom: {
          display_name: 'Display name',
          first_name: 'First name',
        },
      },
    };
  });

  it('renders the label and input', () => {
    render(<I18nTextInput label="Display name" value="" onChange={vi.fn()} placeholder="Type a value" />);

    expect(screen.getByText('Display name')).toBeInTheDocument();
    expect(screen.getByPlaceholderText('Type a value')).toBeInTheDocument();
  });

  it('shows the resolved value when input has an i18n template', () => {
    render(<I18nTextInput label="Display name" value="{{t(custom:display_name)}}" onChange={vi.fn()} />);

    expect(screen.getByText('Resolved value')).toBeInTheDocument();
    expect(screen.getByText('Resolved Display')).toBeInTheDocument();
  });

  it('uses provided labels when given', () => {
    render(
      <I18nTextInput
        label="Display name"
        value="{{t(custom:display_name)}}"
        onChange={vi.fn()}
        labels={{resolvedValueLabel: 'Translated value'}}
      />,
    );

    expect(screen.getByText('Translated value')).toBeInTheDocument();
  });

  it('opens popover and switches to create mode', async () => {
    const user = userEvent.setup();
    const {container} = render(
      <I18nTextInput label="Display name" value="" onChange={vi.fn()} defaultNewKey="Display name" />,
    );

    const configureButton = container.querySelector('button');
    expect(configureButton).not.toBeNull();
    await user.click(configureButton!);

    await user.click(screen.getByText('Create new translation'));

    expect(screen.getByDisplayValue('display_name')).toBeInTheDocument();
    expect(screen.getByText('Create')).toBeInTheDocument();
    expect(screen.getByText('Cancel')).toBeInTheDocument();
  });

  it('creates a translation, calls onTranslationCreated, and emits the template value', async () => {
    const onChange = vi.fn();
    const user = userEvent.setup();

    mockLanguages.value = undefined;
    mockMutate.mockImplementation((_payload: unknown, callbacks?: {onSuccess?: () => void}) => {
      callbacks?.onSuccess?.();
    });

    const {container} = render(
      <I18nTextInput
        label="Display name"
        value=""
        onChange={onChange}
        defaultNewKey="Display name"
        onTranslationCreated={mockOnTranslationCreated}
      />,
    );

    const configureButton = container.querySelector('button');
    await user.click(configureButton!);
    await user.click(screen.getByText('Create new translation'));

    await user.type(screen.getByPlaceholderText('e.g., First Name'), 'Shown Display Name');
    await user.click(screen.getByText('Create'));

    expect(mockMutate).toHaveBeenCalled();
    expect(mockMutate.mock.calls[0][0]).toEqual({
      language: 'en-US',
      namespace: 'custom',
      key: 'display_name',
      value: 'Shown Display Name',
    });

    expect(mockAddResourceBundle).toHaveBeenCalledWith(
      'en-US',
      'custom',
      {display_name: 'Shown Display Name'},
      true,
      true,
    );
    expect(onChange).toHaveBeenCalledWith('{{t(custom:display_name)}}');
    expect(mockOnTranslationCreated).toHaveBeenCalled();
  });
});
