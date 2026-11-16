// 【通常无需修改｜R05、R06、R11】负责网关与 game 之间的请求响应打包；普通业务只使用具体协议结构，不手写编码。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
package clusterlink

import (
	"reflect"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/haihhhhh/ht_server_frame/access"
	pb "github.com/haihhhhh/ht_server_game_template/common/protocol/generated"
)

// Request 在内部 RPC 边界统一确定请求类型并解码，业务只接收具体协议对象。
func (c Command) Request() (proto.Message, error) {
	request, err := c.Action.newRequest()
	if err != nil {
		return nil, err
	}
	if request == nil {
		if len(c.Payload) != 0 {
			return nil, access.ErrForbidden
		}
		return nil, nil
	}
	if len(c.Payload) > 48*1024 || proto.Unmarshal(c.Payload, request) != nil {
		return nil, access.ErrForbidden
	}
	return request, nil
}

func (action Action) newRequest() (proto.Message, error) {
	var request proto.Message
	switch action {
	case Enter, Roles, Admit:
		request = new(pb.EnterReq)
	case Current, Check:
		request = new(pb.CurrentReq)
	case Zones:
		request = new(pb.ListZonesReq)
	case Leave:
		request = new(pb.LeaveReq)
	case Preferences:
		request = new(pb.SetPreferencesReq)
	case AttackBoss:
		request = new(pb.AttackBossReq)
	case UseExpItem:
		request = new(pb.UseExpItemReq)
	case Attach, Disconnect:
		return nil, nil
	default:
		request = pb.NewPlayerForwardRequest(string(action))
		if request == nil {
			return nil, access.ErrForbidden
		}
	}
	return request, nil
}

// EncodeRequest 保留信封中的身份与归属信息，只负责业务数据编码。
func EncodeRequest(command Command, request proto.Message) (*wrappers.BytesValue, error) {
	if nilPayload(request) || proto.Size(request) > 48*1024 {
		return nil, access.ErrForbidden
	}
	expected, err := command.Action.newRequest()
	if err != nil || expected == nil || proto.MessageName(expected) != proto.MessageName(request) {
		return nil, access.ErrForbidden
	}
	command.Payload, err = proto.Marshal(request)
	if err != nil {
		return nil, err
	}
	return Encode(command)
}

func EncodeReply(response proto.Message) (*wrappers.BytesValue, error) {
	if nilPayload(response) || proto.Size(response) > 48*1024 {
		return nil, access.ErrForbidden
	}
	payload, err := proto.Marshal(response)
	if err != nil {
		return nil, err
	}
	return Encode(Reply{Payload: payload})
}

func (r Reply) DecodeInto(response proto.Message) error {
	if nilPayload(response) || len(r.Payload) > 48*1024 {
		return access.ErrForbidden
	}
	return proto.Unmarshal(r.Payload, response)
}

func nilPayload(message proto.Message) bool {
	return message == nil || (reflect.ValueOf(message).Kind() == reflect.Ptr && reflect.ValueOf(message).IsNil())
}
