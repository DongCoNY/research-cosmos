// Code generated by protoc-gen-gogo. DO NOT EDIT.
// source: auction/v1/msgs.proto

package types

import (
	context "context"
	fmt "fmt"
	types "github.com/cosmos/cosmos-sdk/types"
	_ "github.com/gogo/protobuf/gogoproto"
	grpc1 "github.com/gogo/protobuf/grpc"
	proto "github.com/gogo/protobuf/proto"
	_ "google.golang.org/genproto/googleapis/api/annotations"
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

// MsgBid is a message type for placing a bid on an auction
type MsgBid struct {
	AuctionId uint64      `protobuf:"varint,1,opt,name=auction_id,json=auctionId,proto3" json:"auction_id,omitempty"`
	Bidder    string      `protobuf:"bytes,2,opt,name=bidder,proto3" json:"bidder,omitempty"`
	Amount    *types.Coin `protobuf:"bytes,3,opt,name=amount,proto3" json:"amount,omitempty"`
}

func (m *MsgBid) Reset()         { *m = MsgBid{} }
func (m *MsgBid) String() string { return proto.CompactTextString(m) }
func (*MsgBid) ProtoMessage()    {}
func (*MsgBid) Descriptor() ([]byte, []int) {
	return fileDescriptor_b11bec07389e4372, []int{0}
}
func (m *MsgBid) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *MsgBid) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_MsgBid.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *MsgBid) XXX_Merge(src proto.Message) {
	xxx_messageInfo_MsgBid.Merge(m, src)
}
func (m *MsgBid) XXX_Size() int {
	return m.Size()
}
func (m *MsgBid) XXX_DiscardUnknown() {
	xxx_messageInfo_MsgBid.DiscardUnknown(m)
}

var xxx_messageInfo_MsgBid proto.InternalMessageInfo

func (m *MsgBid) GetAuctionId() uint64 {
	if m != nil {
		return m.AuctionId
	}
	return 0
}

func (m *MsgBid) GetBidder() string {
	if m != nil {
		return m.Bidder
	}
	return ""
}

func (m *MsgBid) GetAmount() *types.Coin {
	if m != nil {
		return m.Amount
	}
	return nil
}

type MsgBidResponse struct {
	Success bool `protobuf:"varint,1,opt,name=success,proto3" json:"success,omitempty"`
}

func (m *MsgBidResponse) Reset()         { *m = MsgBidResponse{} }
func (m *MsgBidResponse) String() string { return proto.CompactTextString(m) }
func (*MsgBidResponse) ProtoMessage()    {}
func (*MsgBidResponse) Descriptor() ([]byte, []int) {
	return fileDescriptor_b11bec07389e4372, []int{1}
}
func (m *MsgBidResponse) XXX_Unmarshal(b []byte) error {
	return m.Unmarshal(b)
}
func (m *MsgBidResponse) XXX_Marshal(b []byte, deterministic bool) ([]byte, error) {
	if deterministic {
		return xxx_messageInfo_MsgBidResponse.Marshal(b, m, deterministic)
	} else {
		b = b[:cap(b)]
		n, err := m.MarshalToSizedBuffer(b)
		if err != nil {
			return nil, err
		}
		return b[:n], nil
	}
}
func (m *MsgBidResponse) XXX_Merge(src proto.Message) {
	xxx_messageInfo_MsgBidResponse.Merge(m, src)
}
func (m *MsgBidResponse) XXX_Size() int {
	return m.Size()
}
func (m *MsgBidResponse) XXX_DiscardUnknown() {
	xxx_messageInfo_MsgBidResponse.DiscardUnknown(m)
}

var xxx_messageInfo_MsgBidResponse proto.InternalMessageInfo

func (m *MsgBidResponse) GetSuccess() bool {
	if m != nil {
		return m.Success
	}
	return false
}

func init() {
	proto.RegisterType((*MsgBid)(nil), "auction.v1.MsgBid")
	proto.RegisterType((*MsgBidResponse)(nil), "auction.v1.MsgBidResponse")
}

func init() { proto.RegisterFile("auction/v1/msgs.proto", fileDescriptor_b11bec07389e4372) }

