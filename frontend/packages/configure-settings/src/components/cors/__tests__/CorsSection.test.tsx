// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {renderWithProviders} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import type {CorsConfigResponse} from '../../../models/responses';

const mockRefetch = vi.fn();
const mockUseGetCorsConfig =
  vi.fn<() => {data: CorsConfigResponse | undefined; isLoading: boolean; error: Error | null; refetch?: () => void}>();
vi.mock('../../../api/useGetCorsConfig', () => ({
  default: () => ({refetch: mockRefetch, ...mockUseGetCorsConfig()}),
}));

const mockMutate = vi.fn();
const mockReset = vi.fn();
// The save-failure surface reads the mutation's own error state, so each test declares it.
let updateState: {isError: boolean; error: Error | null} = {isError: false, error: null};
vi.mock('../../../api/useUpdateCorsConfig', () => ({
  default: () => ({mutate: mockMutate, isPending: false, reset: mockReset, ...updateState}),
}));

const {default: CorsSection} = await import('../CorsSection');

function makeData(overrides?: Partial<CorsConfigResponse>): CorsConfigResponse {
  return {
    readOnly: {allowedOrigins: ['https://console.example.com']},
    writable: {allowedOrigins: ['https://app.acme.com']},
    merged: {allowedOrigins: []},
    ...overrides,
  };
}

