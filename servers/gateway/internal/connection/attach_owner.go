package connection

import (
	"context"
	"errors"

	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/haihhhhh/ht_server_frame/errorcode"
	"github.com/haihhhhh/ht_server_frame/placement"
	"github.com/haihhhhh/ht_server_game_template/common/clusterlink"
	"github.com/haihhhhh/ht_server_game_template/common/errcode"
)

// 发出换绑前记住本次目标。回包未知时，旧绑定和本次尝试都留给下线任务核对。
func (connectionManager *Manager) sendAttach(ctx context.Context, link *backendLink, target backendTarget, route string, command clusterlink.Command) (clusterlink.Reply, error) {
	// 再次进入本方法只可能是上个候选明确回复了改投，新目标尚未发送。
	link.pending = backendTarget{}
	command.Allocation = target.assignment.AllocationID
	command.Gateway = target.binding.Gateway
	command.Token = target.binding.Token
	command.Order = target.binding.Order
	packet, err := clusterlink.Encode(command)
	if err != nil {
		return clusterlink.Reply{}, err
	}
	link.pending = target
	response := new(wrappers.BytesValue)
	if err = connectionManager.App.RPCTo(ctx, target.assignment.ServerID, route, response, packet); err != nil {
		code := errorcode.Of(err)
		if errors.Is(err, errorcode.ErrRPCNotSent) || code == errcode.CodePlayerBusy || code == errcode.CodeAccountBusy {
			link.pending = backendTarget{}
		}
		return clusterlink.Reply{}, err
	}
	var reply clusterlink.Reply
	err = clusterlink.Decode(response, &reply)
	return reply, err
}

// attachOwner 只在候选明确答复“别人已认领，我未绑定”时改投一次；任何 RPC 错误直接返回。
func attachOwner(assignment placement.Assignment, claim bool, send func(placement.Assignment, bool) (clusterlink.Reply, error)) (placement.Assignment, clusterlink.Reply, error) {
	reply, err := send(assignment, claim)
	if err != nil {
		return assignment, reply, err
	}
	if owner := reply.Redirect; owner != nil {
		if !claim || reply.Assignment != nil || owner.Scope != assignment.Scope || owner.ServerID == "" || owner.AllocationID == "" || owner.AllocationID == assignment.AllocationID {
			return assignment, reply, errors.New("connection: invalid ownership redirect")
		}
		assignment = *owner
		reply, err = send(assignment, false)
		if err != nil {
			return assignment, reply, err
		}
	}
	owner := reply.Assignment
	if reply.Redirect != nil || owner == nil || owner.Scope != assignment.Scope || owner.ServerID != assignment.ServerID || owner.AllocationID != assignment.AllocationID {
		return assignment, reply, errors.New("connection: invalid confirmed assignment")
	}
	return assignment, reply, nil
}
