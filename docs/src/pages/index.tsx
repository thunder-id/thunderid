/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import Head from '@docusaurus/Head';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import CommunitySection from '@site/src/components/HomePage/CommunitySection';
import EventBanner from '@site/src/components/HomePage/EventBanner';
import FooterSection from '@site/src/components/HomePage/FooterSection';
import HeroSection from '@site/src/components/HomePage/HeroSection';
import ProductOverviewSection from '@site/src/components/HomePage/ProductOverviewSection';
import SDKShowcaseSection from '@site/src/components/HomePage/SDKShowcaseSection';
import WorkflowSection from '@site/src/components/HomePage/WorkflowSection';
import Layout from '@theme/Layout';
import type {ReactNode} from 'react';

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
