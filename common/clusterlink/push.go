package clusterlink

import (
	"reflect"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/haihhhhh/ht_server_frame/access"
	pb "github.com/haihhhhh/ht_server_game_template/common/protocol/generated"
)

// ConnectionPush 是服务间的定向推送信封，客户端不能指定目标连接。
type ConnectionPush struct {
	Binding   Binding
	MessageID uint16
	Payload   []byte
}

var pushCatalog, pushCatalogErr = pb.NewProtocolCatalog()

func EncodePush(binding Binding, message proto.Message) (*wrappers.BytesValue, error) {
	if pushCatalogErr != nil {
		return nil, pushCatalogErr
	}
	if nilPayload(message) || proto.Size(message) > 48*1024 {
		return nil, access.ErrForbidden
	}
	id, ok := pushCatalog.MessageID(message)
	if !ok {
		return nil, access.ErrForbidden
	}
	payload, err := proto.Marshal(message)
	if err != nil {
		return nil, err
	}
	return Encode(ConnectionPush{Binding: binding, MessageID: id, Payload: payload})
}

func (push ConnectionPush) Message() (proto.Message, error) {
	if pushCatalogErr != nil {
		return nil, pushCatalogErr
	}
	name, ok := pushCatalog.MessageName(push.MessageID)
	if !ok || len(push.Payload) > 48*1024 {
		return nil, access.ErrForbidden
	}
	messageType := proto.MessageType(name)
	if messageType == nil || messageType.Kind() != reflect.Ptr {
		return nil, access.ErrForbidden
	}
	message := reflect.New(messageType.Elem()).Interface().(proto.Message)
	if err := proto.Unmarshal(push.Payload, message); err != nil {
		return nil, access.ErrForbidden
	}
	return message, nil
}
