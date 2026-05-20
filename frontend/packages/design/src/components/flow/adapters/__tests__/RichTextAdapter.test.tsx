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

/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @typescript-eslint/no-unsafe-assignment */
/* eslint-disable @typescript-eslint/no-unsafe-member-access */
/* eslint-disable @typescript-eslint/no-unsafe-return */

import {cleanup} from '@testing-library/react';
import {describe, it, expect, vi, afterEach} from 'vitest';
import type {FlowComponent} from '../../../../models/flow';
import renderWithProviders from '../../../../test/renderWithProviders';
import RichTextAdapter from '../RichTextAdapter';

afterEach(() => {
  cleanup();
});

vi.mock('@wso2/oxygen-ui', () => ({
  Alert: ({children}: any) => children,
  Box: ({sx, dangerouslySetInnerHTML}: any) => (
    <div
      data-testid="rich-text-box"
      data-align={sx?.textAlign}
      // eslint-disable-next-line react/no-danger
      dangerouslySetInnerHTML={dangerouslySetInnerHTML}
    />
  ),
  extendTheme: vi.fn(),
  OxygenUIThemeProvider: ({children}: any) => children,
  Snackbar: ({children}: any) => children,
}));

const baseComponent: FlowComponent = {
  id: 'rich-1',
  type: 'RICH_TEXT',
  label: '<p>Hello <strong>World</strong></p>',
};

