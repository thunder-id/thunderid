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

import {BlogPostProvider, useBlogPost} from '@docusaurus/plugin-content-blog/client';
import {HtmlClassNameProvider, ThemeClassNames} from '@docusaurus/theme-common';
import type {Props} from '@theme/BlogPostPage';
import BlogPostPageMetadata from '@theme/BlogPostPage/Metadata';
import BlogPostPageStructuredData from '@theme/BlogPostPage/StructuredData';
import ContentVisibility from '@theme/ContentVisibility';
import Layout from '@theme/Layout';
import {Box} from '@wso2/oxygen-ui';
import clsx from 'clsx';
import type {ReactNode} from 'react';
import BlogPostFooterNav from '@site/src/components/Blog/BlogPostFooterNav';
import BlogPostHero from '@site/src/components/Blog/BlogPostHero';
import BlogPostProse from '@site/src/components/Blog/BlogPostProse';
import BlogPostSidebar from '@site/src/components/Blog/BlogPostSidebar';

function BlogPostPageContent({sidebar, children}: {sidebar: Props['sidebar']; children: ReactNode}): ReactNode {
  const content = useBlogPost();
  const {metadata} = content;

  return (
    <Layout>
      <ContentVisibility metadata={metadata} />
      <BlogPostHero content={content} />
      <Box
        sx={{
          maxWidth: 1200,
          width: '100%',
          mx: 'auto',
          px: {xs: 2, sm: 4},
          pt: {xs: 5, md: 7},
          pb: {xs: 6, md: 9},
          display: 'grid',
          gridTemplateColumns: {xs: '1fr', md: 'minmax(0, 1fr) 280px'},
          gap: {xs: 5, md: 8},
          alignItems: 'start',
        }}
      >
        <Box component="article">
          <BlogPostProse>{children}</BlogPostProse>
          <BlogPostFooterNav content={content} />
        </Box>
        <BlogPostSidebar content={content} sidebar={sidebar} />
      </Box>
    </Layout>
  );
}

export default function BlogPostPage(props: Props): ReactNode {
  const BlogPostContent = props.content;
  return (
    <BlogPostProvider content={props.content} isBlogPostPage>
      <HtmlClassNameProvider
        className={clsx(
          ThemeClassNames.wrapper.blogPages,
          ThemeClassNames.page.blogPostPage,
        )}>
        <BlogPostPageMetadata />
        <BlogPostPageStructuredData />
        <BlogPostPageContent sidebar={props.sidebar}>
          <BlogPostContent />
        </BlogPostPageContent>
      </HtmlClassNameProvider>
    </BlogPostProvider>
  );
}
