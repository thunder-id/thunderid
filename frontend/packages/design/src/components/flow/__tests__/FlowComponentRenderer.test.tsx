// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import {screen, cleanup, fireEvent} from '@testing-library/react';
import type {EmbeddedFlowComponent} from '@thunderid/react';
import {describe, it, expect, afterEach, vi} from 'vitest';
import renderWithProviders from '../../../test/renderWithProviders';
import FlowComponentRenderer from '../FlowComponentRenderer';

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

const noop = () => undefined;
const identity = (s: string | undefined) => s;

describe('FlowComponentRenderer KEY_VALUE_LIST routing', () => {
  it('renders KeyValueListAdapter when component type is KEY_VALUE_LIST', () => {
    const component = {
      id: 'kv-1',
      type: 'KEY_VALUE_LIST',
      source: 'linkingPromptDetails',
    } as unknown as EmbeddedFlowComponent;

    renderWithProviders(
      <FlowComponentRenderer
        component={component}
        index={0}
        values={{}}
        isLoading={false}
        resolve={identity}
        onInputChange={noop}
        onSubmit={noop}
        additionalData={{linkingPromptDetails: [{label: 'Email', value: 'jane@example.com'}]}}
      />,
    );

    expect(screen.getByText('jane@example.com')).toBeTruthy();
  });
});

describe('FlowComponentRenderer — COPYABLE_TEXT routing', () => {
  it('renders CopyableTextAdapter when component type is COPYABLE_TEXT', () => {
    const component = {
      id: 'copyable-1',
      type: 'COPYABLE_TEXT',
      source: 'inviteLink',
      label: 'Invite Link',
    } as unknown as EmbeddedFlowComponent;

    renderWithProviders(
      <FlowComponentRenderer
        component={component}
        index={0}
        values={{}}
        isLoading={false}
        resolve={identity}
        onInputChange={noop}
        onSubmit={noop}
        additionalData={{inviteLink: 'https://example.com/invite/abc123'}}
      />,
    );

    expect(screen.getByText('https://example.com/invite/abc123')).toBeTruthy();
  });

  it('renders the label from COPYABLE_TEXT component', () => {
    const component = {
      id: 'copyable-2',
      type: 'COPYABLE_TEXT',
      source: 'inviteLink',
      label: 'Invite Link',
    } as unknown as EmbeddedFlowComponent;

    renderWithProviders(
      <FlowComponentRenderer
        component={component}
        index={0}
        values={{}}
        isLoading={false}
        resolve={identity}
        onInputChange={noop}
        onSubmit={noop}
        additionalData={{inviteLink: 'https://example.com/invite/xyz'}}
      />,
    );

    expect(screen.getByText('Invite Link')).toBeTruthy();
  });

  it('renders CopyableTextAdapter with empty value when source key is absent from additionalData', () => {
    const component = {
      id: 'copyable-3',
      type: 'COPYABLE_TEXT',
      source: 'missingKey',
    } as unknown as EmbeddedFlowComponent;

    renderWithProviders(
      <FlowComponentRenderer
        component={component}
        index={0}
        values={{}}
        isLoading={false}
        resolve={identity}
        onInputChange={noop}
        onSubmit={noop}
        additionalData={{inviteLink: 'https://example.com/invite/abc123'}}
      />,
    );

    // CopyableTextAdapter still renders with empty value — Copy button present
    expect(screen.getByRole('button')).toBeTruthy();
  });

  it('renders QrCodeAdapter when component type is QR_CODE', () => {
    const component = {
      id: 'qr-1',
      type: 'QR_CODE',
      source: 'openid4vpWalletUri',
    } as unknown as EmbeddedFlowComponent;

    renderWithProviders(
      <FlowComponentRenderer
        component={component}
        index={0}
        values={{}}
        isLoading={false}
        resolve={identity}
        onInputChange={noop}
        onSubmit={noop}
        additionalData={{openid4vpWalletUri: 'eudi-openid4vp://?client_id=test&request_uri=https%3A%2F%2Fexample.com'}}
      />,
    );

    expect(screen.getByRole('link', {name: 'Open wallet on this device'})).toBeTruthy();
  });

  it('returns null for QR_CODE when source key is absent from additionalData', () => {
    const component = {
      id: 'qr-2',
      type: 'QR_CODE',
      source: 'openid4vpWalletUri',
    } as unknown as EmbeddedFlowComponent;

    const {container} = renderWithProviders(
      <FlowComponentRenderer
        component={component}
        index={0}
        values={{}}
        isLoading={false}
        resolve={identity}
        onInputChange={noop}
        onSubmit={noop}
        additionalData={{}}
      />,
    );

    expect(container.firstChild).toBeNull();
  });

  it('returns null for an unknown component type', () => {
    const component = {
      id: 'unknown-1',
      type: 'UNKNOWN_TYPE',
    } as unknown as EmbeddedFlowComponent;

    const {container} = renderWithProviders(
      <FlowComponentRenderer
        component={component}
        index={0}
        values={{}}
        isLoading={false}
        resolve={identity}
        onInputChange={noop}
        onSubmit={noop}
      />,
    );

    expect(container.firstChild).toBeNull();
  });
});

