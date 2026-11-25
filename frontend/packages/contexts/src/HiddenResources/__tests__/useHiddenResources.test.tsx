// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, type JSX} from 'react';
import {createRoot, type Root} from 'react-dom/client';
import {afterEach, beforeEach, describe, expect, it} from 'vitest';
import type {HiddenResources} from '../HiddenResourcesContext';
import HiddenResourcesProvider from '../HiddenResourcesProvider';
import useHiddenResources from '../useHiddenResources';

const APP_ID = '01a19b04-d7f7-7790-ad53-9139664d0a6a';
const RS_ID = '01a1864b-ef42-7c41-b0a2-97f53fd3b1e4';

const HIDDEN: HiddenResources = {
  applicationClientIds: ['ADMIN_CONSOLE'],
  resourceServerIdentifiers: ['var:DATA_PLANE_URL'],
  ids: {application: [APP_ID], resource_server: [RS_ID]},
};

/** Each check, by name, with what it answers for the probed values. */
const PROBES: Record<string, (checks: ReturnType<typeof useHiddenResources>) => boolean> = {
  clientId: (checks) => checks.isHiddenApplication('ADMIN_CONSOLE'),
  clientIdOtherCase: (checks) => checks.isHiddenApplication('admin_console'),
  otherClientId: (checks) => checks.isHiddenApplication('CONSOLE'),
  noClientId: (checks) => checks.isHiddenApplication(undefined),
  identifier: (checks) => checks.isHiddenResourceServer('var:DATA_PLANE_URL'),
  otherIdentifier: (checks) => checks.isHiddenResourceServer('https://api.example.com'),
  noIdentifier: (checks) => checks.isHiddenResourceServer(undefined),
  appId: (checks) => checks.isHiddenResource('application', APP_ID),
  rsIdOtherCase: (checks) => checks.isHiddenResource('resource_server', RS_ID.toUpperCase()),
  idOfOtherType: (checks) => checks.isHiddenResource('resource_server', APP_ID),
  typeNotListed: (checks) => checks.isHiddenResource('role', APP_ID),
  noId: (checks) => checks.isHiddenResource('application', undefined),
};

let container: HTMLDivElement;
let root: Root;

function Probe(): JSX.Element {
  const checks = useHiddenResources();
  return (
    <ul>
      {Object.entries(PROBES).map(([name, probe]) => (
        <li key={name} data-testid={name}>
          {String(probe(checks))}
        </li>
      ))}
    </ul>
  );
}

function render(hidden?: HiddenResources): void {
  act(() => {
    root.render(
      hidden ? (
        <HiddenResourcesProvider hidden={hidden}>
          <Probe />
        </HiddenResourcesProvider>
      ) : (
        <Probe />
      ),
    );
  });
}

function answer(name: string): string | null | undefined {
  return container.querySelector(`[data-testid="${name}"]`)?.textContent;
}

describe('useHiddenResources', () => {
  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
    root = createRoot(container);
  });

  afterEach(() => {
    act(() => root.unmount());
    container.remove();
  });

  it('hides an application by its client id, ignoring case', () => {
    render(HIDDEN);

    expect(answer('clientId')).toBe('true');
    expect(answer('clientIdOtherCase')).toBe('true');
    expect(answer('otherClientId')).toBe('false');
    expect(answer('noClientId')).toBe('false');
  });

  it('hides a resource server by its identifier', () => {
    render(HIDDEN);

    expect(answer('identifier')).toBe('true');
    expect(answer('otherIdentifier')).toBe('false');
    expect(answer('noIdentifier')).toBe('false');
  });

  it('hides a resource by its type and id', () => {
    render(HIDDEN);

    expect(answer('appId')).toBe('true');
    expect(answer('rsIdOtherCase')).toBe('true');
    expect(answer('idOfOtherType')).toBe('false');
    expect(answer('typeNotListed')).toBe('false');
    expect(answer('noId')).toBe('false');
  });

  it('hides nothing without a provider', () => {
    render();

    expect(answer('clientId')).toBe('false');
    expect(answer('identifier')).toBe('false');
    expect(answer('appId')).toBe('false');
  });
});
