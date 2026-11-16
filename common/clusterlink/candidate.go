package clusterlink

import (
	"errors"
	"hash/fnv"

	"github.com/haihhhhh/ht_server_frame/app"
	"github.com/haihhhhh/ht_server_frame/placement"
)

var ErrNoCandidate = errors.New("clusterlink: no candidate server")

// SelectCandidate 仅在查不到现有归属时调用，按原来的哈希规则选择一个节点。
func SelectCandidate(application *app.App, serverType, scope string) (placement.Assignment, error) {
	ids, err := application.ServerIDs(serverType)
	if err != nil {
		return placement.Assignment{}, err
	}
	if len(ids) == 0 {
		return placement.Assignment{}, ErrNoCandidate
	}
	hash := fnv.New64a()
	_, _ = hash.Write([]byte(scope))
	return placement.NewCandidate(scope, ids[hash.Sum64()%uint64(len(ids))]), nil
}
