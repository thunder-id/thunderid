// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import React, {type CSSProperties, type HTMLAttributes, type ReactElement, type Ref} from 'react';

/**
 * Props interface for the Action component.
 */
export interface ActionProps extends HTMLAttributes<HTMLButtonElement> {
  /**
   * Cursor style for the action button.
   */
  cursor?: CSSProperties['cursor'];
}

/**
 * Action component.
 *
 * @param props - Props injected to the component.
 * @returns The Action component.
 */
function Action(
  {className, cursor = 'pointer', style, ...rest}: ActionProps,
  ref: Ref<HTMLButtonElement>,
): ReactElement {
  return (
    <button
      ref={ref}
      type="button"
      className={className}
      style={
        {
          ...style,
          cursor,
          backgroundColor: 'transparent',
          border: 'none',
          transition: 'background-color 0.2s ease',
          height: '100%',
          width: '32px',
          padding: '4px',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
        } as CSSProperties
      }
      onMouseEnter={(e) => {
        e.currentTarget.style.backgroundColor = 'rgba(0, 0, 0, 0.15)';
      }}
      onMouseLeave={(e) => {
        e.currentTarget.style.backgroundColor = 'transparent';
      }}
      {...rest}
    />
  );
}

export default React.forwardRef(Action);
