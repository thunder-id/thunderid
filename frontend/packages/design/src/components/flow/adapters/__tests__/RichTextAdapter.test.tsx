// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-explicit-any */
/* eslint-disable @typescript-eslint/no-unsafe-assignment */
/* eslint-disable @typescript-eslint/no-unsafe-member-access */
/* eslint-disable @typescript-eslint/no-unsafe-return */

import {cleanup, fireEvent} from '@testing-library/react';
import {describe, it, expect, vi, afterEach} from 'vitest';
import type {FlowComponent} from '../../../../models/flow';
import renderWithProviders from '../../../../test/renderWithProviders';
import RichTextAdapter from '../RichTextAdapter';

afterEach(() => {
  cleanup();
});

vi.mock('@wso2/oxygen-ui', () => ({
  Alert: ({children}: any) => children,
  Box: ({id, sx, onClick, dangerouslySetInnerHTML}: any) => (
    // eslint-disable-next-line jsx-a11y/click-events-have-key-events, jsx-a11y/no-static-element-interactions
    <div
      data-testid="rich-text-box"
      id={id}
      data-align={sx?.textAlign}
      onClick={onClick}
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

  describe('anchor target attribute handling', () => {
    it('preserves target="_blank" on anchor tags', () => {
      const component: FlowComponent = {
        id: 'rich-link',
        type: 'RICH_TEXT',
        label: 'Read our <a href="https://example.com/terms" target="_blank">Terms</a>.',
      };
      const {getByTestId} = renderWithProviders(
        <RichTextAdapter component={component} resolve={(s: string | undefined) => s} />,
      );
      const anchor = getByTestId('rich-text-box').querySelector('a');
      expect(anchor).not.toBeNull();
      expect(anchor?.getAttribute('target')).toBe('_blank');
    });

    it('forces rel="noopener noreferrer" on target="_blank" anchors', () => {
      const component: FlowComponent = {
        id: 'rich-link-no-rel',
        type: 'RICH_TEXT',
        label: 'Read our <a href="https://example.com/terms" target="_blank">Terms</a>.',
      };
      const {getByTestId} = renderWithProviders(
        <RichTextAdapter component={component} resolve={(s: string | undefined) => s} />,
      );
      const anchor = getByTestId('rich-text-box').querySelector('a');
      expect(anchor?.getAttribute('rel')).toBe('noopener noreferrer');
    });

    it('overrides author-supplied rel on target="_blank" anchors', () => {
      const component: FlowComponent = {
        id: 'rich-link-bad-rel',
        type: 'RICH_TEXT',
        label: '<a href="https://example.com" target="_blank" rel="opener">Link</a>',
      };
      const {getByTestId} = renderWithProviders(
        <RichTextAdapter component={component} resolve={(s: string | undefined) => s} />,
      );
      const anchor = getByTestId('rich-text-box').querySelector('a');
      expect(anchor?.getAttribute('rel')).toBe('noopener noreferrer');
    });
  });

  describe('sign-up URL handling', () => {
    const signUpLabel =
      '<p data-component-ref="self-sign-up-link">Don\'t have an account? <a href="#" data-action-ref="action_signup">Sign up</a></p>';
    const signUpComponent: FlowComponent = {
      id: 'signup-richtext',
      type: 'RICH_TEXT',
      label: signUpLabel,
      action: {ref: 'action_signup'},
    };

    it('returns null when registration is disabled', () => {
      const resolve = (template: string | undefined) =>
        template?.includes('isRegistrationFlowEnabled') ? 'false' : template;

      const {queryByTestId} = renderWithProviders(<RichTextAdapter component={signUpComponent} resolve={resolve} />);
      expect(queryByTestId('rich-text-box')).not.toBeInTheDocument();
    });

    it('renders the sign-up link when registration is enabled and the server resolves the URL', () => {
      const resolve = (template: string | undefined) => {
        if (template?.includes('isRegistrationFlowEnabled')) return 'true';
        return template;
      };

      const {getByTestId} = renderWithProviders(<RichTextAdapter component={signUpComponent} resolve={resolve} />);
      const box = getByTestId('rich-text-box');
      expect(box).toBeInTheDocument();
      expect(box.innerHTML).toContain('Sign up');
    });
  });

  describe('recovery URL handling', () => {
    const recoveryLabel =
      '<p data-component-ref="recovery-link"><a href="#" data-action-ref="action_forgot_password">Forgot password?</a></p>';
    const recoveryComponent: FlowComponent = {
      id: 'recovery-richtext',
      type: 'RICH_TEXT',
      label: recoveryLabel,
      action: {ref: 'action_forgot_password'},
    };
    const resolveRecovery = (enabled: boolean) => (template: string | undefined) =>
      template?.includes('isRecoveryFlowEnabled') ? String(enabled) : template;

    it('returns null when the recovery flow is disabled', () => {
      const {queryByTestId} = renderWithProviders(
        <RichTextAdapter component={recoveryComponent} resolve={resolveRecovery(false)} />,
      );
      expect(queryByTestId('rich-text-box')).not.toBeInTheDocument();
    });

    it('renders the recovery link when the recovery flow is enabled', () => {
      const {getByTestId} = renderWithProviders(
        <RichTextAdapter component={recoveryComponent} resolve={resolveRecovery(true)} />,
      );
      expect(getByTestId('rich-text-box').innerHTML).toContain('Forgot password?');
    });

    it('dispatches the synthesized action with the supplied values on click', () => {
      const onSubmit = vi.fn();
      const {getByTestId} = renderWithProviders(
        <RichTextAdapter
          component={recoveryComponent}
          resolve={resolveRecovery(true)}
          values={{username: 'alice'}}
          onSubmit={onSubmit}
        />,
      );

      fireEvent.click(getByTestId('rich-text-box').querySelector('a')!);

      expect(onSubmit).toHaveBeenCalledWith(
        expect.objectContaining({id: 'action_forgot_password', ref: 'action_forgot_password'}),
        {username: 'alice'},
      );
    });

    it('does not throw when clicked without an onSubmit handler', () => {
      const {getByTestId} = renderWithProviders(
        <RichTextAdapter component={recoveryComponent} resolve={resolveRecovery(true)} />,
      );

      expect(() => fireEvent.click(getByTestId('rich-text-box').querySelector('a')!)).not.toThrow();
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
