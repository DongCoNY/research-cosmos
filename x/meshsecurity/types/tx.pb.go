// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: osmosis/meshsecurity/v1beta1/tx.proto

package types

import (
	context "context"
	fmt "fmt"
	types "github.com/cosmos/cosmos-sdk/types"
	_ "github.com/cosmos/cosmos-sdk/types/msgservice"
	_ "github.com/cosmos/cosmos-sdk/types/tx/amino"
	_ "github.com/cosmos/gogoproto/gogoproto"
	grpc1 "github.com/cosmos/gogoproto/grpc"
	proto "github.com/cosmos/gogoproto/proto"
	grpc "google.golang.org/grpc"
	codes "google.golang.org/grpc/codes"
	status "google.golang.org/grpc/status"
	io "io"
	math "math"
	math_bits "math/bits"
)

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = fmt.Errorf
var _ = math.Inf

// This is a compile-time assertion to ensure that this generated file
// is compatible with the proto package it is being compiled against.
// A compilation error at this line likely means your copy of the
// proto package needs to be updated.
const _ = proto.GoGoProtoPackageIsVersion3 // please upgrade the proto package

// MsgSetVirtualStakingMaxCap creates or updates a maximum cap limit for virtual
// staking coins to the given contract.
type MsgSetVirtualStakingMaxCap struct {
	// Authority is the address that controls the module (defaults to x/gov unless
	// overwritten).
	Authority string `protobuf:"bytes,1,opt,name=authority,proto3" json:"authority,omitempty"`
	// Contract is the address of the smart contract that is given permission
	// do virtual staking which includes minting and burning staking tokens.
	Contract string `protobuf:"bytes,2,opt,name=contract,proto3" json:"contract,omitempty"`
	// MaxCap is the limit up this the virtual tokens can be minted.
	MaxCap types.Coin `protobuf:"bytes,3,opt,name=max_cap,json=maxCap,proto3" json:"max_cap"`
}

func (m *MsgSetVirtualStakingMaxCap) Reset()         { *m = MsgSetVirtualStakingMaxCap{} }
func (m *MsgSetVirtualStakingMaxCap) String() string { return proto.CompactTextString(m) }
func (*MsgSetVirtualStakingMaxCap) ProtoMessage()    {}
func (*MsgSetVirtualStakingMaxCap) Descriptor() ([]byte, []int) {
	return fileDescriptor_ca993316ec9770c4, []int{0}
}
func (m *MsgSetVirtualStakingMaxCap) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *MsgSetVirtualStakingMaxCap) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_MsgSetVirtualStakingMaxCap.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *MsgSetVirtualStakingMaxCap) XXX_Merge(src proto.Message) {
	xxx_messageInfo_MsgSetVirtualStakingMaxCap.Merge(m, src)
}
func (m *MsgSetVirtualStakingMaxCap) XXX_Size() int {
	return m.Size()
}
func (m *MsgSetVirtualStakingMaxCap) XXX_DiscardUnknown() {
	xxx_messageInfo_MsgSetVirtualStakingMaxCap.DiscardUnknown(m)
}

var xxx_messageInfo_MsgSetVirtualStakingMaxCap proto.InternalMessageInfo

// MsgSetVirtualStakingMaxCap returns result data.
type MsgSetVirtualStakingMaxCapResponse struct {
}

func (m *MsgSetVirtualStakingMaxCapResponse) Reset()         { *m = MsgSetVirtualStakingMaxCapResponse{} }
func (m *MsgSetVirtualStakingMaxCapResponse) String() string { return proto.CompactTextString(m) }
func (*MsgSetVirtualStakingMaxCapResponse) ProtoMessage()    {}
func (*MsgSetVirtualStakingMaxCapResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_ca993316ec9770c4, []int{1}
}
func (m *MsgSetVirtualStakingMaxCapResponse) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *MsgSetVirtualStakingMaxCapResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_MsgSetVirtualStakingMaxCapResponse.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *MsgSetVirtualStakingMaxCapResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_MsgSetVirtualStakingMaxCapResponse.Merge(m, src)
}
func (m *MsgSetVirtualStakingMaxCapResponse) XXX_Size() int {
	return m.Size()
}
func (m *MsgSetVirtualStakingMaxCapResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_MsgSetVirtualStakingMaxCapResponse.DiscardUnknown(m)
}

