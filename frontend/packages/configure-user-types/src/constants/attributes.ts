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

import type {Attribute} from '../types/user-types';

/**
 * The predefined library of attributes offered when building a user type schema.
 * This is a static, front-end-only convenience list; selecting an attribute
 * seeds a schema property with sensible defaults. Edit this file to change the
 * set of suggested attributes. Attributes are listed in the order below.
 */
const ATTRIBUTES: Attribute[] = [
  {id: 'username', displayName: 'Username', dataType: 'string', required: true, unique: true, credential: false},
  {
    id: 'email',
    displayName: 'Email',
    dataType: 'string',
    regex: '^[^@\\s]+@[^@\\s]+\\.[^@\\s]+$',
    required: true,
    unique: true,
    credential: false,
  },
  {id: 'given_name', displayName: 'First Name', dataType: 'string', required: false, unique: false, credential: false},
  {id: 'family_name', displayName: 'Last Name', dataType: 'string', required: false, unique: false, credential: false},
  {id: 'phone', displayName: 'Phone Number', dataType: 'string', required: false, unique: false, credential: false},
  {id: 'password', displayName: 'Password', dataType: 'string', required: false, unique: false, credential: true},
  {
    id: 'display_name',
    displayName: 'Display Name',
    dataType: 'string',
    required: false,
    unique: false,
    credential: false,
  },
  {id: 'name', displayName: 'Full Name', dataType: 'string', required: false, unique: false, credential: false},
  {
    id: 'middle_name',
    displayName: 'Middle Name',
    dataType: 'string',
    required: false,
    unique: false,
    credential: false,
  },
  {id: 'nickname', displayName: 'Nickname', dataType: 'string', required: false, unique: false, credential: false},
  {id: 'picture', displayName: 'Picture', dataType: 'string', required: false, unique: false, credential: false},
  {id: 'birthdate', displayName: 'Birthdate', dataType: 'date', required: false, unique: false, credential: false},
  {id: 'gender', displayName: 'Gender', dataType: 'string', required: false, unique: false, credential: false},
  {id: 'locale', displayName: 'Locale', dataType: 'string', required: false, unique: false, credential: false},
  {id: 'zoneinfo', displayName: 'Time Zone', dataType: 'string', required: false, unique: false, credential: false},
  {
    id: 'preferred_language',
    displayName: 'Preferred Language',
    dataType: 'string',
    required: false,
    unique: false,
    credential: false,
  },
  {id: 'profile', displayName: 'Profile URL', dataType: 'string', required: false, unique: false, credential: false},
  {id: 'website', displayName: 'Website', dataType: 'string', required: false, unique: false, credential: false},
  {id: 'title', displayName: 'Title', dataType: 'string', required: false, unique: false, credential: false},
  {
    id: 'email_verified',
    displayName: 'Email Verified',
    dataType: 'boolean',
    required: false,
    unique: false,
    credential: false,
  },
  {
    id: 'phone_verified',
    displayName: 'Phone Verified',
    dataType: 'boolean',
    required: false,
    unique: false,
    credential: false,
  },
  {id: 'active', displayName: 'Active', dataType: 'boolean', required: false, unique: false, credential: false},
  {
    id: 'street_address',
    displayName: 'Street Address',
    dataType: 'string',
    required: false,
    unique: false,
    credential: false,
  },
  {id: 'locality', displayName: 'City', dataType: 'string', required: false, unique: false, credential: false},
  {id: 'region', displayName: 'State/Region', dataType: 'string', required: false, unique: false, credential: false},
  {
    id: 'postal_code',
    displayName: 'Postal Code',
    dataType: 'string',
    required: false,
    unique: false,
    credential: false,
  },
  {id: 'country', displayName: 'Country', dataType: 'string', required: false, unique: false, credential: false},
  {
    id: 'employee_number',
    displayName: 'Employee Number',
    dataType: 'string',
    required: false,
    unique: false,
    credential: false,
  },
  {id: 'department', displayName: 'Department', dataType: 'string', required: false, unique: false, credential: false},
  {id: 'division', displayName: 'Division', dataType: 'string', required: false, unique: false, credential: false},
  {
    id: 'organization',
    displayName: 'Organization',
    dataType: 'string',
    required: false,
    unique: false,
    credential: false,
  },
  {
    id: 'cost_center',
    displayName: 'Cost Center',
    dataType: 'string',
    required: false,
    unique: false,
    credential: false,
  },
  {id: 'manager', displayName: 'Manager', dataType: 'string', required: false, unique: false, credential: false},
];

export default ATTRIBUTES;
