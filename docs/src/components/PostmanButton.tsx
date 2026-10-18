// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import PostmanLogo from './icons/PostmanLogo';

export interface PostmanButtonProps {
  /** URL to download the Postman collection JSON. */
  collectionUrl: string;
  /** The name of the Postman collection file to be downloaded. */
  downloadFileName: string;
  /**
   * Icon-only, for docking inline among other small toolbar buttons where there's no room
   * for the full label (e.g. the API reference's own "Developer Tools" row). Accessible name
   * moves to `title`/`aria-label` instead of visible text.
   */
  compact?: boolean;
}

const hoverOn = (el: HTMLElement) => {
  el.style.color = 'var(--scalar-color-1, var(--ifm-font-color-base))';
  el.style.backgroundColor = 'var(--scalar-background-2, rgba(128,128,128,0.1))';
};

const hoverOff = (el: HTMLElement) => {
  el.style.color = 'var(--scalar-color-2, var(--ifm-font-color-secondary))';
  el.style.backgroundColor = 'transparent';
};

export default function PostmanButton({collectionUrl, downloadFileName, compact = false}: PostmanButtonProps) {
  if (compact) {
    return (
      <a
        href={collectionUrl}
        download={downloadFileName}
        title="Download Postman Collection"
        aria-label="Download Postman Collection"
        style={{
          display: 'inline-flex',
          alignItems: 'center',
          justifyContent: 'center',
          width: '32px',
          height: '32px',
          borderRadius: '4px',
          color: 'var(--scalar-color-2, var(--ifm-font-color-secondary))',
          textDecoration: 'none',
          transition: 'background-color 0.15s, color 0.15s',
          cursor: 'pointer',
        }}
        onMouseEnter={(e) => hoverOn(e.currentTarget)}
        onMouseLeave={(e) => hoverOff(e.currentTarget)}
      >
        <PostmanLogo size={16} />
      </a>
    );
  }

  return (
    <a
      href={collectionUrl}
      download={downloadFileName}
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: '6px',
        padding: '7px 9px',
        borderRadius: '4px',
        fontSize: '0.875rem',
        lineHeight: 1,
        fontWeight: 400,
        color: 'var(--scalar-color-2, var(--ifm-font-color-secondary))',
        textDecoration: 'none',
        whiteSpace: 'nowrap',
        transition: 'background-color 0.15s, color 0.15s',
        cursor: 'pointer',
      }}
      onMouseEnter={(e) => hoverOn(e.currentTarget)}
      onMouseLeave={(e) => hoverOff(e.currentTarget)}
    >
      <PostmanLogo size={14} />

      Postman Collection

      {/* Download icon */}
      <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true" style={{flexShrink: 0}}>
        <path d="M21 15v4a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2v-4" />
        <polyline points="7 10 12 15 17 10" />
        <line x1="12" y1="15" x2="12" y2="3" />
      </svg>
    </a>
  );
}
