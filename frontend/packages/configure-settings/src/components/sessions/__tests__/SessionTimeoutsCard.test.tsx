// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {focusManager} from '@tanstack/react-query';
import {screen, waitFor, within} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {CapabilitiesProvider} from '@thunderid/contexts';
import {renderWithProviders} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, it, expect, vi} from 'vitest';
import type {SessionConfigResponse, SessionValue} from '../../../models/session';

/** A request as the card's hooks send it. */
interface SentRequest {
  method: string;
  url: string;
  data?: SessionValue;
}

const request = vi.hoisted(() => vi.fn());
// The shared components read other hooks from the SDK, so only the HTTP client is replaced.
vi.mock('@thunderid/react', async (importOriginal) => {
  const actual = await importOriginal();

  return {
    ...(actual as object),
    useThunderID: () => ({
      http: {request},
    }),
  };
});

const {default: SessionTimeoutsCard} = await import('../SessionTimeoutsCard');

/** The server's built-in defaults: two hours idle, eight hours in all. */
const builtInDefaults: Required<SessionValue> = {idleTimeoutSeconds: 7200, absoluteTimeoutSeconds: 28800};

/** Idle left to its two-hour default; the absolute timeout set here to twelve hours. */
const section = (overrides: Partial<SessionConfigResponse> = {}): SessionConfigResponse => ({
  readOnly: {},
  writable: {absoluteTimeoutSeconds: 43200},
  merged: {idleTimeoutSeconds: 7200, absoluteTimeoutSeconds: 43200},
  ...overrides,
});

/** The server: one `session` section. A PUT is stored unless `refusal` says how to refuse it. */
const server = {
  section: section(),
  refusal: undefined as {status: number; code: string} | undefined,
};

function answer(sent: SentRequest): Promise<unknown> {
  if (sent.method === 'PUT') {
    if (server.refusal) {
      const {status, code} = server.refusal;
      return Promise.reject(
        Object.assign(new Error('Request failed'), {
          response: {status, data: {code, message: 'raw server message', description: 'raw server description'}},
        }),
      );
    }
    server.section = {
      ...server.section,
      writable: sent.data ?? {},
      merged: {...builtInDefaults, ...sent.data},
    };
  }
  return Promise.resolve({data: server.section});
}

const sent = (method: string): SentRequest[] =>
  request.mock.calls.map(([call]) => call as SentRequest).filter((call) => call.method === method);

const idle = (): HTMLElement => screen.getByRole('spinbutton', {name: 'Idle timeout'});
const absolute = (): HTMLElement => screen.getByRole('spinbutton', {name: 'Absolute timeout'});
const unit = (testId: string): HTMLElement => within(screen.getByTestId(`${testId}-unit`)).getByRole('combobox');

/** Types a number into a duration box, in the unit it shows. */
async function enter(box: HTMLElement, value: string): Promise<void> {
  await userEvent.clear(box);
  await userEvent.type(box, value);
}

const viewOnly = {
  can: (_resource: string, action = 'view') => action === 'view',
  canManageRoot: false,
  isLoading: false,
};

