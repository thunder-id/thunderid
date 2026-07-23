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

import Link from '@docusaurus/Link';
import {useBaseUrlUtils} from '@docusaurus/useBaseUrl';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import ThemedImage from '@theme/ThemedImage';
import {Box, Container, Typography} from '@wso2/oxygen-ui';
import {JSX} from 'react';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';

interface FooterColumnProps {
  title: string;
  links: {label: string; href: string}[];
}

function FooterColumn({title, links}: FooterColumnProps) {
  return (
    <Box>
      <Typography
        variant="body2"
        sx={{
          fontWeight: 600,
          mb: 2,
          fontSize: '0.85rem',
          color: 'text.primary',
        }}
      >
        {title}
      </Typography>
      {links.map((link) => (
        <Typography
          key={link.label}
          component={Link}
          href={link.href}
          variant="body2"
          sx={{
            display: 'block',
            mb: 1.5,
            fontSize: '0.8rem',
            color: 'text.secondary',
            textDecoration: 'none',
            '&:hover': {
              color: 'text.primary',
              textDecoration: 'none',
            },
          }}
        >
          {link.label}
        </Typography>
      ))}
    </Box>
  );
}

export default function Footer(): JSX.Element {
  const {withBaseUrl} = useBaseUrlUtils();
  const {siteConfig} = useDocusaurusContext();
  const productConfig = siteConfig.customFields?.product as DocusaurusProductConfig;

  return (
    <Box
      sx={{
        bgcolor: 'background.default',
        color: 'text.primary',
        borderTop: '1px solid',
        borderColor: 'divider',
        pt: {xs: 4, lg: 5},
        pb: 3,
      }}
    >
      <Container maxWidth="lg" sx={{px: {xs: 2, sm: 4}}}>
        <Box
          sx={{
            display: 'grid',
            gridTemplateColumns: {xs: '1fr', sm: 'repeat(2, 1fr)', md: '2fr 1fr 1fr 1fr'},
            gap: {xs: 4, md: 5},
            mb: 4,
          }}
        >
          {/* Brand column */}
          <Box>
            <Box sx={{mb: 3}}>
              <ThemedImage
                sources={{
                  light: withBaseUrl('/assets/images/logo.svg'),
                  dark: withBaseUrl('/assets/images/logo-inverted.svg'),
                }}
                alt={`${productConfig.project.name} Logo`}
                style={{height: 48}}
              />
            </Box>
          </Box>

          {/* Docs + SDKs column */}
          <FooterColumn
            title="Product"
            links={[
              {label: 'Docs', href: '/docs/next/getting-started/get-thunderid'},
              {label: 'APIs', href: '/docs/next/apis'},
              {label: 'SDKs', href: '/sdks'},
            ]}
          />

          {/* Community column */}
          <FooterColumn
            title="Community"
            links={[
              {label: 'Contributing', href: '/docs/next/community/contributing/contribute-ideas'},
              {label: 'Events', href: '/events'},
              {label: 'Discussions', href: productConfig.project.source.github.discussionsUrl},
              {label: 'Report an Issue', href: productConfig.project.source.github.issuesUrl},
            ]}
          />

          {/* Resources column */}
          <FooterColumn
            title="Resources"
            links={[
              {label: 'Releases', href: productConfig.project.source.github.releasesUrl},
              {label: 'GitHub', href: productConfig.project.source.github.url},
              {label: 'Brand Guidelines', href: '/brand'},
            ]}
          />
        </Box>

        {/* Copyright */}
        <Box
          sx={{
            borderTop: '1px solid',
            borderColor: 'divider',
            pt: 3,
            display: 'flex',
            flexWrap: 'wrap',
            justifyContent: 'center',
            alignItems: 'center',
            gap: 2,
          }}
        >
          <Typography
            variant="caption"
            sx={{
              color: 'text.disabled',
              fontSize: '0.75rem',
            }}
          >
            &copy; Copyright Linux Foundation Europe.
          </Typography>
          <Typography
            variant="caption"
            sx={{
              color: 'text.disabled',
              fontSize: '0.75rem',
            }}
          >
            For web site terms of use, trademark policy and other project policies please see {''}
            <Link
              href="https://linuxfoundation.eu/en/policies"
              target="_blank"
              rel="noopener noreferrer"
              underline="hover"
            >
              https://linuxfoundation.eu/en/policies.
            </Link>
          </Typography>
        </Box>
      </Container>
    </Box>
  );
}
