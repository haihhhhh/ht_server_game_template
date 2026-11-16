// 【通常无需修改｜R07、R09】按请求类型找到已登记函数，统一估算队列占用；Prepare 只准备动作，真正执行发生在玩家循环内。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
package modules

import (
	"context"
	"reflect"

	"github.com/golang/protobuf/proto"
	"github.com/haihhhhh/ht_server_frame/access"
)

// queuedActionOverhead 是每条排队任务的固定记账估算，不代表实际分配的内存。
const queuedActionOverhead int64 = 128

type Action func(context.Context, *PlayerComponents) (proto.Message, error)
type handler func(context.Context, *PlayerComponents, proto.Message) (proto.Message, error)

var handlers = make(map[reflect.Type]handler)

// TODO(业务扩展)：用 RegisterAction(Feature, (*Module).方法名) 登记组件方法。
// 分发层在玩家循环内取得当前玩家的组件，业务方法直接使用接收者和已装配依赖。
func RegisterAction[T Module, Q proto.Message, R proto.Message](feature Feature[T], run func(T, context.Context, Q) (R, error)) {
	key := reflect.TypeOf((*Q)(nil)).Elem()
	if _, exists := handlers[key]; exists {
		panic("重复注册玩家请求：" + key.String())
	}
	handlers[key] = func(ctx context.Context, components *PlayerComponents, request proto.Message) (proto.Message, error) {
		module, err := feature.Get(components)
		if err != nil {
			return nil, err
		}
		return run(module, ctx, request.(Q))
	}
}

// Prepare 按请求编码大小估算队列占用；动作只在玩家循环中执行。
func Prepare(request proto.Message) (int64, Action) {
	size := int64(proto.Size(request)) + queuedActionOverhead
	run := handlers[reflect.TypeOf(request)]
	return size, func(ctx context.Context, components *PlayerComponents) (proto.Message, error) {
		if run == nil {
			return nil, access.ErrForbidden
		}
		return run(ctx, components, request)
	}
}
