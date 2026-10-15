// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import type {Resource} from '../../../../../models/resources';
import ExtensionProperties from '../ExtensionProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, fallback?: unknown) =>
      typeof fallback === 'string'
        ? fallback
        : ((fallback as {defaultValue?: string} | undefined)?.defaultValue ?? key),
  }),
}));

interface ConnectionsState {
  data?: {id: string; name: string}[];
  isLoading: boolean;
  isSuccess: boolean;
  error: Error | null;
  refetch: () => void;
}

const {mockFieldError} = vi.hoisted(() => ({mockFieldError: vi.fn<(resourceId?: string, field?: string) => string>()}));
vi.mock('../../../../../hooks/useResourceFieldError', () => ({
  default: (resourceId?: string, field?: string): string => mockFieldError(resourceId, field),
}));

const mockConnections = vi.fn<() => ConnectionsState>();
const mockUseConnectionInstances = vi.fn((type: string) => {
  expect(type).toBe('http');
  return mockConnections();
});
vi.mock('@thunderid/configure-connections', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/configure-connections')>()),
  useConnectionInstances: (type: string) => mockUseConnectionInstances(type),
}));

const loaded = (data: {id: string; name: string}[]): ConnectionsState => ({
  data,
  isLoading: false,
  isSuccess: true,
  error: null,
  refetch: vi.fn(),
});

const createResource = (properties: Record<string, unknown> = {}): Resource =>
  ({
    id: 'extension-1',
    data: {
      action: {executor: {name: 'ExtensionExecutor'}},
      properties,
    },
  }) as unknown as Resource;

