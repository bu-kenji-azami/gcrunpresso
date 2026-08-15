package gcrunpresso

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

func OutputJSONForAPI(w io.Writer, v any) (int, error) {
	b, err := MarshalJSONForAPI(v)
	if err != nil {
		return 0, fmt.Errorf("failed to marshal json: %w", err)
	}
	return w.Write(b)
}

func MustMarshalJSONStringForAPI(v any) string {
	b, err := MarshalJSONForAPI(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func MarshalJSONForAPI(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}
	bs, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	bs = append(bs, '\n')
	return bs, nil
}

func jsonKeyForAPI(s string) string {
	if len(s) == 0 {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}
