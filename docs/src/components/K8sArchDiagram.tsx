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


function Arrow() {
  return (
    <div className="k8s-arch__arrow" aria-hidden="true">
      <svg width="36" height="14" viewBox="0 0 36 14" fill="none">
        <line x1="0" y1="7" x2="30" y2="7" stroke="currentColor" strokeWidth="1.5" />
        <polyline
          points="24,2 30,7 24,12"
          stroke="currentColor"
          strokeWidth="1.5"
          fill="none"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    </div>
  );
}

function DatabaseCylinder() {
  return (
    <svg className="k8s-arch__db-icon" width="38" height="46" viewBox="0 0 38 46" fill="none">
      <ellipse cx="19" cy="9" rx="16" ry="6.5" stroke="currentColor" strokeWidth="1.5" fill="currentColor" fillOpacity="0.18" />
      <line x1="3" y1="9" x2="3" y2="37" stroke="currentColor" strokeWidth="1.5" />
      <line x1="35" y1="9" x2="35" y2="37" stroke="currentColor" strokeWidth="1.5" />
      <path d="M3 37 Q3 43.5 19 43.5 Q35 43.5 35 37" stroke="currentColor" strokeWidth="1.5" fill="currentColor" fillOpacity="0.12" />
      <ellipse cx="19" cy="9" rx="16" ry="6.5" stroke="currentColor" strokeWidth="1.5" fill="currentColor" fillOpacity="0.2" />
    </svg>
  );
}

export function K8sArchDiagram() {
  return (
    <div className="k8s-arch" role="img" aria-label="Kubernetes architecture: User → Ingress → Service → Deployment pods → Postgres Database">
      {/* User */}
      <div className="k8s-arch__node">User</div>

      <Arrow />

      {/* K8s Cluster boundary */}
      <div className="k8s-arch__cluster">
        <div className="k8s-arch__cluster-badge">
          <img src="/assets/images/kubernetes-logo.svg" width="16" height="16" alt="" aria-hidden="true" />
          <span>K8s Cluster</span>
        </div>

        <div className="k8s-arch__cluster-flow">
          <div className="k8s-arch__node">Ingress</div>
          <Arrow />
          <div className="k8s-arch__node">Service</div>
          <Arrow />
          <div className="k8s-arch__deployment">
            <div className="k8s-arch__deployment-label">Deployment</div>
            <div className="k8s-arch__pods">
              <div className="k8s-arch__node k8s-arch__node--pod">Pod — I</div>
              <div className="k8s-arch__node k8s-arch__node--pod">Pod — II</div>
            </div>
          </div>
        </div>
      </div>

      <Arrow />

      {/* Postgres */}
      <div className="k8s-arch__db">
        <DatabaseCylinder />
        <span>Postgres<br />Database</span>
      </div>
    </div>
  );
}