describe('ExtensionProperties', () => {
  const mockOnChange = vi.fn();

  beforeEach(() => {
    vi.clearAllMocks();
    mockFieldError.mockReturnValue('');
    mockConnections.mockReturnValue(
      loaded([
        {id: 'conn-1', name: 'Risk Engine'},
        {id: 'conn-2', name: 'PIN Authenticator'},
      ]),
    );
  });

  describe('connection selector', () => {
    it('lists HTTP endpoint connections and stores the selected id as connectionId', async () => {
      const user = userEvent.setup();
      render(<ExtensionProperties resource={createResource({connectionId: ''})} onChange={mockOnChange} />);

      await user.click(screen.getByRole('combobox', {name: /connection/i}));
      const listbox = screen.getByRole('listbox');
      expect(within(listbox).getByText('Risk Engine')).toBeInTheDocument();
      await user.click(within(listbox).getByText('PIN Authenticator'));

      expect(mockOnChange).toHaveBeenCalledWith('data.properties.connectionId', 'conn-2', expect.anything());
    });

    it('shows the required error from validation on the connection selector', () => {
      mockFieldError.mockImplementation((resourceId, field) =>
        resourceId === 'extension-1' && field === 'data.properties.connectionId' ? 'Connection is required' : '',
      );
      render(<ExtensionProperties resource={createResource({connectionId: ''})} onChange={mockOnChange} />);

      expect(screen.getByText('Connection is required')).toHaveClass('Mui-error');
      expect(screen.queryByText(/come from the connection/)).not.toBeInTheDocument();
    });

    it('shows the connection hint when there is no validation error', () => {
      render(<ExtensionProperties resource={createResource({connectionId: 'conn-1'})} onChange={mockOnChange} />);

      expect(screen.getByText(/come from the connection/)).not.toHaveClass('Mui-error');
    });

    it('shows the selected connection by name', () => {
      render(<ExtensionProperties resource={createResource({connectionId: 'conn-1'})} onChange={mockOnChange} />);

      expect(screen.getByRole('combobox', {name: /connection/i})).toHaveTextContent('Risk Engine');
    });

    it('offers no URL, headers or timeout fields', () => {
      render(<ExtensionProperties resource={createResource()} onChange={mockOnChange} />);

      expect(screen.queryByRole('textbox', {name: /url|header|timeout/i})).not.toBeInTheDocument();
      expect(screen.queryByRole('spinbutton')).not.toBeInTheDocument();
      expect(screen.getAllByRole('combobox')).toHaveLength(4);
    });

    it('tells the administrator to create a connection when none exist', () => {
      mockConnections.mockReturnValue(loaded([]));
      render(<ExtensionProperties resource={createResource()} onChange={mockOnChange} />);

      expect(screen.getByText(/No HTTP endpoint connections exist/)).toBeInTheDocument();
    });

    it('keeps a stored connection id visible when the connection no longer exists', () => {
      render(<ExtensionProperties resource={createResource({connectionId: 'deleted-id'})} onChange={mockOnChange} />);

      expect(screen.getByRole('combobox', {name: /connection/i})).toHaveTextContent('deleted-id');
    });

    it('shows an inline error when the connections cannot be loaded', () => {
      mockConnections.mockReturnValue({
        data: undefined,
        isLoading: false,
        isSuccess: false,
        error: new Error('boom'),
        refetch: vi.fn(),
      });
      render(<ExtensionProperties resource={createResource()} onChange={mockOnChange} />);

      expect(screen.getByText('Failed to load HTTP endpoint connections.')).toBeInTheDocument();
    });

    it('keeps the stored connection id visible when the connections cannot be loaded', () => {
      mockConnections.mockReturnValue({
        data: undefined,
        isLoading: false,
        isSuccess: false,
        error: new Error('boom'),
        refetch: vi.fn(),
      });
      render(<ExtensionProperties resource={createResource({connectionId: 'conn-1'})} onChange={mockOnChange} />);

      expect(screen.getByRole('combobox', {name: /connection/i})).toHaveTextContent('conn-1');
    });
  });

  describe('inputs to send', () => {
    it('stores added inputs as a list in inputKeys', async () => {
      const user = userEvent.setup();
      render(<ExtensionProperties resource={createResource({inputKeys: ['username']})} onChange={mockOnChange} />);

      await user.type(screen.getByRole('combobox', {name: 'Inputs to send'}), 'password{Enter}');

      expect(mockOnChange).toHaveBeenLastCalledWith(
        'data.properties.inputKeys',
        ['username', 'password'],
        expect.anything(),
      );
    });

    it('stores an empty list when every input is removed', async () => {
      const user = userEvent.setup();
      render(<ExtensionProperties resource={createResource({inputKeys: ['username']})} onChange={mockOnChange} />);

      await user.click(within(screen.getByRole('button', {name: 'username'})).getByTestId('CancelIcon'));

      expect(mockOnChange).toHaveBeenLastCalledWith('data.properties.inputKeys', [], expect.anything());
    });

    it('ignores a duplicate input', async () => {
      const user = userEvent.setup();
      render(<ExtensionProperties resource={createResource({inputKeys: ['username']})} onChange={mockOnChange} />);

      await user.type(screen.getByRole('combobox', {name: 'Inputs to send'}), ' username {Enter}');

      expect(mockOnChange).toHaveBeenLastCalledWith('data.properties.inputKeys', ['username'], expect.anything());
    });
  });

  it.each(['inputKeys', 'runtimeDataKeys'])('does not read %s from a comma-separated string', (property) => {
    render(<ExtensionProperties resource={createResource({[property]: 'username, email'})} onChange={mockOnChange} />);

    expect(screen.queryByRole('button', {name: 'username'})).not.toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'username, email'})).not.toBeInTheDocument();
  });

  describe('runtime data to send', () => {
    it('stores added keys as a list in runtimeDataKeys', async () => {
      const user = userEvent.setup();
      render(<ExtensionProperties resource={createResource()} onChange={mockOnChange} />);

      await user.type(screen.getByRole('combobox', {name: 'Runtime data to send'}), 'riskTier{Enter}');

      expect(mockOnChange).toHaveBeenLastCalledWith('data.properties.runtimeDataKeys', ['riskTier'], expect.anything());
    });

    it('clears runtimeDataKeys when the last key is removed', async () => {
      const user = userEvent.setup();
      render(
        <ExtensionProperties resource={createResource({runtimeDataKeys: ['riskTier']})} onChange={mockOnChange} />,
      );

      await user.click(within(screen.getByRole('button', {name: 'riskTier'})).getByTestId('CancelIcon'));

      expect(mockOnChange).toHaveBeenLastCalledWith('data.properties.runtimeDataKeys', undefined, expect.anything());
    });
  });

  describe('stored response fields', () => {
    it('stores added fields as a list in responseKeys', async () => {
      const user = userEvent.setup();
      render(<ExtensionProperties resource={createResource()} onChange={mockOnChange} />);

      await user.type(screen.getByRole('combobox', {name: 'Stored response fields'}), 'riskScore{Enter}');

      expect(mockOnChange).toHaveBeenLastCalledWith('data.properties.responseKeys', ['riskScore'], expect.anything());
    });

    it('reads a comma-separated responseKeys string as a list', () => {
      render(
        <ExtensionProperties resource={createResource({responseKeys: 'username, email'})} onChange={mockOnChange} />,
      );

      expect(screen.getByRole('button', {name: 'username'})).toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'email'})).toBeInTheDocument();
    });

    it('clears responseKeys when the last field is removed', async () => {
      const user = userEvent.setup();
      render(<ExtensionProperties resource={createResource({responseKeys: ['username']})} onChange={mockOnChange} />);

      await user.click(within(screen.getByRole('button', {name: 'username'})).getByTestId('CancelIcon'));

      expect(mockOnChange).toHaveBeenLastCalledWith('data.properties.responseKeys', undefined, expect.anything());
    });
  });
});
