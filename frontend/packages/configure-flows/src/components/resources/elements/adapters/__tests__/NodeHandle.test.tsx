// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-call, @typescript-eslint/no-unsafe-member-access */

import {render, act} from '@testing-library/react';
import {ReactFlowProvider, Position} from '@xyflow/react';
import type {ReactNode} from 'react';
import {describe, it, expect, vi, beforeEach, afterEach} from 'vitest';
import NodeHandle from '../NodeHandle';

// Use a variable to store the RAF spy - will be set up in beforeEach
// eslint-disable-next-line @typescript-eslint/no-explicit-any
let mockRequestAnimationFrame: any;

// Mock @xyflow/react
const mockUpdateNodeInternals = vi.fn();
const mockUseNodeId = vi.fn(() => 'node-1');

vi.mock('@xyflow/react', async () => {
  const actual = await vi.importActual('@xyflow/react');
  return {
    ...actual,
    useNodeId: () => mockUseNodeId(),
    useUpdateNodeInternals: () => mockUpdateNodeInternals,
    Handle: ({id, type, position}: {id: string; type: string; position: string}) => (
      <div data-testid="handle" data-id={id} data-type={type} data-position={position} />
    ),
  };
});

describe('NodeHandle', () => {
  const createWrapper = () => {
    function Wrapper({children}: {children: ReactNode}) {
      return <ReactFlowProvider>{children}</ReactFlowProvider>;
    }
    return Wrapper;
  };

  beforeEach(() => {
    vi.clearAllMocks();
    // Spy on window.requestAnimationFrame and make it execute callback synchronously
    mockRequestAnimationFrame = vi.spyOn(window, 'requestAnimationFrame').mockImplementation((callback) => {
      callback(0);
      return 0;
    });
    mockUseNodeId.mockReturnValue('node-1');
  });

  afterEach(() => {
    mockRequestAnimationFrame.mockRestore();
  });

  describe('Rendering', () => {
    it('should render Handle component', () => {
      const {getByTestId} = render(<NodeHandle id="handle-1" type="source" position={Position.Right} />, {
        wrapper: createWrapper(),
      });

      expect(getByTestId('handle')).toBeInTheDocument();
    });

    it('should pass id to Handle', () => {
      const {getByTestId} = render(<NodeHandle id="my-handle" type="source" position={Position.Right} />, {
        wrapper: createWrapper(),
      });

      expect(getByTestId('handle')).toHaveAttribute('data-id', 'my-handle');
    });

    it('should pass type to Handle', () => {
      const {getByTestId} = render(<NodeHandle id="handle-1" type="target" position={Position.Left} />, {
        wrapper: createWrapper(),
      });

      expect(getByTestId('handle')).toHaveAttribute('data-type', 'target');
    });

    it('should pass position to Handle', () => {
      const {getByTestId} = render(<NodeHandle id="handle-1" type="source" position={Position.Bottom} />, {
        wrapper: createWrapper(),
      });

      expect(getByTestId('handle')).toHaveAttribute('data-position', Position.Bottom);
    });
  });

  describe('Position Key Updates', () => {
    it('should not update node internals on initial mount', () => {
      render(<NodeHandle id="handle-1" type="source" position={Position.Right} positionKey={0} />, {
        wrapper: createWrapper(),
      });

      expect(mockUpdateNodeInternals).not.toHaveBeenCalled();
    });

    it('should update node internals when positionKey changes', () => {
      // Use a wrapper that's consistent across rerenders
      const Wrapper = createWrapper();
      const {rerender} = render(
        <Wrapper>
          <NodeHandle id="handle-1" type="source" position={Position.Right} positionKey={0} />
        </Wrapper>,
      );

      // Clear mocks after initial render to isolate the rerender behavior
      mockUpdateNodeInternals.mockClear();
      mockRequestAnimationFrame.mockClear();

      act(() => {
        rerender(
          <Wrapper>
            <NodeHandle id="handle-1" type="source" position={Position.Right} positionKey={1} />
          </Wrapper>,
        );
      });

      // The RAF mock executes callback synchronously, so check immediately
      expect(mockRequestAnimationFrame).toHaveBeenCalled();
      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('node-1');
    });

    it('should not update when positionKey stays the same', () => {
      const {rerender} = render(<NodeHandle id="handle-1" type="source" position={Position.Right} positionKey={5} />, {
        wrapper: createWrapper(),
      });

      rerender(
        <ReactFlowProvider>
          <NodeHandle id="handle-1" type="source" position={Position.Right} positionKey={5} />
        </ReactFlowProvider>,
      );

      expect(mockUpdateNodeInternals).not.toHaveBeenCalled();
    });

    it('should handle string positionKey', () => {
      const Wrapper = createWrapper();
      const {rerender} = render(
        <Wrapper>
          <NodeHandle id="handle-1" type="source" position={Position.Right} positionKey="key-1" />
        </Wrapper>,
      );

      // Clear mocks after initial render to isolate the rerender behavior
      mockUpdateNodeInternals.mockClear();

      act(() => {
        rerender(
          <Wrapper>
            <NodeHandle id="handle-1" type="source" position={Position.Right} positionKey="key-2" />
          </Wrapper>,
        );
      });

      expect(mockUpdateNodeInternals).toHaveBeenCalledWith('node-1');
    });
  });

  describe('No Node ID', () => {
    it('should not update when nodeId is null', () => {
      mockUseNodeId.mockReturnValue(null as unknown as string);

      const {rerender} = render(<NodeHandle id="handle-1" type="source" position={Position.Right} positionKey={0} />, {
        wrapper: createWrapper(),
      });

      rerender(
        <ReactFlowProvider>
          <NodeHandle id="handle-1" type="source" position={Position.Right} positionKey={1} />
        </ReactFlowProvider>,
      );

      expect(mockUpdateNodeInternals).not.toHaveBeenCalled();
    });
  });

  describe('Default Props', () => {
    it('should work without positionKey', () => {
      const {getByTestId} = render(<NodeHandle id="handle-1" type="source" position={Position.Right} />, {
        wrapper: createWrapper(),
      });

      expect(getByTestId('handle')).toBeInTheDocument();
    });
  });

  describe('Handle Props Spreading', () => {
    it('should spread additional handle props', () => {
      const {getByTestId} = render(<NodeHandle id="handle-1" type="source" position={Position.Right} />, {
        wrapper: createWrapper(),
      });

      const handle = getByTestId('handle');
      expect(handle).toHaveAttribute('data-id', 'handle-1');
      expect(handle).toHaveAttribute('data-type', 'source');
    });
  });
});
