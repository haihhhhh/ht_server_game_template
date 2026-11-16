package gamedata

import generated "github.com/haihhhhh/ht_server_game_template/common/gamedata/generated"

// StarterExpItemID 是创建角色时赠送的经验道具，道具表必须包含这个编号。
const StarterExpItemID int32 = 1001

// Item 返回道具行的值；当前各字段均为数值。
func (data Data) Item(id int32) (generated.ItemEntry, bool) {
	row := data.raw.TbItem.Get(id)
	if row == nil {
		return generated.ItemEntry{}, false
	}
	return *row, true
}
