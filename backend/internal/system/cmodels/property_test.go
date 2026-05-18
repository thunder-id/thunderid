package cmodels

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSerializePropertiesToJSONObject_EmptySlice(t *testing.T) {
	result, err := SerializePropertiesToJSONObject([]Property{})
	assert.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestSerializePropertiesToJSONObject_NilSlice(t *testing.T) {
	result, err := SerializePropertiesToJSONObject(nil)
	assert.NoError(t, err)
	assert.Equal(t, "", result)
}

func TestSerializePropertiesToJSONObject_MultipleProperties(t *testing.T) {
	props := []Property{
		{name: "client_id", value: "my-client", isSecret: false},
		{name: "api_key", value: "secret-val", isSecret: true},
	}

	result, err := SerializePropertiesToJSONObject(props)
	require.NoError(t, err)
	assert.NotEmpty(t, result)

	deserialized, err := DeserializePropertiesFromJSONObject(result)
	require.NoError(t, err)
	assert.Len(t, deserialized, 2)

	byName := make(map[string]Property, len(deserialized))
	for _, p := range deserialized {
		byName[p.name] = p
	}

	clientProp := byName["client_id"]
	assert.Equal(t, "my-client", clientProp.value)
	assert.False(t, clientProp.isSecret)

	apiKeyProp := byName["api_key"]
	assert.Equal(t, "secret-val", apiKeyProp.value)
	assert.True(t, apiKeyProp.isSecret)
}

func TestSerializePropertiesToJSONObject_PreservesIsSecretFlag(t *testing.T) {
	props := []Property{
		{name: "secret_prop", value: "hidden", isSecret: true},
		{name: "plain_prop", value: "visible", isSecret: false},
	}

	result, err := SerializePropertiesToJSONObject(props)
	require.NoError(t, err)

	deserialized, err := DeserializePropertiesFromJSONObject(result)
	require.NoError(t, err)

	byName := make(map[string]Property, len(deserialized))
	for _, p := range deserialized {
		byName[p.name] = p
	}

	assert.True(t, byName["secret_prop"].isSecret)
	assert.False(t, byName["plain_prop"].isSecret)
}

func TestDeserializePropertiesFromJSONObject_EmptyString(t *testing.T) {
	result, err := DeserializePropertiesFromJSONObject("")
	assert.NoError(t, err)
	assert.Empty(t, result)
}

func TestDeserializePropertiesFromJSONObject_ValidJSON(t *testing.T) {
	jsonStr := `{"client_id":{"value":"my-client","isSecret":false},"token":{"value":"abc","isSecret":true}}`

	result, err := DeserializePropertiesFromJSONObject(jsonStr)
	require.NoError(t, err)
	assert.Len(t, result, 2)

	sort.Slice(result, func(i, j int) bool { return result[i].name < result[j].name })

	assert.Equal(t, "client_id", result[0].name)
	assert.Equal(t, "my-client", result[0].value)
	assert.False(t, result[0].isSecret)

	assert.Equal(t, "token", result[1].name)
	assert.Equal(t, "abc", result[1].value)
	assert.True(t, result[1].isSecret)
}

func TestDeserializePropertiesFromJSONObject_InvalidJSON(t *testing.T) {
	result, err := DeserializePropertiesFromJSONObject("{invalid")
	assert.Error(t, err)
	assert.Nil(t, result)
}

func TestSerializeDeserializePropertiesFromJSONObject_Roundtrip(t *testing.T) {
	original := []Property{
		{name: "key1", value: "val1", isSecret: false},
		{name: "key2", value: "val2", isSecret: true},
		{name: "key3", value: "val3", isSecret: false},
	}

	serialized, err := SerializePropertiesToJSONObject(original)
	require.NoError(t, err)

	deserialized, err := DeserializePropertiesFromJSONObject(serialized)
	require.NoError(t, err)
	assert.Len(t, deserialized, len(original))

	sort.Slice(original, func(i, j int) bool { return original[i].name < original[j].name })
	sort.Slice(deserialized, func(i, j int) bool { return deserialized[i].name < deserialized[j].name })

	for i := range original {
		assert.Equal(t, original[i].name, deserialized[i].name)
		assert.Equal(t, original[i].value, deserialized[i].value)
		assert.Equal(t, original[i].isSecret, deserialized[i].isSecret)
	}
}