describe('FlowComponentRenderer — BLOCK with STACK-nested actions', () => {
  const blockWithStackedActions = {
    id: 'block_onboarding_mode',
    type: 'BLOCK',
    components: [
      {
        id: 'stack_actions',
        type: 'STACK',
        direction: 'row',
        justify: 'center',
        components: [
          {id: 'action_create', type: 'ACTION', label: 'Create User', variant: 'PRIMARY', eventType: 'SUBMIT'},
          {id: 'action_invite', type: 'ACTION', label: 'Invite User', variant: 'OUTLINED', eventType: 'SUBMIT'},
        ],
      },
    ],
  } as unknown as EmbeddedFlowComponent;

  it('renders submit actions nested inside a stack', () => {
    renderWithProviders(
      <FlowComponentRenderer
        component={blockWithStackedActions}
        index={0}
        values={{}}
        isLoading={false}
        resolve={identity}
        onInputChange={noop}
        onSubmit={noop}
      />,
    );

    expect(screen.getByRole('button', {name: 'Create User'})).toBeTruthy();
    expect(screen.getByRole('button', {name: 'Invite User'})).toBeTruthy();
  });

  it('exposes the component id on rendered action buttons', () => {
    renderWithProviders(
      <FlowComponentRenderer
        component={blockWithStackedActions}
        index={0}
        values={{}}
        isLoading={false}
        resolve={identity}
        onInputChange={noop}
        onSubmit={noop}
      />,
    );

    // Consumers (e.g. the flow preview) map DOM buttons back to their flow
    // components through this id.
    expect(screen.getByRole('button', {name: 'Create User'}).id).toBe('action_create');
    expect(screen.getByRole('button', {name: 'Invite User'}).id).toBe('action_invite');
  });

  it('dispatches the secondary stacked action on click', () => {
    const onSubmit = vi.fn();
    renderWithProviders(
      <FlowComponentRenderer
        component={blockWithStackedActions}
        index={0}
        values={{}}
        isLoading={false}
        resolve={identity}
        onInputChange={noop}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.click(screen.getByRole('button', {name: 'Invite User'}));

    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({id: 'action_invite'}), {});
  });

  it('dispatches the primary stacked action via form submit', () => {
    const onSubmit = vi.fn();
    renderWithProviders(
      <FlowComponentRenderer
        component={blockWithStackedActions}
        index={0}
        values={{}}
        isLoading={false}
        resolve={identity}
        onInputChange={noop}
        onSubmit={onSubmit}
      />,
    );

    fireEvent.click(screen.getByRole('button', {name: 'Create User'}));

    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({id: 'action_create'}), {});
  });

  it('renders a STACK without items as a flex layout', () => {
    renderWithProviders(
      <FlowComponentRenderer
        component={blockWithStackedActions}
        index={0}
        values={{}}
        isLoading={false}
        resolve={identity}
        onInputChange={noop}
        onSubmit={noop}
      />,
    );

    expect(getComputedStyle(document.getElementById('stack_actions')!).display).toBe('flex');
  });
});

