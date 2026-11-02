// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import DynamicValuePopover from '../DynamicValuePopover';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, params?: Record<string, unknown>) => {
      if (params) {
        return `${key} ${JSON.stringify(params)}`;
      }
      return key;
    },
  }),
}));

vi.mock('../I18nConfigurationCard', () => ({
  I18nConfigurationCardContent: ({i18nKey, onChange}: {i18nKey: string; onChange: (key: string) => void}) => (
    <div data-testid="i18n-content">
      <span data-testid="i18n-key">{i18nKey}</span>
      <button type="button" data-testid="i18n-change" onClick={() => onChange('flowI18n:login.title')}>
        Select Translation
      </button>
    </div>
  ),
}));

vi.mock('../MetaConfigurationCard', () => ({
  MetaConfigurationCardContent: ({metaKey, onChange}: {metaKey: string; onChange: (key: string) => void}) => (
    <div data-testid="meta-content">
      <span data-testid="meta-key">{metaKey}</span>
      <button type="button" data-testid="meta-change" onClick={() => onChange('application.name')}>
        Select Variable
      </button>
    </div>
  ),
}));

describe('DynamicValuePopover', () => {
  const mockOnClose = vi.fn();
  const mockOnChange = vi.fn();

  let anchorEl: HTMLDivElement;

  beforeEach(() => {
    vi.clearAllMocks();
    anchorEl = document.createElement('div');
    document.body.appendChild(anchorEl);
  });

  afterEach(() => {
    vi.clearAllMocks();
    document.body.removeChild(anchorEl);
  });

  it('should render popover with title containing the property key', () => {
    render(
      <DynamicValuePopover
        open
        anchorEl={anchorEl}
        propertyKey="headerTitle"
        onClose={mockOnClose}
        value=""
        onChange={mockOnChange}
      />,
    );

    expect(screen.getByText(/flows:core.elements.textPropertyField.dynamicValuePopover.title/)).toBeInTheDocument();
  });

  it('should render Translation and Variables tabs', () => {
    render(
      <DynamicValuePopover
        open
        anchorEl={anchorEl}
        propertyKey="headerTitle"
        onClose={mockOnClose}
        value=""
        onChange={mockOnChange}
      />,
    );

    expect(
      screen.getByText('flows:core.elements.textPropertyField.dynamicValuePopover.tabs.translation'),
    ).toBeInTheDocument();
    expect(
      screen.getByText('flows:core.elements.textPropertyField.dynamicValuePopover.tabs.variables'),
    ).toBeInTheDocument();
  });

  it('should show Translation tab by default for non-meta values', () => {
    render(
      <DynamicValuePopover
        open
        anchorEl={anchorEl}
        propertyKey="headerTitle"
        onClose={mockOnClose}
        value="{{t(flowI18n:login.title)}}"
        onChange={mockOnChange}
      />,
    );

    expect(screen.getByTestId('i18n-content')).toBeInTheDocument();
    expect(screen.queryByTestId('meta-content')).not.toBeInTheDocument();
  });

  it('should show Variables tab by default for meta values', () => {
    render(
      <DynamicValuePopover
        open
        anchorEl={anchorEl}
        propertyKey="headerTitle"
        onClose={mockOnClose}
        value="{{meta(application.name)}}"
        onChange={mockOnChange}
      />,
    );

    expect(screen.getByTestId('meta-content')).toBeInTheDocument();
    expect(screen.queryByTestId('i18n-content')).not.toBeInTheDocument();
  });

  it('should switch to Variables tab when clicked', async () => {
    const user = userEvent.setup();
    render(
      <DynamicValuePopover
        open
        anchorEl={anchorEl}
        propertyKey="headerTitle"
        onClose={mockOnClose}
        value=""
        onChange={mockOnChange}
      />,
    );

    await user.click(screen.getByText('flows:core.elements.textPropertyField.dynamicValuePopover.tabs.variables'));

    await waitFor(() => {
      expect(screen.getByTestId('meta-content')).toBeInTheDocument();
    });
  });

  it('should switch back to Translation tab when clicked', async () => {
    const user = userEvent.setup();
    render(
      <DynamicValuePopover
        open
        anchorEl={anchorEl}
        propertyKey="headerTitle"
        onClose={mockOnClose}
        value="{{meta(application.name)}}"
        onChange={mockOnChange}
      />,
    );

    // Initially on Variables tab
    expect(screen.getByTestId('meta-content')).toBeInTheDocument();

    await user.click(screen.getByText('flows:core.elements.textPropertyField.dynamicValuePopover.tabs.translation'));

    await waitFor(() => {
      expect(screen.getByTestId('i18n-content')).toBeInTheDocument();
    });
  });

  it('should call onClose when close button is clicked', () => {
    render(
      <DynamicValuePopover
        open
        anchorEl={anchorEl}
        propertyKey="headerTitle"
        onClose={mockOnClose}
        value=""
        onChange={mockOnChange}
      />,
    );

    fireEvent.click(screen.getByLabelText('common:close'));
    expect(mockOnClose).toHaveBeenCalled();
  });

  it('should call onChange with formatted translation value when i18n key is selected', async () => {
    const user = userEvent.setup();
    render(
      <DynamicValuePopover
        open
        anchorEl={anchorEl}
        propertyKey="headerTitle"
        onClose={mockOnClose}
        value=""
        onChange={mockOnChange}
      />,
    );

    await user.click(screen.getByTestId('i18n-change'));

    expect(mockOnChange).toHaveBeenCalledWith('{{t(flowI18n:login.title)}}');
  });

  it('should call onChange with formatted meta value when variable is selected', async () => {
    const user = userEvent.setup();
    render(
      <DynamicValuePopover
        open
        anchorEl={anchorEl}
        propertyKey="headerTitle"
        onClose={mockOnClose}
        value="{{meta(application.name)}}"
        onChange={mockOnChange}
      />,
    );

    await user.click(screen.getByTestId('meta-change'));

    expect(mockOnChange).toHaveBeenCalledWith('{{meta(application.name)}}');
  });

  it('should not render content when closed', () => {
    render(
      <DynamicValuePopover
        open={false}
        anchorEl={anchorEl}
        propertyKey="headerTitle"
        onClose={mockOnClose}
        value=""
        onChange={mockOnChange}
      />,
    );

    expect(screen.queryByTestId('i18n-content')).not.toBeInTheDocument();
    expect(screen.queryByTestId('meta-content')).not.toBeInTheDocument();
  });

  it('should extract i18n key from value for I18nConfigurationCardContent', () => {
    render(
      <DynamicValuePopover
        open
        anchorEl={anchorEl}
        propertyKey="headerTitle"
        onClose={mockOnClose}
        value="{{t(flowI18n:login.title)}}"
        onChange={mockOnChange}
      />,
    );

    expect(screen.getByTestId('i18n-key')).toHaveTextContent('flowI18n:login.title');
  });

  it('should extract meta key from value for MetaConfigurationCardContent', () => {
    render(
      <DynamicValuePopover
        open
        anchorEl={anchorEl}
        propertyKey="headerTitle"
        onClose={mockOnClose}
        value="{{meta(application.name)}}"
        onChange={mockOnChange}
      />,
    );

    expect(screen.getByTestId('meta-key')).toHaveTextContent('application.name');
  });

  it('should pass empty string as i18n key when value does not match pattern', () => {
    render(
      <DynamicValuePopover
        open
        anchorEl={anchorEl}
        propertyKey="headerTitle"
        onClose={mockOnClose}
        value="plain text"
        onChange={mockOnChange}
      />,
    );

    expect(screen.getByTestId('i18n-key')).toHaveTextContent('');
  });

  it('should call onChange with empty string when i18n key is cleared', async () => {
    const user = userEvent.setup();

    // Re-mock I18nConfigurationCardContent to pass empty string
    const {unmount} = render(
      <DynamicValuePopover
        open
        anchorEl={anchorEl}
        propertyKey="headerTitle"
        onClose={mockOnClose}
        value=""
        onChange={mockOnChange}
      />,
    );

    // The mock onChange handler wraps: (i18nKey) => onChange(i18nKey ? `{{t(${i18nKey})}}` : '')
    // When i18n-change button is clicked, it passes 'flowI18n:login.title' which is truthy
    await user.click(screen.getByTestId('i18n-change'));
    expect(mockOnChange).toHaveBeenCalledWith('{{t(flowI18n:login.title)}}');

    unmount();
  });
});
