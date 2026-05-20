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

import {withVendorCSSClassPrefix} from '@thunderid/browser';
import {type Component, type SetupContext, type VNode, defineComponent, h, type PropType} from 'vue';

type TypographyProps = Readonly<{
  component: string | undefined;
  variant:
    | 'h1'
    | 'h2'
    | 'h3'
    | 'h4'
    | 'h5'
    | 'h6'
    | 'subtitle1'
    | 'subtitle2'
    | 'body1'
    | 'body2'
    | 'caption'
    | 'overline';
}>;

const Typography: Component = defineComponent({
  name: 'Typography',
  props: {
    component: {
      default: undefined,
      type: String as PropType<string>,
    },
    variant: {
      default: 'body1',
      type: String as PropType<
        'h1' | 'h2' | 'h3' | 'h4' | 'h5' | 'h6' | 'subtitle1' | 'subtitle2' | 'body1' | 'body2' | 'caption' | 'overline'
      >,
    },
  },
  setup(props: TypographyProps, {slots, attrs}: SetupContext): () => VNode {
    return (): VNode => {
      const tagMap: Record<string, string> = {
        body1: 'p',
        body2: 'p',
        caption: 'span',
        h1: 'h1',
        h2: 'h2',
        h3: 'h3',
        h4: 'h4',
        h5: 'h5',
        h6: 'h6',
        overline: 'span',
        subtitle1: 'h6',
        subtitle2: 'h6',
      };

      const tag: string = props.component || tagMap[props.variant] || 'p';

      return h(
        tag,
        {
          class: [
            withVendorCSSClassPrefix('typography'),
            withVendorCSSClassPrefix(`typography--${props.variant}`),
            (attrs.class as string) || '',
          ]
            .filter(Boolean)
            .join(' '),
          style: attrs.style,
        },
        slots['default']?.(),
      );
    };
  },
});

export default Typography;
