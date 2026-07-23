/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {render, screen, waitFor} from '@testing-library/react';
import {Position} from '@xyflow/react';
import {useContext, useEffect} from 'react';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import EdgeGeometryContext from '../../../context/EdgeGeometryContext';
import EdgePathsContext from '../../../context/EdgePathsContext';
import type {EdgeInput} from '../../../utils/calculateEdgePath';
import EdgePathsProvider from '../EdgePathsProvider';

const {mockState} = vi.hoisted(() => ({
  mockState: {nodes: [] as unknown[]},
}));

vi.mock('@xyflow/react', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@xyflow/react')>();
  return {
    ...actual,
    useStore: (selector: (state: unknown) => unknown) => selector(mockState),
    useStoreApi: () => ({getState: () => mockState}),
  };
});

vi.mock('../../../hooks/useFlowConfig', () => ({
  default: () => ({edgeStyle: 'step'}),
}));

function edgeInput(id: string, sourceX: number, sourceY: number, targetX: number, targetY: number): EdgeInput {
  return {
    id,
    sourcePosition: Position.Right,
    sourceX,
    sourceY,
    targetPosition: Position.Left,
    targetX,
    targetY,
  };
}

function Probe({inputs}: {inputs: EdgeInput[]}) {
  const registry = useContext(EdgeGeometryContext);
  const paths = useContext(EdgePathsContext);

  useEffect(() => {
    inputs.forEach((input) => registry?.register(input));
  }, [registry, inputs]);

  return (
    <div data-testid="paths">
      {paths ? [...paths.entries()].map(([id, result]) => `${id}=${result.path}`).join(';') : 'none'}
    </div>
  );
}

describe('EdgePathsProvider', () => {
  beforeEach(() => {
    mockState.nodes = [];
  });

  it('should publish combined paths once edges register their geometry', async () => {
    render(
      <EdgePathsProvider>
        <Probe inputs={[edgeInput('e1', 0, 0, 300, 200), edgeInput('e2', 0, 400, 300, 250)]} />
      </EdgePathsProvider>,
    );

    await waitFor(() => {
      const text = screen.getByTestId('paths').textContent ?? '';
      expect(text).toContain('e1=M 0,0 ');
      expect(text).toContain('e2=M 0,400 ');
    });
  });

  it('should keep each combined path anchored to its endpoints', async () => {
    render(
      <EdgePathsProvider>
        <Probe inputs={[edgeInput('e1', 0, 0, 300, 200), edgeInput('e2', 0, 0, 300, 200)]} />
      </EdgePathsProvider>,
    );

    await waitFor(() => {
      const text = screen.getByTestId('paths').textContent ?? '';
      const paths = text.split(';');
      expect(paths).toHaveLength(2);
      paths.forEach((entry) => {
        expect(entry).toMatch(/=M 0,0 /);
        expect(entry.endsWith('L 300,200')).toBe(true);
      });
    });
  });

  it('should publish no paths while a node is being dragged', async () => {
    mockState.nodes = [{dragging: true, id: 'a', position: {x: 0, y: 0}}];

    render(
      <EdgePathsProvider>
        <Probe inputs={[edgeInput('e1', 0, 0, 300, 200), edgeInput('e2', 0, 400, 300, 250)]} />
      </EdgePathsProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('paths').textContent).toBe('none');
    });
  });

  it('should publish no paths while fewer than two edges are registered', async () => {
    render(
      <EdgePathsProvider>
        <Probe inputs={[edgeInput('e1', 0, 0, 300, 200)]} />
      </EdgePathsProvider>,
    );

    await waitFor(() => {
      expect(screen.getByTestId('paths').textContent).toBe('none');
    });
  });
});
