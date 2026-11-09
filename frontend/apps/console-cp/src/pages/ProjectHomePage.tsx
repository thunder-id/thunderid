// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle} from '@thunderid/components';
import {GatewaysOverview} from '@thunderid/configure-gateways';
import {useProjectSelection} from '@thunderid/configure-projects';
import {PageContent} from '@wso2/oxygen-ui';
import type {JSX} from 'react';

/**
 * A project's home: the gateways its configuration is deployed and promoted through, with every
 * deploy and promotion from here scoped to the project.
 */
export default function ProjectHomePage(): JSX.Element {
  const {selectedProject: project} = useProjectSelection();
  // Where this project's pipeline is edited outside the console: its address names the project by its handle.
  const editPipelineUrl = project
    ? window.__THUNDERID_RUNTIME_CONFIG__?.deploy?.edit_pipeline_url?.replace(
        '{project}',
        encodeURIComponent(project.handle),
      )
    : undefined;
  return (
    <PageContent>
      <DocumentTitle title={project?.displayName} />
      <GatewaysOverview eyebrow={project?.displayName} editPipelineUrl={editPipelineUrl} />
    </PageContent>
  );
}
