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

export default function TypeScriptLogo({size = 64}: {size?: number}) {
  return (
    <svg width={size} height={size} viewBox="0 0 512 512" fill="none">
      <rect width="512" height="512" rx="64" fill="#3178C6" />
      <path
        fill="#fff"
        d="M317 407v50c8.1 4.2 17.7 7.3 28.9 9.4 11.2 2.1 23 3.1 35.4 3.1 12.1 0 23.6-1.2 34.5-3.5 10.9-2.3 20.5-6.2 28.7-11.5 8.2-5.4 14.7-12.4 19.5-21 4.8-8.7 7.2-19.4 7.2-32.2 0-9.3-1.4-17.4-4.2-24.4a56.7 56.7 0 0 0-12-18.5c-5.2-5.3-11.5-10.1-18.8-14.4-7.3-4.3-15.5-8.3-24.6-12.1-6.7-2.8-12.7-5.5-18-8.1-5.3-2.6-9.8-5.3-13.5-8-3.7-2.8-6.6-5.7-8.6-8.9-2-3.1-3-6.7-3-10.7 0-3.7.9-7 2.7-10 1.8-3 4.3-5.5 7.5-7.6 3.3-2.1 7.3-3.7 12-4.9 4.8-1.1 10.1-1.7 15.9-1.7 4.3 0 8.8.3 13.5 1 4.8.6 9.5 1.6 14.3 3 4.8 1.3 9.5 3 14 5.1 4.6 2.1 8.8 4.5 12.7 7.3v-46.7c-7.6-2.9-15.8-5.1-24.8-6.5-9-1.4-19.2-2.1-30.8-2.1-12 0-23.4 1.3-34.1 3.9-10.7 2.6-20.1 6.7-28.2 12.3-8.1 5.6-14.5 12.7-19.2 21.3-4.7 8.6-7.1 18.9-7.1 30.9 0 15.3 4.4 28.3 13.2 39.1 8.8 10.7 22.2 19.8 40.1 27.2 7 2.9 13.6 5.7 19.7 8.4 6.1 2.8 11.4 5.6 15.8 8.6 4.5 2.9 8 6.2 10.6 9.6 2.6 3.5 3.9 7.5 3.9 12 0 3.5-.9 6.7-2.6 9.7-1.7 3-4.3 5.5-7.7 7.7-3.5 2.1-7.8 3.8-13 5-5.2 1.2-11.2 1.8-18.1 1.8-11.8 0-23.5-2.1-35-6.2-11.6-4.1-22.3-10.3-32.2-18.6zM246.7 233.2H311v-41.1H128v41.1h64v183h54.7z"
      />
    </svg>
  );
}
