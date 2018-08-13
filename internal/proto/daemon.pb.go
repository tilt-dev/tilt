// Code generated by protoc-gen-go. DO NOT EDIT.
// source: internal/proto/daemon.proto

/*
Package proto is a generated protocol buffer package.

It is generated from these files:
	internal/proto/daemon.proto

It has these top-level messages:
	Service
	Cmd
	Mount
	Repo
	GitRepo
	Output
	CreateServiceReply
*/
package proto

import proto1 "github.com/golang/protobuf/proto"
import fmt "fmt"
import math "math"

import (
	context "golang.org/x/net/context"
	grpc "google.golang.org/grpc"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto1.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto1.ProtoPackageIsVersion2 // please upgrade the proto package

type Service struct {
	K8SYaml        string   `protobuf:"bytes,1,opt,name=k8s_yaml,json=k8sYaml" json:"k8s_yaml,omitempty"`
	DockerfileText string   `protobuf:"bytes,2,opt,name=dockerfile_text,json=dockerfileText" json:"dockerfile_text,omitempty"`
	Mounts         []*Mount `protobuf:"bytes,3,rep,name=mounts" json:"mounts,omitempty"`
	Steps          []*Cmd   `protobuf:"bytes,4,rep,name=steps" json:"steps,omitempty"`
	DockerfileTag  string   `protobuf:"bytes,5,opt,name=dockerfile_tag,json=dockerfileTag" json:"dockerfile_tag,omitempty"`
}

func (m *Service) Reset()                    { *m = Service{} }
func (m *Service) String() string            { return proto1.CompactTextString(m) }
func (*Service) ProtoMessage()               {}
func (*Service) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{0} }

func (m *Service) GetK8SYaml() string {
	if m != nil {
		return m.K8SYaml
	}
	return ""
}

func (m *Service) GetDockerfileText() string {
	if m != nil {
		return m.DockerfileText
	}
	return ""
}

func (m *Service) GetMounts() []*Mount {
	if m != nil {
		return m.Mounts
	}
	return nil
}

func (m *Service) GetSteps() []*Cmd {
	if m != nil {
		return m.Steps
	}
	return nil
}

func (m *Service) GetDockerfileTag() string {
	if m != nil {
		return m.DockerfileTag
	}
	return ""
}

type Cmd struct {
	Argv []string `protobuf:"bytes,1,rep,name=argv" json:"argv,omitempty"`
}

func (m *Cmd) Reset()                    { *m = Cmd{} }
func (m *Cmd) String() string            { return proto1.CompactTextString(m) }
func (*Cmd) ProtoMessage()               {}
func (*Cmd) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{1} }

func (m *Cmd) GetArgv() []string {
	if m != nil {
		return m.Argv
	}
	return nil
}

type Mount struct {
	Repo          *Repo  `protobuf:"bytes,1,opt,name=repo" json:"repo,omitempty"`
	ContainerPath string `protobuf:"bytes,2,opt,name=container_path,json=containerPath" json:"container_path,omitempty"`
}

func (m *Mount) Reset()                    { *m = Mount{} }
func (m *Mount) String() string            { return proto1.CompactTextString(m) }
func (*Mount) ProtoMessage()               {}
func (*Mount) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{2} }

func (m *Mount) GetRepo() *Repo {
	if m != nil {
		return m.Repo
	}
	return nil
}

func (m *Mount) GetContainerPath() string {
	if m != nil {
		return m.ContainerPath
	}
	return ""
}

type Repo struct {
	// Types that are valid to be assigned to RepoType:
	//	*Repo_GitRepo
	RepoType isRepo_RepoType `protobuf_oneof:"repo_type"`
}

func (m *Repo) Reset()                    { *m = Repo{} }
func (m *Repo) String() string            { return proto1.CompactTextString(m) }
func (*Repo) ProtoMessage()               {}
func (*Repo) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{3} }

type isRepo_RepoType interface{ isRepo_RepoType() }

type Repo_GitRepo struct {
	GitRepo *GitRepo `protobuf:"bytes,1,opt,name=git_repo,json=gitRepo,oneof"`
}

