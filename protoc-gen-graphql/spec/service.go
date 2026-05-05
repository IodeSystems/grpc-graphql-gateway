package spec

import (
	"github.com/ysugimoto/grpc-graphql-gateway/graphql"
	"google.golang.org/protobuf/proto"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
)

// Service spec wraps ServiceDescriptorProto with GraphqlService option.
type Service struct {
	descriptor *descriptorpb.ServiceDescriptorProto
	Option     *graphql.GraphqlService
	*File
	paths   []int
	methods []*Method

	Queries       []*Query
	Mutations     []*Mutation
	Subscriptions []*Subscription
}

func NewService(
	d *descriptorpb.ServiceDescriptorProto,
	f *File,
	paths ...int,
) *Service {

	var o *graphql.GraphqlService
	if opts := d.GetOptions(); opts != nil && proto.HasExtension(opts, graphql.E_Service) {
		if service, ok := proto.GetExtension(opts, graphql.E_Service).(*graphql.GraphqlService); ok {
			o = service
		}
	}

	s := &Service{
		descriptor:    d,
		Option:        o,
		File:          f,
		paths:         paths,
		methods:       make([]*Method, 0),
		Queries:       make([]*Query, 0),
		Mutations:     make([]*Mutation, 0),
		Subscriptions: make([]*Subscription, 0),
	}

	for i, m := range d.GetMethod() {
		ps := make([]int, len(paths))
		copy(ps, paths)
		s.methods = append(s.methods, NewMethod(m, s, append(ps, 4, i)...)) // nolint: mnd
	}
	return s
}

func (s *Service) Comment() string {
	return s.File.getComment(s.paths)
}

func (s *Service) Name() string {
	return s.descriptor.GetName()
}

func (s *Service) Methods() []*Method {
	return s.methods
}

func (s *Service) Host() string {
	if s.Option == nil {
		return ""
	}
	return s.Option.GetHost()
}

func (s *Service) Insecure() bool {
	if s.Option == nil {
		return false
	}
	return s.Option.GetInsecure()
}
