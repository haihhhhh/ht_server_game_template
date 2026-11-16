package gamedata

import "fmt"

// ValidateGameplay 在发布新快照前检查道具与等级的关联，不让错误配置进入业务。
func (data Data) ValidateGameplay() error {
	if len(data.levels) < 2 {
		return fmt.Errorf("等级配置至少包含初始级和最高级")
	}
	if _, exists := data.Item(StarterExpItemID); !exists {
		return fmt.Errorf("道具表缺少新角色的初始经验道具 %d", StarterExpItemID)
	}
	for index, level := range data.levels {
		if level.Level != int32(index+1) || level.Experience < 0 || level.Experience > 1000000000000 {
			return fmt.Errorf("等级必须从1连续递增，经验必须在安全范围内")
		}
		if index == 0 && level.Experience != 0 || index > 0 && level.Experience <= data.levels[index-1].Experience {
			return fmt.Errorf("初始经验必须为0，后续等级经验必须递增")
		}
	}
	for id, item := range data.raw.TbItem.GetDataMap() {
		if id <= 0 || item.Experience <= 0 || item.Experience > data.MaxExperience() || item.MinLevel < 1 || int(item.MinLevel) > len(data.levels) {
			return fmt.Errorf("道具 %d 的经验或使用等级配置无效", id)
		}
	}
	return nil
}