describe('FlowComponentRenderer — STACK grid layout (items)', () => {
  const triggerAction = (id: string, label: string) => ({
    id,
    type: 'ACTION',
    label,
    variant: 'OUTLINE',
    eventType: 'TRIGGER',
  });

  const blockWithGridTriggers = (items: unknown, count = 4): EmbeddedFlowComponent => {
    const block = {
      id: 'block_login_id',
      type: 'BLOCK',
      components: [
        {
          id: 'stack_login_ids',
          type: 'STACK',
          direction: 'col',
          justify: 'center',
          items,
          components: Array.from({length: count}, (_, i) => triggerAction(`login_id_${i}`, `Option ${i}`)),
        },
      ],
    } as unknown as EmbeddedFlowComponent;
    return block;
  };

  const renderComponent = (component: EmbeddedFlowComponent, onSubmit: (...args: unknown[]) => void = noop) =>
    renderWithProviders(
      <FlowComponentRenderer
        component={component}
        index={0}
        values={{}}
        isLoading={false}
        resolve={identity}
        onInputChange={noop}
        onSubmit={onSubmit}
      />,
    );

  // A computed template is either the literal `repeat(n, ...)` or, once the browser
  // resolves it, one entry per track. Counting whitespace alone gets the literal
  // form wrong, so parse the repeat count when it is still present.
  const trackCount = (template: string): number => {
    const repeatMatch = /^repeat\((\d+),/.exec(template);
    return repeatMatch ? Number(repeatMatch[1]) : template.split(' ').length;
  };

  const gridColumnCount = (id: string): number => {
    return trackCount(getComputedStyle(document.getElementById(id)!).gridTemplateColumns);
  };

  it('renders four trigger actions in a 2-column grid when items is 2', () => {
    renderComponent(blockWithGridTriggers(2));

    const stack = document.getElementById('stack_login_ids')!;
    expect(getComputedStyle(stack).display).toBe('grid');
    expect(gridColumnCount('stack_login_ids')).toBe(2);
    ['Option 0', 'Option 1', 'Option 2', 'Option 3'].forEach((label) => {
      expect(screen.getByRole('button', {name: label})).toBeTruthy();
    });
  });

  it('dispatches a trigger action rendered inside a grid stack', () => {
    const onSubmit = vi.fn();
    renderComponent(blockWithGridTriggers(2), onSubmit);

    fireEvent.click(screen.getByRole('button', {name: 'Option 2'}));

    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({id: 'login_id_2'}), {});
  });

  it('renders submit actions in a grid and keeps them wired', () => {
    const onSubmit = vi.fn();
    const block = {
      id: 'block_submits',
      type: 'BLOCK',
      components: [
        {
          id: 'stack_submits',
          type: 'STACK',
          items: 2,
          components: [
            {id: 'action_create', type: 'ACTION', label: 'Create User', variant: 'PRIMARY', eventType: 'SUBMIT'},
            {id: 'action_invite', type: 'ACTION', label: 'Invite User', variant: 'OUTLINED', eventType: 'SUBMIT'},
          ],
        },
      ],
    } as unknown as EmbeddedFlowComponent;

    renderComponent(block, onSubmit);

    expect(getComputedStyle(document.getElementById('stack_submits')!).display).toBe('grid');
    fireEvent.click(screen.getByRole('button', {name: 'Invite User'}));
    expect(onSubmit).toHaveBeenCalledWith(expect.objectContaining({id: 'action_invite'}), {});
  });

  it('renders a standalone STACK with items as a grid', () => {
    const stack = {
      id: 'stack_standalone',
      type: 'STACK',
      items: 2,
      components: [triggerAction('opt_a', 'Option A'), triggerAction('opt_b', 'Option B')],
    } as unknown as EmbeddedFlowComponent;

    renderComponent(stack);

    expect(getComputedStyle(document.getElementById('stack_standalone')!).display).toBe('grid');
    expect(gridColumnCount('stack_standalone')).toBe(2);
  });

  it('supports arbitrary column counts with auto-flowing rows', () => {
    renderComponent(blockWithGridTriggers(3, 5));

    expect(gridColumnCount('stack_login_ids')).toBe(3);
    expect(screen.getAllByRole('button')).toHaveLength(5);
  });

  it('spreads the columns apart when justify distributes free space', () => {
    const block = blockWithGridTriggers(2) as unknown as {components: Record<string, unknown>[]};
    block.components[0].justify = 'space-between';

    renderComponent(block as unknown as EmbeddedFlowComponent);

    const stack = document.getElementById('stack_login_ids')!;
    const buttons = Array.from(stack.querySelectorAll<HTMLButtonElement>('button'), (button: HTMLButtonElement) =>
      button.getBoundingClientRect(),
    );
    const stackBox = stack.getBoundingClientRect();

    // First column starts at the left edge, second column ends at the right edge.
    expect(Math.round(buttons[0].left)).toBe(Math.round(stackBox.left));
    expect(Math.round(buttons[1].right)).toBe(Math.round(stackBox.right));
    // The two columns are no longer touching, which is what 1fr tracks would give.
    expect(buttons[1].left - buttons[0].right).toBeGreaterThan(0);
  });

  it('accepts items provided as a numeric string', () => {
    renderComponent(blockWithGridTriggers('2'));

    expect(getComputedStyle(document.getElementById('stack_login_ids')!).display).toBe('grid');
  });

  it('falls back to flex when items is not a number', () => {
    renderComponent(blockWithGridTriggers('garbage'));

    expect(getComputedStyle(document.getElementById('stack_login_ids')!).display).toBe('flex');
  });

  it('falls back to flex when items is a malformed numeric string', () => {
    renderComponent(blockWithGridTriggers('2invalid'));

    expect(getComputedStyle(document.getElementById('stack_login_ids')!).display).toBe('flex');
  });

  it('keeps the flex layout when items is 1, so previously authored stacks are unchanged', () => {
    renderComponent(blockWithGridTriggers(1));

    expect(getComputedStyle(document.getElementById('stack_login_ids')!).display).toBe('flex');
  });

  it('lays four children out on two rows when items is 2', () => {
    renderComponent(blockWithGridTriggers(2));

    const stack = document.getElementById('stack_login_ids')!;
    const tops = Array.from(stack.querySelectorAll<HTMLButtonElement>('button'), (button: HTMLButtonElement) =>
      Math.round(button.getBoundingClientRect().top),
    );
    expect(new Set(tops).size).toBe(2);
  });

  it('treats items as the row count when direction is column', () => {
    const block = blockWithGridTriggers(2) as unknown as {components: Record<string, unknown>[]};
    block.components[0].direction = 'column';

    renderComponent(block as unknown as EmbeddedFlowComponent);

    const stack = document.getElementById('stack_login_ids')!;
    const style = getComputedStyle(stack);
    expect(style.display).toBe('grid');
    expect(trackCount(style.gridTemplateRows)).toBe(2);
    expect(style.gridAutoFlow).toContain('column');
  });

  it('keeps the flex layout when items is absent', () => {
    const block = blockWithGridTriggers(2) as unknown as {components: Record<string, unknown>[]};
    delete block.components[0].items;

    renderComponent(block as unknown as EmbeddedFlowComponent);

    expect(getComputedStyle(document.getElementById('stack_login_ids')!).display).toBe('flex');
  });

  it('applies custom classes on stacks nested inside blocks', () => {
    const block = blockWithGridTriggers(2) as unknown as {components: Record<string, unknown>[]};
    block.components[0].classes = 'custom-stack';

    renderComponent(block as unknown as EmbeddedFlowComponent);

    const stack = document.getElementById('stack_login_ids')!;
    expect(stack.className).toContain('Flow--stack');
    expect(stack.className).toContain('custom-stack');
  });
});

