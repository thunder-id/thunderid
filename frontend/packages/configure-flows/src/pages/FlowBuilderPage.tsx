// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ReactFlowProvider} from '@xyflow/react';
import FlowBuilder from '../components/FlowBuilder';
import FlowBuilderProvider from '../context/FlowBuilderProvider';

export default function FlowBuilderPage() {
  return (
    <FlowBuilderProvider>
      <ReactFlowProvider>
        <FlowBuilder />
      </ReactFlowProvider>
    </FlowBuilderProvider>
  );
}
