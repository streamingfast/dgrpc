// Code generated by protoc-gen-go. DO NOT EDIT.
// versions:
// 	protoc-gen-go v1.31.0
// 	protoc        (unknown)
// source: acme/v1/acme.proto

package pbacme

import (
	protoreflect "google.golang.org/protobuf/reflect/protoreflect"
	protoimpl "google.golang.org/protobuf/runtime/protoimpl"
	reflect "reflect"
	sync "sync"
)

const (
	// Verify that this generated code is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(20 - protoimpl.MinVersion)
	// Verify that runtime/protoimpl is sufficiently up-to-date.
	_ = protoimpl.EnforceVersion(protoimpl.MaxVersion - 20)
)

type GetPingRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ClientId              string `protobuf:"bytes,1,opt,name=client_id,json=clientId,proto3" json:"client_id,omitempty"`
	Message               string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
	ResponseDelayInMillis uint64 `protobuf:"varint,3,opt,name=response_delay_in_millis,json=responseDelayInMillis,proto3" json:"response_delay_in_millis,omitempty"`
}

func (x *GetPingRequest) Reset() {
	*x = GetPingRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_acme_v1_acme_proto_msgTypes[0]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *GetPingRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*GetPingRequest) ProtoMessage() {}

func (x *GetPingRequest) ProtoReflect() protoreflect.Message {
	mi := &file_acme_v1_acme_proto_msgTypes[0]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use GetPingRequest.ProtoReflect.Descriptor instead.
func (*GetPingRequest) Descriptor() ([]byte, []int) {
	return file_acme_v1_acme_proto_rawDescGZIP(), []int{0}
}

func (x *GetPingRequest) GetClientId() string {
	if x != nil {
		return x.ClientId
	}
	return ""
}

func (x *GetPingRequest) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

func (x *GetPingRequest) GetResponseDelayInMillis() uint64 {
	if x != nil {
		return x.ResponseDelayInMillis
	}
	return 0
}

type StreamPingRequest struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ClientId              string `protobuf:"bytes,1,opt,name=client_id,json=clientId,proto3" json:"client_id,omitempty"`
	Message               string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
	ResponseDelayInMillis uint64 `protobuf:"varint,3,opt,name=response_delay_in_millis,json=responseDelayInMillis,proto3" json:"response_delay_in_millis,omitempty"`
	TerminatesAfterMillis uint64 `protobuf:"varint,4,opt,name=terminates_after_millis,json=terminatesAfterMillis,proto3" json:"terminates_after_millis,omitempty"`
}

func (x *StreamPingRequest) Reset() {
	*x = StreamPingRequest{}
	if protoimpl.UnsafeEnabled {
		mi := &file_acme_v1_acme_proto_msgTypes[1]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *StreamPingRequest) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*StreamPingRequest) ProtoMessage() {}