var xxx_messageInfo_MsgSetVirtualStakingMaxCapResponse proto.InternalMessageInfo

func init() {
	proto.RegisterType((*MsgSetVirtualStakingMaxCap)(nil), "osmosis.meshsecurity.v1beta1.MsgSetVirtualStakingMaxCap")
	proto.RegisterType((*MsgSetVirtualStakingMaxCapResponse)(nil), "osmosis.meshsecurity.v1beta1.MsgSetVirtualStakingMaxCapResponse")
}

func init() {
	proto.RegisterFile("osmosis/meshsecurity/v1beta1/tx.proto", fileDescriptor_ca993316ec9770c4)
}

var fileDescriptor_ca993316ec9770c4 = []byte{
	// 378 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0xe2, 0x52, 0xcd, 0x2f, 0xce, 0xcd,
	0x2f, 0xce, 0x2c, 0xd6, 0xcf, 0x4d, 0x2d, 0xce, 0x28, 0x4e, 0x4d, 0x2e, 0x2d, 0xca, 0x2c, 0xa9,
	0xd4, 0x2f, 0x33, 0x4c, 0x4a, 0x2d, 0x49, 0x34, 0xd4, 0x2f, 0xa9, 0xd0, 0x2b, 0x28, 0xca, 0x2f,
	0xc9, 0x17, 0x92, 0x81, 0x2a, 0xd3, 0x43, 0x56, 0xa6, 0x07, 0x55, 0x26, 0x25, 0x97, 0x0c, 0x96,
	0xd6, 0x4f, 0x4a, 0x2c, 0x4e, 0x85, 0xeb, 0x4d, 0xce, 0xcf, 0xcc, 0x83, 0xe8, 0x96, 0x12, 0x87,
	0xca, 0xe7, 0x16, 0xa7, 0xeb, 0x97, 0x19, 0x82, 0x28, 0xa8, 0x84, 0x48, 0x7a, 0x7e, 0x7a, 0x3e,
	0x98, 0xa9, 0x0f, 0x62, 0x41, 0x45, 0x05, 0x13, 0x73, 0x33, 0xf3, 0xf2, 0xf5, 0xc1, 0x24, 0x44,
	0x48, 0xe9, 0x0c, 0x23, 0x97, 0x94, 0x6f, 0x71, 0x7a, 0x70, 0x6a, 0x49, 0x58, 0x66, 0x51, 0x49,
	0x69, 0x62, 0x4e, 0x70, 0x49, 0x62, 0x76, 0x66, 0x5e, 0xba, 0x6f, 0x62, 0x85, 0x73, 0x62, 0x81,
	0x90, 0x0c, 0x17, 0x67, 0x62, 0x69, 0x49, 0x46, 0x3e, 0xc8, 0x55, 0x12, 0x8c, 0x0a, 0x8c, 0x1a,
	0x9c, 0x41, 0x08, 0x01, 0x21, 0x29, 0x2e, 0x8e, 0xe4, 0xfc, 0xbc, 0x92, 0xa2, 0xc4, 0xe4, 0x12,
	0x09, 0x26, 0xb0, 0x24, 0x9c, 0x2f, 0x64, 0xc1, 0xc5, 0x9e, 0x9b, 0x58, 0x11, 0x9f, 0x9c, 0x58,
	0x20, 0xc1, 0xac, 0xc0, 0xa8, 0xc1, 0x6d, 0x24, 0xa9, 0x07, 0x71, 0xac, 0x1e, 0xc8, 0x33, 0x30,
	0x1f, 0xea, 0x39, 0xe7, 0x67, 0xe6, 0x39, 0xb1, 0x9c, 0xb8, 0x27, 0xcf, 0x10, 0xc4, 0x96, 0x0b,
	0xb6, 0xd3, 0xca, 0xaa, 0xe9, 0xf9, 0x06, 0x2d, 0x84, 0x2d, 0x5d, 0xcf, 0x37, 0x68, 0xa9, 0xa3,
	0x04, 0x22, 0x6e, 0xf7, 0x2a, 0xa9, 0x70, 0x29, 0xe1, 0x96, 0x0d, 0x4a, 0x2d, 0x2e, 0xc8, 0xcf,
	0x2b, 0x4e, 0x35, 0x9a, 0xcb, 0xc8, 0xc5, 0xec, 0x5b, 0x9c, 0x2e, 0x34, 0x95, 0x91, 0x4b, 0x1c,
	0x97, 0xcf, 0x2d, 0xf4, 0xf0, 0xc5, 0x8c, 0x1e, 0x6e, 0x5b, 0xa4, 0x1c, 0xc8, 0xd5, 0x09, 0x73,
	0x9f, 0x53, 0xcc, 0x89, 0x87, 0x72, 0x0c, 0x27, 0x1e, 0xc9, 0x31, 0x5e, 0x78, 0x24, 0xc7, 0xf8,
	0xe0, 0x91, 0x1c, 0xe3, 0x84, 0xc7, 0x72, 0x0c, 0x17, 0x1e, 0xcb, 0x31, 0xdc, 0x78, 0x2c, 0xc7,
	0x10, 0x65, 0x97, 0x9e, 0x59, 0x92, 0x51, 0x9a, 0xa4, 0x97, 0x9c, 0x9f, 0xab, 0x0f, 0xb5, 0x49,
	0x37, 0x27, 0x31, 0x09, 0x92, 0xd2, 0x74, 0x61, 0xf6, 0xe9, 0x16, 0xa7, 0x64, 0xeb, 0x57, 0xa0,
	0xa6, 0xbe, 0x92, 0xca, 0x82, 0xd4, 0xe2, 0x24, 0x36, 0x70, 0xcc, 0x1b, 0x03, 0x02, 0x00, 0x00,
	0xff, 0xff, 0x7a, 0xbb, 0x3f, 0x0f, 0xa2, 0x02, 0x00, 0x00,
}

