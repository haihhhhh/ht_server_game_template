// 【通常无需修改｜进入、在线、退出】按装配顺序通知组件，定时回调只发给实现 Tick 的组件。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
package modules

import (
	"fmt"
	"time"
)

// Manager 只负责生命周期；业务通过所属模块的类型化引用调用组件。
type Manager struct {
	Modules      []Module
	tickHandlers []tickHandler
}

func (componentManager *Manager) AfterLoad() error {
	for _, module := range componentManager.Modules {
		if err := module.AfterLoad(); err != nil {
			return fmt.Errorf("component %s after load: %w", module.Name(), err)
		}
	}
	return nil
}

func (componentManager *Manager) Enter(now time.Time) {
	for _, module := range componentManager.Modules {
		module.Enter(now)
	}
}
func (componentManager *Manager) Tick(now time.Time) {
	for _, handler := range componentManager.tickHandlers {
		handler.Tick(now)
	}
}
func (componentManager *Manager) Leave(now time.Time) {
	for _, module := range componentManager.Modules {
		module.Leave(now)
	}
}
