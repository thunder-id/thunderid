// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen, waitFor} from '@thunderid/test-utils';
import {useCallback, useState, type JSX} from 'react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import EditTokenSettings from '../EditTokenSettings';
import type {Application} from '@thunderid/configure-applications';

// Stable mock references, created via vi.hoisted so the hoisted vi.mock factories can use them.
// New identities on every render would re-fire the schema-fetch effect in a loop.
const {mockHttp, mockGetServerUrl, mockLogger} = vi.hoisted(() => ({
  mockHttp: {request: vi.fn().mockResolvedValue({data: {id: 'schema-1', name: 'default', schema: {}}})},
  mockGetServerUrl: vi.fn().mockReturnValue('https://api.example.com'),
  mockLogger: {error: vi.fn(), info: vi.fn(), debug: vi.fn()},
}));

vi.mock('@thunderid/configure-user-types', () => ({
  useGetUserTypes: () => ({data: {types: [{id: 'schema-1', name: 'default'}]}, isLoading: false}),
}));

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: mockHttp}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: () => ({getServerUrl: mockGetServerUrl})};
});

vi.mock('@thunderid/logger', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/logger')>();
  return {...actual, useLogger: () => mockLogger};
});

const CLICKABLE = ['email', 'family_name', 'given_name'];

// Stands in for the attribute chips so a click can be driven by attribute name. Also echoes the
// list the component considers currently selected, which is what regressed.
vi.mock('../TokenUserAttributesSection', () => ({
  default: ({
    sharedAttributes,
    onAttributeClick,
  }: {
    sharedAttributes?: string[];
    onAttributeClick?: (attr: string, scope: 'shared') => void;
  }) => (
    <div>
      <div data-testid="shared-attributes">{(sharedAttributes ?? []).join(',')}</div>
      {['email', 'family_name', 'given_name'].map((attr) => (
        <button
          key={attr}
          type="button"
          data-testid={`chip-${attr}`}
          onClick={() => onAttributeClick?.(attr, 'shared')}
        >
          {attr}
        </button>
      ))}
    </div>
  ),
}));

// Hoisted out of the harness so its identity is stable across renders — a fresh object would give
// `allowedUserTypes` a new reference each render and re-fire the schema-fetch effect indefinitely.
const baseApplication = {id: 'app-1', name: 'App Native App', allowedUserTypes: ['default']} as Application;

/**
 * Mirrors how ApplicationEditPage owns the pending edits: `onFieldChange` accumulates into
 * `editedApp`, which is fed straight back to the component. Without that round trip the component
 * has no way to see its own earlier edits, which is the regression under test.
 */
function Harness({assertion = undefined}: {assertion?: Application['assertion']}): JSX.Element {
  const [editedApp, setEditedApp] = useState<Partial<Application>>({});

  const onFieldChange = useCallback((field: keyof Application, value: unknown) => {
    setEditedApp((prev) => ({...prev, [field]: value}));
  }, []);

  const application = assertion ? ({...baseApplication, assertion} as Application) : baseApplication;

  return (
    <>
      <EditTokenSettings application={application} editedApp={editedApp} onFieldChange={onFieldChange} />
      <div data-testid="pending-attributes">{(editedApp.assertion?.userAttributes ?? []).join(',')}</div>
      <div data-testid="pending-validity">{String(editedApp.assertion?.validityPeriod ?? '')}</div>
    </>
  );
}

describe('EditTokenSettings native-mode attribute selection', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it('accumulates successive attribute selections instead of replacing the previous one', async () => {
    render(<Harness />);

    for (const attr of CLICKABLE) {
      fireEvent.click(screen.getByTestId(`chip-${attr}`));
      // Each click must observe the attributes added by the ones before it.
      await waitFor(() => {
        expect(screen.getByTestId('pending-attributes').textContent).toContain(attr);
      });
    }

    expect(screen.getByTestId('pending-attributes').textContent).toBe('email,family_name,given_name');
    expect(screen.getByTestId('shared-attributes').textContent).toBe('email,family_name,given_name');
  });

  it('keeps stored attributes when a further one is added', async () => {
    render(<Harness assertion={{validityPeriod: 3600, userAttributes: ['email']}} />);

    fireEvent.click(screen.getByTestId('chip-given_name'));

    await waitFor(() => {
      expect(screen.getByTestId('pending-attributes').textContent).toBe('email,given_name');
    });
  });

  it('deselects only the clicked attribute', async () => {
    render(<Harness assertion={{validityPeriod: 3600, userAttributes: ['email', 'family_name', 'given_name']}} />);

    fireEvent.click(screen.getByTestId('chip-family_name'));

    await waitFor(() => {
      expect(screen.getByTestId('pending-attributes').textContent).toBe('email,given_name');
    });
  });

  it('does not drop pending attributes when the validity period is then changed', async () => {
    render(<Harness assertion={{validityPeriod: 7200, userAttributes: []}} />);

    await waitFor(() => {
      expect(screen.getByDisplayValue('7200')).toBeInTheDocument();
    });

    fireEvent.click(screen.getByTestId('chip-email'));

    await waitFor(() => {
      expect(screen.getByTestId('pending-attributes').textContent).toBe('email');
    });

    // The validity period is written to the same assertion object, so it must merge with, rather
    // than replace, the attribute selection made above.
    fireEvent.change(screen.getByDisplayValue('7200'), {target: {value: '900'}});

    await waitFor(() => {
      expect(screen.getByTestId('pending-validity').textContent).toBe('900');
    });
    expect(screen.getByTestId('pending-attributes').textContent).toBe('email');
  });
});
