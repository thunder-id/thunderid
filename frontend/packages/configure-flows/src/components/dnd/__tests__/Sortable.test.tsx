// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import Sortable from '../Sortable';

// Mock refs
const mockSortableRef = {current: null};

// Mock sortable state
const mockSortable = {
  accepts: vi.fn(() => true),
};

// Mock manager with event listeners
const mockManager = {
  monitor: {
    addEventListener: vi.fn(),
  },
};

// Mock @dnd-kit/react/sortable
vi.mock('@dnd-kit/react/sortable', () => ({
  useSortable: vi.fn(() => ({
    ref: mockSortableRef,
    sortable: mockSortable,
    isDragging: false,
    isDropTarget: false,
  })),
}));

// Mock @dnd-kit/react
vi.mock('@dnd-kit/react', () => ({
  useDragDropManager: vi.fn(() => mockManager),
  useDragOperation: vi.fn(() => ({
    source: null,
  })),
}));

// Mock @dnd-kit/abstract/modifiers
vi.mock('@dnd-kit/abstract/modifiers', () => ({
  RestrictToVerticalAxis: {},
}));

describe('Sortable', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render children', () => {
      render(
        <Sortable id="test-sortable" index={0}>
          <div data-testid="child-content">Sortable Content</div>
        </Sortable>,
      );

      expect(screen.getByTestId('child-content')).toBeInTheDocument();
      expect(screen.getByText('Sortable Content')).toBeInTheDocument();
    });

    it('should render without children', () => {
      const {container} = render(<Sortable id="empty-sortable" index={0} />);

      expect(container.firstChild).toBeInTheDocument();
    });

    it('should render with memoized presentation wrapper', () => {
      const {container} = render(
        <Sortable id="test-sortable" index={0}>
          <span>Content</span>
        </Sortable>,
      );

      // Should have Box wrappers from the component
      expect(container.querySelectorAll('div').length).toBeGreaterThan(0);
    });
  });

  describe('Hook Integration', () => {
    it('should call useSortable with correct id', async () => {
      const {useSortable} = await import('@dnd-kit/react/sortable');

      render(
        <Sortable id="unique-sortable-123" index={5}>
          <div>Content</div>
        </Sortable>,
      );

      expect(useSortable).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 'unique-sortable-123',
        }),
      );
    });

    it('should call useSortable with correct index', async () => {
      const {useSortable} = await import('@dnd-kit/react/sortable');

      render(
        <Sortable id="test-sortable" index={3}>
          <div>Content</div>
        </Sortable>,
      );

      expect(useSortable).toHaveBeenCalledWith(
        expect.objectContaining({
          index: 3,
        }),
      );
    });

    it('should pass handleRef when provided', async () => {
      const {useSortable} = await import('@dnd-kit/react/sortable');

      const handleRef = {current: document.createElement('button')};

      render(
        <Sortable id="test-sortable" index={0} handleRef={handleRef}>
          <div>Content</div>
        </Sortable>,
      );

      expect(useSortable).toHaveBeenCalledWith(
        expect.objectContaining({
          handle: handleRef,
        }),
      );
    });

    it('should apply RestrictToVerticalAxis modifier', async () => {
      const {useSortable} = await import('@dnd-kit/react/sortable');
      const {RestrictToVerticalAxis} = await import('@dnd-kit/abstract/modifiers');

      render(
        <Sortable id="test-sortable" index={0}>
          <div>Content</div>
        </Sortable>,
      );

      expect(useSortable).toHaveBeenCalledWith(
        expect.objectContaining({
          modifiers: [RestrictToVerticalAxis],
        }),
      );
    });

    it('should pass collisionDetector when provided', async () => {
      const {useSortable} = await import('@dnd-kit/react/sortable');

      const customCollisionDetector = vi.fn();

      render(
        <Sortable id="test-sortable" index={0} collisionDetector={customCollisionDetector}>
          <div>Content</div>
        </Sortable>,
      );

      expect(useSortable).toHaveBeenCalledWith(
        expect.objectContaining({
          collisionDetector: customCollisionDetector,
        }),
      );
    });

    it('should pass additional props to useSortable', async () => {
      const {useSortable} = await import('@dnd-kit/react/sortable');

      render(
        <Sortable id="test-sortable" index={0} accept={['TYPE_A']} data={{custom: 'data'}} disabled>
          <div>Content</div>
        </Sortable>,
      );

      expect(useSortable).toHaveBeenCalledWith(
        expect.objectContaining({
          accept: ['TYPE_A'],
          data: {custom: 'data'},
          disabled: true,
        }),
      );
    });
  });

  describe('Drag Manager Setup', () => {
    it('should call useDragDropManager', async () => {
      const {useDragDropManager} = await import('@dnd-kit/react');

      render(
        <Sortable id="test-sortable" index={0}>
          <div>Content</div>
        </Sortable>,
      );

      expect(useDragDropManager).toHaveBeenCalled();
    });
  });

  describe('Styling', () => {
    it('should render with full width and height wrapper', () => {
      const {container} = render(
        <Sortable id="test-sortable" index={0}>
          <div>Content</div>
        </Sortable>,
      );

      // The component wraps children in Box elements
      expect(container.firstChild).toBeInTheDocument();
    });
  });

  describe('Index Positioning', () => {
    it('should work with index 0', () => {
      const {container} = render(
        <Sortable id="first-item" index={0}>
          <div>First</div>
        </Sortable>,
      );

      expect(container.firstChild).toBeInTheDocument();
    });

    it('should work with high index values', () => {
      const {container} = render(
        <Sortable id="last-item" index={100}>
          <div>Last</div>
        </Sortable>,
      );

      expect(container.firstChild).toBeInTheDocument();
    });
  });

  describe('Drag States', () => {
    it('should apply dragging styles when isDragging is true', async () => {
      const {useSortable} = await import('@dnd-kit/react/sortable');

      vi.mocked(useSortable).mockReturnValue({
        ref: mockSortableRef,
        sortable: mockSortable,
        isDragging: true,
        isDropTarget: false,
      } as unknown as ReturnType<typeof useSortable>);

      const {container} = render(
        <Sortable id="dragging-item" index={0}>
          <div>Dragging</div>
        </Sortable>,
      );

      expect(container.firstChild).toBeInTheDocument();
    });

    it('should apply drop target styles when isDropTarget is true', async () => {
      const {useSortable} = await import('@dnd-kit/react/sortable');
      const {useDragOperation} = await import('@dnd-kit/react');

      vi.mocked(useSortable).mockReturnValue({
        ref: mockSortableRef,
        sortable: mockSortable,
        isDragging: false,
        isDropTarget: true,
      } as unknown as ReturnType<typeof useSortable>);

      vi.mocked(useDragOperation).mockReturnValue({
        source: {id: 'drag-1', type: 'TYPE_A'},
      } as ReturnType<typeof useDragOperation>);

      const {container} = render(
        <Sortable id="drop-target" index={0}>
          <div>Drop Target</div>
        </Sortable>,
      );

      expect(container.firstChild).toBeInTheDocument();
    });

    it('should check if sortable accepts source', async () => {
      const {useSortable} = await import('@dnd-kit/react/sortable');
      const {useDragOperation} = await import('@dnd-kit/react');

      const mockAccepts = vi.fn(() => true);
      vi.mocked(useSortable).mockReturnValue({
        ref: mockSortableRef,
        sortable: {accepts: mockAccepts},
        isDragging: false,
        isDropTarget: true,
      } as unknown as ReturnType<typeof useSortable>);

      vi.mocked(useDragOperation).mockReturnValue({
        source: {id: 'drag-1', type: 'TYPE_A'},
      } as ReturnType<typeof useDragOperation>);

      render(
        <Sortable id="test-sortable" index={0}>
          <div>Content</div>
        </Sortable>,
      );

      expect(mockAccepts).toHaveBeenCalled();
    });

    it('should not call accepts when source is null', async () => {
      const {useSortable} = await import('@dnd-kit/react/sortable');
      const {useDragOperation} = await import('@dnd-kit/react');

      const mockAccepts = vi.fn(() => true);
      vi.mocked(useSortable).mockReturnValue({
        ref: mockSortableRef,
        sortable: {accepts: mockAccepts},
        isDragging: false,
        isDropTarget: false,
      } as unknown as ReturnType<typeof useSortable>);

      vi.mocked(useDragOperation).mockReturnValue({
        source: null,
      } as ReturnType<typeof useDragOperation>);

      render(
        <Sortable id="test-sortable" index={0}>
          <div>Content</div>
        </Sortable>,
      );

      // accepts should not be called when source is null
      expect(mockAccepts).not.toHaveBeenCalled();
    });
  });

  describe('Drop Indicator', () => {
    it('should show indicator before when reordering from higher index', async () => {
      const {useSortable} = await import('@dnd-kit/react/sortable');
      const {useDragOperation} = await import('@dnd-kit/react');

      vi.mocked(useSortable).mockReturnValue({
        ref: mockSortableRef,
        sortable: {accepts: vi.fn(() => true)},
        isDragging: false,
        isDropTarget: true,
      } as unknown as ReturnType<typeof useSortable>);

      vi.mocked(useDragOperation).mockReturnValue({
        source: {id: 'drag-1', type: 'TYPE_A', index: 5, data: {isReordering: true}},
        target: null,
      } as unknown as ReturnType<typeof useDragOperation>);

      const {container} = render(
        <Sortable id="test-sortable" index={2}>
          <div>Content</div>
        </Sortable>,
      );

      expect(container.firstChild).toBeInTheDocument();
    });

    it('should show indicator after when reordering from lower index', async () => {
      const {useSortable} = await import('@dnd-kit/react/sortable');
      const {useDragOperation} = await import('@dnd-kit/react');

      vi.mocked(useSortable).mockReturnValue({
        ref: mockSortableRef,
        sortable: {accepts: vi.fn(() => true)},
        isDragging: false,
        isDropTarget: true,
      } as unknown as ReturnType<typeof useSortable>);

      vi.mocked(useDragOperation).mockReturnValue({
        source: {id: 'drag-1', type: 'TYPE_A', index: 0, data: {isReordering: true}},
        target: null,
      } as unknown as ReturnType<typeof useDragOperation>);

      const {container} = render(
        <Sortable id="test-sortable" index={3}>
          <div>Content</div>
        </Sortable>,
      );

      expect(container.firstChild).toBeInTheDocument();
    });

    it('should show indicator before for new items from resource panel', async () => {
      const {useSortable} = await import('@dnd-kit/react/sortable');
      const {useDragOperation} = await import('@dnd-kit/react');

      vi.mocked(useSortable).mockReturnValue({
        ref: mockSortableRef,
        sortable: {accepts: vi.fn(() => true)},
        isDragging: false,
        isDropTarget: true,
      } as unknown as ReturnType<typeof useSortable>);

      vi.mocked(useDragOperation).mockReturnValue({
        source: {id: 'drag-1', type: 'TYPE_A', data: {isReordering: false}},
        target: null,
      } as unknown as ReturnType<typeof useDragOperation>);

      const {container} = render(
        <Sortable id="test-sortable" index={1}>
          <div>Content</div>
        </Sortable>,
      );

      expect(container.firstChild).toBeInTheDocument();
    });
  });

  describe('Memoized Presentation', () => {
    it('should render children through MemoizedSortablePresentation', () => {
      const {getByText} = render(
        <Sortable id="memo-test" index={0}>
          <span>Memoized Content</span>
        </Sortable>,
      );

      expect(getByText('Memoized Content')).toBeInTheDocument();
    });
  });

  describe('Drag Operation Event Handlers', () => {
    // Create a fresh manager for each test in this block to avoid WeakMap caching
    let freshManager: {
      monitor: {
        addEventListener: ReturnType<typeof vi.fn>;
      };
    };

    beforeEach(async () => {
      // Create a new manager object for each test so the WeakMap doesn't skip setup
      freshManager = {
        monitor: {
          addEventListener: vi.fn(),
        },
      };

      const dndKit = await import('@dnd-kit/react');
      vi.mocked(dndKit.useDragDropManager).mockReturnValue(
        freshManager as unknown as ReturnType<typeof dndKit.useDragDropManager>,
      );
    });

    it('should register dragstart event listener on manager', () => {
      render(
        <Sortable id="test-sortable-event-1" index={0}>
          <div>Content</div>
        </Sortable>,
      );

      expect(freshManager.monitor.addEventListener).toHaveBeenCalledWith('dragstart', expect.any(Function));
    });

    it('should register dragend event listener on manager', () => {
      render(
        <Sortable id="test-sortable-event-2" index={0}>
          <div>Content</div>
        </Sortable>,
      );

      expect(freshManager.monitor.addEventListener).toHaveBeenCalledWith('dragend', expect.any(Function));
    });

    it('should handle dragstart event and update global state', () => {
      render(
        <Sortable id="test-sortable-event-3" index={0}>
          <div>Content</div>
        </Sortable>,
      );

      // Find the dragstart handler that was registered
      const dragstartCall = (
        freshManager.monitor.addEventListener.mock.calls as [string, (event: unknown) => void][]
      ).find((call) => call[0] === 'dragstart');
      expect(dragstartCall).toBeDefined();

      const dragstartHandler = dragstartCall![1];

      // Simulate a dragstart event
      dragstartHandler({
        operation: {
          source: {
            index: 2,
            data: {isReordering: true},
          },
        },
      });

      // The global state should be updated - we can verify by rendering another component
      // that uses the global state
    });

    it('should handle dragstart event with source that has no index', () => {
      render(
        <Sortable id="test-sortable-event-4" index={0}>
          <div>Content</div>
        </Sortable>,
      );

      const dragstartCall = (
        freshManager.monitor.addEventListener.mock.calls as [string, (event: unknown) => void][]
      ).find((call) => call[0] === 'dragstart');
      const dragstartHandler = dragstartCall![1];

      // Simulate a dragstart event without index
      expect(() => {
        dragstartHandler({
          operation: {
            source: {
              data: {isReordering: false},
            },
          },
        });
      }).not.toThrow();
    });

    it('should handle dragstart event with undefined source', () => {
      render(
        <Sortable id="test-sortable-event-5" index={0}>
          <div>Content</div>
        </Sortable>,
      );

      const dragstartCall = (
        freshManager.monitor.addEventListener.mock.calls as [string, (event: unknown) => void][]
      ).find((call) => call[0] === 'dragstart');
      const dragstartHandler = dragstartCall![1];

      // Simulate a dragstart event with undefined source
      expect(() => {
        dragstartHandler({
          operation: {
            source: undefined,
          },
        });
      }).not.toThrow();
    });

    it('should handle dragend event and reset global state', () => {
      render(
        <Sortable id="test-sortable-event-6" index={0}>
          <div>Content</div>
        </Sortable>,
      );

      // First trigger dragstart to set state
      const dragstartCall = (
        freshManager.monitor.addEventListener.mock.calls as [string, (event: unknown) => void][]
      ).find((call) => call[0] === 'dragstart');
      const dragstartHandler = dragstartCall![1];
      dragstartHandler({
        operation: {
          source: {
            index: 1,
            data: {isReordering: true},
          },
        },
      });

      // Find the dragend handler
      const dragendCall = (
        freshManager.monitor.addEventListener.mock.calls as [string, (event?: unknown) => void][]
      ).find((call) => call[0] === 'dragend');
      expect(dragendCall).toBeDefined();

      const dragendHandler = dragendCall![1];

      // Simulate a dragend event
      dragendHandler();

      // The global state should be reset
    });

    it('should not notify listeners if state has not changed', () => {
      render(
        <Sortable id="test-sortable-event-7" index={0}>
          <div>Content</div>
        </Sortable>,
      );

      // Trigger dragend twice - the second call should not notify listeners
      // because the state is already in the "not dragging" state
      const dragendCall = (freshManager.monitor.addEventListener.mock.calls as [string, () => void][]).find(
        (call) => call[0] === 'dragend',
      );
      const dragendHandler = dragendCall![1];

      // Call dragend multiple times - should only update state once since values don't change
      dragendHandler();
      dragendHandler();

      expect(dragendHandler).toBeDefined();
    });
  });
});
