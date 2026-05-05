package spec

import (
	"strings"

	"github.com/ysugimoto/grpc-graphql-gateway/graphql"
	"google.golang.org/protobuf/proto"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
)

// Method spec wraps MethodDescriptorProto with GraphqlQuery and GraphqlMutation options.
type Method struct {
	descriptor *descriptorpb.MethodDescriptorProto
	Service    *Service
	Schema     *graphql.GraphqlSchema
	*File

	paths []int
}

func NewMethod(
	m *descriptorpb.MethodDescriptorProto,
	s *Service,
	paths ...int,
) *Method {

	var schema *graphql.GraphqlSchema
	if opts := m.GetOptions(); opts != nil && proto.HasExtension(opts, graphql.E_Schema) {
		if v, ok := proto.GetExtension(opts, graphql.E_Schema).(*graphql.GraphqlSchema); ok {
			schema = v
		}
	}

	return &Method{
		descriptor: m,
		Service:    s,
		Schema:     schema,
		File:       s.File,
		paths:      paths,
	}
}

// -- common functions

func (m *Method) Comment() string {
	return m.File.getComment(m.paths)
}

func (m *Method) ServiceName() string {
	return m.Service.Name()
}

func (m *Method) Name() string {
	return m.descriptor.GetName()
}

func (m *Method) Input() string {
	return strings.TrimPrefix(m.descriptor.GetInputType(), ".")
}

func (m *Method) Output() string {
	return strings.TrimPrefix(m.descriptor.GetOutputType(), ".")
}
