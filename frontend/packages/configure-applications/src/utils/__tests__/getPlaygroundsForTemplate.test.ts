// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it, vi} from 'vitest';
import getPlaygroundsForTemplate from '../getPlaygroundsForTemplate';

vi.mock('../../config/TechnologyBasedApplicationTemplateMetadata', () => ({
  default: [
    {
      template: {
        id: 'react',
        playgrounds: [
          {
            label: 'React',
            environment: 'stackblitz',
            url: 'https://stackblitz.com/fork/github/thunder-id/javascript-sdks/tree/main/samples/react/quickstart?file=README.md',
          },
        ],
      },
    },
    {
      template: {
        id: 'mcp-client',
      },
    },
  ],
}));

vi.mock('../../config/PlatformBasedApplicationTemplateMetadata', () => ({
  default: [
    {
      template: {
        id: 'browser',
      },
    },
    {
      template: {
        // Native mobile platforms have no runnable environment, so no playgrounds.
        id: 'mobile',
      },
    },
  ],
}));

describe('getPlaygroundsForTemplate', () => {
  it('returns null when no template ID is provided', () => {
    expect(getPlaygroundsForTemplate(undefined)).toBeNull();
  });

  it('returns the playgrounds for a technology-based template', () => {
    expect(getPlaygroundsForTemplate('react')).toEqual([
      {
        label: 'React',
        environment: 'stackblitz',
        url: 'https://stackblitz.com/fork/github/thunder-id/javascript-sdks/tree/main/samples/react/quickstart?file=README.md',
      },
    ]);
  });

  it('resolves embedded template variants to their base template', () => {
    expect(getPlaygroundsForTemplate('react-embedded')).toEqual([
      {
        label: 'React',
        environment: 'stackblitz',
        url: 'https://stackblitz.com/fork/github/thunder-id/javascript-sdks/tree/main/samples/react/quickstart?file=README.md',
      },
    ]);
  });

  it('returns null when the template has no playground configured', () => {
    expect(getPlaygroundsForTemplate('mcp-client')).toBeNull();
    expect(getPlaygroundsForTemplate('browser')).toBeNull();
  });

  it('returns null for a template with no runnable environment (e.g. native mobile)', () => {
    expect(getPlaygroundsForTemplate('mobile')).toBeNull();
  });

  it('returns null for an unknown template ID', () => {
    expect(getPlaygroundsForTemplate('unknown-template')).toBeNull();
  });
});
