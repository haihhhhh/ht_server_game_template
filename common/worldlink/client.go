package worldlink

import (
	"context"
	"errors"

	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/haihhhhh/ht_server_frame/app"
	"github.com/haihhhhh/ht_server_frame/placement"
	"github.com/haihhhhh/ht_server_game_template/common/clusterlink"
)

// Claim 仅在 Boss 尚无归属时调用。即使回复丢失，这一步也不会扣血。
func Claim(ctx context.Context, application *app.App, candidate placement.Assignment) (placement.Assignment, error) {
	var owner placement.Assignment
	request, err := clusterlink.Encode(candidate)
	if err != nil {
		return owner, err
	}
	reply := new(wrappers.BytesValue)
	if err = application.RPCTo(ctx, candidate.ServerID, "world.Boss.Claim", reply, request); err != nil {
		return owner, err
	}
	if clusterlink.Decode(reply, &owner) != nil || owner.Scope != candidate.Scope || owner.ServerID == "" || owner.AllocationID == "" {
		return placement.Assignment{}, errors.New("invalid boss owner reply")
	}
	return owner, nil
}

// CallAttack 只负责内部 RPC 编解码，负责人选择与业务规则留在调用者。
func CallAttack(ctx context.Context, server *app.App, node string, attack Attack) (Result, error) {
	var result Result
	request, err := clusterlink.Encode(attack)
	if err != nil {
		return result, err
	}
	reply := new(wrappers.BytesValue)
	if err = server.RPCTo(ctx, node, "world.Boss.Attack", reply, request); err != nil {
		return result, err
	}
	if clusterlink.Decode(reply, &result) != nil {
		return result, errors.New("invalid public service reply")
	}
	return result, nil
}
