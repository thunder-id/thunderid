// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Head from '@docusaurus/Head';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import type {ReactNode} from 'react';
import CommunitySection from '@site/src/components/HomePage/CommunitySection';
import EventBanner from '@site/src/components/HomePage/EventBanner';
import FooterSection from '@site/src/components/HomePage/FooterSection';
import HeroSection from '@site/src/components/HomePage/HeroSection';
import ProductOverviewSection from '@site/src/components/HomePage/ProductOverviewSection';
import SDKShowcaseSection from '@site/src/components/HomePage/SDKShowcaseSection';
import WorkflowSection from '@site/src/components/HomePage/WorkflowSection';

export default function Homepage(): ReactNode {
  const {siteConfig} = useDocusaurusContext();

  return (
    <Layout title={siteConfig.tagline}>
      <Head>
        <link rel="prefetch" href="/assets/css/elements.min.css" />
      </Head>
      <div>
        <EventBanner />
        <HeroSection />
        <ProductOverviewSection />
        <SDKShowcaseSection />
        <WorkflowSection />
        <CommunitySection />
        <FooterSection />
      </div>
    </Layout>
  );
}
