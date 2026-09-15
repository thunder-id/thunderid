// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import Link from '@docusaurus/Link';
import {useBaseUrlUtils} from '@docusaurus/useBaseUrl';
import useDocusaurusContext from '@docusaurus/useDocusaurusContext';
import ThemedImage from '@theme/ThemedImage';
import {Box, Container, Typography} from '@wso2/oxygen-ui';
import {JSX} from 'react';
import type {DocusaurusProductConfig} from '@site/docusaurus.product.config';
import {useDocsUrl} from '@site/src/hooks/useDocsUrl';

interface FooterColumnProps {
  title: string;
  links: {label: string; href: string}[];
}

function FooterColumn({title, links}: FooterColumnProps) {
  const docsUrl = useDocsUrl();
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
          href={docsUrl(link.href)}
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
              {label: 'Contributing', href: '/community/contributing/propose-a-feature'},
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