func (*Repo_GitRepo) isRepo_RepoType() {}

func (m *Repo) GetRepoType() isRepo_RepoType {
	if m != nil {
		return m.RepoType
	}
	return nil
}

func (m *Repo) GetGitRepo() *GitRepo {
	if x, ok := m.GetRepoType().(*Repo_GitRepo); ok {
		return x.GitRepo
	}
	return nil
}

// XXX_OneofFuncs is for the internal use of the proto package.
func (*Repo) XXX_OneofFuncs() (func(msg proto1.Message, b *proto1.Buffer) error, func(msg proto1.Message, tag, wire int, b *proto1.Buffer) (bool, error), func(msg proto1.Message) (n int), []interface{}) {
	return _Repo_OneofMarshaler, _Repo_OneofUnmarshaler, _Repo_OneofSizer, []interface{}{
		(*Repo_GitRepo)(nil),
	}
}

func _Repo_OneofMarshaler(msg proto1.Message, b *proto1.Buffer) error {
	m := msg.(*Repo)
	// repo_type
	switch x := m.RepoType.(type) {
	case *Repo_GitRepo:
		b.EncodeVarint(1<<3 | proto1.WireBytes)
		if err := b.EncodeMessage(x.GitRepo); err != nil {
			return err
		}
	case nil:
	default:
		return fmt.Errorf("Repo.RepoType has unexpected type %T", x)
	}
	return nil
}

func _Repo_OneofUnmarshaler(msg proto1.Message, tag, wire int, b *proto1.Buffer) (bool, error) {
	m := msg.(*Repo)
	switch tag {
	case 1: // repo_type.git_repo
		if wire != proto1.WireBytes {
			return true, proto1.ErrInternalBadWireType
		}
		msg := new(GitRepo)
		err := b.DecodeMessage(msg)
		m.RepoType = &Repo_GitRepo{msg}
		return true, err
	default:
		return false, nil
	}
}

func _Repo_OneofSizer(msg proto1.Message) (n int) {
	m := msg.(*Repo)
	// repo_type
	switch x := m.RepoType.(type) {
	case *Repo_GitRepo:
		s := proto1.Size(x.GitRepo)
		n += proto1.SizeVarint(1<<3 | proto1.WireBytes)
		n += proto1.SizeVarint(uint64(s))
		n += s
	case nil:
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in oneof", x))
	}
	return n
}

type GitRepo struct {
	LocalPath string `protobuf:"bytes,1,opt,name=local_path,json=localPath" json:"local_path,omitempty"`
}

func (m *GitRepo) Reset()                    { *m = GitRepo{} }
func (m *GitRepo) String() string            { return proto1.CompactTextString(m) }
func (*GitRepo) ProtoMessage()               {}
func (*GitRepo) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{4} }

func (m *GitRepo) GetLocalPath() string {
	if m != nil {
		return m.LocalPath
	}
	return ""
}

type Output struct {
	Stdout []byte `protobuf:"bytes,1,opt,name=stdout,proto3" json:"stdout,omitempty"`
	Stderr []byte `protobuf:"bytes,2,opt,name=stderr,proto3" json:"stderr,omitempty"`
}

func (m *Output) Reset()                    { *m = Output{} }
func (m *Output) String() string            { return proto1.CompactTextString(m) }
func (*Output) ProtoMessage()               {}
func (*Output) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{5} }

func (m *Output) GetStdout() []byte {
	if m != nil {
		return m.Stdout
	}
	return nil
}

func (m *Output) GetStderr() []byte {
	if m != nil {
		return m.Stderr
	}
	return nil
}

type CreateServiceReply struct {
	// Types that are valid to be assigned to Body:
	//	*CreateServiceReply_Output
	Body isCreateServiceReply_Body `protobuf_oneof:"body"`
}

func (m *CreateServiceReply) Reset()                    { *m = CreateServiceReply{} }
func (m *CreateServiceReply) String() string            { return proto1.CompactTextString(m) }
func (*CreateServiceReply) ProtoMessage()               {}
func (*CreateServiceReply) Descriptor() ([]byte, []int) { return fileDescriptor0, []int{6} }

