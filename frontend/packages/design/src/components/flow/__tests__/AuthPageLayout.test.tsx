/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {screen, cleanup} from '@testing-library/react';
import {describe, it, expect, afterEach} from 'vitest';
import renderWithProviders from '../../../test/renderWithProviders';
import AuthPageLayout from '../AuthPageLayout';

afterEach(() => {
  cleanup();
});

describe('AuthPageLayout', () => {
  it('renders children', () => {
    renderWithProviders(
      <AuthPageLayout isLoading={false}>
        <span>Page content</span>
      </AuthPageLayout>,
    );
    expect(screen.getByText('Page content')).toBeTruthy();
  });

  it('renders a main element', () => {
    renderWithProviders(
      <AuthPageLayout isLoading={false}>
        <span>Content</span>
      </AuthPageLayout>,
    );
    expect(screen.getByRole('main')).toBeTruthy();
  });

  describe('isLoading prop', () => {
    it('renders children when isLoading is false', () => {
      renderWithProviders(
        <AuthPageLayout isLoading={false}>
          <span>Page content</span>
        </AuthPageLayout>,
      );
      expect(screen.getByText('Page content')).toBeTruthy();
    });

    it('renders CircularProgress when isLoading is true', () => {
      renderWithProviders(
        <AuthPageLayout isLoading={true}>
          <span>Page content</span>
        </AuthPageLayout>,
      );
      expect(screen.queryByText('Page content')).toBeNull();
      expect(screen.getByRole('progressbar')).toBeTruthy();
    });
  });

  describe('variant prop', () => {
    it('applies product name prefixed CSS root class when variant is provided', () => {
      renderWithProviders(
        <AuthPageLayout isLoading={false} variant="SignIn">
          <span>Content</span>
        </AuthPageLayout>,
      );
      const main = screen.getByRole('main');
      expect(main.classList.contains('ThunderIDSignIn--root')).toBe(true);
    });

    it('does not apply product prefix CSS class when variant is not provided', () => {
      renderWithProviders(
        <AuthPageLayout isLoading={false}>
          <span>Content</span>
        </AuthPageLayout>,
      );
      const main = screen.getByRole('main');
      const thunderClasses = Array.from(main.classList).filter((c) => c.startsWith('ThunderID'));
      expect(thunderClasses).toHaveLength(0);
    });
  });

  describe('background prop', () => {
    it('renders without errors when background is provided', () => {
      renderWithProviders(
        <AuthPageLayout isLoading={false} background="#ff0000">
          <span>Content</span>
        </AuthPageLayout>,
      );
      expect(screen.getByRole('main')).toBeTruthy();
    });

    it('renders without errors when background is a gradient', () => {
      renderWithProviders(
        <AuthPageLayout isLoading={false} background="linear-gradient(135deg, #667eea 0%, #764ba2 100%)">
          <span>Content</span>
        </AuthPageLayout>,
      );
      expect(screen.getByRole('main')).toBeTruthy();
    });

    it('renders without errors when background is not provided (uses theme default)', () => {
      renderWithProviders(
        <AuthPageLayout isLoading={false}>
          <span>Content</span>
        </AuthPageLayout>,
      );
      expect(screen.getByRole('main')).toBeTruthy();
    });
  });

  describe('combined props', () => {
    it('renders with both variant and background', () => {
      renderWithProviders(
        <AuthPageLayout isLoading={false} variant="SignUp" background="#f0f0f0">
          <span>Sign up form</span>
        </AuthPageLayout>,
      );
      const main = screen.getByRole('main');
      expect(main.classList.contains('ThunderIDSignUp--root')).toBe(true);
      expect(screen.getByText('Sign up form')).toBeTruthy();
    });

    it('renders multiple children correctly', () => {
      renderWithProviders(
        <AuthPageLayout isLoading={false}>
          <span>First child</span>
          <span>Second child</span>
        </AuthPageLayout>,
      );
      expect(screen.getByText('First child')).toBeTruthy();
      expect(screen.getByText('Second child')).toBeTruthy();
    });
  });
});