describe('RichTextAdapter', () => {
  it('renders resolved HTML content', () => {
    const {getByTestId} = renderWithProviders(
      <RichTextAdapter component={baseComponent} resolve={(s: string | undefined) => s} />,
    );
    expect(getByTestId('rich-text-box').innerHTML).toBe('<p>Hello <strong>World</strong></p>');
  });

  it('uses resolved label from resolve function', () => {
    const {getByTestId} = renderWithProviders(
      <RichTextAdapter component={baseComponent} resolve={() => '<em>Resolved</em>'} />,
    );
    expect(getByTestId('rich-text-box').innerHTML).toBe('<em>Resolved</em>');
  });

  it('falls back to component.label when resolve returns undefined', () => {
    const {getByTestId} = renderWithProviders(<RichTextAdapter component={baseComponent} resolve={() => undefined} />);
    expect(getByTestId('rich-text-box').innerHTML).toBe('<p>Hello <strong>World</strong></p>');
  });

  it('renders empty string when resolve returns undefined and label is not a string', () => {
    const component = {...baseComponent, label: undefined};
    const {getByTestId} = renderWithProviders(<RichTextAdapter component={component} resolve={() => undefined} />);
    expect(getByTestId('rich-text-box').innerHTML).toBe('');
  });

  it('aligns text to center when isDesignEnabled is true', () => {
    const {getByTestId} = renderWithProviders(
      <RichTextAdapter component={baseComponent} resolve={(s: string | undefined) => s} />,
      {designContext: {isDesignEnabled: true}},
    );
    expect(getByTestId('rich-text-box')).toHaveAttribute('data-align', 'center');
  });

  it('aligns text to left when isDesignEnabled is false', () => {
    const {getByTestId} = renderWithProviders(
      <RichTextAdapter component={baseComponent} resolve={(s: string | undefined) => s} />,
    );
    expect(getByTestId('rich-text-box')).toHaveAttribute('data-align', 'left');
  });

  describe('sign-up URL handling', () => {
    const signUpLabel = '<p>Don\'t have an account? <a href="{{meta(application.sign_up_url)}}">Sign up</a></p>';
    const signUpComponent: FlowComponent = {
      id: 'signup-richtext',
      type: 'RICH_TEXT',
      label: signUpLabel,
    };

    it('returns null when registration is disabled', () => {
      const resolve = (template: string | undefined) =>
        template?.includes('isRegistrationFlowEnabled') ? 'false' : template;

      const {queryByTestId} = renderWithProviders(
        <RichTextAdapter component={signUpComponent} resolve={resolve} signUpFallbackUrl="/signup" />,
      );
      expect(queryByTestId('rich-text-box')).not.toBeInTheDocument();
    });

    it('renders the sign-up link when registration is enabled and the server resolves the URL', () => {
      const resolve = (template: string | undefined) => {
        if (template?.includes('isRegistrationFlowEnabled')) return 'true';
        return template?.replace('{{meta(application.sign_up_url)}}', '/custom/signup');
      };

      const {getByTestId} = renderWithProviders(<RichTextAdapter component={signUpComponent} resolve={resolve} />);
      const box = getByTestId('rich-text-box');
      expect(box).toBeInTheDocument();
      expect(box.innerHTML).toContain('/custom/signup');
    });

    it('uses signUpFallbackUrl when the server does not resolve the sign-up URL template', () => {
      const resolve = (template: string | undefined) =>
        template?.includes('isRegistrationFlowEnabled') ? 'true' : template;

      const {getByTestId} = renderWithProviders(
        <RichTextAdapter component={signUpComponent} resolve={resolve} signUpFallbackUrl="/signup?client_id=abc" />,
      );
      expect(getByTestId('rich-text-box').innerHTML).toContain('/signup?client_id=abc');
    });

    it('renders sign-up content without href substitution when signUpFallbackUrl is not provided', () => {
      const resolve = (template: string | undefined) =>
        template?.includes('isRegistrationFlowEnabled') ? 'true' : template;

      const {getByTestId} = renderWithProviders(<RichTextAdapter component={signUpComponent} resolve={resolve} />);
      // Component renders (registration enabled) but no fallback URL is substituted
      expect(getByTestId('rich-text-box')).toBeInTheDocument();
      expect(getByTestId('rich-text-box').innerHTML).not.toContain('/signup?');
    });
  });

  describe('sign-in URL handling', () => {
    const signInLabel = '<p>Go back to <a href="{{meta(application.sign_in_url)}}">Sign in</a></p>';
    const signInComponent: FlowComponent = {
      id: 'signin-richtext',
      type: 'RICH_TEXT',
      label: signInLabel,
    };

    it('renders the sign-in link when the server resolves the URL', () => {
      const resolve = (template: string | undefined) =>
        template?.replace('{{meta(application.sign_in_url)}}', '/custom/signin');

      const {getByTestId} = renderWithProviders(<RichTextAdapter component={signInComponent} resolve={resolve} />);
      expect(getByTestId('rich-text-box').innerHTML).toContain('/custom/signin');
    });

    it('uses signInFallbackUrl when the server does not resolve the sign-in URL template', () => {
      const {getByTestId} = renderWithProviders(
        <RichTextAdapter
          component={signInComponent}
          resolve={(template: string | undefined) => template}
          signInFallbackUrl="/signin?client_id=abc"
        />,
      );

      expect(getByTestId('rich-text-box').innerHTML).toContain('/signin?client_id=abc');
    });
  });

  describe('application URL handling', () => {
    const applicationUrlLabel = '<p>Go back to <a href="{{meta(application.url)}}">Application</a></p>';
    const applicationUrlComponent: FlowComponent = {
      id: 'application-url-richtext',
      type: 'RICH_TEXT',
      label: applicationUrlLabel,
    };

    it('renders the application link when the server resolves the URL', () => {
      const resolve = (template: string | undefined) =>
        template?.replace('{{meta(application.url)}}', 'https://app.example.com');

      const {getByTestId} = renderWithProviders(
        <RichTextAdapter component={applicationUrlComponent} resolve={resolve} />,
      );

      expect(getByTestId('rich-text-box').innerHTML).toContain('https://app.example.com');
    });

    it('returns null when the application URL is missing', () => {
      const {queryByTestId} = renderWithProviders(
        <RichTextAdapter component={applicationUrlComponent} resolve={(template: string | undefined) => template} />,
      );

      expect(queryByTestId('rich-text-box')).not.toBeInTheDocument();
    });
  });
});
