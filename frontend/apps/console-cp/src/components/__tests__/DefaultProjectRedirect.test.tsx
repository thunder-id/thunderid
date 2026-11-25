// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {MemoryRouter, Route, Routes} from 'react-router';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import DefaultProjectRedirect from '../DefaultProjectRedirect';

const selection = vi.hoisted(() => ({
  projects: [] as {id: string; handle: string; displayName: string}[],
  isLoading: false,
}));

vi.mock('@thunderid/configure-projects', () => ({
  useProjectSelection: () => selection,
}));

vi.mock('@thunderid/components', () => ({
  PageLoadingAnimation: () => <div data-testid="loading" />,
}));

function renderAtRoot(): void {
  render(
    <MemoryRouter initialEntries={['/']}>
      <Routes>
        <Route path="/" element={<DefaultProjectRedirect />} />
        <Route path="/home" element={<div>organization overview</div>} />
        <Route path="/project/:projectHandle" element={<div>project home</div>} />
      </Routes>
    </MemoryRouter>,
  );
}

describe('DefaultProjectRedirect', () => {
  beforeEach(() => {
    selection.projects = [
      {id: 'p-shop', handle: 'shop', displayName: 'Shop'},
      {id: 'p-default', handle: 'default', displayName: 'Default'},
    ];
    selection.isLoading = false;
  });

  it('waits while the projects are read', () => {
    selection.isLoading = true;
    renderAtRoot();
    expect(screen.getByTestId('loading')).toBeInTheDocument();
  });

  it("opens the default project's home", () => {
    renderAtRoot();
    expect(screen.getByText('project home')).toBeInTheDocument();
  });

  it('opens the organization overview when there is no default project', () => {
    selection.projects = [{id: 'p-shop', handle: 'shop', displayName: 'Shop'}];
    renderAtRoot();
    expect(screen.getByText('organization overview')).toBeInTheDocument();
  });
});
