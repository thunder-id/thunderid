// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

package utils

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildPaginationLinks_MiddlePage(t *testing.T) {
	links := BuildPaginationLinks("/items", 5, 5, 20, "")
	require.Len(t, links, 4)
	assert.Equal(t, "first", links[0].Rel)
	assert.Equal(t, "/items?offset=0&limit=5", links[0].Href)
	assert.Equal(t, "prev", links[1].Rel)
	assert.Equal(t, "/items?offset=0&limit=5", links[1].Href)
	assert.Equal(t, "next", links[2].Rel)
	assert.Equal(t, "/items?offset=10&limit=5", links[2].Href)
	assert.Equal(t, "last", links[3].Rel)
	assert.Equal(t, "/items?offset=15&limit=5", links[3].Href)
}

func TestBuildPaginationLinks_FirstPage(t *testing.T) {
	links := BuildPaginationLinks("/items", 10, 0, 25, "")
	require.Len(t, links, 2)
	assert.Equal(t, "next", links[0].Rel)
	assert.Equal(t, "last", links[1].Rel)
}

func TestBuildPaginationLinks_LastPage(t *testing.T) {
	links := BuildPaginationLinks("/items", 10, 20, 25, "")
	require.Len(t, links, 2)
	assert.Equal(t, "first", links[0].Rel)
	assert.Equal(t, "prev", links[1].Rel)
}

func TestBuildPaginationLinks_SinglePage(t *testing.T) {
	links := BuildPaginationLinks("/items", 10, 0, 5, "")
	require.Len(t, links, 0)
}

func TestBuildPaginationLinks_ZeroLimit(t *testing.T) {
	links := BuildPaginationLinks("/items", 0, 0, 10, "")
	require.Len(t, links, 0)
}

func TestBuildPaginationLinks_NegativeLimit(t *testing.T) {
	links := BuildPaginationLinks("/items", -1, 0, 10, "")
	require.Len(t, links, 0)
}

func TestBuildPaginationLinks_WithExtraQuery(t *testing.T) {
	links := BuildPaginationLinks("/items", 5, 5, 20, "&include=display")
	require.Len(t, links, 4)
	assert.Equal(t, "/items?offset=0&limit=5&include=display", links[0].Href)
	assert.Equal(t, "/items?offset=0&limit=5&include=display", links[1].Href)
	assert.Equal(t, "/items?offset=10&limit=5&include=display", links[2].Href)
	assert.Equal(t, "/items?offset=15&limit=5&include=display", links[3].Href)
}

func TestFilterQueryParam(t *testing.T) {
	tests := []struct {
		name    string
		filters map[string]interface{}
		want    string
	}{
		{"no filter", nil, ""},
		{"string value is quoted", map[string]interface{}{"ouId": "ou-1"}, "&filter=ouId+eq+%22ou-1%22"},
		{"string value is kept verbatim", map[string]interface{}{"path": `a\b`}, "&filter=path+eq+%22a%5Cb%22"},
		{"number value is bare", map[string]interface{}{"age": int64(30)}, "&filter=age+eq+30"},
		{"boolean value is bare", map[string]interface{}{"active": true}, "&filter=active+eq+true"},
		{
			"several are joined in attribute order",
			map[string]interface{}{"ouId": "ou-1", "email": "a@b.c"},
			"&filter=email+eq+%22a%40b.c%22+and+ouId+eq+%22ou-1%22",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, FilterQueryParam(tc.filters))
		})
	}
}

func TestPaginateOwnedThenShared(t *testing.T) {
	owned := []string{"o1", "o2", "o3"}
	shared := []string{"s1", "s2", "s3", "s4"}
	tests := []struct {
		name       string
		limit      int
		offset     int
		wantOwned  []string
		wantShared []string
	}{
		{"page of owned only", 2, 0, []string{"o1", "o2"}, []string{}},
		{"page ends on the last owned", 3, 0, []string{"o1", "o2", "o3"}, []string{}},
		{"page spans owned and shared", 3, 2, []string{"o3"}, []string{"s1", "s2"}},
		{"page of shared only", 2, 4, []string{}, []string{"s2", "s3"}},
		{"last page is short", 5, 5, []string{}, []string{"s3", "s4"}},
		{"offset past the end", 5, 7, []string{}, []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			listOwned := func(limit, offset int) ([]string, error) {
				return owned[offset : offset+limit], nil
			}
			loadShared := func(ids []string) ([]string, error) {
				return ids, nil
			}

			gotOwned, gotShared, err := PaginateOwnedThenShared(
				len(owned), listOwned, shared, loadShared, func(id string) string { return id }, tc.limit, tc.offset)

			require.NoError(t, err)
			assert.Equal(t, tc.wantOwned, gotOwned)
			assert.Equal(t, tc.wantShared, gotShared)
		})
	}
}

func TestPaginateOwnedThenShared_KeepsSharedOrderAndSkipsMissing(t *testing.T) {
	loadShared := func(ids []string) ([]string, error) {
		require.Equal(t, []string{"s1", "s2", "s3"}, ids)
		return []string{"s3", "s1"}, nil
	}

	_, gotShared, err := PaginateOwnedThenShared(0, nil, []string{"s1", "s2", "s3"}, loadShared,
		func(id string) string { return id }, 10, 0)

	require.NoError(t, err)
	assert.Equal(t, []string{"s1", "s3"}, gotShared)
}

func TestPaginateOwnedThenShared_Errors(t *testing.T) {
	failure := errors.New("store error")
	idOf := func(id string) string { return id }

	_, _, err := PaginateOwnedThenShared(1, func(int, int) ([]string, error) { return nil, failure },
		nil, nil, idOf, 10, 0)
	assert.ErrorIs(t, err, failure)

	_, _, err = PaginateOwnedThenShared(0, nil, []string{"s1"},
		func([]string) ([]string, error) { return nil, failure }, idOf, 10, 0)
	assert.ErrorIs(t, err, failure)
}
