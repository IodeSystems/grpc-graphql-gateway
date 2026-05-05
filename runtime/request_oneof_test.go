package runtime

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// dynamicOneofMessage builds a proto.Message at test time whose schema is:
//
//	message LookupRequest {
//	  oneof key {
//	    string name = 1;
//	    int32  id   = 2;
//	    bytes  blob = 3;
//	  }
//	}
//
// We use dynamicpb so we can exercise the runtime without committing a
// generated .pb.go to runtime/.
func dynamicOneofMessage(t *testing.T) protoreflect.MessageDescriptor {
	t.Helper()
	file := &descriptorpb.FileDescriptorProto{
		Name:    proto.String("oneof_runtime_test.proto"),
		Syntax:  proto.String("proto3"),
		Package: proto.String("oneoftest.runtime"),
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("LookupRequest"),
			Field: []*descriptorpb.FieldDescriptorProto{
				{
					Name:       proto.String("name"),
					Number:     proto.Int32(1),
					Type:       descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
					Label:      descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					OneofIndex: proto.Int32(0),
					JsonName:   proto.String("name"),
				},
				{
					Name:       proto.String("id"),
					Number:     proto.Int32(2),
					Type:       descriptorpb.FieldDescriptorProto_TYPE_INT32.Enum(),
					Label:      descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					OneofIndex: proto.Int32(0),
					JsonName:   proto.String("id"),
				},
				{
					Name:       proto.String("blob"),
					Number:     proto.Int32(3),
					Type:       descriptorpb.FieldDescriptorProto_TYPE_BYTES.Enum(),
					Label:      descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
					OneofIndex: proto.Int32(0),
					JsonName:   proto.String("blob"),
				},
			},
			OneofDecl: []*descriptorpb.OneofDescriptorProto{{Name: proto.String("key")}},
		}},
	}
	fd, err := protodesc.NewFile(file, nil)
	require.NoError(t, err)
	return fd.Messages().Get(0)
}

func TestMarshalRequestOneofString(t *testing.T) {
	desc := dynamicOneofMessage(t)
	msg := dynamicpb.NewMessage(desc)

	err := MarshalRequest(map[string]interface{}{"name": "alice"}, msg, false)
	require.NoError(t, err)

	got := msg.Get(desc.Fields().ByName("name")).String()
	assert.Equal(t, "alice", got)
}

func TestMarshalRequestOneofInt32(t *testing.T) {
	desc := dynamicOneofMessage(t)
	msg := dynamicpb.NewMessage(desc)

	// graphql-go delivers ints as Go's `int`; json.Marshal turns that into
	// a plain JSON number, which protojson accepts for int32 fields.
	err := MarshalRequest(map[string]interface{}{"id": 42}, msg, false)
	require.NoError(t, err)

	got := msg.Get(desc.Fields().ByName("id")).Int()
	assert.Equal(t, int64(42), got)
}

func TestMarshalRequestOneofBytesBase64(t *testing.T) {
	desc := dynamicOneofMessage(t)
	msg := dynamicpb.NewMessage(desc)

	// proto3 JSON serialises bytes as base64. Same convention as elsewhere
	// in the runtime — clients that already work with bytes elsewhere don't
	// need a separate code path for oneof.
	err := MarshalRequest(map[string]interface{}{"blob": "aGVsbG8="}, msg, false)
	require.NoError(t, err)

	got := msg.Get(desc.Fields().ByName("blob")).Bytes()
	assert.Equal(t, []byte("hello"), got)
}

func TestMarshalRequestOneofMutualExclusion(t *testing.T) {
	desc := dynamicOneofMessage(t)
	msg := dynamicpb.NewMessage(desc)

	// protojson rejects multiple variants in a single oneof — that's the
	// server-side enforcement we documented in the README caveats.
	err := MarshalRequest(map[string]interface{}{"name": "alice", "id": 42}, msg, false)
	assert.Error(t, err, "supplying two oneof variants should error")
}
