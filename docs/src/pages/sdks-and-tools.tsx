// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Layout from '@theme/Layout';
import {JSX} from 'react';
import EcosystemPage from '@site/src/components/Ecosystem';

export default function SdksPage(): JSX.Element {
  return (
    <Layout
      description="Official SDKs, framework integrations, and agent tooling for seamless authentication."
      title="SDKs & Tools"
    >
      <EcosystemPage />
    </Layout>
  );
}