var fileDescriptor_b11bec07389e4372 = []byte{
	// 370 bytes of a gzipped FileDescriptorProto
	0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02, 0xff, 0x64, 0x91, 0xcf, 0x6a, 0xdb, 0x40,
	0x10, 0xc6, 0xbd, 0x76, 0x51, 0xeb, 0x2d, 0x6d, 0xa9, 0x68, 0xa9, 0x2a, 0x8a, 0x6c, 0x74, 0x32,
	0xa5, 0xdd, 0x45, 0xee, 0xb9, 0x14, 0xab, 0x87, 0xd2, 0x83, 0x7b, 0x10, 0xf4, 0xd0, 0x5e, 0xca,
	0x4a, 0xbb, 0x6c, 0x17, 0x2c, 0x8d, 0xd0, 0xae, 0x44, 0x7c, 0xcd, 0x13, 0x04, 0x72, 0xca, 0x1b,
	0xe5, 0x68, 0xc8, 0x25, 0x27, 0x13, 0xe4, 0x3c, 0x48, 0xd0, 0xbf, 0x24, 0x24, 0xb7, 0x99, 0x6f,
	0x66, 0xbe, 0xfd, 0xed, 0x0c, 0x7e, 0xcb, 0xca, 0xc4, 0x28, 0xc8, 0x68, 0x15, 0xd0, 0x54, 0x4b,
	0x4d, 0xf2, 0x02, 0x0c, 0xd8, 0xb8, 0x97, 0x49, 0x15, 0xb8, 0x6f, 0x24, 0x48, 0x68, 0x65, 0xda,
	0x44, 0x5d, 0x87, 0xeb, 0x25, 0xa0, 0x53, 0xd0, 0x34, 0x66, 0x5a, 0xd0, 0x2a, 0x88, 0x85, 0x61,
	0x01, 0x4d, 0x40, 0x65, 0x7d, 0xfd, 0x83, 0x04, 0x90, 0x1b, 0x41, 0x59, 0xae, 0x28, 0xcb, 0x32,
	0x30, 0xac, 0xf1, 0xeb, 0xfd, 0xfd, 0x33, 0x84, 0xad, 0xb5, 0x96, 0xa1, 0xe2, 0xf6, 0x27, 0x3c,
	0x3c, 0xf6, 0x4f, 0x71, 0x07, 0xcd, 0xd1, 0xe2, 0x49, 0xf8, 0xa2, 0xde, 0xcf, 0xa6, 0xab, 0x4e,
	0xfd, 0xc9, 0xa3, 0x29, 0x1b, 0x42, 0xdb, 0xc7, 0x56, 0xac, 0x38, 0x17, 0x85, 0x33, 0x9e, 0xa3,
	0xc5, 0x34, 0xc4, 0xf5, 0x7e, 0x66, 0x85, 0xad, 0x12, 0xf5, 0x15, 0xfb, 0x2b, 0xb6, 0x58, 0x0a,
	0x65, 0x66, 0x9c, 0xc9, 0x1c, 0x2d, 0x9e, 0x2f, 0xdf, 0x93, 0x8e, 0x95, 0x34, 0xac, 0xa4, 0x67,
	0x25, 0xdf, 0x41, 0x65, 0xdd, 0xf8, 0xaa, 0x6d, 0x8e, 0xfa, 0x21, 0xff, 0x23, 0x7e, 0xd9, 0xa1,
	0x45, 0x42, 0xe7, 0x90, 0x69, 0x61, 0x3b, 0xf8, 0xa9, 0x2e, 0x93, 0x44, 0x68, 0xdd, 0xf2, 0x3d,
	0x8b, 0x86, 0x74, 0xf9, 0x1b, 0x4f, 0xd6, 0x5a, 0xda, 0xbf, 0xf0, 0xa4, 0xf9, 0x8a, 0x4d, 0xee,
	0xd6, 0x46, 0x3a, 0x0f, 0xd7, 0x7d, 0xac, 0x0d, 0xbe, 0xfe, 0xbb, 0xe3, 0x8b, 0xeb, 0xd3, 0xf1,
	0x6b, 0xff, 0x15, 0xbd, 0x77, 0x85, 0x58, 0xf1, 0xf0, 0xcf, 0x79, 0xed, 0xa1, 0x5d, 0xed, 0xa1,
	0xab, 0xda, 0x43, 0x27, 0x07, 0x6f, 0xb4, 0x3b, 0x78, 0xa3, 0xcb, 0x83, 0x37, 0xfa, 0xfb, 0x4d,
	0x2a, 0xf3, 0xbf, 0x8c, 0x49, 0x02, 0x29, 0xfd, 0x51, 0xb0, 0x4a, 0x99, 0xed, 0xe7, 0xb0, 0x50,
	0x5c, 0x8a, 0x87, 0x69, 0x0a, 0xbc, 0xdc, 0x08, 0x7a, 0x74, 0xeb, 0x6d, 0xb6, 0xb9, 0xd0, 0xb1,
	0xd5, 0x1e, 0xe0, 0xcb, 0x4d, 0x00, 0x00, 0x00, 0xff, 0xff, 0x88, 0xd2, 0xd6, 0x2e, 0xf9, 0x01,
	0x00, 0x00,
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
	Bid(ctx context.Context, in *MsgBid, opts ...grpc.CallOption) (*MsgBidResponse, error)
}

