// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, screen, waitFor, within} from '@testing-library/react';
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

const scopeCard = (name: string) => within(screen.getByRole('region', {name}));

const sourcesField = (card: ReturnType<typeof scopeCard>, directive: string): HTMLElement =>
  card.getByRole('combobox', {name: `Sources for ${directive}`});

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

  it('shows the Shared card and a card for each path, each with its own header', () => {
    loaded();
    renderWithProviders(<CspSection />);

    expect(screen.getByRole('switch', {name: 'Toggle Report only'})).not.toBeChecked();

    const all = scopeCard('Shared');
    expect(sourcesField(all, 'default-src')).toBeInTheDocument();
    expect(sourcesField(all, 'frame-src')).toBeInTheDocument();
    expect(all.getByRole('button', {name: 'Remove directive'})).toBeInTheDocument();
    expect(all.queryByRole('combobox', {name: 'Sources for font-src'})).toBeNull();

    const consoleCard = scopeCard('Console');
    expect(consoleCard.getByText('Applies to pages under /console/.')).toBeInTheDocument();
    expect(sourcesField(consoleCard, 'font-src')).toBeInTheDocument();
    expect(consoleCard.getByRole('group', {name: 'Header sent for Console'})).toHaveTextContent(
      "font-src 'self' data:",
    );
  });

  it('extends a read-only directive on a path and saves its existing sources too', async () => {
    const user = userEvent.setup();
    mockMutate.mockImplementation((...args: unknown[]) => {
      (args[1] as {onSuccess?: () => void} | undefined)?.onSuccess?.();
    });
    loaded();
    renderWithProviders(<CspSection />);

    await user.type(sourcesField(scopeCard('Console'), 'font-src'), 'https://fonts.example.com{Enter}');
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
    const all = scopeCard('Shared');

    await user.click(all.getAllByRole('button', {name: "'self'"})[0]);
    await user.keyboard('{Backspace}');
    await user.type(sourcesField(all, 'script-src'), 'https://cdn.example.com{Enter}');
    await user.type(sourcesField(all, 'default-src'), "'self'{Enter}");
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
    const all = scopeCard('Shared');

    await user.click(sourcesField(all, 'img-src'));
    await user.keyboard('{Backspace}');

    expect(all.getByText('Add at least one source, or reset the directive to its default.')).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Save changes'})).toBeDisabled();
  });

  it('resets an overridden directive to its default', async () => {
    const user = userEvent.setup();
    loaded(makeData({writable: {directives: {'img-src': ['https:']}}}));
    renderWithProviders(<CspSection />);

    await user.click(scopeCard('Shared').getByRole('button', {name: 'Reset to default'}));
    await user.click(screen.getByRole('button', {name: 'Save changes'}));

    expect(mockMutate).toHaveBeenCalledWith({data: {}}, expect.anything());
  });

  it('adds a directive the scope does not have and removes it again', async () => {
    const user = userEvent.setup();
    loaded();
    renderWithProviders(<CspSection />);
    const all = scopeCard('Shared');

    await user.click(all.getByRole('button', {name: 'Add directive'}));
    await user.type(sourcesField(all, 'script-src-elem'), "'self'{Enter}");
    expect(screen.getByRole('button', {name: 'Save changes'})).toBeInTheDocument();

    const removeButtons = all.getAllByRole('button', {name: 'Remove directive'});
    await user.click(removeButtons[removeButtons.length - 1]);
    expect(all.queryByRole('combobox', {name: 'Sources for script-src-elem'})).toBeNull();
    expect(screen.queryByRole('button', {name: 'Save changes'})).toBeNull();
  });

  it('changes the directive of a new row', async () => {
    const user = userEvent.setup();
    loaded();
    renderWithProviders(<CspSection />);
    const consoleCard = scopeCard('Console');

    await user.click(consoleCard.getByRole('button', {name: 'Add directive'}));
    const newDirective = consoleCard
      .getAllByRole('combobox', {name: 'Directive'})
      .find((el) => el.getAttribute('aria-disabled') !== 'true')!;
    await user.click(newDirective);
    await user.click(screen.getByRole('option', {name: 'media-src'}));
    await user.type(sourcesField(consoleCard, 'media-src'), "'self'{Enter}");
    await user.click(screen.getByRole('button', {name: 'Save changes'}));

    expect(mockMutate).toHaveBeenCalledWith(
      {
        data: {
          directives: {'frame-src': ["'self'"]},
          paths: [{location: '/console/', directives: {'media-src': ["'self'"]}}],
        },
      },
      expect.anything(),
    );
  });

  it('flags a source containing a separator and blocks the save', async () => {
    const user = userEvent.setup();
    loaded();
    renderWithProviders(<CspSection />);

    await user.type(sourcesField(scopeCard('Shared'), 'img-src'), 'data:;{Enter}');

    expect(screen.getByText("A source can't contain a semicolon or comma.")).toBeInTheDocument();
    expect(screen.getByRole('button', {name: 'Save changes'})).toBeDisabled();
  });

  it('updates each header with the draft and the report-only choice', async () => {
    const user = userEvent.setup();
    loaded();
    renderWithProviders(<CspSection />);
    const consoleHeader = scopeCard('Console').getByRole('group', {name: 'Header sent for Console'});

    expect(consoleHeader).toHaveTextContent(/^Content-Security-Policy: default-src 'none';/);
    expect(consoleHeader).toHaveTextContent("frame-src 'self'");

    await user.type(sourcesField(scopeCard('Console'), 'img-src'), 'https:{Enter}');
    expect(consoleHeader).toHaveTextContent("img-src 'self' https:");
    expect(
      scopeCard('Shared').getByRole('group', {name: 'Header sent when the Console or Gate has no section of its own'}),
    ).toHaveTextContent("img-src 'self';");

    await user.click(screen.getByRole('switch', {name: 'Toggle Report only'}));
    expect(consoleHeader).toHaveTextContent(/^Content-Security-Policy-Report-Only: /);
  });

  it('edits and saves a path that only the saved layer has', async () => {
    const user = userEvent.setup();
    loaded(makeData({writable: {paths: [{location: '/custom/', directives: {'img-src': ['data:']}}]}}));
    renderWithProviders(<CspSection />);

    await user.type(sourcesField(scopeCard('/custom/'), 'img-src'), 'https:{Enter}');
    await user.click(screen.getByRole('button', {name: 'Save changes'}));

    expect(mockMutate).toHaveBeenCalledWith(
      {data: {paths: [{location: '/custom/', directives: {'img-src': ['data:', 'https:']}}]}},
      expect.anything(),
    );
  });

  it('clears the draft once the save completes', async () => {
    const user = userEvent.setup();
    let finishSave: (() => void) | undefined;
    mockMutate.mockImplementation((...args: unknown[]) => {
      finishSave = (args[1] as {onSuccess?: () => void} | undefined)?.onSuccess;
    });
    loaded();
    const {rerender} = renderWithProviders(<CspSection />);

    await user.type(sourcesField(scopeCard('Shared'), 'img-src'), 'data:{Enter}');
    await user.click(screen.getByRole('button', {name: 'Save changes'}));
    expect(mockMutate).toHaveBeenCalledWith(
      {data: {directives: {'frame-src': ["'self'"], 'img-src': ["'self'", 'data:']}}},
      expect.anything(),
    );

    loaded(makeData({writable: {directives: {'frame-src': ["'self'"], 'img-src': ["'self'", 'data:']}}}));
    act(() => finishSave!());
    rerender(<CspSection />);

    expect(scopeCard('Shared').getByRole('button', {name: 'data:'})).toBeInTheDocument();
    expect(screen.queryByRole('button', {name: 'Save changes'})).toBeNull();
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
