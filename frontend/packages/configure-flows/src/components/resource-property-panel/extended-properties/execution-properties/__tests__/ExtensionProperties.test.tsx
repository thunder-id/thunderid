// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import executors from '../../../../../data/executors.json';
import type {Resource} from '../../../../../models/resources';
import ExtensionProperties from '../ExtensionProperties';

vi.mock('react-i18next', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-i18next')>()),
  useTranslation: () => ({
    t: (key: string, fallback?: unknown) => {
      if (typeof fallback === 'string') {
        return fallback;
      }
      const options = (fallback ?? {}) as Record<string, string>;
      return (options['defaultValue'] ?? key).replace(/\{\{(\w+)\}\}/g, (_match, name: string) => options[name] ?? '');
    },
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
    it('lists service endpoint connections and stores the selected id as connectionId', async () => {
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
      expect(screen.getAllByRole('spinbutton').map((field) => field.id)).toEqual(['extension-max-attempts']);
      expect(screen.getAllByRole('combobox')).toHaveLength(4);
    });

    it('tells the administrator to create a connection when none exist', () => {
      mockConnections.mockReturnValue(loaded([]));
      render(<ExtensionProperties resource={createResource()} onChange={mockOnChange} />);

      expect(screen.getByText(/No service endpoint connections exist/)).toBeInTheDocument();
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

      expect(screen.getByText('Failed to load service endpoint connections.')).toBeInTheDocument();
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

    it('clears inputKeys rather than storing an empty list when every input is removed', async () => {
      const user = userEvent.setup();
      render(<ExtensionProperties resource={createResource({inputKeys: ['username']})} onChange={mockOnChange} />);

      await user.click(within(screen.getByRole('button', {name: 'username'})).getByTestId('CancelIcon'));

      expect(mockOnChange).toHaveBeenLastCalledWith('data.properties.inputKeys', undefined, expect.anything());
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

  describe('stored field namespace', () => {
    it('names the namespace of this step in the stored fields hint', () => {
      render(<ExtensionProperties resource={createResource()} onChange={mockOnChange} />);

      expect(screen.getByText(/stored under extension\.extension-1\./)).toBeInTheDocument();
      expect(screen.getByText(/extension\.extension-1\.riskScore/)).toBeInTheDocument();
      expect(screen.getByText(/When empty, every top-level field is stored/)).toBeInTheDocument();
      expect(screen.queryByText(/every allowed field is stored/)).not.toBeInTheDocument();
    });

    it('tells the administrator to name data from an earlier extension by its full key', () => {
      render(<ExtensionProperties resource={createResource()} onChange={mockOnChange} />);

      expect(screen.getByText(/by its full key, such as extension\.risk_check\.riskScore/)).toBeInTheDocument();
    });
  });

  describe('max attempts', () => {
    const maxAttemptsField = (): HTMLElement => screen.getByRole('spinbutton', {name: 'Max attempts'});

    it('shows the default of 5 when maxAttempts is unset', () => {
      render(<ExtensionProperties resource={createResource()} onChange={mockOnChange} />);

      expect(maxAttemptsField()).toHaveValue(5);
      expect(screen.getByText(/How many rejected answers this step accepts/)).toBeInTheDocument();
    });

    it('shows the stored maxAttempts', () => {
      render(<ExtensionProperties resource={createResource({maxAttempts: 3})} onChange={mockOnChange} />);

      expect(maxAttemptsField()).toHaveValue(3);
    });

    it('stores a value from 1 to 20 as a number', async () => {
      const user = userEvent.setup();
      render(<ExtensionProperties resource={createResource()} onChange={mockOnChange} />);

      await user.clear(maxAttemptsField());
      await user.type(maxAttemptsField(), '12');
      await user.tab();

      expect(mockOnChange).toHaveBeenLastCalledWith('data.properties.maxAttempts', 12, expect.anything());
    });

    it.each(['0', '21', '2.5', ''])('shows a validation message for %j and does not store it', async (input) => {
      const user = userEvent.setup();
      render(<ExtensionProperties resource={createResource({maxAttempts: 3})} onChange={mockOnChange} />);

      await user.clear(maxAttemptsField());
      if (input !== '') {
        await user.type(maxAttemptsField(), input);
      }
      await user.tab();

      expect(screen.getByText('Enter a whole number from 1 to 20.')).toHaveClass('Mui-error');
      expect(mockOnChange).not.toHaveBeenCalledWith(
        'data.properties.maxAttempts',
        expect.anything(),
        expect.anything(),
      );
    });

    it('clears the validation message once the value is valid again', async () => {
      const user = userEvent.setup();
      render(<ExtensionProperties resource={createResource()} onChange={mockOnChange} />);

      await user.clear(maxAttemptsField());
      await user.type(maxAttemptsField(), '25');
      expect(screen.getByText('Enter a whole number from 1 to 20.')).toBeInTheDocument();

      await user.clear(maxAttemptsField());
      await user.type(maxAttemptsField(), '7{Enter}');

      expect(screen.queryByText('Enter a whole number from 1 to 20.')).not.toBeInTheDocument();
      expect(mockOnChange).toHaveBeenLastCalledWith('data.properties.maxAttempts', 7, expect.anything());
    });
  });

  it('adds the extension step from the resource panel without an empty key list', () => {
    const template = (executors as {data?: {action?: {executor?: {name?: string}}; properties?: object}}[]).find(
      (entry) => entry.data?.action?.executor?.name === 'ExtensionExecutor',
    );

    expect(template?.data?.properties).toBeDefined();
    Object.values(template?.data?.properties ?? {}).forEach((value) => {
      expect(Array.isArray(value) && value.length === 0).toBe(false);
    });
  });
});