type msgClient struct {
	cc grpc1.ClientConn
}

func NewMsgClient(cc grpc1.ClientConn) MsgClient {
	return &msgClient{cc}
}

func (c *msgClient) Bid(ctx context.Context, in *MsgBid, opts ...grpc.CallOption) (*MsgBidResponse, error) {
	out := new(MsgBidResponse)
	err := c.cc.Invoke(ctx, "/auction.v1.Msg/Bid", in, out, opts...)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// MsgServer is the server API for Msg service.
type MsgServer interface {
	Bid(context.Context, *MsgBid) (*MsgBidResponse, error)
}

// UnimplementedMsgServer can be embedded to have forward compatible implementations.
type UnimplementedMsgServer struct {
}

func (*UnimplementedMsgServer) Bid(ctx context.Context, req *MsgBid) (*MsgBidResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "method Bid not implemented")
}

func RegisterMsgServer(s grpc1.Server, srv MsgServer) {
	s.RegisterService(&_Msg_serviceDesc, srv)
}

func _Msg_Bid_Handler(srv interface{}, ctx context.Context, dec func(interface{}) error, interceptor grpc.UnaryServerInterceptor) (interface{}, error) {
	in := new(MsgBid)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(MsgServer).Bid(ctx, in)
	}
	info := &grpc.UnaryServerInfo{
		Server:     srv,
		FullMethod: "/auction.v1.Msg/Bid",
	}
	handler := func(ctx context.Context, req interface{}) (interface{}, error) {
		return srv.(MsgServer).Bid(ctx, req.(*MsgBid))
	}
	return interceptor(ctx, in, info, handler)
}

var _Msg_serviceDesc = grpc.ServiceDesc{
	ServiceName: "auction.v1.Msg",
	HandlerType: (*MsgServer)(nil),
	Methods: []grpc.MethodDesc{
		{
			MethodName: "Bid",
			Handler:    _Msg_Bid_Handler,
		},
	},
	Streams:  []grpc.StreamDesc{},
	Metadata: "auction/v1/msgs.proto",
}

func (m *MsgBid) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *MsgBid) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *MsgBid) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Amount != nil {
		{
			size, err := m.Amount.MarshalToSizedBuffer(dAtA[:i])
			if err != nil {
				return 0, err
			}
			i -= size
			i = encodeVarintMsgs(dAtA, i, uint64(size))
		}
		i--
		dAtA[i] = 0x1a
	}
	if len(m.Bidder) > 0 {
		i -= len(m.Bidder)
		copy(dAtA[i:], m.Bidder)
		i = encodeVarintMsgs(dAtA, i, uint64(len(m.Bidder)))
		i--
		dAtA[i] = 0x12
	}
	if m.AuctionId != 0 {
		i = encodeVarintMsgs(dAtA, i, uint64(m.AuctionId))
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func (m *MsgBidResponse) Marshal() (dAtA []byte, err error) {
	size := m.Size()
	dAtA = make([]byte, size)
	n, err := m.MarshalToSizedBuffer(dAtA[:size])
	if err != nil {
		return nil, err
	}
	return dAtA[:n], nil
}

func (m *MsgBidResponse) MarshalTo(dAtA []byte) (int, error) {
	size := m.Size()
	return m.MarshalToSizedBuffer(dAtA[:size])
}

func (m *MsgBidResponse) MarshalToSizedBuffer(dAtA []byte) (int, error) {
	i := len(dAtA)
	_ = i
	var l int
	_ = l
	if m.Success {
		i--
		if m.Success {
			dAtA[i] = 1
		} else {
			dAtA[i] = 0
		}
		i--
		dAtA[i] = 0x8
	}
	return len(dAtA) - i, nil
}

func encodeVarintMsgs(dAtA []byte, offset int, v uint64) int {
	offset -= sovMsgs(v)
	base := offset
	for v >= 1<<7 {
		dAtA[offset] = uint8(v&0x7f | 0x80)
		v >>= 7
		offset++
	}
	dAtA[offset] = uint8(v)
	return base
}
func (m *MsgBid) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.AuctionId != 0 {
		n += 1 + sovMsgs(uint64(m.AuctionId))
	}
	l = len(m.Bidder)
	if l > 0 {
		n += 1 + l + sovMsgs(uint64(l))
	}
	if m.Amount != nil {
		l = m.Amount.Size()
		n += 1 + l + sovMsgs(uint64(l))
	}
	return n
}

