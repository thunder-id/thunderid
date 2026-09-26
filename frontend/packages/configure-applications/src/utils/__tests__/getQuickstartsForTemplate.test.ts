// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it, vi} from 'vitest';
import getQuickstartsForTemplate from '../getQuickstartsForTemplate';

vi.mock('../../config/TechnologyBasedApplicationTemplateMetadata', () => ({
  default: [
    {
      template: {
        id: 'react',
        quickstarts: [
          {
            label: 'React',
            docsUrl: 'https://thunderid.dev/docs/next/guides/getting-started/connect-your-application/react/',
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
        id: 'mobile',
        quickstarts: [
          {
            label: 'iOS',
            docsUrl: 'https://thunderid.dev/docs/next/guides/getting-started/connect-your-application/ios/',
          },
          {
            label: 'Android',
            docsUrl: 'https://thunderid.dev/docs/next/guides/getting-started/connect-your-application/android/',
          },
          {
            label: 'Flutter',
            docsUrl: 'https://thunderid.dev/docs/next/guides/getting-started/connect-your-application/flutter/',
          },
        ],
      },
    },
  ],
}));

describe('getQuickstartsForTemplate', () => {
  it('returns null when no template ID is provided', () => {
    expect(getQuickstartsForTemplate(undefined)).toBeNull();
  });

  it('returns the quickstart guides for a technology-based template', () => {
    expect(getQuickstartsForTemplate('react')).toEqual([
      {
        label: 'React',
        docsUrl: 'https://thunderid.dev/docs/next/guides/getting-started/connect-your-application/react/',
      },
    ]);
  });

  it('resolves embedded template variants to their base template', () => {
    expect(getQuickstartsForTemplate('react-embedded')).toEqual([
      {
        label: 'React',
        docsUrl: 'https://thunderid.dev/docs/next/guides/getting-started/connect-your-application/react/',
      },
    ]);
  });

  it('returns one quickstart guide per platform for a multi-platform template', () => {
    const quickstarts = getQuickstartsForTemplate('mobile');

    expect(quickstarts).toHaveLength(3);
    expect(quickstarts?.map((q) => q.label)).toEqual(['iOS', 'Android', 'Flutter']);
  });

  it('returns null when the template has no quickstart configured', () => {
    expect(getQuickstartsForTemplate('mcp-client')).toBeNull();
    expect(getQuickstartsForTemplate('browser')).toBeNull();
  });

  it('returns null for an unknown template ID', () => {
    expect(getQuickstartsForTemplate('unknown-template')).toBeNull();
  });
});
