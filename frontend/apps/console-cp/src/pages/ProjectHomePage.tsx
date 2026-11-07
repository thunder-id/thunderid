// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle} from '@thunderid/components';
import {GatewaysOverview, useProjects, type Project} from '@thunderid/configure-gateways';
import {useProjectSelection} from '@thunderid/configure-projects';
import {PageContent} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useParams} from 'react-router';
import RouteProjectScope from '../components/RouteProjectScope';

/**
 * A project's home: the gateways its configuration is deployed and promoted through, with every
 * deploy and promotion from here scoped to the project.
 */
export default function ProjectHomePage(): JSX.Element {
  const {projectId = ''} = useParams<{projectId: string}>();
  const {data: projects} = useProjects();
  const project = projects?.find((entry: Project) => entry.id === projectId);
  // Where this project's pipeline is edited outside the console: its address names the project by its handle.
  const handle = useProjectSelection().projects.find((entry) => entry.id === projectId)?.handle;
  const editPipelineUrl = handle
    ? window.__THUNDERID_RUNTIME_CONFIG__?.deploy?.edit_pipeline_url?.replace('{project}', encodeURIComponent(handle))
    : undefined;
  return (
    <PageContent>
      <DocumentTitle title={project?.displayName} />
      <RouteProjectScope>
        <GatewaysOverview eyebrow={project?.displayName} editPipelineUrl={editPipelineUrl} />
      </RouteProjectScope>
    </PageContent>
  );
}
