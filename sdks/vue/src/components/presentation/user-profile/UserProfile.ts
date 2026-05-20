/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {ThunderIDError, User, withVendorCSSClassPrefix} from '@thunderid/browser';
import {type Component, type PropType, type SetupContext, type VNode, defineComponent, h, ref, type Ref} from 'vue';
import BaseUserProfile from './BaseUserProfile';
import updateMeProfile from '../../../api/updateMeProfile';
import useI18n from '../../../composables/useI18n';
import useThunderID from '../../../composables/useThunderID';
import useUser from '../../../composables/useUser';

type UserProfileProps = Readonly<{
  avatarSize: 'sm' | 'md' | 'lg';
  cardLayout: boolean;
  cardVariant: 'elevated' | 'outlined' | 'flat';
  className: string;
  compact: boolean;
  editable: boolean;
  hideFields: string[];
  showAvatar: boolean;
  showFields: string[];
  title: string;
}>;

const UserProfile: Component = defineComponent({
  name: 'UserProfile',
  props: {
    /** Avatar circle size. */
    avatarSize: {
      default: 'lg',
      type: String as PropType<'sm' | 'md' | 'lg'>,
    },
    /** Whether to render the component inside a Card wrapper. */
    cardLayout: {default: true, type: Boolean},
    /** Shadow / border style of the card wrapper. */
    cardVariant: {
      default: 'elevated',
      type: String as PropType<'elevated' | 'outlined' | 'flat'>,
    },
    /** Extra CSS class added to the root element. */
    className: {default: '', type: String},
    /** Tighter spacing — useful when embedded in a modal or dropdown. */
    compact: {default: false, type: Boolean},
    /** Whether fields can be edited inline. */
    editable: {default: true, type: Boolean},
    /** Fields to hide by name. */
    hideFields: {default: () => [], type: Array as PropType<string[]>},
    /** Whether to render the avatar hero section. */
    showAvatar: {default: true, type: Boolean},
    /** Fields to show exclusively (empty = show all). */
    showFields: {default: () => [], type: Array as PropType<string[]>},
    /** Card header title. */
    title: {default: 'Profile', type: String},
  },
  setup(props: UserProfileProps, {slots}: SetupContext): () => VNode {
    const {baseUrl, instanceId} = useThunderID();
    const {flattenedProfile, profile, schemas, onUpdateProfile} = useUser();
    const {t} = useI18n();

    const error: Ref<string | null> = ref<string | null>(null);

    async function handleProfileUpdate(payload: any): Promise<void> {
      if (!baseUrl) return;

      error.value = null;

      try {
        const response: User = await updateMeProfile({baseUrl, instanceId, payload});
        onUpdateProfile(response);
      } catch (caughtError: unknown) {
        let message: string = t('user.profile.update.generic.error') || 'Failed to update profile. Please try again.';

        if (caughtError instanceof ThunderIDError) {
          message = caughtError.message;
        }

        error.value = message;
      }
    }

    return (): VNode =>
      h(
        BaseUserProfile,
        {
          avatarSize: props.avatarSize,
          cardLayout: props.cardLayout,
          cardVariant: props.cardVariant,
          class: withVendorCSSClassPrefix('user-profile--styled'),
          className: props.className,
          compact: props.compact,
          editable: props.editable,
          error: error.value,
          flattenedProfile: flattenedProfile?.value,
          hideFields: props.hideFields,
          onUpdate: handleProfileUpdate,
          profile: profile?.value?.profile ?? flattenedProfile?.value,
          schemas: schemas?.value,
          showAvatar: props.showAvatar,
          showFields: props.showFields,
          title: props.title,
        },
        slots,
      );
  },
});

export default UserProfile;
