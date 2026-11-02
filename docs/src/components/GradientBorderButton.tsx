// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {styled, keyframes, Button, type ButtonProps} from '@wso2/oxygen-ui';
import {forwardRef} from 'react';

const spin = keyframes`
  0% {
    --gradient-angle: 0deg;
  }
  100% {
    --gradient-angle: 360deg;
  }
`;

// Register CSS @property for animatable custom property
if (typeof window !== 'undefined' && 'CSS' in window && 'registerProperty' in CSS) {
  try {
    CSS.registerProperty({
      name: '--gradient-angle',
      syntax: '<angle>',
      initialValue: '0deg',
      inherits: false,
    });
  } catch {
    // Property already registered
  }
}

const StyledGradientButton = styled(Button)(() => ({
  position: 'relative',
  display: 'inline-flex',
  borderRadius: '999px',
  padding: '8px 16px',
  border: '2px solid transparent',
  background: 'transparent',
  color: 'var(--mui-palette-text-primary)',
  fontWeight: 600,
  textTransform: 'none',
  backgroundClip: 'padding-box',
  isolation: 'isolate',
  '&::before': {
    content: '""',
    position: 'absolute',
    inset: 0,
    borderRadius: '999px',
    padding: '2px',
    background: 'conic-gradient(from var(--gradient-angle), #667eea, #764ba2, #f093fb, #4facfe, #00f2fe, #667eea)',
    WebkitMask: 'linear-gradient(#fff 0 0) content-box, linear-gradient(#fff 0 0)',
    WebkitMaskComposite: 'xor',
    maskComposite: 'exclude',
    animation: `${spin} 4s linear infinite`,
    zIndex: -1,
  },
  '&:hover': {
    background: 'var(--mui-palette-action-hover)',
    '&::before': {
      animationPlayState: 'paused',
    },
  },
  '&.Mui-disabled': {
    '&::before': {
      animationPlayState: 'paused',
      opacity: 0.6,
    },
  },
}));

/**
 * A button component with an animated conic-gradient border, used to flag AI-related actions
 * (e.g. copying an LLM prompt) the same way the console's application integration guides do.
 */
const GradientBorderButton = forwardRef<HTMLButtonElement, ButtonProps>((props, ref) => (
  <StyledGradientButton ref={ref} variant="text" disableRipple disableFocusRipple {...props} />
));

GradientBorderButton.displayName = 'GradientBorderButton';

export default GradientBorderButton;
