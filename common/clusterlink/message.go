// clusterlink 只定义项目内部 RPC 信封；客户端不能选择节点或指定连接身份。
package clusterlink

import (
	"encoding/json"
	"errors"

	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/haihhhhh/ht_server_frame/access"
	"github.com/haihhhhh/ht_server_frame/placement"
)

type Binding struct {
	Gateway  string
	Identity access.Identity
	Token    string
	Order    LoginOrder
}

// LoginOrder 只为登录排序；归属记录版本区分账号重建，不是数据库写入权限。
type LoginOrder struct {
	OwnerRevision int64
	Sequence      uint64
}

func (o LoginOrder) Valid() bool {
	return o.OwnerRevision > 0 && o.Sequence > 0
}
func (o LoginOrder) Before(other LoginOrder) bool {
	return o.OwnerRevision < other.OwnerRevision || (o.OwnerRevision == other.OwnerRevision && o.Sequence < other.Sequence)
}
func (b Binding) SameConnection(other Binding) bool {
	return b.Gateway == other.Gateway && b.Identity == other.Identity && b.Token == other.Token
}

type Command struct {
	Allocation string
	Claim      bool `json:",omitempty"` // 仅首次关联允许接收节点登记指定的候选分配。
	Gateway    string
	Action     Action
	Payload    []byte
	Token      string
	Order      LoginOrder
}
type Reply struct {
	Assignment *placement.Assignment `json:",omitempty"`
	Redirect   *placement.Assignment `json:",omitempty"` // 竞争失败且尚未执行业务时返回实际归属。
	Payload    []byte
	Previous   Binding
	Current    Binding
}

func Encode(v any) (*wrappers.BytesValue, error) {
	b, e := json.Marshal(v)
	if len(b) > 48*1024 {
		return nil, errors.New("internal command too large")
	}
	return &wrappers.BytesValue{Value: b}, e
}
func Decode(m *wrappers.BytesValue, v any) error {
	if m == nil || len(m.Value) > 48*1024 {
		return errors.New("invalid internal command")
	}
	return json.Unmarshal(m.Value, v)
}
