// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import MetaConfigurationCard, {MetaConfigurationCardContent} from '../MetaConfigurationCard';

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

vi.mock('../DynamicValueSyntax', () => ({
  default: ({value}: {value: string}) => <span data-testid="dynamic-value-syntax">{value}</span>,
}));

describe('MetaConfigurationCardContent', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should render the variable path label', () => {
    render(<MetaConfigurationCardContent metaKey="" onChange={mockOnChange} />);

    expect(screen.getByText('flows:core.elements.textPropertyField.metaCard.variablePath')).toBeInTheDocument();
  });

  it('should render autocomplete with placeholder', () => {
    render(<MetaConfigurationCardContent metaKey="" onChange={mockOnChange} />);

    expect(
      screen.getByPlaceholderText('flows:core.elements.textPropertyField.metaCard.variablePathPlaceholder'),
    ).toBeInTheDocument();
  });

  it('should display the initial metaKey value', () => {
    render(<MetaConfigurationCardContent metaKey="application.name" onChange={mockOnChange} />);

    const input = screen.getByRole('combobox');
    expect(input).toHaveValue('application.name');
  });

  it('should show formatted syntax when a value is entered', () => {
    render(<MetaConfigurationCardContent metaKey="ou.name" onChange={mockOnChange} />);

    expect(screen.getByTestId('dynamic-value-syntax')).toHaveTextContent('{{meta(ou.name)}}');
  });

  it('should not show formatted syntax when value is empty', () => {
    render(<MetaConfigurationCardContent metaKey="" onChange={mockOnChange} />);

    expect(screen.queryByTestId('dynamic-value-syntax')).not.toBeInTheDocument();
  });

  it('should call onChange when user types a custom value', async () => {
    const user = userEvent.setup();
    render(<MetaConfigurationCardContent metaKey="" onChange={mockOnChange} />);

    const input = screen.getByRole('combobox');
    await user.type(input, 'application.id');

    await waitFor(() => {
      expect(mockOnChange).toHaveBeenCalled();
    });
  });

  it('should show autocomplete options with common meta fields', async () => {
    const user = userEvent.setup();
    render(<MetaConfigurationCardContent metaKey="" onChange={mockOnChange} />);

    const input = screen.getByRole('combobox');
    await user.click(input);

    await waitFor(() => {
      expect(screen.getByText('application.id')).toBeInTheDocument();
      expect(screen.getByText('application.name')).toBeInTheDocument();
      expect(screen.getByText('ou.name')).toBeInTheDocument();
    });
  });

  it('should call onChange when an option is selected from autocomplete', async () => {
    const user = userEvent.setup();
    render(<MetaConfigurationCardContent metaKey="" onChange={mockOnChange} />);

    const input = screen.getByRole('combobox');
    await user.click(input);

    await waitFor(() => {
      expect(screen.getByText('application.name')).toBeInTheDocument();
    });

    await user.click(screen.getByText('application.name'));

    expect(mockOnChange).toHaveBeenCalledWith('application.name');
  });
});

describe('MetaConfigurationCard', () => {
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
      <MetaConfigurationCard
        open
        anchorEl={anchorEl}
        propertyKey="headerTitle"
        onClose={mockOnClose}
        metaKey=""
        onChange={mockOnChange}
      />,
    );

    expect(screen.getByText(/flows:core.elements.textPropertyField.metaCard.title/)).toBeInTheDocument();
  });

  it('should render close button', () => {
    render(
      <MetaConfigurationCard
        open
        anchorEl={anchorEl}
        propertyKey="headerTitle"
        onClose={mockOnClose}
        metaKey=""
        onChange={mockOnChange}
      />,
    );

    expect(screen.getByLabelText('common:close')).toBeInTheDocument();
  });

  it('should call onClose when close button is clicked', () => {
    render(
      <MetaConfigurationCard
        open
        anchorEl={anchorEl}
        propertyKey="headerTitle"
        onClose={mockOnClose}
        metaKey=""
        onChange={mockOnChange}
      />,
    );

    fireEvent.click(screen.getByLabelText('common:close'));
    expect(mockOnClose).toHaveBeenCalled();
  });

  it('should not render content when closed', () => {
    render(
      <MetaConfigurationCard
        open={false}
        anchorEl={anchorEl}
        propertyKey="headerTitle"
        onClose={mockOnClose}
        metaKey=""
        onChange={mockOnChange}
      />,
    );

    expect(screen.queryByText('flows:core.elements.textPropertyField.metaCard.variablePath')).not.toBeInTheDocument();
  });

  it('should render MetaConfigurationCardContent with correct props', () => {
    render(
      <MetaConfigurationCard
        open
        anchorEl={anchorEl}
        propertyKey="headerTitle"
        onClose={mockOnClose}
        metaKey="application.name"
        onChange={mockOnChange}
      />,
    );

    const input = screen.getByRole('combobox');
    expect(input).toHaveValue('application.name');
  });
});