func (m *MsgBidResponse) Size() (n int) {
	if m == nil {
		return 0
	}
	var l int
	_ = l
	if m.Success {
		n += 2
	}
	return n
}

func sovMsgs(x uint64) (n int) {
	return (math_bits.Len64(x|1) + 6) / 7
}
func sozMsgs(x uint64) (n int) {
	return sovMsgs(uint64((x << 1) ^ uint64((int64(x) >> 63))))
}
func (m *MsgBid) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowMsgs
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
			return fmt.Errorf("proto: MsgBid: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: MsgBid: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field AuctionId", wireType)
			}
			m.AuctionId = 0
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMsgs
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				m.AuctionId |= uint64(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
		case 2:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Bidder", wireType)
			}
			var stringLen uint64
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMsgs
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
				return ErrInvalidLengthMsgs
			}
			postIndex := iNdEx + intStringLen
			if postIndex < 0 {
				return ErrInvalidLengthMsgs
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			m.Bidder = string(dAtA[iNdEx:postIndex])
			iNdEx = postIndex
		case 3:
			if wireType != 2 {
				return fmt.Errorf("proto: wrong wireType = %d for field Amount", wireType)
			}
			var msglen int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMsgs
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
				return ErrInvalidLengthMsgs
			}
			postIndex := iNdEx + msglen
			if postIndex < 0 {
				return ErrInvalidLengthMsgs
			}
			if postIndex > l {
				return io.ErrUnexpectedEOF
			}
			if m.Amount == nil {
				m.Amount = &types.Coin{}
			}
			if err := m.Amount.Unmarshal(dAtA[iNdEx:postIndex]); err != nil {
				return err
			}
			iNdEx = postIndex
		default:
			iNdEx = preIndex
			skippy, err := skipMsgs(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthMsgs
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
func (m *MsgBidResponse) Unmarshal(dAtA []byte) error {
	l := len(dAtA)
	iNdEx := 0
	for iNdEx < l {
		preIndex := iNdEx
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return ErrIntOverflowMsgs
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
			return fmt.Errorf("proto: MsgBidResponse: wiretype end group for non-group")
		}
		if fieldNum <= 0 {
			return fmt.Errorf("proto: MsgBidResponse: illegal tag %d (wire type %d)", fieldNum, wire)
		}
		switch fieldNum {
		case 1:
			if wireType != 0 {
				return fmt.Errorf("proto: wrong wireType = %d for field Success", wireType)
			}
			var v int
			for shift := uint(0); ; shift += 7 {
				if shift >= 64 {
					return ErrIntOverflowMsgs
				}
				if iNdEx >= l {
					return io.ErrUnexpectedEOF
				}
				b := dAtA[iNdEx]
				iNdEx++
				v |= int(b&0x7F) << shift
				if b < 0x80 {
					break
				}
			}
			m.Success = bool(v != 0)
		default:
			iNdEx = preIndex
			skippy, err := skipMsgs(dAtA[iNdEx:])
			if err != nil {
				return err
			}
			if (skippy < 0) || (iNdEx+skippy) < 0 {
				return ErrInvalidLengthMsgs
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
func skipMsgs(dAtA []byte) (n int, err error) {
	l := len(dAtA)
	iNdEx := 0
	depth := 0
	for iNdEx < l {
		var wire uint64
		for shift := uint(0); ; shift += 7 {
			if shift >= 64 {
				return 0, ErrIntOverflowMsgs
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
					return 0, ErrIntOverflowMsgs
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
					return 0, ErrIntOverflowMsgs
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
				return 0, ErrInvalidLengthMsgs
			}
			iNdEx += length
		case 3:
			depth++
		case 4:
			if depth == 0 {
				return 0, ErrUnexpectedEndOfGroupMsgs
			}
			depth--
		case 5:
			iNdEx += 4
		default:
			return 0, fmt.Errorf("proto: illegal wireType %d", wireType)
		}
		if iNdEx < 0 {
			return 0, ErrInvalidLengthMsgs
		}
		if depth == 0 {
			return iNdEx, nil
		}
	}
	return 0, io.ErrUnexpectedEOF
}

var (
	ErrInvalidLengthMsgs        = fmt.Errorf("proto: negative length found during unmarshaling")
	ErrIntOverflowMsgs          = fmt.Errorf("proto: integer overflow")
	ErrUnexpectedEndOfGroupMsgs = fmt.Errorf("proto: unexpected end of group")
)
