// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import {screen, cleanup} from '@testing-library/react';
import {describe, it, expect, afterEach} from 'vitest';
import DesignContext from '../../../../contexts/Design/DesignContext';
import type {FlowComponent} from '../../../../models/flow';
import renderWithProviders from '../../../../test/renderWithProviders';
import TextAdapter from '../TextAdapter';

afterEach(() => {
  cleanup();
});

const baseComponent: FlowComponent = {
  id: 'text-1',
  type: 'TEXT',
  label: 'Hello World',
};

describe('TextAdapter', () => {
  it('renders the resolved label', () => {
    renderWithProviders(<TextAdapter component={baseComponent} resolve={(s) => s} />);
    expect(screen.getByText('Hello World')).toBeTruthy();
  });

  it('passes resolved label through the resolve function', () => {
    const component = {...baseComponent, label: '{{t(greet)}}'};
    renderWithProviders(<TextAdapter component={component} resolve={() => 'Resolved Text'} />);
    expect(screen.getByText('Resolved Text')).toBeTruthy();
  });

  it('applies product prefix CSS class names', () => {
    renderWithProviders(<TextAdapter component={baseComponent} resolve={(s) => s} />);
    const el = screen.getByText('Hello World');
    expect(el.className).toContain('ThunderIDFlow--text');
  });

  it('uses center alignment when design mode is enabled and no align prop', () => {
    renderWithProviders(<TextAdapter component={baseComponent} resolve={(s) => s} />, {
      designContext: {isDesignEnabled: true},
    });
    const el = screen.getByText('Hello World');
    expect(window.getComputedStyle(el).textAlign).toBe('center');
  });

  it('uses component.align when provided, overriding design mode', () => {
    const component = {...baseComponent, align: 'center'};
    renderWithProviders(<TextAdapter component={component} resolve={(s) => s} />, {
      designContext: {isDesignEnabled: false},
    });
    const el = screen.getByText('Hello World');
    expect(window.getComputedStyle(el).textAlign).toBe('center');
  });

  it('renders left-aligned when no DesignProvider is mounted', () => {
    renderWithProviders(
      <DesignContext.Provider value={undefined}>
        <TextAdapter component={baseComponent} resolve={(s) => s} />
      </DesignContext.Provider>,
    );
    const el = screen.getByText('Hello World');
    expect(window.getComputedStyle(el).textAlign).toBe('left');
  });

  it('uses component.align when no DesignProvider is mounted', () => {
    const component = {...baseComponent, align: 'center'};
    renderWithProviders(
      <DesignContext.Provider value={undefined}>
        <TextAdapter component={component} resolve={(s) => s} />
      </DesignContext.Provider>,
    );
    const el = screen.getByText('Hello World');
    expect(window.getComputedStyle(el).textAlign).toBe('center');
  });

  it('falls back to left alignment when no align and design mode is disabled', () => {
    renderWithProviders(<TextAdapter component={baseComponent} resolve={(s) => s} />, {
      designContext: {isDesignEnabled: false},
    });
    const el = screen.getByText('Hello World');
    expect(window.getComputedStyle(el).textAlign).toBe('left');
  });
});
