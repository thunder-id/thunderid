import React from 'react';
import Link from '@docusaurus/Link';

interface JourneyCardProps {
  step: number;
  title: string;
  description: string;
  href: string;
  linkText: string;
}

interface JourneyCardsProps {
  children: React.ReactNode;
}

export function JourneyCard({step, title, description, href, linkText}: JourneyCardProps) {
  return (
    <Link
      to={href}
      style={{textDecoration: 'none', color: 'inherit', display: 'block'}}
    >
      <div style={{border: '1px solid var(--ifm-color-emphasis-300)', borderRadius: '10px', padding: '1.25rem', cursor: 'pointer', height: '100%'}}>
        <div style={{fontSize: '1rem', fontWeight: 700, marginBottom: '0.4rem'}}>Step {step} → {title}</div>
        <div style={{fontSize: '0.875rem', color: 'var(--ifm-color-emphasis-700)', marginBottom: '0.75rem'}}>{description}</div>
        <div style={{fontSize: '0.875rem', color: 'var(--ifm-color-primary)', fontWeight: 600}}>{linkText} →</div>
      </div>
    </Link>
  );
}

export function JourneyCards({children}: JourneyCardsProps) {
  return (
    <div style={{display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))', gap: '1rem', marginTop: '1.5rem'}}>
      {children}
    </div>
  );
}
