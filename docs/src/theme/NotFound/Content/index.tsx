import React, {type ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useBaseUrl from '@docusaurus/useBaseUrl';
import Translate from '@docusaurus/Translate';
import type {Props} from '@theme/NotFound/Content';
import Heading from '@theme/Heading';

export default function NotFoundContent({className}: Props): ReactNode {
  const illustrationUrl = useBaseUrl('/img/404-illustration.png');

  return (
      <main className={clsx('container margin-vert--xl', className)}>
        <div className="row">
          <div className="col col--10 col--offset-1" style={{textAlign: 'center'}}>
            <img
                src={illustrationUrl}
                alt="A small robot mascot holding a 404 flag next to broken gears"
                style={{
                  maxWidth: '600px',
                  width: '100%',
                  marginBottom: '1.5rem',
                }}
            />
            <Heading as="h1" className="hero__title">
              <Translate
                  id="theme.NotFound.title"
                  description="The title of the 404 page">
                Page Not Found
              </Translate>
            </Heading>
            <p style={{fontSize: '1.1rem', opacity: 0.85, maxWidth: '500px', margin: '0 auto 2rem'}}>
              <Translate
                  id="theme.NotFound.p1"
                  description="The first paragraph of the 404 page">
                We could not find what you were looking for...
              </Translate>
            </p>
            <div
                style={{
                  display: 'flex',
                  justifyContent: 'center',
                  gap: '1rem',
                  flexWrap: 'wrap',
                }}>
              <Link
                  className="button button--outline button--secondary button--lg"
                  to="/">
                ← Go to Homepage
              </Link>
              <Link
                  className="button button--primary button--lg"
                  to="/docs/next/">
                Browse Docs →
              </Link>
            </div>
          </div>
        </div>
      </main>
  );
}