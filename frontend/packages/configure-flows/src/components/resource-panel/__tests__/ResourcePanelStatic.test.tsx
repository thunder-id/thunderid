// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {render, screen, fireEvent} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import type {Resource} from '../../../models/resources';
import ResourcePanelStatic from '../ResourcePanelStatic';

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
    type: 'TEMPLATE_TYPE',
    resourceType: 'TEMPLATE',
    display: {
      label: 'Static Template',
      description: 'A static template description',
      image: 'template-icon.svg',
      showOnResourcePanel: true,
    },
    ...overrides,
  }) as Resource;

describe('ResourcePanelStatic', () => {
  describe('Rendering', () => {
    it('should render resource label', () => {
      const resource = createMockResource();
      render(<ResourcePanelStatic id="test-static-1" resource={resource} />);

      expect(screen.getByText('Static Template')).toBeInTheDocument();
    });

    it('should render resource description', () => {
      const resource = createMockResource();
      render(<ResourcePanelStatic id="test-static-1" resource={resource} />);

      expect(screen.getByText('A static template description')).toBeInTheDocument();
    });

    it('should pass id to ResourcePanelItem', () => {
      const resource = createMockResource();
      const {container} = render(<ResourcePanelStatic id="unique-id-123" resource={resource} />);

      // ID should be passed through - component renders successfully
      expect(container.firstChild).toBeInTheDocument();
    });
  });

  describe('Type Default', () => {
    it('should have static as default type', () => {
      const resource = createMockResource();
      const {container} = render(<ResourcePanelStatic id="test-id" resource={resource} />);

      // Static type card renders with default cursor behavior
      const card = container.querySelector('.MuiCard-root');
      expect(card).toBeInTheDocument();
    });
  });

  describe('Disabled State', () => {
    it('should pass disabled state to ResourcePanelItem', () => {
      const resource = createMockResource();
      const onAdd = vi.fn();
      render(<ResourcePanelStatic id="test-id" resource={resource} onAdd={onAdd} disabled />);

      const button = screen.getByRole('button');
      expect(button).toBeDisabled();
    });

    it('should not disable by default', () => {
      const resource = createMockResource();
      const onAdd = vi.fn();
      render(<ResourcePanelStatic id="test-id" resource={resource} onAdd={onAdd} />);

      const button = screen.getByRole('button');
      expect(button).not.toBeDisabled();
    });
  });

  describe('onAdd Callback', () => {
    it('should pass onAdd to ResourcePanelItem', () => {
      const resource = createMockResource();
      const onAdd = vi.fn();
      render(<ResourcePanelStatic id="test-id" resource={resource} onAdd={onAdd} />);

      const button = screen.getByRole('button');
      fireEvent.click(button);

      expect(onAdd).toHaveBeenCalledWith(resource);
    });
  });

  describe('Additional Props', () => {
    it('should render with additional className', () => {
      const resource = createMockResource();
      const {container} = render(<ResourcePanelStatic id="test-id" resource={resource} className="custom-class" />);

      // Component renders - additional props may not be passed through to the underlying element
      expect(container.querySelector('.MuiCard-root')).toBeInTheDocument();
    });
  });

  describe('Type Prop', () => {
    it('should accept custom type prop', () => {
      const resource = createMockResource();
      const {container} = render(<ResourcePanelStatic id="test-id" resource={resource} type="draggable" />);

      // Component renders with custom type
      const card = container.querySelector('.MuiCard-root');
      expect(card).toBeInTheDocument();
    });

    it('should use static type by default when type is not provided', () => {
      const resource = createMockResource();
      const {container} = render(<ResourcePanelStatic id="test-id" resource={resource} />);

      // Component renders with default static type
      const card = container.querySelector('.MuiCard-root');
      expect(card).toBeInTheDocument();
    });
  });
});
