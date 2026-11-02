// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, act} from '@testing-library/react';
import {describe, it, expect} from 'vitest';
import useEdgeStyleSelector from '../useEdgeStyleSelector';

describe('useEdgeStyleSelector', () => {
  it('should initialize with null anchorEl', () => {
    const {result} = renderHook(() => useEdgeStyleSelector());

    expect(result.current.anchorEl).toBeNull();
  });

  it('should return handleClick function', () => {
    const {result} = renderHook(() => useEdgeStyleSelector());

    expect(typeof result.current.handleClick).toBe('function');
  });

  it('should return handleClose function', () => {
    const {result} = renderHook(() => useEdgeStyleSelector());

    expect(typeof result.current.handleClose).toBe('function');
  });

  it('should set anchorEl on handleClick', () => {
    const {result} = renderHook(() => useEdgeStyleSelector());

    const mockElement = document.createElement('button');
    const mockEvent = {
      currentTarget: mockElement,
    } as unknown as React.MouseEvent<HTMLElement>;

    act(() => {
      result.current.handleClick(mockEvent);
    });

    expect(result.current.anchorEl).toBe(mockElement);
  });

  it('should reset anchorEl to null on handleClose', () => {
    const {result} = renderHook(() => useEdgeStyleSelector());

    const mockElement = document.createElement('button');
    const mockEvent = {
      currentTarget: mockElement,
    } as unknown as React.MouseEvent<HTMLElement>;

    // First open the menu
    act(() => {
      result.current.handleClick(mockEvent);
    });

    expect(result.current.anchorEl).toBe(mockElement);

    // Then close it
    act(() => {
      result.current.handleClose();
    });

    expect(result.current.anchorEl).toBeNull();
  });

  it('should handle multiple open/close cycles', () => {
    const {result} = renderHook(() => useEdgeStyleSelector());

    const mockElement1 = document.createElement('button');
    const mockElement2 = document.createElement('div');

    // First cycle
    act(() => {
      result.current.handleClick({currentTarget: mockElement1} as unknown as React.MouseEvent<HTMLElement>);
    });
    expect(result.current.anchorEl).toBe(mockElement1);

    act(() => {
      result.current.handleClose();
    });
    expect(result.current.anchorEl).toBeNull();

    // Second cycle with different element
    act(() => {
      result.current.handleClick({currentTarget: mockElement2} as unknown as React.MouseEvent<HTMLElement>);
    });
    expect(result.current.anchorEl).toBe(mockElement2);

    act(() => {
      result.current.handleClose();
    });
    expect(result.current.anchorEl).toBeNull();
  });

  it('should preserve function references between renders', () => {
    const {result, rerender} = renderHook(() => useEdgeStyleSelector());

    const initialHandleClick = result.current.handleClick;
    const initialHandleClose = result.current.handleClose;

    rerender();

    // useCallback should maintain stable references
    expect(result.current.handleClick).toBe(initialHandleClick);
    expect(result.current.handleClose).toBe(initialHandleClose);
  });

  it('should return an object with correct shape', () => {
    const {result} = renderHook(() => useEdgeStyleSelector());

    expect(result.current).toHaveProperty('anchorEl');
    expect(result.current).toHaveProperty('handleClick');
    expect(result.current).toHaveProperty('handleClose');
    expect(Object.keys(result.current)).toHaveLength(3);
  });
});