describe('FlowComponentRenderer — standalone RESEND routing', () => {
  const resendComponent = {
    id: 'action_resend',
    type: 'RESEND',
    category: 'ACTION',
    eventType: 'SUBMIT',
    label: 'Resend',
  } as unknown as EmbeddedFlowComponent;

  const renderResend = (onSubmit: (...args: unknown[]) => void = noop) =>
    renderWithProviders(
      <FlowComponentRenderer
        component={resendComponent}
        index={0}
        values={{otp: '123456'}}
        isLoading={false}
        resolve={identity}
        onInputChange={noop}
        onSubmit={onSubmit}
      />,
    );

  it('renders a RESEND component that sits outside a block', () => {
    renderResend();

    expect(screen.getByText('Resend')).toBeTruthy();
  });

  it('dispatches its own action when clicked', () => {
    const onSubmit = vi.fn();
    renderResend(onSubmit);

    fireEvent.click(screen.getByText('Resend'));

    expect(onSubmit).toHaveBeenCalledWith(resendComponent, {otp: '123456'});
  });

  it('renders a RESEND component nested in a stack', () => {
    const stack = {
      id: 'stack_actions',
      type: 'STACK',
      direction: 'col',
      components: [resendComponent],
    } as unknown as EmbeddedFlowComponent;

    renderWithProviders(
      <FlowComponentRenderer
        component={stack}
        index={0}
        values={{}}
        isLoading={false}
        resolve={identity}
        onInputChange={noop}
        onSubmit={noop}
      />,
    );

    expect(screen.getByText('Resend')).toBeTruthy();
  });

  it('disables the button while the flow is loading', () => {
    renderWithProviders(
      <FlowComponentRenderer
        component={resendComponent}
        index={0}
        values={{}}
        isLoading
        resolve={identity}
        onInputChange={noop}
        onSubmit={noop}
      />,
    );

    expect((document.getElementById('action_resend') as HTMLButtonElement).disabled).toBe(true);
  });
});