func (x *StreamPingRequest) ProtoReflect() protoreflect.Message {
	mi := &file_acme_v1_acme_proto_msgTypes[1]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use StreamPingRequest.ProtoReflect.Descriptor instead.
func (*StreamPingRequest) Descriptor() ([]byte, []int) {
	return file_acme_v1_acme_proto_rawDescGZIP(), []int{1}
}

func (x *StreamPingRequest) GetClientId() string {
	if x != nil {
		return x.ClientId
	}
	return ""
}

func (x *StreamPingRequest) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

func (x *StreamPingRequest) GetResponseDelayInMillis() uint64 {
	if x != nil {
		return x.ResponseDelayInMillis
	}
	return 0
}

func (x *StreamPingRequest) GetTerminatesAfterMillis() uint64 {
	if x != nil {
		return x.TerminatesAfterMillis
	}
	return 0
}

type PingResponse struct {
	state         protoimpl.MessageState
	sizeCache     protoimpl.SizeCache
	unknownFields protoimpl.UnknownFields

	ServerId string `protobuf:"bytes,1,opt,name=server_id,json=serverId,proto3" json:"server_id,omitempty"`
	Message  string `protobuf:"bytes,2,opt,name=message,proto3" json:"message,omitempty"`
}

func (x *PingResponse) Reset() {
	*x = PingResponse{}
	if protoimpl.UnsafeEnabled {
		mi := &file_acme_v1_acme_proto_msgTypes[2]
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		ms.StoreMessageInfo(mi)
	}
}

func (x *PingResponse) String() string {
	return protoimpl.X.MessageStringOf(x)
}

func (*PingResponse) ProtoMessage() {}

func (x *PingResponse) ProtoReflect() protoreflect.Message {
	mi := &file_acme_v1_acme_proto_msgTypes[2]
	if protoimpl.UnsafeEnabled && x != nil {
		ms := protoimpl.X.MessageStateOf(protoimpl.Pointer(x))
		if ms.LoadMessageInfo() == nil {
			ms.StoreMessageInfo(mi)
		}
		return ms
	}
	return mi.MessageOf(x)
}

// Deprecated: Use PingResponse.ProtoReflect.Descriptor instead.
func (*PingResponse) Descriptor() ([]byte, []int) {
	return file_acme_v1_acme_proto_rawDescGZIP(), []int{2}
}

func (x *PingResponse) GetServerId() string {
	if x != nil {
		return x.ServerId
	}
	return ""
}

func (x *PingResponse) GetMessage() string {
	if x != nil {
		return x.Message
	}
	return ""
}

var File_acme_v1_acme_proto protoreflect.FileDescriptor

var file_acme_v1_acme_proto_rawDesc = []byte{
	0x0a, 0x12, 0x61, 0x63, 0x6d, 0x65, 0x2f, 0x76, 0x31, 0x2f, 0x61, 0x63, 0x6d, 0x65, 0x2e, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x12, 0x07, 0x61, 0x63, 0x6d, 0x65, 0x2e, 0x76, 0x31, 0x22, 0x80, 0x01,
	0x0a, 0x0e, 0x47, 0x65, 0x74, 0x50, 0x69, 0x6e, 0x67, 0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74,
	0x12, 0x1b, 0x0a, 0x09, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20,
	0x01, 0x28, 0x09, 0x52, 0x08, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74, 0x49, 0x64, 0x12, 0x18, 0x0a,
	0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07,
	0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x37, 0x0a, 0x18, 0x72, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x5f, 0x64, 0x65, 0x6c, 0x61, 0x79, 0x5f, 0x69, 0x6e, 0x5f, 0x6d, 0x69, 0x6c,
	0x6c, 0x69, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x04, 0x52, 0x15, 0x72, 0x65, 0x73, 0x70, 0x6f,
	0x6e, 0x73, 0x65, 0x44, 0x65, 0x6c, 0x61, 0x79, 0x49, 0x6e, 0x4d, 0x69, 0x6c, 0x6c, 0x69, 0x73,
	0x22, 0xbb, 0x01, 0x0a, 0x11, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x50, 0x69, 0x6e, 0x67, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x12, 0x1b, 0x0a, 0x09, 0x63, 0x6c, 0x69, 0x65, 0x6e, 0x74,
	0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28, 0x09, 0x52, 0x08, 0x63, 0x6c, 0x69, 0x65, 0x6e,
	0x74, 0x49, 0x64, 0x12, 0x18, 0x0a, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x02,
	0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6d, 0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x12, 0x37, 0x0a,
	0x18, 0x72, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x5f, 0x64, 0x65, 0x6c, 0x61, 0x79, 0x5f,
	0x69, 0x6e, 0x5f, 0x6d, 0x69, 0x6c, 0x6c, 0x69, 0x73, 0x18, 0x03, 0x20, 0x01, 0x28, 0x04, 0x52,
	0x15, 0x72, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x44, 0x65, 0x6c, 0x61, 0x79, 0x49, 0x6e,
	0x4d, 0x69, 0x6c, 0x6c, 0x69, 0x73, 0x12, 0x36, 0x0a, 0x17, 0x74, 0x65, 0x72, 0x6d, 0x69, 0x6e,
	0x61, 0x74, 0x65, 0x73, 0x5f, 0x61, 0x66, 0x74, 0x65, 0x72, 0x5f, 0x6d, 0x69, 0x6c, 0x6c, 0x69,
	0x73, 0x18, 0x04, 0x20, 0x01, 0x28, 0x04, 0x52, 0x15, 0x74, 0x65, 0x72, 0x6d, 0x69, 0x6e, 0x61,
	0x74, 0x65, 0x73, 0x41, 0x66, 0x74, 0x65, 0x72, 0x4d, 0x69, 0x6c, 0x6c, 0x69, 0x73, 0x22, 0x45,
	0x0a, 0x0c, 0x50, 0x69, 0x6e, 0x67, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x1b,
	0x0a, 0x09, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x5f, 0x69, 0x64, 0x18, 0x01, 0x20, 0x01, 0x28,
	0x09, 0x52, 0x08, 0x73, 0x65, 0x72, 0x76, 0x65, 0x72, 0x49, 0x64, 0x12, 0x18, 0x0a, 0x07, 0x6d,
	0x65, 0x73, 0x73, 0x61, 0x67, 0x65, 0x18, 0x02, 0x20, 0x01, 0x28, 0x09, 0x52, 0x07, 0x6d, 0x65,
	0x73, 0x73, 0x61, 0x67, 0x65, 0x32, 0x88, 0x01, 0x0a, 0x08, 0x50, 0x69, 0x6e, 0x67, 0x50, 0x6f,
	0x6e, 0x67, 0x12, 0x39, 0x0a, 0x07, 0x47, 0x65, 0x74, 0x50, 0x69, 0x6e, 0x67, 0x12, 0x17, 0x2e,
	0x61, 0x63, 0x6d, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x47, 0x65, 0x74, 0x50, 0x69, 0x6e, 0x67, 0x52,
	0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x15, 0x2e, 0x61, 0x63, 0x6d, 0x65, 0x2e, 0x76, 0x31,
	0x2e, 0x50, 0x69, 0x6e, 0x67, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x12, 0x41, 0x0a,
	0x0a, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x50, 0x69, 0x6e, 0x67, 0x12, 0x1a, 0x2e, 0x61, 0x63,
	0x6d, 0x65, 0x2e, 0x76, 0x31, 0x2e, 0x53, 0x74, 0x72, 0x65, 0x61, 0x6d, 0x50, 0x69, 0x6e, 0x67,
	0x52, 0x65, 0x71, 0x75, 0x65, 0x73, 0x74, 0x1a, 0x15, 0x2e, 0x61, 0x63, 0x6d, 0x65, 0x2e, 0x76,
	0x31, 0x2e, 0x50, 0x69, 0x6e, 0x67, 0x52, 0x65, 0x73, 0x70, 0x6f, 0x6e, 0x73, 0x65, 0x30, 0x01,
	0x42, 0x3b, 0x5a, 0x39, 0x67, 0x69, 0x74, 0x68, 0x75, 0x62, 0x2e, 0x63, 0x6f, 0x6d, 0x2f, 0x73,
	0x74, 0x72, 0x65, 0x61, 0x6d, 0x69, 0x6e, 0x67, 0x66, 0x61, 0x73, 0x74, 0x2f, 0x64, 0x67, 0x72,
	0x70, 0x63, 0x2f, 0x65, 0x78, 0x61, 0x6d, 0x70, 0x6c, 0x65, 0x73, 0x2f, 0x70, 0x62, 0x2f, 0x61,
	0x63, 0x6d, 0x65, 0x2f, 0x76, 0x31, 0x3b, 0x70, 0x62, 0x61, 0x63, 0x6d, 0x65, 0x62, 0x06, 0x70,
	0x72, 0x6f, 0x74, 0x6f, 0x33,
}

var (
	file_acme_v1_acme_proto_rawDescOnce sync.Once
	file_acme_v1_acme_proto_rawDescData = file_acme_v1_acme_proto_rawDesc
)

func file_acme_v1_acme_proto_rawDescGZIP() []byte {
	file_acme_v1_acme_proto_rawDescOnce.Do(func() {
		file_acme_v1_acme_proto_rawDescData = protoimpl.X.CompressGZIP(file_acme_v1_acme_proto_rawDescData)
	})
	return file_acme_v1_acme_proto_rawDescData
}

var file_acme_v1_acme_proto_msgTypes = make([]protoimpl.MessageInfo, 3)
var file_acme_v1_acme_proto_goTypes = []interface{}{
	(*GetPingRequest)(nil),    // 0: acme.v1.GetPingRequest
	(*StreamPingRequest)(nil), // 1: acme.v1.StreamPingRequest
	(*PingResponse)(nil),      // 2: acme.v1.PingResponse
}
var file_acme_v1_acme_proto_depIdxs = []int32{
	0, // 0: acme.v1.PingPong.GetPing:input_type -> acme.v1.GetPingRequest
	1, // 1: acme.v1.PingPong.StreamPing:input_type -> acme.v1.StreamPingRequest
	2, // 2: acme.v1.PingPong.GetPing:output_type -> acme.v1.PingResponse
	2, // 3: acme.v1.PingPong.StreamPing:output_type -> acme.v1.PingResponse
	2, // [2:4] is the sub-list for method output_type
	0, // [0:2] is the sub-list for method input_type
	0, // [0:0] is the sub-list for extension type_name
	0, // [0:0] is the sub-list for extension extendee
	0, // [0:0] is the sub-list for field type_name
}

func init() { file_acme_v1_acme_proto_init() }
func file_acme_v1_acme_proto_init() {
	if File_acme_v1_acme_proto != nil {
		return
	}
	if !protoimpl.UnsafeEnabled {
		file_acme_v1_acme_proto_msgTypes[0].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*GetPingRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_acme_v1_acme_proto_msgTypes[1].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*StreamPingRequest); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
		file_acme_v1_acme_proto_msgTypes[2].Exporter = func(v interface{}, i int) interface{} {
			switch v := v.(*PingResponse); i {
			case 0:
				return &v.state
			case 1:
				return &v.sizeCache
			case 2:
				return &v.unknownFields
			default:
				return nil
			}
		}
	}
	type x struct{}
	out := protoimpl.TypeBuilder{
		File: protoimpl.DescBuilder{
			GoPackagePath: reflect.TypeOf(x{}).PkgPath(),
			RawDescriptor: file_acme_v1_acme_proto_rawDesc,
			NumEnums:      0,
			NumMessages:   3,
			NumExtensions: 0,
			NumServices:   1,
		},
		GoTypes:           file_acme_v1_acme_proto_goTypes,
		DependencyIndexes: file_acme_v1_acme_proto_depIdxs,
		MessageInfos:      file_acme_v1_acme_proto_msgTypes,
	}.Build()
	File_acme_v1_acme_proto = out.File
	file_acme_v1_acme_proto_rawDesc = nil
	file_acme_v1_acme_proto_goTypes = nil
	file_acme_v1_acme_proto_depIdxs = nil
}
