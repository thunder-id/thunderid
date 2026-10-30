// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, type ReactNode} from 'react';
import {createRoot, type Root} from 'react-dom/client';
import {describe, it, expect, beforeEach, afterEach} from 'vitest';
import ProjectScopeProvider from '../ProjectScopeProvider';
import useProjectScope from '../useProjectScope';

function ProjectId() {
  const {projectId, rootOuId} = useProjectScope();
  return (
    <>
      <span data-testid="project-id">{projectId ?? 'none'}</span>
      <span data-testid="root-ou-id">{rootOuId ?? 'none'}</span>
    </>
  );
}

let container: HTMLDivElement;
let root: Root;

function render(children: ReactNode) {
  act(() => {
    root.render(children);
  });
}

function projectId(): string {
  return container.querySelector('[data-testid="project-id"]')?.textContent ?? '';
}

function rootOuId(): string {
  return container.querySelector('[data-testid="root-ou-id"]')?.textContent ?? '';
}

beforeEach(() => {
  container = document.createElement('div');
  document.body.appendChild(container);
  root = createRoot(container);
});

afterEach(() => {
  act(() => root.unmount());
  container.remove();
});

describe('ProjectScopeProvider', () => {
  it('has no project without a provider', () => {
    render(<ProjectId />);

    expect(projectId()).toBe('none');
  });

  it('provides the selected project', () => {
    render(
      <ProjectScopeProvider projectId="p1" rootOuId="ou1">
        <ProjectId />
      </ProjectScopeProvider>,
    );

    expect(projectId()).toBe('p1');
    expect(rootOuId()).toBe('ou1');
  });

  it('treats an empty project ID as no project', () => {
    render(
      <ProjectScopeProvider projectId="" rootOuId="ou1">
        <ProjectId />
      </ProjectScopeProvider>,
    );

    expect(projectId()).toBe('none');
    expect(rootOuId()).toBe('none');
  });
});
