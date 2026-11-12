import React, {type ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import useBaseUrl from '@docusaurus/useBaseUrl';
import Translate from '@docusaurus/Translate';
import type {Props} from '@theme/NotFound/Content';
import Heading from '@theme/Heading';
import ThemedImage from '@theme/ThemedImage';
import styles from './styles.module.css';

export default function NotFoundContent({className}: Props): ReactNode {
    return (
        <main className={clsx('container margin-vert--xl', className)}>
            <div className="row">
                <div className={clsx('col col--10 col--offset-1', styles.notFoundWrapper)}>
                    <ThemedImage
                        alt="A small robot mascot holding a 404 flag next to broken gears"
                        className={styles.illustration}
                        sources={{
                            light: useBaseUrl('/img/404-illustration-light.png'),
                            dark: useBaseUrl('/img/404-illustration-dark.png'),
                        }}
                    />
                    <Heading as="h1" className="hero__title">
                        <Translate
                            id="theme.NotFound.title"
                            description="The title of the 404 page">
                            Page Not Found
                        </Translate>
                    </Heading>
                    <p className={styles.subtitle}>
                        <Translate
                            id="theme.NotFound.p1"
                            description="The first paragraph of the 404 page">
                            Even our bolt couldn't track this one down. The page you're looking for isn't here.
                        </Translate>
                    </p>
                    <div className={styles.actions}>
                        <Link
                            className={clsx('button button--outline button--secondary button--lg', styles.actionButton)}
                            to="/">
                            ← Go to Homepage
                        </Link>
                        <Link
                            className={clsx('button button--primary button--lg', styles.actionButton)}
                            to="/docs/next/"
                            style={{color: '#ffffff'}}>
                            Browse Docs →
                        </Link>
                    </div>
                </div>
            </div>
        </main>
    );
}