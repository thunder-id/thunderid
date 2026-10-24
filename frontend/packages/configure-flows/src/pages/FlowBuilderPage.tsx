// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle} from '@thunderid/components';
import {ReactFlowProvider} from '@xyflow/react';
import {useTranslation} from 'react-i18next';
import FlowBuilder from '../components/FlowBuilder';
import FlowBuilderProvider from '../context/FlowBuilderProvider';

export default function FlowBuilderPage() {
  const {t} = useTranslation();

  return (
    <FlowBuilderProvider>
      <DocumentTitle title={t('navigation:pages.flows', 'Flows')} />
      <ReactFlowProvider>
        <FlowBuilder />
      </ReactFlowProvider>
    </FlowBuilderProvider>
  );
}