type isCreateServiceReply_Body interface{ isCreateServiceReply_Body() }

type CreateServiceReply_Output struct {
	Output *Output `protobuf:"bytes,1,opt,name=output,oneof"`
}

func (*CreateServiceReply_Output) isCreateServiceReply_Body() {}

func (m *CreateServiceReply) GetBody() isCreateServiceReply_Body {
	if m != nil {
		return m.Body
	}
	return nil
}

func (m *CreateServiceReply) GetOutput() *Output {
	if x, ok := m.GetBody().(*CreateServiceReply_Output); ok {
		return x.Output
	}
	return nil
}

// XXX_OneofFuncs is for the internal use of the proto package.
func (*CreateServiceReply) XXX_OneofFuncs() (func(msg proto1.Message, b *proto1.Buffer) error, func(msg proto1.Message, tag, wire int, b *proto1.Buffer) (bool, error), func(msg proto1.Message) (n int), []interface{}) {
	return _CreateServiceReply_OneofMarshaler, _CreateServiceReply_OneofUnmarshaler, _CreateServiceReply_OneofSizer, []interface{}{
		(*CreateServiceReply_Output)(nil),
	}
}

func _CreateServiceReply_OneofMarshaler(msg proto1.Message, b *proto1.Buffer) error {
	m := msg.(*CreateServiceReply)
	// body
	switch x := m.Body.(type) {
	case *CreateServiceReply_Output:
		b.EncodeVarint(1<<3 | proto1.WireBytes)
		if err := b.EncodeMessage(x.Output); err != nil {
			return err
		}
	case nil:
	default:
		return fmt.Errorf("CreateServiceReply.Body has unexpected type %T", x)
	}
	return nil
}

func _CreateServiceReply_OneofUnmarshaler(msg proto1.Message, tag, wire int, b *proto1.Buffer) (bool, error) {
	m := msg.(*CreateServiceReply)
	switch tag {
	case 1: // body.output
		if wire != proto1.WireBytes {
			return true, proto1.ErrInternalBadWireType
		}
		msg := new(Output)
		err := b.DecodeMessage(msg)
		m.Body = &CreateServiceReply_Output{msg}
		return true, err
	default:
		return false, nil
	}
}

func _CreateServiceReply_OneofSizer(msg proto1.Message) (n int) {
	m := msg.(*CreateServiceReply)
	// body
	switch x := m.Body.(type) {
	case *CreateServiceReply_Output:
		s := proto1.Size(x.Output)
		n += proto1.SizeVarint(1<<3 | proto1.WireBytes)
		n += proto1.SizeVarint(uint64(s))
		n += s
	case nil:
	default:
		panic(fmt.Sprintf("proto: unexpected type %T in oneof", x))
	}
	return n
}

