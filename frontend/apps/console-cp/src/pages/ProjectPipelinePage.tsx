// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle} from '@thunderid/components';
import {GatewaysOverview} from '@thunderid/configure-gateways';
import {useProjectSelection} from '@thunderid/configure-projects';
import {PageContent} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import AutoDeploySwitch from '../components/AutoDeploySwitch';

/**
 * A project's deployment pipeline: its versions, and the gateways they are deployed to and promoted
 * through, with every deploy and promotion scoped to the project.
 */
export default function ProjectPipelinePage(): JSX.Element {
  const {t} = useTranslation();
  const {selectedProject: project} = useProjectSelection();
  // Where this project's pipeline is edited outside the console: its address names the project by its handle.
  const editPipelineUrl = project
    ? window.__THUNDERID_RUNTIME_CONFIG__?.deploy?.edit_pipeline_url?.replace(
        '{project}',
        encodeURIComponent(project.handle),
      )
    : undefined;
  const title = t('home:project.pipelineTitle', 'Project deployment pipeline');
  return (
    <PageContent>
      <DocumentTitle title={title} />
      <GatewaysOverview
        editPipelineUrl={editPipelineUrl}
        pipelineTitle={title}
        showIntro={false}
        pipelineActions={<AutoDeploySwitch />}
      />
    </PageContent>
  );
}
