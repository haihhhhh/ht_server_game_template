package gamedata

import (
	"cmp"
	"slices"

	generated "github.com/haihhhhh/ht_server_game_template/common/gamedata/generated"
)

// 保留原表行序，单独按等级整理索引，供经验查询和配置校验共用。
func sortedLevels(table *generated.TbLevel) []*generated.LevelEntry {
	levels := slices.Clone(table.GetDataList())
	slices.SortFunc(levels, func(a, b *generated.LevelEntry) int { return cmp.Compare(a.Level, b.Level) })
	return levels
}

// LevelForExp 根据累计经验查等级；热重载下调上限时不删除玩家已有经验。
func (data Data) LevelForExp(experience int64) int64 {
	level := int64(1)
	for _, row := range data.levels {
		if experience < row.Experience {
			break
		}
		level = int64(row.Level)
	}
	return level
}

// MaxExperience 取最高等级的累计经验，调用前配置必须已通过校验。
func (data Data) MaxExperience() int64 {
	return data.levels[len(data.levels)-1].Experience
}
