package spec

import (
	"strings"

	"path/filepath"

	"github.com/iancoleman/strcase"
	descriptorpb "google.golang.org/protobuf/types/descriptorpb"
)

// Oneof groups the variant fields of a single user-declared oneof on
// a Message. Synthetic oneofs (proto3 optional) are excluded.
type Oneof struct {
	Name   string   // proto name, e.g. "body"
	GoName string   // Go field name on the message, e.g. "Body"
	Fields []*Field // variants in proto declaration order
}

// Message spec wraps DescriptorProto
type Message struct {
	descriptor *descriptorpb.DescriptorProto
	*File

	prefix []string
	paths  []int
	fields []*Field

	*Dependencies
	PluckFields []*Field
}

func NewMessage(
	d *descriptorpb.DescriptorProto,
	f *File,
	prefix []string,
	isCamel bool,
	paths ...int,
) *Message {

	m := &Message{
		descriptor: d,
		File:       f,
		prefix:     prefix,
		paths:      paths,
		fields:     make([]*Field, 0),

		Dependencies: NewDependencies(),
	}
	for i, field := range d.GetField() {
		ps := make([]int, len(paths))
		copy(ps, paths)
		ff := NewField(field, f, isCamel, append(ps, 2, i)...) // nolint: mnd
		if !ff.IsOmit() {
			m.fields = append(m.fields, ff)
		}
	}

	// Bind oneof metadata onto each member field so the template can emit
	// per-variant resolvers and request fixups. Skip synthetic oneofs that
	// proto3's optional keyword generates internally.
	decls := d.GetOneofDecl()
	for _, ff := range m.fields {
		if ff.descriptor.OneofIndex == nil || ff.descriptor.GetProto3Optional() {
			continue
		}
		idx := int(ff.descriptor.GetOneofIndex())
		if idx < 0 || idx >= len(decls) {
			continue
		}
		ff.oneofGoName = strcase.ToCamel(decls[idx].GetName())
		ff.wrapperFieldName = strcase.ToCamel(ff.Name())
		ff.wrapperGoType = m.TypeName() + "_" + ff.wrapperFieldName
	}
	return m
}

// Oneofs returns the user-declared oneofs on this message in proto
// declaration order. Synthetic oneofs are excluded.
func (m *Message) Oneofs() []*Oneof {
	decls := m.descriptor.GetOneofDecl()
	if len(decls) == 0 {
		return nil
	}
	result := make([]*Oneof, 0, len(decls))
	for i, decl := range decls {
		var members []*Field
		for _, ff := range m.fields {
			if !ff.IsOneof() {
				continue
			}
			if int(ff.descriptor.GetOneofIndex()) == i {
				members = append(members, ff)
			}
		}
		if len(members) == 0 {
			continue
		}
		result = append(result, &Oneof{
			Name:   decl.GetName(),
			GoName: strcase.ToCamel(decl.GetName()),
			Fields: members,
		})
	}
	return result
}

func (m *Message) Fields() []*Field {
	return m.fields
}

func (m *Message) setRequiredFields() {
	for _, f := range m.fields {
		f.setRequiredField()
	}
}

func (m *Message) TypeFields() []*Field {
	if m.PluckFields == nil {
		return m.Fields()
	}
	return m.PluckFields
}

func (m *Message) Comment() string {
	if IsGooglePackage(m) {
		return ""
	}
	return m.File.getComment(m.paths)
}

func (m *Message) Name() string {
	var p string
	if len(m.prefix) > 0 {
		p = strings.Join(m.prefix, ".") + "."
	}
	return p + m.descriptor.GetName()
}

func (m *Message) TypeName() string {
	var p string
	if len(m.prefix) > 0 {
		p = strings.Join(m.prefix, "_") + "_"
	}
	return p + m.descriptor.GetName()
}

func (m *Message) SingleName() string {
	spl := strings.Split(m.Name(), ".")
	return spl[len(spl)-1]
}

func (m *Message) StructName(ptr bool) string {
	gopkg := m.GoPackage()
	if gopkg == mainPackage {
		gopkg = ""
	} else {
		gopkg = filepath.Base(gopkg) + "."
	}
	var sign string
	if ptr {
		sign = "*"
	}
	return sign + gopkg + m.Name()
}

func (m *Message) FullPath() string {
	return m.File.Package() + "." + m.Name()
}

func (m *Message) Interfaces() (ifs []*Message) {
	for _, f := range m.fields {
		if !f.IsCyclic {
			continue
		}
		msg, ok := f.DependType.(*Message)
		if !ok {
			continue
		}
		ifs = append(ifs, msg)
	}
	return ifs
}
