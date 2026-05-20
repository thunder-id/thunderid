import {useDoc} from '@docusaurus/plugin-content-docs/client';
import Heading from '@theme/Heading';
import MDXContent from '@theme/MDXContent';
import React, {type ReactNode} from 'react';
// @ts-ignore — JS module, no types
import CopyPageButton from 'docusaurus-plugin-copy-page-button/src/CopyPageButton';

function useSyntheticTitle(): string | null {
  const {metadata, frontMatter, contentTitle} = useDoc();
  const shouldRender =
    !frontMatter.hide_title && typeof contentTitle === 'undefined';
  if (!shouldRender) {
    return null;
  }
  return metadata.title;
}

export default function DocItemContent({children}: {children: ReactNode}): ReactNode {
  const syntheticTitle = useSyntheticTitle();
  const {metadata, frontMatter} = useDoc();
  const isHomePage = metadata.id === 'index';
  const showButton = !isHomePage && !frontMatter.hide_title;

  return (
    <div className="theme-doc-markdown markdown doc-content-with-copy-btn">
      {syntheticTitle && (
        <header>
          <Heading as="h1">{syntheticTitle}</Heading>
        </header>
      )}
      {showButton && (
        <div className="copy-page-btn-wrapper">
          <CopyPageButton
            enabledActions={['copy', 'view', 'chatgpt', 'claude', 'gemini']}
          />
        </div>
      )}
      <MDXContent>{children}</MDXContent>
    </div>
  );
}
