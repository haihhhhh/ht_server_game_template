// TODO(业务扩展)【R10】从本次配置快照读取道具规则，先检查全部条件，再消耗道具和增加经验。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
package inventory

// UseExpItem 在玩家循环内组合业务，不跨组件发消息、不额外加锁。
func (bag *Module) UseExpItem(itemID int32, count int64) error {
	item, exists := bag.config.Item(itemID)
	if !exists {
		return ErrItem
	}
	if err := bag.CheckConsume(itemID, count, int64(item.MinLevel)); err != nil {
		return err
	}
	// 道具配置经验不超过 10^12，数量不超过 10^6，乘积不会溢出 int64。
	total, err := bag.profile.PrepareAddExp(count * item.Experience)
	if err != nil {
		return err
	}
	if err = bag.Consume(itemID, count, int64(item.MinLevel)); err != nil {
		return err
	}
	bag.profile.ApplyExp(total)
	return nil
}
