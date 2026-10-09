// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import {screen, cleanup} from '@testing-library/react';
import {describe, it, expect, afterEach} from 'vitest';
import type {FlowComponent} from '../../../../models/flow';
import renderWithProviders from '../../../../test/renderWithProviders';
import KeyValueListAdapter from '../KeyValueListAdapter';

afterEach(cleanup);

const component: FlowComponent = {
  id: 'details-1',
  type: 'KEY_VALUE_LIST',
  source: 'linkingPromptDetails',
};

const pairs = [
  {label: 'Email', value: 'alexa.perera@acme.io'},
  {label: 'Username', value: 'alexa.perera'},
];

describe('KeyValueListAdapter', () => {
  // Additional data carries strings, so an executor publishes the pairs JSON-encoded.
  it('renders the pairs the source key holds as an encoded array', () => {
    renderWithProviders(
      <KeyValueListAdapter
        component={component}
        resolve={(s) => s}
        additionalData={{linkingPromptDetails: JSON.stringify(pairs)}}
      />,
    );

    expect(screen.getByText('Email')).toBeTruthy();
    expect(screen.getByText('alexa.perera@acme.io')).toBeTruthy();
    expect(screen.getByText('Username')).toBeTruthy();
    expect(screen.getByText('alexa.perera')).toBeTruthy();
  });

  it('renders the pairs the source key holds as an array', () => {
    renderWithProviders(
      <KeyValueListAdapter component={component} resolve={(s) => s} additionalData={{linkingPromptDetails: pairs}} />,
    );

    expect(screen.getByText('alexa.perera@acme.io')).toBeTruthy();
  });

  it('renders an optional label above the pairs', () => {
    renderWithProviders(
      <KeyValueListAdapter
        component={{...component, label: 'Matched account'}}
        resolve={(s) => s}
        additionalData={{linkingPromptDetails: pairs}}
      />,
    );

    expect(screen.getByText('Matched account')).toBeTruthy();
  });

  // A pair with no value is a blank row that tells the End-User nothing.
  it('drops pairs carrying no value', () => {
    renderWithProviders(
      <KeyValueListAdapter
        component={component}
        resolve={(s) => s}
        additionalData={{linkingPromptDetails: [...pairs, {label: 'Organization', value: ''}]}}
      />,
    );

    expect(screen.queryByText('Organization')).toBeNull();
  });

  it('drops pairs whose value or label is not a string', () => {
    renderWithProviders(
      <KeyValueListAdapter
        component={component}
        resolve={(s) => s}
        additionalData={{
          linkingPromptDetails: [
            ...pairs,
            {label: 'Organization', value: {name: 'Acme'}},
            {label: 'Age', value: 30},
            {label: {text: 'Phone'}, value: '+94 77 123 4567'},
          ],
        }}
      />,
    );

    expect(screen.getByText('alexa.perera@acme.io')).toBeTruthy();
    expect(screen.queryByText('Organization')).toBeNull();
    expect(screen.queryByText('Age')).toBeNull();
    expect(screen.queryByText('+94 77 123 4567')).toBeNull();
  });

  it.each([
    ['the source key is absent', {}],
    ['there is no additional data at all', undefined],
    ['the source holds no valid JSON', {linkingPromptDetails: 'not json'}],
    ['the source holds something that is not a list', {linkingPromptDetails: '{"label":"Email"}'}],
    ['the list is empty', {linkingPromptDetails: '[]'}],
  ])('renders nothing when %s', (_case, additionalData) => {
    const {container} = renderWithProviders(
      <KeyValueListAdapter component={component} resolve={(s) => s} additionalData={additionalData} />,
    );

    expect(container.textContent).toBe('');
  });

  it('renders nothing when the component binds no source', () => {
    const {container} = renderWithProviders(
      <KeyValueListAdapter
        component={{id: 'details-1', type: 'KEY_VALUE_LIST'}}
        resolve={(s) => s}
        additionalData={{linkingPromptDetails: pairs}}
      />,
    );

    expect(container.textContent).toBe('');
  });
});
