// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@testing-library/react';
import {OxygenUIThemeProvider} from '@wso2/oxygen-ui';
import type {ReactNode} from 'react';
import {afterEach, describe, expect, it} from 'vitest';
import useConsoleSurface from '../useConsoleSurface';

const wrapper = ({children}: {children: ReactNode}) => <OxygenUIThemeProvider>{children}</OxygenUIThemeProvider>;

describe('useConsoleSurface', () => {
  afterEach(() => {
    document.body.removeAttribute('style');
    document.documentElement.removeAttribute('style');
  });

  it('paints the theme background color, whatever the page does', () => {
    const {result} = renderHook(() => useConsoleSurface(true), {wrapper});

    expect(result.current.pageSx).toMatchObject({backgroundColor: 'background.default'});
  });

  it('takes what the page paints from the page itself, not from a theme override', () => {
    document.body.style.backgroundImage = 'linear-gradient(rgb(255, 0, 0), rgb(0, 0, 255))';
    document.body.style.backgroundAttachment = 'fixed';

    const {result} = renderHook(() => useConsoleSurface(true), {wrapper});

    expect(result.current.pageSx).toMatchObject({
      backgroundAttachment: 'fixed',
      backgroundImage: 'linear-gradient(rgb(255, 0, 0), rgb(0, 0, 255))',
    });
  });

  it('reads the root element when the body paints nothing', () => {
    document.documentElement.style.backgroundImage = 'linear-gradient(rgb(0, 128, 0), rgb(0, 0, 0))';

    const {result} = renderHook(() => useConsoleSurface(true), {wrapper});

    expect(result.current.pageSx).toMatchObject({backgroundImage: 'linear-gradient(rgb(0, 128, 0), rgb(0, 0, 0))'});
  });

  it('adds nothing when the page paints no image', () => {
    const {result} = renderHook(() => useConsoleSurface(true), {wrapper});

    expect(result.current.pageSx).not.toHaveProperty('backgroundImage');
  });

  it('does not read the page while the view is closed', () => {
    document.body.style.backgroundImage = 'linear-gradient(rgb(255, 0, 0), rgb(0, 0, 255))';

    const {result} = renderHook(() => useConsoleSurface(false), {wrapper});

    expect(result.current.pageSx).not.toHaveProperty('backgroundImage');
  });
});
