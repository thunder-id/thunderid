// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-assignment, @typescript-eslint/prefer-nullish-coalescing, @typescript-eslint/no-unsafe-member-access */

import {render, screen} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import {PreviewScreenType} from '../../models/custom-text-preference';
import FlowBuilderProvider from '../FlowBuilderProvider';

// Mock FlowBuilderCoreProvider
vi.mock('../FlowBuilderCoreProvider', () => ({
  default: ({children, screenTypes}: {children: React.ReactNode; screenTypes: PreviewScreenType[]}) => (
    <div data-testid="flow-builder-core-provider" data-screen-types={JSON.stringify(screenTypes)}>
      {children}
    </div>
  ),
}));

// Mock ResourceProperties
vi.mock('../../components/resource-property-panel/ResourceProperties', () => ({
  default: () => <div>Resource Properties</div>,
}));

// Mock ElementFactory
vi.mock('../../components/resources/elements/ElementFactory', () => ({
  default: () => <div>Element Factory</div>,
}));

describe('FlowBuilderProvider', () => {
  describe('Component Structure', () => {
    it('should render FlowBuilderCoreProvider', () => {
      render(
        <FlowBuilderProvider>
          <div data-testid="child">Child Content</div>
        </FlowBuilderProvider>,
      );

      expect(screen.getByTestId('flow-builder-core-provider')).toBeInTheDocument();
    });

    it('should render children inside FlowBuilderCoreProvider', () => {
      render(
        <FlowBuilderProvider>
          <div data-testid="child">Child Content</div>
        </FlowBuilderProvider>,
      );

      const coreProvider = screen.getByTestId('flow-builder-core-provider');
      const child = screen.getByTestId('child');

      expect(coreProvider).toContainElement(child);
    });
  });

  describe('Screen Types Configuration', () => {
    it('should pass correct screen types to FlowBuilderCoreProvider', () => {
      render(
        <FlowBuilderProvider>
          <div>Content</div>
        </FlowBuilderProvider>,
      );

      const coreProvider = screen.getByTestId('flow-builder-core-provider');
      const screenTypes = JSON.parse(coreProvider.getAttribute('data-screen-types') || '[]');

      expect(screenTypes).toContain(PreviewScreenType.SIGN_UP);
      expect(screenTypes).toContain(PreviewScreenType.COMMON);
      expect(screenTypes).toContain(PreviewScreenType.EMAIL_LINK_EXPIRY);
      expect(screenTypes).toContain(PreviewScreenType.SMS_OTP);
      expect(screenTypes).toContain(PreviewScreenType.EMAIL_OTP);
    });

    it('should have 5 screen types configured', () => {
      render(
        <FlowBuilderProvider>
          <div>Content</div>
        </FlowBuilderProvider>,
      );

      const coreProvider = screen.getByTestId('flow-builder-core-provider');
      const screenTypes = JSON.parse(coreProvider.getAttribute('data-screen-types') || '[]');

      expect(screenTypes).toHaveLength(5);
    });

    it('should have SIGN_UP as the first screen type (primary)', () => {
      render(
        <FlowBuilderProvider>
          <div>Content</div>
        </FlowBuilderProvider>,
      );

      const coreProvider = screen.getByTestId('flow-builder-core-provider');
      const screenTypes = JSON.parse(coreProvider.getAttribute('data-screen-types') || '[]');

      expect(screenTypes[0]).toBe(PreviewScreenType.SIGN_UP);
    });
  });

  describe('Children Rendering', () => {
    it('should render children content', () => {
      render(
        <FlowBuilderProvider>
          <div data-testid="child">Child Content</div>
        </FlowBuilderProvider>,
      );

      expect(screen.getByTestId('child')).toHaveTextContent('Child Content');
    });

    it('should render multiple children', () => {
      render(
        <FlowBuilderProvider>
          <div data-testid="child-1">First Child</div>
          <div data-testid="child-2">Second Child</div>
        </FlowBuilderProvider>,
      );

      expect(screen.getByTestId('child-1')).toBeInTheDocument();
      expect(screen.getByTestId('child-2')).toBeInTheDocument();
    });
  });
});
