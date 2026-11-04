// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import {cleanup, fireEvent, screen} from '@testing-library/react';
import {TEST_CN_PREFIX} from '@thunderid/test-utils';
import {afterEach, describe, it, expect, vi} from 'vitest';
import type {FlowComponent} from '../../../../models/flow';
import renderWithProviders from '../../../../test/renderWithProviders';
import SubmitButtonAdapter from '../SubmitButtonAdapter';

afterEach(() => {
  cleanup();
});

const component: FlowComponent = {
  id: 'action_submit',
  type: 'ACTION',
  label: 'Create agent',
  variant: 'PRIMARY',
  eventType: 'SUBMIT',
};

describe('SubmitButtonAdapter', () => {
  it('renders the resolved label', () => {
    renderWithProviders(<SubmitButtonAdapter component={component} disabled={false} resolve={() => 'Resolved'} />);

    expect(screen.getByRole('button', {name: 'Resolved'})).toBeTruthy();
  });

  it('submits the surrounding form by default', () => {
    renderWithProviders(<SubmitButtonAdapter component={component} disabled={false} resolve={(s) => s} />);

    expect(screen.getByRole('button').getAttribute('type')).toBe('submit');
  });

  it('fires its own action instead of submitting the form when onClick is given', () => {
    const onClick = vi.fn();
    renderWithProviders(
      <SubmitButtonAdapter component={component} disabled={false} resolve={(s) => s} onClick={onClick} />,
    );

    fireEvent.click(screen.getByRole('button'));

    expect(screen.getByRole('button').getAttribute('type')).toBe('button');
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it('does not fire its action while it is disabled', () => {
    const onClick = vi.fn();
    renderWithProviders(<SubmitButtonAdapter component={component} disabled resolve={(s) => s} onClick={onClick} />);

    fireEvent.click(screen.getByRole('button'));

    expect(screen.getByRole('button').hasAttribute('disabled')).toBe(true);
    expect(onClick).not.toHaveBeenCalled();
  });

  it('applies the primary class for a PRIMARY action', () => {
    renderWithProviders(<SubmitButtonAdapter component={component} disabled={false} resolve={(s) => s} />);

    expect(screen.getByRole('button').classList.contains(`${TEST_CN_PREFIX}Button--primary`)).toBe(true);
  });

  it('applies the outlined class for any other variant', () => {
    renderWithProviders(
      <SubmitButtonAdapter component={{...component, variant: 'SECONDARY'}} disabled={false} resolve={(s) => s} />,
    );

    expect(screen.getByRole('button').classList.contains(`${TEST_CN_PREFIX}Button--outlined`)).toBe(true);
  });

  it('carries the component id', () => {
    renderWithProviders(<SubmitButtonAdapter component={component} disabled={false} resolve={(s) => s} />);

    expect(screen.getByRole('button').id).toBe('action_submit');
  });
});
