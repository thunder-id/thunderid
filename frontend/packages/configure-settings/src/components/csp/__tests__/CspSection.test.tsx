// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {screen, waitFor} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {CapabilitiesProvider} from '@thunderid/contexts';
import {renderWithProviders} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import type {CspConfigResponse} from '../../../models/responses';

const mockRefetch = vi.fn();
const mockUseGetCspConfig =
  vi.fn<() => {data: CspConfigResponse | undefined; isLoading: boolean; error: Error | null; refetch?: () => void}>();
vi.mock('../../../api/useGetCspConfig', () => ({
  default: () => ({refetch: mockRefetch, ...mockUseGetCspConfig()}),
}));

const mockMutate = vi.fn();
const mockReset = vi.fn();
let updateState: {isError: boolean; error: Error | null} = {isError: false, error: null};
vi.mock('../../../api/useUpdateCspConfig', () => ({
  default: () => ({mutate: mockMutate, isPending: false, reset: mockReset, ...updateState}),
}));

const {default: CspSection} = await import('../CspSection');

function makeData(overrides?: Partial<CspConfigResponse>): CspConfigResponse {
  return {
    readOnly: {
      reportOnly: false,
      paths: [{location: '/console/', directives: {'font-src': ["'self'", 'data:']}}],
    },
    writable: {directives: {'frame-src': ["'self'"]}},
    merged: {reportOnly: false},
    ...overrides,
  };
}

function loaded(data: CspConfigResponse = makeData()): void {
  mockUseGetCspConfig.mockReturnValue({data, isLoading: false, error: null});
}

const viewOnly = {
  can: (_resource: string, action = 'view') => action === 'view',
  isLoading: false,
};

const sourcesField = (directive: string): HTMLElement =>
  screen.getByRole('combobox', {name: `Sources for ${directive}`});

async function selectScope(user: ReturnType<typeof userEvent.setup>, scope: string): Promise<void> {
  await user.click(screen.getByRole('combobox', {name: 'Applies to'}));
  await user.click(screen.getByRole('option', {name: scope}));
}

