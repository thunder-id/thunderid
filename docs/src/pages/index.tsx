// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Head from '@docusaurus/Head';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import Layout from '@theme/Layout';
import type {ReactNode} from 'react';
import CommunitySection from '@site/src/components/HomePage/CommunitySection';
import DevelopersSection from '@site/src/components/HomePage/DevelopersSection';
import EventBanner from '@site/src/components/HomePage/EventBanner';
import FeaturesSection from '@site/src/components/HomePage/FeaturesSection';
import FooterSection from '@site/src/components/HomePage/FooterSection';
import HeroSection from '@site/src/components/HomePage/HeroSection';
import OpenFoundationsSection from '@site/src/components/HomePage/OpenFoundationsSection';
import ProductOverviewSection from '@site/src/components/HomePage/ProductOverviewSection';
import UseCasesSection from '@site/src/components/HomePage/UseCasesSection';

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
        <OpenFoundationsSection />
        <UseCasesSection />
        <DevelopersSection />
        <FeaturesSection />
        <CommunitySection />
        <FooterSection />
      </div>
    </Layout>
  );
}