// Reference imports to suppress errors if they are not otherwise used.
var _ context.Context
var _ grpc.ClientConn

// This is a compile-time assertion to ensure that this generated file
// is compatible with the grpc package it is being compiled against.
const _ = grpc.SupportPackageIsVersion4

// MsgClient is the client API for Msg service.
//
// For semantics around ctx use and closing/ending streaming RPCs, please refer to https://godoc.org/google.golang.org/grpc#ClientConn.NewStream.
type MsgClient interface {
	// SetVirtualStakingMaxCap creates or updates a maximum cap limit for virtual
	// staking coins
	SetVirtualStakingMaxCap(ctx context.Context, in *MsgSetVirtualStakingMaxCap, opts ...grpc.CallOption) (*MsgSetVirtualStakingMaxCapResponse, error)
}

type msgClient struct {
	cc grpc1.ClientConn
}

func NewMsgClient(cc grpc1.ClientConn) MsgClient {
	return &msgClient{cc}
}

func (c *msgClient) SetVirtualStakingMaxCap(ctx context.Context, in *MsgSetVirtualStakingMaxCap, opts ...grpc.CallOption) (*MsgSetVirtualStakingMaxCapResponse, error) {
	out := new(MsgSetVirtualStakingMaxCapResponse)
	err := c.cc.Invoke(ctx, "/osmosis.meshsecurity.v1beta1.Msg/SetVirtualStakingMaxCap", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// MsgServer is the server API for Msg service.
type MsgServer interface {
	// SetVirtualStakingMaxCap creates or updates a maximum cap limit for virtual
	// staking coins
	SetVirtualStakingMaxCap(context.Context, *MsgSetVirtualStakingMaxCap) (*MsgSetVirtualStakingMaxCapResponse, error)
}

// UnimplementedMsgServer can be embedded to have forward compatible implementations.
type UnimplementedMsgServer struct {
}

func (*UnimplementedMsgServer) SetVirtualStakingMaxCap(ctx context.Context, req *MsgSetVirtualStakingMaxCap) (*MsgSetVirtualStakingMaxCapResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method SetVirtualStakingMaxCap not implemented")
}

func RegisterMsgServer(s grpc1.Server, srv MsgServer) {
	s.RegisterService(&_Msg_serviceDesc, srv)
}

func _Msg_SetVirtualStakingMaxCap_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MsgSetVirtualStakingMaxCap)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MsgServer).SetVirtualStakingMaxCap(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/osmosis.meshsecurity.v1beta1.Msg/SetVirtualStakingMaxCap",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MsgServer).SetVirtualStakingMaxCap(ctx, req.(*MsgSetVirtualStakingMaxCap))
	}
	return interceptor(ctx, in, info, handler)
}

