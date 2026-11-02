// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import type {Resource} from '../../../models/resources';
import ResourcePanelDraggable from '../ResourcePanelDraggable';

// Mock Draggable component
vi.mock('../../dnd/Draggable', () => ({
  default: ({
    children,
    id,
    data,
    type,
    accept,
    disabled,
  }: {
    children: React.ReactNode;
    id: string;
    data: object;
    type: string;
    accept: string[];
    disabled: boolean;
  }) => (
    <div
      data-testid="draggable-wrapper"
      data-id={id}
      data-type={type}
      data-accept={JSON.stringify(accept)}
      data-disabled={disabled}
      data-dragged={JSON.stringify(data)}
    >
      {children}
    </div>
  ),
}));

// Mock useColorScheme
vi.mock('@wso2/oxygen-ui', async () => {
  const actual = await vi.importActual('@wso2/oxygen-ui');
  return {
    ...actual,
    useColorScheme: () => ({mode: 'light', systemMode: 'light'}),
  };
});

// Mock resolveStaticResourcePath
vi.mock('../../../utils/resolveStaticResourcePath', () => ({
  default: (path: string) => `/static/${path}`,
}));

const createMockResource = (overrides: Partial<Resource> = {}): Resource =>
  ({
    type: 'DRAGGABLE_STEP',
    resourceType: 'STEP',
    display: {
      label: 'Draggable Step',
      description: 'A draggable step description',
      image: 'step-icon.svg',
      showOnResourcePanel: true,
    },
    ...overrides,
  }) as Resource;

describe('ResourcePanelDraggable', () => {
  describe('Draggable Wrapper', () => {
    it('should wrap content in Draggable component', () => {
      const resource = createMockResource();
      render(<ResourcePanelDraggable id="step-1" resource={resource} onAdd={vi.fn()} />);

      expect(screen.getByTestId('draggable-wrapper')).toBeInTheDocument();
    });

    it('should pass id to Draggable', () => {
      const resource = createMockResource();
      render(<ResourcePanelDraggable id="unique-step-id" resource={resource} onAdd={vi.fn()} />);

      expect(screen.getByTestId('draggable-wrapper')).toHaveAttribute('data-id', 'unique-step-id');
    });

    it('should pass resource type to Draggable', () => {
      const resource = createMockResource({type: 'CUSTOM_STEP'} as Partial<Resource>);
      render(<ResourcePanelDraggable id="step-1" resource={resource} onAdd={vi.fn()} />);

      expect(screen.getByTestId('draggable-wrapper')).toHaveAttribute('data-type', 'CUSTOM_STEP');
    });

    it('should set accept to match resource type', () => {
      const resource = createMockResource({type: 'CUSTOM_STEP'} as Partial<Resource>);
      render(<ResourcePanelDraggable id="step-1" resource={resource} onAdd={vi.fn()} />);

      const wrapper = screen.getByTestId('draggable-wrapper');
      const accept = JSON.parse(wrapper.getAttribute('data-accept') ?? '[]') as string[];
      expect(accept).toContain('CUSTOM_STEP');
    });

    it('should pass dragged resource in data prop', () => {
      const resource = createMockResource();
      render(<ResourcePanelDraggable id="step-1" resource={resource} onAdd={vi.fn()} />);

      const wrapper = screen.getByTestId('draggable-wrapper');
      const data = JSON.parse(wrapper.getAttribute('data-dragged') ?? '{}') as {dragged: Resource};
      expect(data.dragged).toEqual(resource);
    });
  });

  describe('ResourcePanelItem Integration', () => {
    it('should render resource label', () => {
      const resource = createMockResource();
      render(<ResourcePanelDraggable id="step-1" resource={resource} onAdd={vi.fn()} />);

      expect(screen.getByText('Draggable Step')).toBeInTheDocument();
    });

    it('should render resource description', () => {
      const resource = createMockResource();
      render(<ResourcePanelDraggable id="step-1" resource={resource} onAdd={vi.fn()} />);

      expect(screen.getByText('A draggable step description')).toBeInTheDocument();
    });
  });

  describe('Type Default', () => {
    it('should have draggable as default type', () => {
      const resource = createMockResource();
      const {container} = render(<ResourcePanelDraggable id="step-1" resource={resource} onAdd={vi.fn()} />);

      // Draggable type card renders
      const card = container.querySelector('.MuiCard-root');
      expect(card).toBeInTheDocument();
    });
  });

  describe('Disabled State', () => {
    it('should pass disabled=true to Draggable', () => {
      const resource = createMockResource();
      render(<ResourcePanelDraggable id="step-1" resource={resource} onAdd={vi.fn()} disabled />);

      expect(screen.getByTestId('draggable-wrapper')).toHaveAttribute('data-disabled', 'true');
    });

    it('should pass disabled=false by default', () => {
      const resource = createMockResource();
      render(<ResourcePanelDraggable id="step-1" resource={resource} onAdd={vi.fn()} />);

      expect(screen.getByTestId('draggable-wrapper')).toHaveAttribute('data-disabled', 'false');
    });

    it('should disable add button when disabled', () => {
      const resource = createMockResource();
      const onAdd = vi.fn();
      render(<ResourcePanelDraggable id="step-1" resource={resource} onAdd={onAdd} disabled />);

      const button = screen.getByRole('button');
      expect(button).toBeDisabled();
    });
  });

  describe('onAdd Callback', () => {
    it('should pass onAdd to ResourcePanelItem', () => {
      const resource = createMockResource();
      const onAdd = vi.fn();
      render(<ResourcePanelDraggable id="step-1" resource={resource} onAdd={onAdd} />);

      const button = screen.getByRole('button');
      fireEvent.click(button);

      expect(onAdd).toHaveBeenCalledWith(resource);
    });

    it('should not call onAdd when disabled', () => {
      const resource = createMockResource();
      const onAdd = vi.fn();
      render(<ResourcePanelDraggable id="step-1" resource={resource} onAdd={onAdd} disabled />);

      const button = screen.getByRole('button');
      fireEvent.click(button);

      expect(onAdd).not.toHaveBeenCalled();
    });
  });

  describe('Additional Props', () => {
    it('should pass additional HTML attributes', () => {
      const resource = createMockResource();
      render(
        <ResourcePanelDraggable
          id="step-1"
          resource={resource}
          onAdd={vi.fn()}
          data-testid="draggable-item"
          className="custom-class"
        />,
      );

      // The Draggable wrapper receives the data-testid through rest props
      expect(screen.getByTestId('draggable-wrapper')).toBeInTheDocument();
    });
  });

  describe('Type Prop', () => {
    it('should accept custom type prop', () => {
      const resource = createMockResource();
      const {container} = render(
        <ResourcePanelDraggable id="step-1" resource={resource} onAdd={vi.fn()} type="static" />,
      );

      // Component renders with custom type
      const card = container.querySelector('.MuiCard-root');
      expect(card).toBeInTheDocument();
    });

    it('should use draggable type by default when type is not provided', () => {
      const resource = createMockResource();
      const {container} = render(<ResourcePanelDraggable id="step-1" resource={resource} onAdd={vi.fn()} />);

      // Component renders with default draggable type
      const card = container.querySelector('.MuiCard-root');
      expect(card).toBeInTheDocument();
    });
  });
});