describe('CorsSection', () => {
  beforeEach(() => {
    mockUseGetCorsConfig.mockReset();
    mockMutate.mockReset();
    mockReset.mockReset();
    mockRefetch.mockReset();
    updateState = {isError: false, error: null};
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('does not render the editor while loading', () => {
    mockUseGetCorsConfig.mockReturnValue({data: undefined, isLoading: true, error: null});
    renderWithProviders(<CorsSection />);
    expect(screen.queryByRole('button', {name: 'Add origin'})).toBeNull();
  });

  it('shows an alert on load error', () => {
    mockUseGetCorsConfig.mockReturnValue({data: undefined, isLoading: false, error: new Error('load failed')});
    renderWithProviders(<CorsSection />);
    expect(screen.getByRole('alert')).toBeInTheDocument();
  });

  it('refetches when the load error is retried', async () => {
    const user = userEvent.setup();
    mockUseGetCorsConfig.mockReturnValue({data: undefined, isLoading: false, error: new Error('load failed')});
    renderWithProviders(<CorsSection />);

    await user.click(screen.getByRole('button', {name: /refresh/i}));

    expect(mockRefetch).toHaveBeenCalled();
  });

  it('renders read-only origins (incl. regex patterns), editable origins, and the Add control', () => {
    mockUseGetCorsConfig.mockReturnValue({
      data: makeData({readOnly: {allowedOrigins: ['https://console.example.com', {regex: '^https://x$'}]}}),
      isLoading: false,
      error: null,
    });
    renderWithProviders(<CorsSection />);

    expect(screen.getByDisplayValue('https://console.example.com')).toHaveAttribute('readonly');
    expect(screen.getByDisplayValue('^https://x$')).toHaveAttribute('readonly');
    expect(screen.getByDisplayValue('https://app.acme.com')).not.toHaveAttribute('readonly');
    expect(screen.getByRole('button', {name: 'Add origin'})).toBeInTheDocument();
    expect(screen.getByText("Some origins are read-only because they're managed declaratively.")).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Remove origin'})).toBeInTheDocument();
  });

  it('preselects each row type from the saved entry shape', () => {
    mockUseGetCorsConfig.mockReturnValue({
      data: makeData({
        readOnly: {allowedOrigins: [{regex: '^https://ro\\.io$'}]},
        writable: {allowedOrigins: ['https://app.acme.com', {regex: '^https://rw\\.io$'}]},
      }),
      isLoading: false,
      error: null,
    });
    renderWithProviders(<CorsSection />);

    const [readOnlyType, originType, regexType] = screen.getAllByRole('combobox', {name: 'Entry type'});
    expect(readOnlyType).toHaveTextContent('Regex');
    expect(originType).toHaveTextContent('Origin');
    expect(regexType).toHaveTextContent('Regex');
  });

  it('locks the type selector on read-only rows and offers no remove action for them', () => {
    mockUseGetCorsConfig.mockReturnValue({
      data: makeData({readOnly: {allowedOrigins: ['https://console.example.com']}, writable: {allowedOrigins: []}}),
      isLoading: false,
      error: null,
    });
    renderWithProviders(<CorsSection />);

    expect(screen.getByRole('combobox', {name: 'Entry type'})).toHaveAttribute('aria-disabled', 'true');
    expect(screen.queryByRole('button', {name: 'Remove origin'})).toBeNull();
  });

  it('saves a row switched to Regex as a {regex} entry, even when its text is a valid origin', async () => {
    const user = userEvent.setup();
    mockUseGetCorsConfig.mockReturnValue({
      data: makeData({readOnly: {allowedOrigins: []}, writable: {allowedOrigins: ['https://app.example.com']}}),
      isLoading: false,
      error: null,
    });
    renderWithProviders(<CorsSection />);

    await user.click(screen.getByRole('combobox', {name: 'Entry type'}));
    await user.click(screen.getByRole('option', {name: 'Regex'}));

    await user.click(await screen.findByRole('button', {name: 'Save changes'}));

    expect(mockMutate).toHaveBeenCalledWith(
      expect.objectContaining({data: {allowedOrigins: [{regex: 'https://app.example.com'}]}}),
      expect.anything(),
    );
  });

  it('does not save an origin row that carries a path', async () => {
    const user = userEvent.setup();
    mockUseGetCorsConfig.mockReturnValue({
      data: makeData({readOnly: {allowedOrigins: []}, writable: {allowedOrigins: []}}),
      isLoading: false,
      error: null,
    });
    renderWithProviders(<CorsSection />);

    await user.click(screen.getByRole('button', {name: 'Add origin'}));
    // Changed without blurring, so the row error is not visible yet and Save stays enabled. This
    // used to be silently promoted to a regex rather than rejected.
    fireEvent.change(screen.getByPlaceholderText('https://app.example.com'), {
      target: {value: 'https://example.com/path'},
    });
    fireEvent.click(screen.getByRole('button', {name: 'Save changes'}));

    expect(mockMutate).not.toHaveBeenCalled();
  });

  it('removes an editable origin when its delete button is clicked', async () => {
    const user = userEvent.setup();
    mockUseGetCorsConfig.mockReturnValue({
      data: makeData({readOnly: {allowedOrigins: []}, writable: {allowedOrigins: ['https://remove.example.com']}}),
      isLoading: false,
      error: null,
    });
    renderWithProviders(<CorsSection />);

    expect(screen.getByDisplayValue('https://remove.example.com')).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: 'Remove origin'}));
    expect(screen.queryByDisplayValue('https://remove.example.com')).toBeNull();
  });

  it('adds an editable row when Add origin is clicked', async () => {
    const user = userEvent.setup();
    mockUseGetCorsConfig.mockReturnValue({
      data: makeData({writable: {allowedOrigins: []}}),
      isLoading: false,
      error: null,
    });
    renderWithProviders(<CorsSection />);

    expect(screen.queryByPlaceholderText('https://app.example.com')).toBeNull();
    await user.click(screen.getByRole('button', {name: 'Add origin'}));
    expect(screen.getByPlaceholderText('https://app.example.com')).toBeInTheDocument();
  });

  it('saves the edited origins via the update mutation and clears the unsaved bar on success', async () => {
    const user = userEvent.setup();
    mockMutate.mockImplementation((...args: unknown[]) => {
      const opts = args[1] as {onSuccess?: () => void} | undefined;
      opts?.onSuccess?.();
    });
    mockUseGetCorsConfig.mockReturnValue({
      data: makeData({readOnly: {allowedOrigins: []}, writable: {allowedOrigins: []}}),
      isLoading: false,
      error: null,
    });
    renderWithProviders(<CorsSection />);

    await user.click(screen.getByRole('button', {name: 'Add origin'}));
    await user.type(screen.getByPlaceholderText('https://app.example.com'), 'https://new.example.com');

    const saveButton = await screen.findByRole('button', {name: 'Save changes'});
    await user.click(saveButton);

    expect(mockMutate).toHaveBeenCalledWith(
      expect.objectContaining({data: {allowedOrigins: ['https://new.example.com']}}),
      expect.anything(),
    );
    // onSuccess → reset() clears the overlay, so the unsaved bar disappears.
    await waitFor(() => {
      expect(screen.queryByRole('button', {name: 'Save changes'})).toBeNull();
    });
  });

  it('does not save when a row fails the submit-time validation guard', async () => {
    const user = userEvent.setup();
    mockUseGetCorsConfig.mockReturnValue({
      data: makeData({readOnly: {allowedOrigins: []}, writable: {allowedOrigins: []}}),
      isLoading: false,
      error: null,
    });
    renderWithProviders(<CorsSection />);

    await user.click(screen.getByRole('button', {name: 'Add origin'}));
    // Change without blurring (fireEvent), so the row-level error isn't shown yet and Save is enabled.
    // '(bad' is neither a valid origin nor a compilable regex, so the submit-time guard must block it.
    fireEvent.change(screen.getByPlaceholderText('https://app.example.com'), {target: {value: '(bad'}});
    fireEvent.click(screen.getByRole('button', {name: 'Save changes'}));

    expect(mockMutate).not.toHaveBeenCalled();
  });

  it('reverts the draft when the unsaved bar is reset', async () => {
    const user = userEvent.setup();
    mockUseGetCorsConfig.mockReturnValue({
      data: makeData({readOnly: {allowedOrigins: []}, writable: {allowedOrigins: ['https://app.acme.com']}}),
      isLoading: false,
      error: null,
    });
    renderWithProviders(<CorsSection />);

    await user.click(screen.getByRole('button', {name: 'Add origin'}));
    // Every editable row carries the placeholder, so address the row that was just added.
    const addedField = screen.getAllByPlaceholderText('https://app.example.com').at(-1)!;
    await user.type(addedField, 'https://new.example.com');
    await user.click(await screen.findByRole('button', {name: 'Reset'}));

    expect(screen.queryByDisplayValue('https://new.example.com')).toBeNull();
    expect(screen.getByDisplayValue('https://app.acme.com')).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Save changes'})).toBeNull();
    expect(mockMutate).not.toHaveBeenCalled();
  });

  it('surfaces a save failure on the unsaved bar and clears it once the draft changes again', async () => {
    const user = userEvent.setup();
    updateState = {isError: true, error: new Error('save failed')};
    mockUseGetCorsConfig.mockReturnValue({
      data: makeData({readOnly: {allowedOrigins: []}, writable: {allowedOrigins: []}}),
      isLoading: false,
      error: null,
    });
    renderWithProviders(<CorsSection />);

    await user.click(screen.getByRole('button', {name: 'Add origin'}));
    await user.type(screen.getByPlaceholderText('https://app.example.com'), 'https://new.example.com');

    // The bar renders the resolved catalog message, never the server's own error text.
    const bar = await screen.findByText('Failed to update allowed origins.');
    expect(bar).toBeInTheDocument();
    expect(screen.queryByText('save failed')).toBeNull();

    // Editing again invalidates the stale failure, so the mutation state is reset.
    await user.type(screen.getByDisplayValue('https://new.example.com'), 'x');
    expect(mockReset).toHaveBeenCalled();
  });

  it('blocks Save when a row is a duplicate', async () => {
    const user = userEvent.setup();
    mockUseGetCorsConfig.mockReturnValue({
      data: makeData({readOnly: {allowedOrigins: []}, writable: {allowedOrigins: []}}),
      isLoading: false,
      error: null,
    });
    renderWithProviders(<CorsSection />);

    await user.click(screen.getByRole('button', {name: 'Add origin'}));
    await user.click(screen.getByRole('button', {name: 'Add origin'}));
    const inputs = screen.getAllByPlaceholderText('https://app.example.com');
    await user.type(inputs[0], 'https://dup.example.com');
    await user.type(inputs[1], 'https://dup.example.com');

    const saveButton = await screen.findByRole('button', {name: 'Save changes'});
    expect(saveButton).toBeDisabled();
    expect(mockMutate).not.toHaveBeenCalled();
  });
});
