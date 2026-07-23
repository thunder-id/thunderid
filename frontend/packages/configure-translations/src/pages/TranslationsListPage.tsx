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

import {useLogger} from '@thunderid/logger/react';
import {Button, PageContent, PageTitle} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import {useCallback, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import TranslationsList from '@/components/TranslationsList';
import useTranslationRoutes from '@/hooks/useTranslationRoutes';

/**
 * Page that lists all configured translation languages in a data grid.
 *
 * Displays each language with its flag emoji, display name, and BCP 47 code.
 * Provides an "Add Language" action that navigates to the creation wizard, and
 * a per-row actions menu with an "Edit" option that navigates to the edit page
 * for that language.
 *
 * @returns JSX element rendering the translations list page
 *
 * @example
 * ```tsx
 * // Rendered automatically by the router at /translations
 * import TranslationsListPage from './TranslationsListPage';
 *
 * function App() {
 *   return <TranslationsListPage />;
 * }
 * ```
 *
 * @public
 */
export default function TranslationsListPage(): JSX.Element {
  const {t} = useTranslation('translations');
  const navigate = useNavigate();
  const logger = useLogger('TranslationsListPage');
  const routes = useTranslationRoutes();

  const handleAddLanguage = useCallback(() => {
    (async (): Promise<void> => {
      await navigate(routes.create());
    })().catch((_error: unknown) => {
      logger.error('Failed to navigate to translation create page', {error: _error});
    });
  }, [navigate, logger, routes]);

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>{t('page.title')}</PageTitle.Header>
        <PageTitle.SubHeader>{t('page.subtitle')}</PageTitle.SubHeader>
        <PageTitle.Actions>
          <Button variant="contained" startIcon={<Plus size={18} />} onClick={handleAddLanguage}>
            {t('listing.addLanguage')}
          </Button>
        </PageTitle.Actions>
      </PageTitle>

      <TranslationsList />
    </PageContent>
  );
}
