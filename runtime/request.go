package runtime

import (
	"errors"
	"fmt"

	"encoding/json"
	"io"
	"net/http"

	"github.com/iancoleman/strcase"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type GraphqlRequest struct {
	Query         string                 `json:"query"`
	Variables     map[string]interface{} `json:"variables"`
	OperationName string                 `json:"operationName"`
}

// ParseRequest parses graphql query and variables from each request methods
func parseRequest(r *http.Request) (*GraphqlRequest, error) {
	var body []byte

	// Get request body
	switch r.Method {
	case http.MethodPost:
		buf, err := io.ReadAll(r.Body)
		if err != nil {
			return nil, errors.New("malformed request body, " + err.Error())
		}
		body = buf
	case http.MethodGet:
		body = []byte(r.URL.Query().Get("query"))
	default:
		return nil, errors.New("invalid request method: '" + r.Method + "'")
	}

	// And try to parse
	var req GraphqlRequest
	if err := json.Unmarshal(body, &req); err != nil {
		// If error, the request body may come with single query line
		req.Query = string(body)
	}
	return &req, nil
}

// MarshalRequest marshals graphql request arguments into a gRPC request
// message.
//
// When v implements proto.Message (the common case for generated code),
// the canonical proto3 JSON path via protojson is used. That handles
// oneof wrapper construction, base64-encoded bytes, enums by name or
// number, well-known types, and accepts both proto and lowerCamel field
// names — so isCamel is a no-op on this path.
//
// For non-proto targets the legacy std-json round-trip is preserved so
// callers outside the generator keep working.
func MarshalRequest(args, v interface{}, isCamel bool) error {
	if args == nil {
		return errors.New("resolved params should be non-nil")
	}
	if msg, ok := v.(proto.Message); ok {
		buf, err := json.Marshal(args)
		if err != nil {
			return fmt.Errorf("encoding args: %w", err)
		}
		return protojson.UnmarshalOptions{DiscardUnknown: true}.Unmarshal(buf, msg)
	}
	m, ok := args.(map[string]interface{})
	if !ok {
		return errors.New("failed to type conversion of map[string]interface{}")
	}
	if isCamel {
		m = toLowerCaseKeys(m)
	}
	buf, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(buf, &v)
}

// Convert to lower case keyname string
func toLowerCaseKeys(args map[string]interface{}) map[string]interface{} {
	lc := make(map[string]interface{})
	for k, v := range args {
		lc[strcase.ToSnake(k)] = marshal(v)
	}
	return lc
}

// marshals interface recursively
func marshal(v interface{}) interface{} {
	switch t := v.(type) {
	case map[string]interface{}:
		return toLowerCaseKeys(t)
	case []interface{}:
		ret := make([]interface{}, len(t))
		for i, si := range t {
			ret[i] = marshal(si)
		}
		return ret
	default:
		return t
	}
}
