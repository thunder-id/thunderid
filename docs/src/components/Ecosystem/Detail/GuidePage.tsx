// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Layout from '@theme/Layout';
import {Box} from '@wso2/oxygen-ui';
import {JSX} from 'react';
import Shell from './Shell';
import type {EcosystemEntry} from '@site/src/types/ecosystem';

/**
 * One guide, on its own page, in the same shell as the entry it belongs to.
 *
 * A guide is built from the same sections an entry is, so the reader sees the
 * same header, the same step and code treatment, and the same rail on both.
 * Moving between them should feel like scrolling, not like changing site.
 */
export default function GuidePage({
  entry,
  route = undefined,
}: {
  entry: EcosystemEntry;
  route?: {customData?: {guideId?: string}};
}): JSX.Element | null {
  const guideId = route?.customData?.guideId;
  const guide = entry.guides?.find((candidate) => candidate.id === guideId);

  if (!guide) return null;

  return (
    <Layout title={`${guide.title} · ${entry.name}`} description={guide.description ?? entry.description}>
      <Box sx={{display: 'flex', flexDirection: 'column', minHeight: '100%'}}>
        <Shell
          entry={entry}
          sections={guide.sections}
          trail={[{label: guide.title}]}
          lead={{title: guide.title, intro: guide.intro, minutes: guide.minutes}}
        />
      </Box>
    </Layout>
  );
}
