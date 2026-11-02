// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {fireEvent, render, screen} from '@testing-library/react';
import type {JSX, ReactNode} from 'react';
import {MemoryRouter, Route, Routes, useLocation} from 'react-router';
import {describe, expect, it, vi} from 'vitest';
import {AgentsListPage, AgentCreateDialog, AgentDetailsPage} from '../../index';

vi.mock('@thunderid/react', () => ({
  AgentList: ({
    actions,
    onSelect,
    title,
  }: {
    actions: ReactNode;
    onSelect: (item: {id: string}) => void;
    title: string;
  }): JSX.Element => (
    <div>
      <h1>{title}</h1>
      {actions}
      <button type="button" onClick={() => onSelect({id: 'x-1'})}>
        pick
      </button>
    </div>
  ),
  AgentCreate: ({
    onCancel,
    onCreated,
  }: {
    onCancel: () => void;
    onCreated: (item: {id: string}) => void;
  }): JSX.Element => (
    <div>
      <button type="button" onClick={() => onCreated({id: 'x-9'})}>
        create
      </button>
      <button type="button" onClick={onCancel}>
        cancel
      </button>
    </div>
  ),
  AgentDetails: ({
    agentId,
    deletable,
    lifecycleActions,
    onDeleted,
  }: {
    agentId: string;
    deletable: boolean;
    lifecycleActions: boolean;
    onDeleted: () => void;
  }): JSX.Element => (
    <>
      <button type="button" onClick={onDeleted}>
        agent {agentId} {deletable ? 'deletable' : 'kept'}
      </button>
      <p>lifecycle {lifecycleActions ? 'on' : 'off'}</p>
    </>
  ),
}));

vi.mock('react-i18next', () => ({
  useTranslation: () => ({t: (key: string, fallback?: string) => (typeof fallback === 'string' ? fallback : key)}),
}));

function Location(): JSX.Element {
  return <p data-testid="location">{useLocation().pathname}</p>;
}

const renderAt = (path: string, element: JSX.Element): void => {
  render(
    <MemoryRouter initialEntries={[path]}>
      <Routes>
        <Route path="/agents" element={element} />
        <Route path="/agents/new" element={element} />
        <Route path="/agents/:id" element={element} />
      </Routes>
      <Location />
    </MemoryRouter>,
  );
};

describe('AgentsListPage', () => {
  it('opens the agent an admin picks', () => {
    renderAt('/agents', <AgentsListPage />);
    fireEvent.click(screen.getByRole('button', {name: 'pick'}));
    expect(screen.getByTestId('location').textContent).toBe('/agents/x-1');
  });

  it('links to the create dialog only when admins may add', () => {
    renderAt('/agents', <AgentsListPage canAdd />);
    expect(screen.getByRole('link', {name: 'Add'}).getAttribute('href')).toBe('/agents/new');
  });

  it('shows no add button by default', () => {
    renderAt('/agents', <AgentsListPage />);
    expect(screen.queryByRole('link')).toBeNull();
  });
});

describe('AgentCreateDialog', () => {
  it('opens the new agent once it is created', () => {
    renderAt('/agents/new', <AgentCreateDialog />);
    fireEvent.click(screen.getByRole('button', {name: 'create'}));
    expect(screen.getByTestId('location').textContent).toBe('/agents/x-9');
  });

  it('returns to the list when cancelled', () => {
    renderAt('/agents/new', <AgentCreateDialog />);
    fireEvent.click(screen.getByRole('button', {name: 'cancel'}));
    expect(screen.getByTestId('location').textContent).toBe('/agents');
  });
});

describe('AgentDetailsPage', () => {
  it('shows the agent from the route and returns to the list once it is deleted', () => {
    renderAt('/agents/a-1', <AgentDetailsPage deletable />);
    expect(screen.getByRole('link', {name: 'Back'}).getAttribute('href')).toBe('/agents');
    fireEvent.click(screen.getByRole('button', {name: 'agent a-1 deletable'}));
    expect(screen.getByTestId('location').textContent).toBe('/agents');
  });

  it('offers no lifecycle actions by default', () => {
    renderAt('/agents/a-1', <AgentDetailsPage />);
    expect(screen.getByText('lifecycle off')).toBeTruthy();
  });

  it('offers lifecycle actions when told to', () => {
    renderAt('/agents/a-1', <AgentDetailsPage lifecycleActions />);
    expect(screen.getByText('lifecycle on')).toBeTruthy();
  });
});