describe('CspSection', () => {
  beforeEach(() => {
    mockUseGetCspConfig.mockReset();
    mockMutate.mockReset();
    mockReset.mockReset();
    mockRefetch.mockReset();
    updateState = {isError: false, error: null};
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('does not render the editor or the report-only switch while loading', () => {
    mockUseGetCspConfig.mockReturnValue({data: undefined, isLoading: true, error: null});
    renderWithProviders(<CspSection />);

    expect(screen.queryByRole('button', {name: 'Add directive'})).toBeNull();
    expect(screen.queryByRole('switch')).toBeNull();
  });

  it('shows an alert on load error and refetches on retry', async () => {
    const user = userEvent.setup();
    mockUseGetCspConfig.mockReturnValue({data: undefined, isLoading: false, error: new Error('load failed')});
    renderWithProviders(<CspSection />);

    expect(screen.getByRole('alert')).toBeInTheDocument();
    await user.click(screen.getByRole('button', {name: /refresh/i}));
    expect(mockRefetch).toHaveBeenCalled();
  });

  it('shows the baseline directives and the saved ones for All pages', () => {
    loaded();
    renderWithProviders(<CspSection />);

    expect(screen.getByRole('switch', {name: 'Toggle Report only'})).not.toBeChecked();
    expect(screen.getByRole('combobox', {name: 'Applies to'})).toHaveTextContent('All pages');
    expect(sourcesField('default-src')).toBeInTheDocument();
    expect(sourcesField('img-src')).toBeInTheDocument();
    expect(sourcesField('frame-src')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Remove directive'})).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Reset to default'})).toBeNull();
  });

  it('removes a read-only source on a path, saving the rest of the directive', async () => {
    const user = userEvent.setup();
    loaded();
    renderWithProviders(<CspSection />);

    await selectScope(user, '/console/');
    expect(screen.getByRole('combobox', {name: 'Applies to'})).toHaveTextContent('/console/');
    await user.click(sourcesField('font-src'));
    await user.keyboard('{Backspace}');
    await user.click(screen.getByRole('button', {name: 'Save changes'}));

    expect(mockMutate).toHaveBeenCalledWith(
      {
        data: {
          directives: {'frame-src': ["'self'"]},
          paths: [{location: '/console/', directives: {'font-src': ["'self'"]}}],
        },
      },
      expect.anything(),
    );
  });

  it('extends a read-only directive on a path and saves its existing sources too', async () => {
    const user = userEvent.setup();
    mockMutate.mockImplementation((...args: unknown[]) => {
      (args[1] as {onSuccess?: () => void} | undefined)?.onSuccess?.();
    });
    loaded();
    renderWithProviders(<CspSection />);

    await selectScope(user, '/console/');
    await user.type(sourcesField('font-src'), 'https://fonts.example.com{Enter}');
    await user.click(await screen.findByRole('button', {name: 'Save changes'}));

    expect(mockMutate).toHaveBeenCalledWith(
      {
        data: {
          directives: {'frame-src': ["'self'"]},
          paths: [{location: '/console/', directives: {'font-src': ["'self'", 'data:', 'https://fonts.example.com']}}],
        },
      },
      expect.anything(),
    );
    await waitFor(() => {
      expect(screen.queryByRole('button', {name: 'Save changes'})).toBeNull();
    });
  });

  it("removes a baseline source and replaces 'none' when a source is added to it", async () => {
    const user = userEvent.setup();
    loaded();
    renderWithProviders(<CspSection />);

    await user.click(screen.getAllByRole('button', {name: "'self'"})[0]);
    await user.keyboard('{Backspace}');
    await user.type(sourcesField('script-src'), 'https://cdn.example.com{Enter}');
    await user.type(sourcesField('default-src'), "'self'{Enter}");
    await user.click(screen.getByRole('button', {name: 'Save changes'}));

    expect(mockMutate).toHaveBeenCalledWith(
      {
        data: {
          directives: {
            'frame-src': ["'self'"],
            'script-src': ['https://cdn.example.com'],
            'default-src': ["'self'"],
          },
        },
      },
      expect.anything(),
    );
  });

  it('blocks the save when every source is removed from a directive with a default value', async () => {
    const user = userEvent.setup();
    loaded();
    renderWithProviders(<CspSection />);

    await user.click(sourcesField('img-src'));
    await user.keyboard('{Backspace}');

    expect(screen.getByText('Add at least one source, or reset the directive to its default.')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Save changes'})).toBeDisabled();
  });

  it('resets an overridden directive to its default', async () => {
    const user = userEvent.setup();
    loaded(makeData({writable: {directives: {'img-src': ['https:']}}}));
    renderWithProviders(<CspSection />);

    await user.click(screen.getByRole('button', {name: 'Reset to default'}));
    await user.click(screen.getByRole('button', {name: 'Save changes'}));

    expect(mockMutate).toHaveBeenCalledWith({data: {}}, expect.anything());
  });

  it('adds a directive the scope does not have and removes it again', async () => {
    const user = userEvent.setup();
    loaded();
    renderWithProviders(<CspSection />);

    await user.click(screen.getByRole('button', {name: 'Add directive'}));
    await user.type(sourcesField('script-src-elem'), "'self'{Enter}");
    expect(screen.getByRole('button', {name: 'Save changes'})).toBeInTheDocument();

    const removeButtons = screen.getAllByRole('button', {name: 'Remove directive'});
    await user.click(removeButtons[removeButtons.length - 1]);
    expect(screen.queryByRole('combobox', {name: 'Sources for script-src-elem'})).toBeNull();
    expect(screen.queryByRole('button', {name: 'Save changes'})).toBeNull();
  });

  it('flags a source containing a separator and blocks the save', async () => {
    const user = userEvent.setup();
    loaded();
    renderWithProviders(<CspSection />);

    await user.type(sourcesField('img-src'), 'data:;{Enter}');

    expect(screen.getByText("A source can't contain a semicolon or comma.")).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Save changes'})).toBeDisabled();
  });

  it('shows the effective header for the selected scope and updates it with the draft', async () => {
    const user = userEvent.setup();
    loaded();
    renderWithProviders(<CspSection />);

    await selectScope(user, '/console/');
    const header = screen.getByRole('group', {name: 'Header sent for /console/'});
    expect(header).toHaveTextContent(/^Content-Security-Policy: default-src 'none';/);
    expect(header).toHaveTextContent("font-src 'self' data:");
    expect(header).toHaveTextContent("frame-src 'self'");

    await user.type(sourcesField('img-src'), 'https:{Enter}');
    expect(header).toHaveTextContent("img-src 'self' https:");

    await user.click(screen.getByRole('switch', {name: 'Toggle Report only'}));
    expect(header).toHaveTextContent(/^Content-Security-Policy-Report-Only: /);
  });

  it('saves the report-only choice', async () => {
    const user = userEvent.setup();
    loaded();
    renderWithProviders(<CspSection />);

    await user.click(screen.getByRole('switch', {name: 'Toggle Report only'}));
    await user.click(await screen.findByRole('button', {name: 'Save changes'}));

    expect(mockMutate).toHaveBeenCalledWith(
      {data: {reportOnly: true, directives: {'frame-src': ["'self'"]}}},
      expect.anything(),
    );
  });

  it('disables Save when the admin may not update server configuration', async () => {
    const user = userEvent.setup();
    loaded();
    renderWithProviders(
      <CapabilitiesProvider capabilities={viewOnly}>
        <CspSection />
      </CapabilitiesProvider>,
    );

    await user.click(screen.getByRole('switch', {name: 'Toggle Report only'}));

    expect(await screen.findByRole('button', {name: 'Save changes'})).toBeDisabled();
  });

  it('shows a CSP-specific message when the server rejects the value', async () => {
    const user = userEvent.setup();
    updateState = {
      isError: true,
      error: Object.assign(new Error('bad'), {response: {data: {code: 'SCF-1003'}}}),
    };
    loaded();
    renderWithProviders(<CspSection />);

    await user.click(screen.getByRole('switch', {name: 'Toggle Report only'}));

    expect(
      await screen.findByText('One or more directives are not valid. Check the directive names and sources.'),
    ).toBeInTheDocument();
  });

  it('resets the draft', async () => {
    const user = userEvent.setup();
    loaded();
    renderWithProviders(<CspSection />);

    await user.click(screen.getByRole('switch', {name: 'Toggle Report only'}));
    await user.click(await screen.findByRole('button', {name: 'Reset'}));

    expect(screen.getByRole('switch', {name: 'Toggle Report only'})).not.toBeChecked();
    expect(screen.queryByRole('button', {name: 'Save changes'})).toBeNull();
  });
});
