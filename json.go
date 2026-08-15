package gcrunpresso

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"cloud.google.com/go/run/apiv2/runpb"
	"github.com/goccy/go-yaml"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

var (
	protoJSONStrict = protojson.UnmarshalOptions{
		DiscardUnknown: false,
	}
	protoJSONPermissive = protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
	protoJSONIndent = protojson.MarshalOptions{
		Multiline:       true,
		Indent:          "  ",
		EmitUnpopulated: false,
	}
)

// YAMLToJSON converts YAML bytes to JSON bytes.
func YAMLToJSON(yamlBytes []byte) ([]byte, error) {
	var obj any
	if err := yaml.Unmarshal(yamlBytes, &obj); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}
	if obj == nil {
		return []byte("{}"), nil
	}
	// Convert map[any]any recursively to map[string]any if any exists
	cleaned := cleanYAMLMap(obj)
	return json.Marshal(cleaned)
}

func cleanYAMLMap(v any) any {
	switch val := v.(type) {
	case map[string]any:
		res := make(map[string]any, len(val))
		for k, v := range val {
			res[k] = cleanYAMLMap(v)
		}
		return res
	case map[any]any:
		res := make(map[string]any, len(val))
		for k, v := range val {
			res[fmt.Sprintf("%v", k)] = cleanYAMLMap(v)
		}
		return res
	case []any:
		res := make([]any, len(val))
		for i, v := range val {
			res[i] = cleanYAMLMap(v)
		}
		return res
	default:
		return val
	}
}

// UnmarshalService decodes YAML/JSON into *runpb.Service.
func UnmarshalService(src []byte, svc *runpb.Service, discardUnknown bool) error {
	jsonBytes, err := YAMLToJSON(src)
	if err != nil {
		return err
	}

	opts := protoJSONStrict
	if discardUnknown {
		opts = protoJSONPermissive
	}

	if err := opts.Unmarshal(jsonBytes, svc); err != nil {
		// Check for Knative v1 schema detection
		if strings.Contains(string(src), "apiVersion: serving.knative.dev/v1") ||
			strings.Contains(string(src), "serving.knative.dev") {
			return fmt.Errorf("invalid service definition: Knative serving.knative.dev/v1 manifest detected. gcrunpresso uses Cloud Run Admin API v2 schema (see documentation): %w", err)
		}
		return fmt.Errorf("failed to unmarshal service definition into Cloud Run v2 Service: %w", err)
	}
	return nil
}

// UnmarshalJob decodes YAML/JSON into *runpb.Job.
func UnmarshalJob(src []byte, job *runpb.Job, discardUnknown bool) error {
	jsonBytes, err := YAMLToJSON(src)
	if err != nil {
		return err
	}

	opts := protoJSONStrict
	if discardUnknown {
		opts = protoJSONPermissive
	}

	if err := opts.Unmarshal(jsonBytes, job); err != nil {
		return fmt.Errorf("failed to unmarshal job definition into Cloud Run v2 Job: %w", err)
	}
	return nil
}

// MarshalService encodes *runpb.Service to formatted JSON bytes.
func MarshalService(svc *runpb.Service) ([]byte, error) {
	b, err := protoJSONIndent.Marshal(svc)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal service to JSON: %w", err)
	}
	var buf bytes.Buffer
	buf.Write(b)
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

// MarshalJob encodes *runpb.Job to formatted JSON bytes.
func MarshalJob(job *runpb.Job) ([]byte, error) {
	b, err := protoJSONIndent.Marshal(job)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal job to JSON: %w", err)
	}
	var buf bytes.Buffer
	buf.Write(b)
	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

// MarshalProtoYAML encodes any proto message to clean YAML bytes.
func MarshalProtoYAML(m proto.Message) ([]byte, error) {
	jsonBytes, err := protoJSONIndent.Marshal(m)
	if err != nil {
		return nil, err
	}
	var obj any
	if err := json.Unmarshal(jsonBytes, &obj); err != nil {
		return nil, err
	}
	return yaml.Marshal(obj)
}

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
