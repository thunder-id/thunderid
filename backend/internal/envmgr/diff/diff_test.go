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

package diff

import (
	"testing"

	"github.com/thunder-id/thunderid/internal/envmgr/bundle"
)

func TestComputeClassifiesChanges(t *testing.T) {
	old := bundle.Parse(`resource_type: application
id: a
name: A
value: 1
---
resource_type: application
id: b
name: B
---
resource_type: flow
id: f
name: F`)

	// a updated, b unchanged, f deleted, c added.
	new := bundle.Parse(`resource_type: application
id: a
name: A
value: 2
---
resource_type: application
id: b
name: B
---
resource_type: application
id: c
name: C`)

	d := Compute(old, new)
	if d.Summary.Added != 1 || d.Summary.Updated != 1 || d.Summary.Deleted != 1 || d.Summary.Unchanged != 1 {
		t.Fatalf("unexpected summary: %+v", d.Summary)
	}
	if !d.HasChanges() {
		t.Fatalf("expected changes")
	}

	byKey := map[string]ResourceChange{}
	for _, c := range d.Changes {
		byKey[c.Key] = c
	}
	if byKey["application/id:a"].Change != Updated {
		t.Fatalf("a should be updated: %+v", byKey["application/id:a"])
	}
	if byKey["application/id:b"].Change != Unchanged {
		t.Fatalf("b should be unchanged")
	}
	if byKey["application/id:c"].Change != Added {
		t.Fatalf("c should be added")
	}
	if byKey["flow/id:f"].Change != Deleted {
		t.Fatalf("f should be deleted")
	}
}

func TestComputeIdenticalHasNoChanges(t *testing.T) {
	res := bundle.Parse("resource_type: application\nid: a\nname: A")
	d := Compute(res, res)
	if d.HasChanges() {
		t.Fatalf("identical bundles should not report changes: %+v", d.Summary)
	}
}

func TestLineDiffMarksAddedAndRemoved(t *testing.T) {
	ops := lineDiff("a\nb\nc", "a\nB\nc")
	var added, removed int
	for _, op := range ops {
		switch op.Kind {
		case "+":
			added++
		case "-":
			removed++
		}
	}
	if added != 1 || removed != 1 {
		t.Fatalf("expected one added and one removed line, got +%d -%d (%+v)", added, removed, ops)
	}
}