func init() {
	proto1.RegisterType((*Service)(nil), "daemon.Service")
	proto1.RegisterType((*Cmd)(nil), "daemon.Cmd")
	proto1.RegisterType((*Mount)(nil), "daemon.Mount")
	proto1.RegisterType((*Repo)(nil), "daemon.Repo")
	proto1.RegisterType((*GitRepo)(nil), "daemon.GitRepo")
	proto1.RegisterType((*Output)(nil), "daemon.Output")
	proto1.RegisterType((*CreateServiceReply)(nil), "daemon.CreateServiceReply")
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// Client API for Daemon service

type DaemonClient interface {
	CreateService(ctx context.Context, in *Service, opts ...grpc.CallOption) (Daemon_CreateServiceClient, error)
}

type daemonClient struct {
	cc *grpc.ClientConn
}

func NewDaemonClient(cc *grpc.ClientConn) DaemonClient {
	return &daemonClient{cc}
}

func (c *daemonClient) CreateService(ctx context.Context, in *Service, opts ...grpc.CallOption) (Daemon_CreateServiceClient, error) {
	stream, err := grpc.NewClientStream(ctx, &_Daemon_serviceDesc.Streams[0], c.cc, "/daemon.Daemon/CreateService", opts...)
	if err != nil {
		return nil, err
	}
	x := &daemonCreateServiceClient{stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

type Daemon_CreateServiceClient interface {
	Recv() (*CreateServiceReply, error)
	grpc.ClientStream
}

type daemonCreateServiceClient struct {
	grpc.ClientStream
}

func (x *daemonCreateServiceClient) Recv() (*CreateServiceReply, error) {
	m := new(CreateServiceReply)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

// Server API for Daemon service

type DaemonServer interface {
	CreateService(*Service, Daemon_CreateServiceServer) error
}

func RegisterDaemonServer(s *grpc.Server, srv DaemonServer) {
	s.RegisterService(&_Daemon_serviceDesc, srv)
}

func _Daemon_CreateService_Handler(srv interface{}, stream grpc.ServerStream) error {
	m := new(Service)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(DaemonServer).CreateService(m, &daemonCreateServiceServer{stream})
}

type Daemon_CreateServiceServer interface {
	Send(*CreateServiceReply) error
	grpc.ServerStream
}

type daemonCreateServiceServer struct {
	grpc.ServerStream
}

func (x *daemonCreateServiceServer) Send(m *CreateServiceReply) error {
	return x.ServerStream.SendMsg(m)
}

var _Daemon_serviceDesc = grpc.ServiceDesc{
	ServiceName: "daemon.Daemon",
	HandlerType: (*DaemonServer)(nil),
	Methods:     []grpc.MethodDesc{},
	Streams: []grpc.StreamDesc{
		{
			StreamName:    "CreateService",
			Handler:       _Daemon_CreateService_Handler,
			ServerStreams: true,
		},
	},
	Metadata: "internal/proto/daemon.proto",
}

func init() { proto1.RegisterFile("internal/proto/daemon.proto", fileDescriptor0) }

var fileDescriptor0 = []byte{
	// 442 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x64, 0x92, 0xc1, 0x8e, 0xd3, 0x30,
	0x10, 0x86, 0x5b, 0xda, 0xa6, 0xdb, 0xe9, 0xb6, 0x2b, 0xf9, 0x80, 0xb2, 0x8b, 0x90, 0x4a, 0xa4,
	0x15, 0x39, 0xa0, 0x06, 0xba, 0x97, 0x4a, 0x5c, 0xa0, 0x45, 0xb0, 0x42, 0x42, 0xac, 0x0c, 0x17,
	0xb8, 0x44, 0x6e, 0x32, 0xa4, 0x56, 0x9d, 0x38, 0x72, 0x26, 0xcb, 0xf6, 0xe5, 0x78, 0x36, 0x14,
	0xc7, 0x2d, 0x05, 0x4e, 0xf6, 0xfc, 0xf3, 0x67, 0xbe, 0x99, 0x8c, 0xe1, 0x89, 0x2c, 0x08, 0x4d,
	0x21, 0x54, 0x54, 0x1a, 0x4d, 0x3a, 0x4a, 0x05, 0xe6, 0xba, 0x98, 0xdb, 0x80, 0x79, 0x6d, 0x14,
	0xfc, 0xea, 0xc2, 0xf0, 0x0b, 0x9a, 0x7b, 0x99, 0x20, 0xbb, 0x84, 0xb3, 0xdd, 0xb2, 0x8a, 0xf7,
	0x22, 0x57, 0x7e, 0x77, 0xd6, 0x0d, 0x47, 0x7c, 0xb8, 0x5b, 0x56, 0xdf, 0x44, 0xae, 0xd8, 0x73,
	0xb8, 0x48, 0x75, 0xb2, 0x43, 0xf3, 0x43, 0x2a, 0x8c, 0x09, 0x1f, 0xc8, 0x7f, 0x64, 0x1d, 0xd3,
	0x3f, 0xf2, 0x57, 0x7c, 0x20, 0x76, 0x0d, 0x5e, 0xae, 0xeb, 0x82, 0x2a, 0xbf, 0x37, 0xeb, 0x85,
	0xe3, 0xc5, 0x64, 0xee, 0xb0, 0x9f, 0x1a, 0x95, 0xbb, 0x24, 0x7b, 0x06, 0x83, 0x8a, 0xb0, 0xac,
	0xfc, 0xbe, 0x75, 0x8d, 0x0f, 0xae, 0x75, 0x9e, 0xf2, 0x36, 0xc3, 0xae, 0x61, 0x7a, 0x8a, 0x14,
	0x99, 0x3f, 0xb0, 0xc4, 0xc9, 0x09, 0x51, 0x64, 0xc1, 0x25, 0xf4, 0xd6, 0x79, 0xca, 0x18, 0xf4,
	0x85, 0xc9, 0xee, 0xfd, 0xee, 0xac, 0x17, 0x8e, 0xb8, 0xbd, 0x07, 0x77, 0x30, 0xb0, 0x54, 0x36,
	0x83, 0xbe, 0xc1, 0x52, 0xdb, 0xa1, 0xc6, 0x8b, 0xf3, 0x03, 0x8c, 0x63, 0xa9, 0xb9, 0xcd, 0x34,
	0xb0, 0x44, 0x17, 0x24, 0x64, 0x81, 0x26, 0x2e, 0x05, 0x6d, 0xdd, 0x78, 0x93, 0xa3, 0x7a, 0x27,
	0x68, 0x1b, 0xbc, 0x85, 0x7e, 0xf3, 0x11, 0x7b, 0x01, 0x67, 0x99, 0xa4, 0xf8, 0xa4, 0xe8, 0xc5,
	0xa1, 0xe8, 0x07, 0x49, 0x8d, 0xe5, 0xb6, 0xc3, 0x87, 0x59, 0x7b, 0x5d, 0x8d, 0x61, 0xd4, 0x38,
	0x63, 0xda, 0x97, 0x18, 0x84, 0x30, 0x74, 0x16, 0xf6, 0x14, 0x40, 0xe9, 0x44, 0xa8, 0x16, 0xd8,
	0xfe, 0xf1, 0x91, 0x55, 0x2c, 0x6c, 0x09, 0xde, 0xe7, 0x9a, 0xca, 0x9a, 0xd8, 0x63, 0xf0, 0x2a,
	0x4a, 0x75, 0x4d, 0xd6, 0x74, 0xce, 0x5d, 0xe4, 0x74, 0x34, 0xc6, 0x76, 0xdb, 0xea, 0x68, 0x4c,
	0xf0, 0x1e, 0xd8, 0xda, 0xa0, 0x20, 0x74, 0x9b, 0xe5, 0x58, 0xaa, 0x3d, 0x0b, 0xc1, 0xd3, 0xb6,
	0x9e, 0x6b, 0x79, 0x7a, 0x68, 0xb9, 0xa5, 0xdc, 0x76, 0xb8, 0xcb, 0xaf, 0x3c, 0xe8, 0x6f, 0x74,
	0xba, 0x5f, 0x7c, 0x04, 0xef, 0x9d, 0xb5, 0xb0, 0x37, 0x30, 0xf9, 0xab, 0x22, 0x3b, 0xce, 0xeb,
	0x84, 0xab, 0xab, 0xe3, 0x0a, 0xff, 0x23, 0x07, 0x9d, 0x97, 0xdd, 0xd5, 0xcd, 0xf7, 0x57, 0x99,
	0xa4, 0x6d, 0xbd, 0x99, 0x27, 0x3a, 0x8f, 0x7e, 0xca, 0x22, 0xcd, 0xa5, 0x52, 0x58, 0x64, 0x11,
	0x49, 0x45, 0xd1, 0x3f, 0x6f, 0xf5, 0xb5, 0x3d, 0x36, 0x9e, 0x3d, 0x6e, 0x7e, 0x07, 0x00, 0x00,
	0xff, 0xff, 0x2c, 0xbc, 0x5e, 0xb0, 0xcb, 0x02, 0x00, 0x00,
}
