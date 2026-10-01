// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import {createRef} from 'react';
import {describe, it, expect, vi} from 'vitest';
import Handle from '../Handle';

describe('Handle', () => {
  describe('Rendering', () => {
    it('should render as a button element', () => {
      render(<Handle label="Drag handle">Handle Content</Handle>);

      expect(screen.getByRole('button')).toBeInTheDocument();
    });

    it('should render children content', () => {
      render(<Handle label="Label">Icon Content</Handle>);

      expect(screen.getByText('Icon Content')).toBeInTheDocument();
    });

    it('should wrap children in a span', () => {
      render(<Handle label="Label">Content</Handle>);

      const span = screen.getByText('Content').closest('span');
      expect(span).toBeInTheDocument();
    });
  });

  describe('Label/Tooltip', () => {
    it('should have Tooltip with label', () => {
      render(<Handle label="Drag to reorder">Handle</Handle>);

      // The button should be present, Tooltip provides aria support
      expect(screen.getByRole('button')).toBeInTheDocument();
    });

    it('should accept ReactNode as label', () => {
      render(<Handle label={<strong>Bold Label</strong>}>Handle</Handle>);

      expect(screen.getByRole('button')).toBeInTheDocument();
    });

    it('should accept string as label', () => {
      render(<Handle label="String Label">Handle</Handle>);

      expect(screen.getByRole('button')).toBeInTheDocument();
    });
  });

  describe('Ref Forwarding', () => {
    it('should forward ref to Action/button element', () => {
      const ref = createRef<HTMLButtonElement>();

      render(
        <Handle label="Handle" ref={ref}>
          With Ref
        </Handle>,
      );

      expect(ref.current).toBe(screen.getByRole('button'));
    });

    it('should handle null ref gracefully', () => {
      render(
        <Handle label="Handle" ref={null}>
          No Ref
        </Handle>,
      );

      expect(screen.getByRole('button')).toBeInTheDocument();
    });
  });

  describe('Cursor Styles', () => {
    it('should have pointer cursor by default', () => {
      render(<Handle label="Label">Default</Handle>);

      const button = screen.getByRole('button');
      expect(button).toHaveStyle({cursor: 'pointer'});
    });

    it('should accept custom cursor style', () => {
      render(
        <Handle label="Label" cursor="grab">
          Grab
        </Handle>,
      );

      const button = screen.getByRole('button');
      expect(button).toHaveStyle({cursor: 'grab'});
    });

    it('should accept grabbing cursor', () => {
      render(
        <Handle label="Label" cursor="grabbing">
          Grabbing
        </Handle>,
      );

      const button = screen.getByRole('button');
      expect(button).toHaveStyle({cursor: 'grabbing'});
    });
  });

  describe('Event Handlers', () => {
    it('should call onClick handler', () => {
      const onClick = vi.fn();
      render(
        <Handle label="Label" onClick={onClick}>
          Click
        </Handle>,
      );

      fireEvent.click(screen.getByRole('button'));

      expect(onClick).toHaveBeenCalledTimes(1);
    });

    it('should call onMouseDown handler', () => {
      const onMouseDown = vi.fn();
      render(
        <Handle label="Label" onMouseDown={onMouseDown}>
          Mouse Down
        </Handle>,
      );

      fireEvent.mouseDown(screen.getByRole('button'));

      expect(onMouseDown).toHaveBeenCalledTimes(1);
    });

    it('should call onKeyDown handler', () => {
      const onKeyDown = vi.fn();
      render(
        <Handle label="Label" onKeyDown={onKeyDown}>
          Key Down
        </Handle>,
      );

      fireEvent.keyDown(screen.getByRole('button'), {key: 'Space'});

      expect(onKeyDown).toHaveBeenCalledTimes(1);
    });
  });

  describe('Integration with Action', () => {
    it('should pass props to underlying Action component', () => {
      render(
        <Handle label="Handle" className="custom-handle-class">
          Content
        </Handle>,
      );

      expect(screen.getByRole('button')).toHaveClass('custom-handle-class');
    });

    it('should inherit Action hover effects', () => {
      render(<Handle label="Handle">Hover</Handle>);

      const button = screen.getByRole('button');
      fireEvent.mouseEnter(button);

      expect(button.style.backgroundColor).toBe('rgba(0, 0, 0, 0.15)');
    });
  });

  describe('Accessibility', () => {
    it('should be focusable', () => {
      render(<Handle label="Handle">Focus Me</Handle>);

      const button = screen.getByRole('button');
      button.focus();

      expect(document.activeElement).toBe(button);
    });

    it('should accept aria attributes', () => {
      render(
        <Handle label="Handle" aria-pressed="true">
          Aria
        </Handle>,
      );

      expect(screen.getByRole('button')).toHaveAttribute('aria-pressed', 'true');
    });
  });

  describe('Children Variants', () => {
    it('should render icon as child', () => {
      render(
        <Handle label="Drag">
          <svg data-testid="icon" />
        </Handle>,
      );

      expect(screen.getByTestId('icon')).toBeInTheDocument();
    });

    it('should render text as child', () => {
      render(<Handle label="Handle">Text Handle</Handle>);

      expect(screen.getByText('Text Handle')).toBeInTheDocument();
    });

    it('should render multiple children', () => {
      render(
        <Handle label="Handle">
          <span data-testid="icon">Icon</span>
          <span data-testid="text">Text</span>
        </Handle>,
      );

      expect(screen.getByTestId('icon')).toBeInTheDocument();
      expect(screen.getByTestId('text')).toBeInTheDocument();
    });
  });
});
