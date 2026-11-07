// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle} from '@thunderid/components';
import {GatewaysOverview, useProjects, type Project} from '@thunderid/configure-gateways';
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
  return (
    <PageContent>
      <DocumentTitle title={project?.displayName} />
      <RouteProjectScope>
        <GatewaysOverview eyebrow={project?.displayName} />
      </RouteProjectScope>
    </PageContent>
  );
}
