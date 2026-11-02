// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen} from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import {describe, it, expect} from 'vitest';
import GradientBorderButton from '../GradientBorderButton';

describe('GradientBorderButton', () => {
  describe('Rendering', () => {
    it('should render button with children text', () => {
      render(<GradientBorderButton>Click Me</GradientBorderButton>);

      expect(screen.getByRole('button', {name: 'Click Me'})).toBeInTheDocument();
    });

    it('should render as text variant by default', () => {
      render(<GradientBorderButton>Button</GradientBorderButton>);

      const button = screen.getByRole('button');

      expect(button.className).toContain('MuiButton-text');
    });

    it('should have ripple disabled by default', () => {
      render(<GradientBorderButton>Button</GradientBorderButton>);

      const button = screen.getByRole('button');

      // Check that the button has the TouchRipple component disabled via the disableRipple prop
      // MUI conditionally renders the ripple span only when ripple is enabled
      const rippleElement = button.querySelector('.MuiTouchRipple-root');
      expect(rippleElement).toBeNull();
    });
  });

  describe('Props', () => {
    it('should accept and apply custom className', () => {
      render(<GradientBorderButton className="custom-class">Button</GradientBorderButton>);

      const button = screen.getByRole('button');

      expect(button.className).toContain('custom-class');
    });

    it('should handle onClick event', async () => {
      const user = userEvent.setup();
      let clicked = false;
      const handleClick = () => {
        clicked = true;
      };

      render(<GradientBorderButton onClick={handleClick}>Click Me</GradientBorderButton>);

      await user.click(screen.getByRole('button'));

      expect(clicked).toBe(true);
    });

    it('should be disabled when disabled prop is true', () => {
      render(<GradientBorderButton disabled>Button</GradientBorderButton>);

      const button = screen.getByRole('button');

      expect(button).toBeDisabled();
    });

    it('should accept startIcon prop', () => {
      render(<GradientBorderButton startIcon={<span data-testid="start-icon">→</span>}>Button</GradientBorderButton>);

      expect(screen.getByTestId('start-icon')).toBeInTheDocument();
    });

    it('should accept endIcon prop', () => {
      render(<GradientBorderButton endIcon={<span data-testid="end-icon">←</span>}>Button</GradientBorderButton>);

      expect(screen.getByTestId('end-icon')).toBeInTheDocument();
    });
  });

  describe('Accessibility', () => {
    it('should support aria-label', () => {
      render(<GradientBorderButton aria-label="Custom Label">Button</GradientBorderButton>);

      expect(screen.getByLabelText('Custom Label')).toBeInTheDocument();
    });

    it('should support aria-describedby', () => {
      render(
        <>
          <GradientBorderButton aria-describedby="description">Button</GradientBorderButton>
          <div id="description">Button description</div>
        </>,
      );

      const button = screen.getByRole('button');

      expect(button).toHaveAttribute('aria-describedby', 'description');
    });

    it('should be keyboard accessible', async () => {
      const user = userEvent.setup();
      let clicked = false;
      const handleClick = () => {
        clicked = true;
      };

      render(<GradientBorderButton onClick={handleClick}>Button</GradientBorderButton>);

      const button = screen.getByRole('button');
      button.focus();
      await user.keyboard('{Enter}');

      expect(clicked).toBe(true);
    });
  });

  describe('Ref Forwarding', () => {
    it('should forward ref to button element', () => {
      let buttonRef: HTMLButtonElement | null = null;
      const refCallback = (ref: HTMLButtonElement | null): void => {
        buttonRef = ref;
      };

      render(<GradientBorderButton ref={refCallback}>Button</GradientBorderButton>);

      expect(buttonRef).toBeInstanceOf(HTMLButtonElement);
      expect(buttonRef).not.toBeNull();
      expect(buttonRef!.tagName).toBe('BUTTON');
    });
  });
});