describe('SessionTimeoutsCard', () => {
  beforeEach(() => {
    server.section = section();
    server.refusal = undefined;
    request.mockReset();
    request.mockImplementation(answer);
  });

  afterEach(() => {
    focusManager.setFocused(undefined);
  });

  it('shows each timeout in effect, and whether it is the default or set here', async () => {
    renderWithProviders(<SessionTimeoutsCard />);

    expect(await screen.findByRole('spinbutton', {name: 'Idle timeout'})).toHaveValue(2);
    expect(unit('session-idle-timeout')).toHaveTextContent('Hours');
    expect(absolute()).toHaveValue(12);
    expect(unit('session-absolute-timeout')).toHaveTextContent('Hours');
    expect(screen.getByText('Default')).toBeVisible();
    expect(screen.getByText('Custom')).toBeVisible();
    // Only the value set here has a way back, and it says which field it belongs to.
    const resets = screen.getAllByRole('button', {name: 'Reset to default'});
    expect(resets).toHaveLength(1);
    expect(resets[0]).toHaveAccessibleDescription('Absolute timeout');
    expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
    expect(sent('GET')[0].url).toMatch(/\/server-config\/session$/);
  });

  it('ties what each timeout does to its controls', async () => {
    renderWithProviders(<SessionTimeoutsCard />);

    expect(await screen.findByRole('spinbutton', {name: 'Idle timeout'})).toHaveAccessibleDescription(
      'Signs the user out after this long without activity.',
    );
    expect(unit('session-absolute-timeout')).toHaveAccessibleDescription(
      "Signs the user out this long after they sign in, even if they're active.",
    );
  });

  it('saves a change with the values already set', async () => {
    renderWithProviders(<SessionTimeoutsCard />);

    await enter(await screen.findByRole('spinbutton', {name: 'Idle timeout'}), '1');
    expect(screen.getByText('You have unsaved changes')).toBeVisible();
    await userEvent.click(screen.getByRole('button', {name: 'Save changes'}));

    await waitFor(() => expect(sent('PUT')).toHaveLength(1));
    expect(sent('PUT')[0]).toMatchObject({
      method: 'PUT',
      data: {absoluteTimeoutSeconds: 43200, idleTimeoutSeconds: 3600},
    });
    expect(sent('PUT')[0].url).toMatch(/\/server-config\/session$/);
    await waitFor(() => expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument());
    // The toast fades in, so it is present before it is visible.
    expect(await screen.findByText('Session settings updated.')).toBeInTheDocument();
    expect(screen.getAllByText('Custom')).toHaveLength(2);
    expect(idle()).toHaveValue(1);
  });

  it('returns a value to its default by leaving it out of the save', async () => {
    renderWithProviders(<SessionTimeoutsCard />);

    await userEvent.click(await screen.findByRole('button', {name: 'Reset to default'}));

    // The default is known only once the save returns the value in effect, so the box is empty.
    expect(absolute()).toHaveValue(null);
    expect(unit('session-absolute-timeout')).toHaveTextContent('Hours');
    expect(screen.getAllByText('Default')).toHaveLength(2);
    expect(screen.queryByText('Enter a whole number greater than zero.')).not.toBeInTheDocument();
    await userEvent.click(screen.getByRole('button', {name: 'Save changes'}));

    await waitFor(() => expect(sent('PUT')).toHaveLength(1));
    expect(sent('PUT')[0].data).toEqual({});
    await waitFor(() => expect(absolute()).toHaveValue(8));
    expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
  });

  // The save's success resets the draft, so an edit made while it is in flight would be lost.
  it('holds the fields and every reset while a save is in flight', async () => {
    let finishSave: () => void = () => undefined;
    request.mockImplementation((call: SentRequest) =>
      call.method === 'PUT'
        ? new Promise((resolve) => {
            finishSave = () => resolve(answer(call));
          })
        : answer(call),
    );
    renderWithProviders(<SessionTimeoutsCard />);

    await enter(await screen.findByRole('spinbutton', {name: 'Idle timeout'}), '1');
    await userEvent.click(screen.getByRole('button', {name: 'Save changes'}));

    expect(await screen.findByRole('button', {name: 'Saving...'})).toBeDisabled();
    expect(idle()).toBeDisabled();
    expect(absolute()).toBeDisabled();
    expect(unit('session-idle-timeout')).toHaveAttribute('aria-disabled', 'true');
    for (const reset of screen.getAllByRole('button', {name: 'Reset to default'})) {
      expect(reset).toBeDisabled();
    }
    expect(screen.getByRole('button', {name: 'Reset'})).toBeDisabled();

    finishSave();

    await waitFor(() => expect(idle()).toBeEnabled());
    expect(idle()).toHaveValue(1);
    expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
  });

  it('discards the draft on reset', async () => {
    renderWithProviders(<SessionTimeoutsCard />);

    await enter(await screen.findByRole('spinbutton', {name: 'Idle timeout'}), '1');
    await userEvent.click(screen.getByRole('button', {name: 'Reset'}));

    expect(idle()).toHaveValue(2);
    expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
    expect(sent('PUT')).toHaveLength(0);
  });

  describe('when the timeouts do not fit together', () => {
    const message = "The idle timeout can't be longer than the absolute timeout.";

    it('refuses an idle timeout longer than the absolute timeout', async () => {
      renderWithProviders(<SessionTimeoutsCard />);

      await enter(await screen.findByRole('spinbutton', {name: 'Idle timeout'}), '13');

      expect(screen.getByText(message)).toBeVisible();
      expect(idle()).toHaveAttribute('aria-invalid', 'true');
      // The error is read after what the timeout does.
      expect(idle()).toHaveAccessibleDescription(`Signs the user out after this long without activity. ${message}`);
      expect(screen.getByRole('button', {name: 'Save changes'})).toBeDisabled();
      expect(sent('PUT')).toHaveLength(0);
    });

    it('says so on the absolute timeout when that is what was changed', async () => {
      renderWithProviders(<SessionTimeoutsCard />);

      await enter(await screen.findByRole('spinbutton', {name: 'Idle timeout'}), '2');
      await enter(screen.getByRole('spinbutton', {name: 'Absolute timeout'}), '1');

      expect(screen.getByText(message)).toBeVisible();
      expect(absolute()).toHaveAttribute('aria-invalid', 'true');
      expect(idle()).toHaveAttribute('aria-invalid', 'false');
      expect(screen.getByRole('button', {name: 'Save changes'})).toBeDisabled();
    });

    // As on the server, an absolute timeout set on its own also cuts the idle timeout in effect.
    it('accepts an absolute timeout shorter than the default idle timeout', async () => {
      renderWithProviders(<SessionTimeoutsCard />);

      await enter(await screen.findByRole('spinbutton', {name: 'Absolute timeout'}), '1');

      expect(screen.queryByText(message)).not.toBeInTheDocument();
      expect(screen.getByRole('button', {name: 'Save changes'})).toBeEnabled();
    });
  });

  describe('when a save is refused', () => {
    it.each([
      [
        'SCF-1003',
        400,
        "These session timeouts aren't valid. The idle timeout must be longer than the activity refresh interval (1 minute by default) and no longer than the absolute timeout.",
      ],
      ['SCF-1006', 400, 'The control plane manages these settings, so they can only be changed there.'],
      ['SCF-9999', 500, 'Failed to update the session settings.'],
    ])('explains %s in the save bar, never in the server words', async (code, status, message) => {
      server.refusal = {status, code};
      renderWithProviders(<SessionTimeoutsCard />);

      await enter(await screen.findByRole('spinbutton', {name: 'Idle timeout'}), '1');
      await userEvent.click(screen.getByRole('button', {name: 'Save changes'}));

      expect(await screen.findByText(message)).toBeVisible();
      expect(screen.queryByText(/raw server/)).not.toBeInTheDocument();
      // The draft is kept, so the operator can correct it and save again.
      expect(idle()).toHaveValue(1);
    });

    it('clears the reason once the draft changes again', async () => {
      server.refusal = {status: 400, code: 'SCF-1003'};
      renderWithProviders(<SessionTimeoutsCard />);

      await enter(await screen.findByRole('spinbutton', {name: 'Idle timeout'}), '1');
      await userEvent.click(screen.getByRole('button', {name: 'Save changes'}));
      expect(await screen.findByText(/These session timeouts aren't valid/)).toBeVisible();

      await enter(idle(), '3');

      expect(screen.queryByText(/These session timeouts aren't valid/)).not.toBeInTheDocument();
    });
  });

  describe('when a declarative resource sets a timeout', () => {
    it('names the declarative resource to edit, and offers no change', async () => {
      server.section = section({
        readOnly: {idleTimeoutSeconds: 3600},
        writable: {},
        merged: {idleTimeoutSeconds: 3600, absoluteTimeoutSeconds: 28800},
      });
      renderWithProviders(<SessionTimeoutsCard />);

      expect(await screen.findByText(/These values come from a declarative resource/)).toBeVisible();
      expect(screen.getByText('server_configs/session.yaml')).toBeVisible();
      expect(idle()).toBeDisabled();
      expect(idle()).toHaveValue(1);
      expect(absolute()).toBeDisabled();
      expect(unit('session-idle-timeout')).toHaveAttribute('aria-disabled', 'true');
      // The resource sets the idle timeout; the absolute timeout is the default.
      expect(screen.getByText('Custom')).toBeVisible();
      expect(screen.getByText('Default')).toBeVisible();
      expect(screen.queryByRole('button', {name: /Reset to default/})).not.toBeInTheDocument();
    });

    // The draft could never be saved, so it is dropped and the values in effect show under the notice.
    it('drops a draft when a later read finds a declarative resource setting a timeout', async () => {
      renderWithProviders(<SessionTimeoutsCard />);
      await enter(await screen.findByRole('spinbutton', {name: 'Idle timeout'}), '1');
      expect(screen.getByText('You have unsaved changes')).toBeVisible();

      server.section = section({readOnly: {absoluteTimeoutSeconds: 43200}, writable: {}});
      focusManager.setFocused(false);
      focusManager.setFocused(true);

      expect(await screen.findByText(/These values come from a declarative resource/)).toBeVisible();
      expect(idle()).toHaveValue(2);
      expect(idle()).toBeDisabled();
      expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
    });

    it('shows no lock notice for a section that can be written', async () => {
      renderWithProviders(<SessionTimeoutsCard />);

      expect(await screen.findByRole('spinbutton', {name: 'Idle timeout'})).toBeEnabled();
      expect(screen.queryByRole('alert')).not.toBeInTheDocument();
    });
  });

  it('shows no timeouts while the section loads', () => {
    request.mockImplementation(() => new Promise(() => undefined));
    renderWithProviders(<SessionTimeoutsCard />);

    expect(screen.getByText('Session timeouts')).toBeVisible();
    expect(screen.queryByRole('spinbutton')).not.toBeInTheDocument();
  });

  it('reports a failed read in the card and reads again on retry', async () => {
    request.mockRejectedValue(new Error('boom'));
    renderWithProviders(<SessionTimeoutsCard />);

    expect(await screen.findByText('Failed to load the session settings.')).toBeVisible();
    await userEvent.click(screen.getByRole('button', {name: 'Refresh'}));

    await waitFor(() => expect(sent('GET')).toHaveLength(2));
  });

  it('is read-only for an admin who may not update server configuration', async () => {
    renderWithProviders(
      <CapabilitiesProvider capabilities={viewOnly}>
        <SessionTimeoutsCard />
      </CapabilitiesProvider>,
    );

    expect(await screen.findByRole('spinbutton', {name: 'Idle timeout'})).toBeDisabled();
    expect(absolute()).toBeDisabled();
    expect(unit('session-absolute-timeout')).toHaveAttribute('aria-disabled', 'true');
    expect(screen.getByText('Custom')).toBeVisible();
    expect(screen.queryByRole('button', {name: /Reset to default/})).not.toBeInTheDocument();
    expect(screen.queryByText('You have unsaved changes')).not.toBeInTheDocument();
  });
});
