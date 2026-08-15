package gcrunpresso_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/kayac/gcrunpresso/v2"
)

type testJSONStruct struct {
	FooBarBaz string            `json:"fooBarBaz"`
	Options   map[string]string `json:"options"`
	Nested    struct {
		FooBar string `json:"fooBar"`
	} `json:"nested"`
}

var testJSONValue = testJSONStruct{
	FooBarBaz: "foo",
	Options: map[string]string{
		"Foo": "xxx",
	},
	Nested: struct {
		FooBar string `json:"fooBar"`
	}{
		FooBar: "foobar",
	},
}

func TestMarshalJSONForAPI(t *testing.T) {
	b, err := gcrunpresso.MarshalJSONForAPI(testJSONValue)
	if err != nil {
		t.Fatal(err)
	}
	expected := "{\n  \"fooBarBaz\": \"foo\",\n  \"options\": {\n    \"Foo\": \"xxx\"\n  },\n  \"nested\": {\n    \"fooBar\": \"foobar\"\n  }\n}\n"
	var expectedBuf bytes.Buffer
	json.Indent(&expectedBuf, []byte(expected), "", "  ")
	if diff := cmp.Diff(expected, string(b)); diff != "" {
		t.Errorf("unexpected json (-want +got):\n%s", diff)
	}
}