var _Msg_serviceDesc = grpc.ServiceDesc{
	ServiceName: "osmosis.meshsecurity.v1beta1.Msg",
	HandlerType: (*MsgServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "SetVirtualStakingMaxCap",
			Handler:    _Msg_SetVirtualStakingMaxCap_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "osmosis/meshsecurity/v1beta1/tx.proto",
}

func (m *MsgSetVirtualStakingMaxCap) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *MsgSetVirtualStakingMaxCap) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *MsgSetVirtualStakingMaxCap) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	{
		size, err := m.MaxCap.MarshalToSizedBuffer(dAtA[:i])
		if err != nil {
			return 0, err
		}
		i -= size
		i = encodeVarintTx(dAtA, i, uint64(size))
	}
	i--
	dAtA[i] = 0x1a
	if len(m.Contract) > 0 {
		i -= len(m.Contract)
		copy(dAtA[i:], m.Contract)
		i = encodeVarintTx(dAtA, i, uint64(len(m.Contract)))
		i--
		dAtA[i] = 0x12
	}
	if len(m.Authority) > 0 {
		i -= len(m.Authority)
		copy(dAtA[i:], m.Authority)
		i = encodeVarintTx(dAtA, i, uint64(len(m.Authority)))
		i--
		dAtA[i] = 0xa
	}
	return len(dAtA) - i, nil
}

func (m *MsgSetVirtualStakingMaxCapResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *MsgSetVirtualStakingMaxCapResponse) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *MsgSetVirtualStakingMaxCapResponse) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	return len(dAtA) - i, nil
}

func encodeVarintTx(dAtA []byte, offset int, v uint64) int {
	offset -= sovTx(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *MsgSetVirtualStakingMaxCap) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	l = len(m.Authority)
	if l > 0 {
		n += 1 + l + sovTx(uint64(l))
	}
	l = len(m.Contract)
	if l > 0 {
		n += 1 + l + sovTx(uint64(l))
	}
	l = m.MaxCap.Size()
	n += 1 + l + sovTx(uint64(l))
	return n
}

func (m *MsgSetVirtualStakingMaxCapResponse) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	return n
}

func sovTx(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozTx(x uint64) (n int) {
	return sovTx(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *MsgSetVirtualStakingMaxCap) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowTx
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: MsgSetVirtualStakingMaxCap: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: MsgSetVirtualStakingMaxCap: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Authority", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTx
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthTx
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthTx
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Authority = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Contract", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTx
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				stringLen |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			intStringLen := int(stringLen)
			if intStringLen < 0 {
				return ErrInvalidLengthTx
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthTx
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Contract = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field MaxCap", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowTx
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				msglen |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if msglen < 0 {
				return ErrInvalidLengthTx
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthTx
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if err := m.MaxCap.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipTx(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthTx
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func (m *MsgSetVirtualStakingMaxCapResponse) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowTx
			}
			if iNdEx >= l {
				return io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= uint64(b&0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		fieldNum := int32(wire >> 3)
		wireType := int(wire & 0x7)
		if wireType == 4 {
			return fmt.Errorf("proto: MsgSetVirtualStakingMaxCapResponse: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: MsgSetVirtualStakingMaxCapResponse: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		default:
			iNdEx = preIndex
			skippy, err := skipTx(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthTx
			}
			if (iNdEx + skippy) > l {
				return io.ErrUnexpectedEOF
			}
			iNdEx += skippy
		}
	}

	if iNdEx > l {
		return io.ErrUnexpectedEOF
	}
	return nil
}
func skipTx(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowTx
			}
			if iNdEx >= l {
				return 0, io.ErrUnexpectedEOF
			}
			b := dAtA[iNdEx]
			iNdEx++
			wire |= (uint64(b) & 0x7F) << shift
			if b < 0x80 {
				break
			}
		}
		wireType := int(wire & 0x7)
		switch wireType {
		case 0:
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowTx
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				iNdEx++
				if dAtA[iNdEx-1] < 0x80 {
					break
				}
			}
		case 1:
			iNdEx += 8
		case 2:
			var length int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return 0, ErrIntOverflowTx
				}
				if iNdEx >= l {
					return 0, io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				length |= (int(b) & 0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			if length < 0 {
				return 0, ErrInvalidLengthTx
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupTx
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthTx
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthTx        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowTx          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupTx = fmt.Errorf("proto: unexpected end of group")
)
