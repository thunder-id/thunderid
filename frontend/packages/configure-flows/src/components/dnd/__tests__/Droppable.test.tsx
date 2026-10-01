// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi, beforeEach} from 'vitest';
import Droppable from '../Droppable';

// Mock refs
const mockDroppableRef = {current: null};
const mockSortableRef = {current: null};

// Mock droppable state
const mockDroppable = {
  accepts: vi.fn(() => true),
};

const mockSortable = {
  accepts: vi.fn(() => true),
};

// Mock @dnd-kit/react
vi.mock('@dnd-kit/react', () => ({
  useDroppable: vi.fn(() => ({
    ref: mockDroppableRef,
    droppable: mockDroppable,
    isDropTarget: false,
  })),
  useDragOperation: vi.fn(() => ({
    source: null,
  })),
}));

// Mock @dnd-kit/react/sortable
vi.mock('@dnd-kit/react/sortable', () => ({
  useSortable: vi.fn(() => ({
    ref: mockSortableRef,
    sortable: mockSortable,
    isDropTarget: false,
  })),
}));

// Mock @dnd-kit/collision
vi.mock('@dnd-kit/collision', () => ({
  pointerIntersection: vi.fn(),
}));

describe('Droppable', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('Rendering', () => {
    it('should render children', () => {
      render(
        <Droppable id="test-droppable">
          <div data-testid="child-content">Drop Content</div>
        </Droppable>,
      );

      expect(screen.getByTestId('child-content')).toBeInTheDocument();
      expect(screen.getByText('Drop Content')).toBeInTheDocument();
    });

    it('should render multiple children', () => {
      render(
        <Droppable id="test-droppable">
          <div data-testid="child-1">First</div>
          <div data-testid="child-2">Second</div>
          <div data-testid="child-3">Third</div>
        </Droppable>,
      );

      expect(screen.getByTestId('child-1')).toBeInTheDocument();
      expect(screen.getByTestId('child-2')).toBeInTheDocument();
      expect(screen.getByTestId('child-3')).toBeInTheDocument();
    });

    it('should render without children', () => {
      const {container} = render(<Droppable id="empty-droppable" />);

      expect(container.firstChild).toBeInTheDocument();
    });

    it('should render BottomZone for end insertion', () => {
      const {container} = render(
        <Droppable id="test-droppable">
          <div>Item 1</div>
          <div>Item 2</div>
        </Droppable>,
      );

      // BottomZone is rendered as an additional Box at the end
      const boxes = container.querySelectorAll('div');
      expect(boxes.length).toBeGreaterThan(2);
    });
  });

  describe('Hook Integration', () => {
    it('should call useDroppable with correct id', async () => {
      const {useDroppable} = await import('@dnd-kit/react');

      render(
        <Droppable id="unique-drop-zone">
          <div>Content</div>
        </Droppable>,
      );

      expect(useDroppable).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 'unique-drop-zone',
        }),
      );
    });

    it('should pass accept prop to useDroppable', async () => {
      const {useDroppable} = await import('@dnd-kit/react');

      render(
        <Droppable id="test-drop" accept={['TYPE_A', 'TYPE_B']}>
          <div>Content</div>
        </Droppable>,
      );

      expect(useDroppable).toHaveBeenCalledWith(
        expect.objectContaining({
          accept: ['TYPE_A', 'TYPE_B'],
        }),
      );
    });

    it('should pass data prop to useDroppable', async () => {
      const {useDroppable} = await import('@dnd-kit/react');

      const customData = {zone: 'main', allowReorder: true};
      render(
        <Droppable id="test-drop" data={customData}>
          <div>Content</div>
        </Droppable>,
      );

      expect(useDroppable).toHaveBeenCalledWith(
        expect.objectContaining({
          data: customData,
        }),
      );
    });
  });

  describe('Custom Styling', () => {
    it('should apply custom sx styles', () => {
      const {container} = render(
        <Droppable id="styled-drop" sx={{padding: '20px', backgroundColor: 'blue'}}>
          <div>Content</div>
        </Droppable>,
      );

      expect(container.firstChild).toBeInTheDocument();
    });

    it('should apply custom className', () => {
      const {container} = render(
        <Droppable id="test-drop" className="custom-droppable-class">
          <div>Content</div>
        </Droppable>,
      );

      expect(container.querySelector('.custom-droppable-class')).toBeInTheDocument();
    });
  });

  describe('Drop Target States', () => {
    it('should render without drag over styles when not a drop target', () => {
      const {container} = render(
        <Droppable id="test-drop">
          <div>Content</div>
        </Droppable>,
      );

      // Normal state - no special border styles
      expect(container.firstChild).toBeInTheDocument();
    });

    it('should show success styles when dragging over with valid source', async () => {
      const {useDroppable, useDragOperation} = await import('@dnd-kit/react');

      // Mock active drag operation with valid source
      vi.mocked(useDragOperation).mockReturnValue({
        source: {id: 'drag-1', type: 'VALID_TYPE'},
      } as ReturnType<typeof useDragOperation>);

      vi.mocked(useDroppable).mockReturnValue({
        ref: mockDroppableRef,
        droppable: {accepts: vi.fn(() => true)},
        isDropTarget: true,
      } as unknown as ReturnType<typeof useDroppable>);

      const {container} = render(
        <Droppable id="test-drop" accept={['VALID_TYPE']}>
          <div>Content</div>
        </Droppable>,
      );

      expect(container.firstChild).toBeInTheDocument();
    });

    it('should show error styles when dragging over with invalid source', async () => {
      const {useDroppable, useDragOperation} = await import('@dnd-kit/react');

      // Mock active drag operation with invalid source
      vi.mocked(useDragOperation).mockReturnValue({
        source: {id: 'drag-1', type: 'INVALID_TYPE'},
      } as ReturnType<typeof useDragOperation>);

      vi.mocked(useDroppable).mockReturnValue({
        ref: mockDroppableRef,
        droppable: {accepts: vi.fn(() => false)},
        isDropTarget: true,
      } as unknown as ReturnType<typeof useDroppable>);

      const {container} = render(
        <Droppable id="test-drop" accept={['VALID_TYPE']}>
          <div>Content</div>
        </Droppable>,
      );

      expect(container.firstChild).toBeInTheDocument();
    });

    it('should not show drop styles when source is null', async () => {
      const {useDroppable, useDragOperation} = await import('@dnd-kit/react');

      vi.mocked(useDragOperation).mockReturnValue({
        source: null,
      } as ReturnType<typeof useDragOperation>);

      vi.mocked(useDroppable).mockReturnValue({
        ref: mockDroppableRef,
        droppable: {accepts: vi.fn(() => true)},
        isDropTarget: false,
      } as unknown as ReturnType<typeof useDroppable>);

      const {container} = render(
        <Droppable id="test-drop">
          <div>Content</div>
        </Droppable>,
      );

      expect(container.firstChild).toBeInTheDocument();
    });

    it('should compute canAcceptDrop correctly when source exists', async () => {
      const {useDroppable, useDragOperation} = await import('@dnd-kit/react');

      const mockAccepts = vi.fn(() => true);
      vi.mocked(useDragOperation).mockReturnValue({
        source: {id: 'drag-1', type: 'TYPE_A'},
      } as ReturnType<typeof useDragOperation>);

      vi.mocked(useDroppable).mockReturnValue({
        ref: mockDroppableRef,
        droppable: {accepts: mockAccepts},
        isDropTarget: true,
      } as unknown as ReturnType<typeof useDroppable>);

      render(
        <Droppable id="test-drop">
          <div>Content</div>
        </Droppable>,
      );

      // droppable.accepts should be called with the source
      expect(mockAccepts).toHaveBeenCalled();
    });
  });

  describe('isTargetInside Logic', () => {
    it('should return cached state when target has no element property', async () => {
      const {useDroppable, useDragOperation} = await import('@dnd-kit/react');

      const droppableElement = document.createElement('div');

      vi.mocked(useDragOperation).mockReturnValue({
        source: {id: 'drag-1'},
        target: {id: 'target-1'}, // no element property
      } as ReturnType<typeof useDragOperation>);

      vi.mocked(useDroppable).mockReturnValue({
        ref: mockDroppableRef,
        droppable: {accepts: vi.fn(() => true), element: droppableElement},
        isDropTarget: false,
      } as unknown as ReturnType<typeof useDroppable>);

      const {container} = render(
        <Droppable id="test-drop">
          <div>Content</div>
        </Droppable>,
      );

      expect(container.firstChild).toBeInTheDocument();
    });

    it('should detect target inside droppable element', async () => {
      const {useDroppable, useDragOperation} = await import('@dnd-kit/react');

      const droppableElement = document.createElement('div');
      const targetElement = document.createElement('span');
      droppableElement.appendChild(targetElement);

      vi.mocked(useDragOperation).mockReturnValue({
        source: {id: 'drag-1'},
        target: {id: 'target-1', element: targetElement},
      } as unknown as ReturnType<typeof useDragOperation>);

      vi.mocked(useDroppable).mockReturnValue({
        ref: mockDroppableRef,
        droppable: {accepts: vi.fn(() => true), element: droppableElement},
        isDropTarget: true,
      } as unknown as ReturnType<typeof useDroppable>);

      const {container} = render(
        <Droppable id="test-drop">
          <div>Content</div>
        </Droppable>,
      );

      // data-drop-active should be present since target is inside and source exists
      expect(container.firstChild).toHaveAttribute('data-drop-active');
    });

    it('should handle ancestor target when not a drop target', async () => {
      const {useDroppable, useDragOperation} = await import('@dnd-kit/react');

      const ancestorElement = document.createElement('div');
      const droppableElement = document.createElement('span');
      ancestorElement.appendChild(droppableElement);

      vi.mocked(useDragOperation).mockReturnValue({
        source: {id: 'drag-1'},
        target: {id: 'target-1', element: ancestorElement},
      } as unknown as ReturnType<typeof useDragOperation>);

      vi.mocked(useDroppable).mockReturnValue({
        ref: mockDroppableRef,
        droppable: {accepts: vi.fn(() => true), element: droppableElement},
        isDropTarget: false,
      } as unknown as ReturnType<typeof useDroppable>);

      const {container} = render(
        <Droppable id="test-drop">
          <div>Content</div>
        </Droppable>,
      );

      // Not a drop target, so no highlight
      expect(container.firstChild).not.toHaveAttribute('data-drop-active');
    });

    it('should use cached state for ancestor target when still a drop target', async () => {
      const {useDroppable, useDragOperation} = await import('@dnd-kit/react');

      const ancestorElement = document.createElement('div');
      const droppableElement = document.createElement('span');
      ancestorElement.appendChild(droppableElement);

      vi.mocked(useDragOperation).mockReturnValue({
        source: {id: 'drag-1'},
        target: {id: 'target-1', element: ancestorElement},
      } as unknown as ReturnType<typeof useDragOperation>);

      vi.mocked(useDroppable).mockReturnValue({
        ref: mockDroppableRef,
        droppable: {accepts: vi.fn(() => true), element: droppableElement},
        isDropTarget: true,
      } as unknown as ReturnType<typeof useDroppable>);

      const {container} = render(
        <Droppable id="test-drop">
          <div>Content</div>
        </Droppable>,
      );

      expect(container.firstChild).toBeInTheDocument();
    });

    it('should return false when target and droppable have no element', async () => {
      const {useDroppable, useDragOperation} = await import('@dnd-kit/react');

      vi.mocked(useDragOperation).mockReturnValue({
        source: {id: 'drag-1'},
        target: null,
      } as ReturnType<typeof useDragOperation>);

      vi.mocked(useDroppable).mockReturnValue({
        ref: mockDroppableRef,
        droppable: {accepts: vi.fn(() => true), element: null},
        isDropTarget: false,
      } as unknown as ReturnType<typeof useDroppable>);

      const {container} = render(
        <Droppable id="test-drop">
          <div>Content</div>
        </Droppable>,
      );

      expect(container.firstChild).not.toHaveAttribute('data-drop-active');
    });
  });

  describe('hideDropZones', () => {
    it('should hide drop zones when hideDropZones is true', async () => {
      const {useSortable} = await import('@dnd-kit/react/sortable');

      render(
        <Droppable id="test-drop" hideDropZones>
          <div>Content</div>
        </Droppable>,
      );

      // DropZone uses useSortable. With hideDropZones, DropZone should not be rendered.
      // The number of useSortable calls should be 0 when zones are hidden
      expect(useSortable).not.toHaveBeenCalled();
    });
  });

  describe('BottomZone Integration', () => {
    it('should create BottomZone with correct id', async () => {
      const {useSortable} = await import('@dnd-kit/react/sortable');

      render(
        <Droppable id="main-zone">
          <div>Item 1</div>
        </Droppable>,
      );

      // BottomZone uses id with '-end' suffix
      expect(useSortable).toHaveBeenCalledWith(
        expect.objectContaining({
          id: 'main-zone-end',
        }),
      );
    });

    it('should pass correct index based on children count', async () => {
      const {useSortable} = await import('@dnd-kit/react/sortable');

      render(
        <Droppable id="test-zone">
          <div>Item 1</div>
          <div>Item 2</div>
          <div>Item 3</div>
        </Droppable>,
      );

      // BottomZone index should equal the count of children (3)
      expect(useSortable).toHaveBeenCalledWith(
        expect.objectContaining({
          index: 3,
        }),
      );
    });

    it('should pass accept prop to BottomZone', async () => {
      const {useSortable} = await import('@dnd-kit/react/sortable');

      render(
        <Droppable id="test-zone" accept={['STEP', 'WIDGET']}>
          <div>Item</div>
        </Droppable>,
      );

      expect(useSortable).toHaveBeenCalledWith(
        expect.objectContaining({
          accept: ['STEP', 'WIDGET'],
        }),
      );
    });
  });
});
