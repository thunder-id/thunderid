/*
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 * http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import "dotenv/config";
import { startSmtp } from "./smtp.js";
import { startHttp } from "./http.js";

const SMTP_HOST = process.env.SMTP_HOST || "127.0.0.1";
const SMTP_PORT = Number(process.env.SMTP_PORT || 2525);
const HTTP_HOST = process.env.HTTP_HOST || "127.0.0.1";
const HTTP_PORT = Number(process.env.HTTP_PORT || 8788);

startSmtp(SMTP_HOST, SMTP_PORT);
startHttp(HTTP_HOST, HTTP_PORT);

console.log(`[smtp-server] Wayfinder local SMTP server started`);
console.log(`  SMTP:  ${SMTP_HOST}:${SMTP_PORT}  (username: dev / password: dev)`);
console.log(`  Inbox: http://${HTTP_HOST}:${HTTP_PORT}`);
