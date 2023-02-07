package main

import (
	"fmt"

	"github.com/douyu/jupiter/pkg"
	"google.golang.org/genproto/googleapis/api/annotations"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

var version = pkg.JupiterVersion()

const (
	httpPkg            = protogen.GoImportPath("net/http")
	contextPkg         = protogen.GoImportPath("context")
	ginPkg             = protogen.GoImportPath("github.com/gin-gonic/gin")
	metadataPkg        = protogen.GoImportPath("google.golang.org/grpc/metadata")
	deprecationComment = "// Deprecated: Do not use."
)

var methodSets = make(map[string]int)

// generateFile generates a _gin.pb.go file.
func generateFile(gen *protogen.Plugin, file *protogen.File) *protogen.GeneratedFile {
	if len(file.Services) == 0 {
		return nil
	}
	filename := file.GeneratedFilenamePrefix + "_gin.pb.go"
	g := gen.NewGeneratedFile(filename, file.GoImportPath)
	g.P("// Code generated by github.com/douyu/jupiter/cmd/protoc-gen-go-gin. DO NOT EDIT.")
	g.P("// versions:")
	g.P(fmt.Sprintf("// - protoc-gen-go-gin %s", version))
	g.P("// - protoc             ", protocVersion(gen))
	g.P()
	g.P("package ", file.GoPackageName)
	g.P()
	g.P("// This is a compile-time assertion to ensure that this generated file")
	g.P("// is compatible with the github.com/douyu/jupiter/cmd/protoc-gen-go-gin package it is being compiled against.")
	g.P("var _ = ", httpPkg.Ident("StatusOK"))
	g.P("var _ = new(", contextPkg.Ident("Context"), ")")
	g.P("var _ = ", metadataPkg.Ident("New"))
	g.P("var _ = ", ginPkg.Ident("Engine"), "{}")
	g.P()

	for _, service := range file.Services {
		genService(gen, file, g, service)
	}
	return g
}

func genService(gen *protogen.Plugin, file *protogen.File, g *protogen.GeneratedFile, s *protogen.Service) {
	if s.Desc.Options().(*descriptorpb.ServiceOptions).GetDeprecated() {
		g.P("//")
		g.P(deprecationComment)
	}
	// HTTP Server.
	sd := &service{
		Name:     s.GoName,
		FullName: string(s.Desc.FullName()),
		FilePath: file.Desc.Path(),
	}

	for _, method := range s.Methods {
		sd.Methods = append(sd.Methods, genMethod(method)...)
	}
	g.P(sd.execute())
}

func genMethod(m *protogen.Method) []*method {
	var methods []*method

	// 存在 http rule 配置
	rule, ok := proto.GetExtension(m.Desc.Options(), annotations.E_Http).(*annotations.HttpRule)
	if rule != nil && ok {
		for _, bind := range rule.AdditionalBindings {
			methods = append(methods, buildHTTPRule(m, bind))
		}
		methods = append(methods, buildHTTPRule(m, rule))
		return methods
	}

	// 不存在走默认流程
	methods = append(methods, defaultMethod(m))
	return methods
}

func defaultMethod(m *protogen.Method) *method {
	return &method{
		Name:    m.GoName,
		Num:     methodSets[m.GoName],
		Request: m.Input.GoIdent.GoName,
		Reply:   m.Output.GoIdent.GoName,
		Path:    "/" + string(m.Parent.Desc.FullName()) + "/" + m.GoName,
		Method:  "POST",
		Comment: m.Comments.Leading.String(),
	}
}

func buildHTTPRule(m *protogen.Method, rule *annotations.HttpRule) *method {
	var (
		path   string
		method string
	)
	switch pattern := rule.Pattern.(type) {
	case *annotations.HttpRule_Get:
		path = pattern.Get
		method = "GET"
	case *annotations.HttpRule_Put:
		path = pattern.Put
		method = "PUT"
	case *annotations.HttpRule_Post:
		path = pattern.Post
		method = "POST"
	case *annotations.HttpRule_Delete:
		path = pattern.Delete
		method = "DELETE"
	case *annotations.HttpRule_Patch:
		path = pattern.Patch
		method = "PATCH"
	case *annotations.HttpRule_Custom:
		path = pattern.Custom.Path
		method = pattern.Custom.Kind
	}
	md := buildMethodDesc(m, method, path)
	return md
}

func buildMethodDesc(m *protogen.Method, httpMethod, path string) *method {
	defer func() { methodSets[m.GoName]++ }()
	md := &method{
		Name:    m.GoName,
		Num:     methodSets[m.GoName],
		Request: m.Input.GoIdent.GoName,
		Reply:   m.Output.GoIdent.GoName,
		Path:    path,
		Method:  httpMethod,
		Comment: m.Comments.Leading.String(),
	}
	md.initPathParams()
	return md
}

func protocVersion(gen *protogen.Plugin) string {
	v := gen.Request.GetCompilerVersion()
	if v == nil {
		return "(unknown)"
	}

	var suffix string
	if s := v.GetSuffix(); s != "" {
		suffix = "-" + s
	}

	return fmt.Sprintf("v%d.%d.%d%s", v.GetMajor(), v.GetMinor(), v.GetPatch(), suffix)
}
