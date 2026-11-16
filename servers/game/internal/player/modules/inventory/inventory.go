package inventory

import (
	"errors"
	"slices"
	"sort"
	"time"

	"github.com/haihhhhh/ht_server_game_template/common/gamedata"
	model "github.com/haihhhhh/ht_server_game_template/common/model/database"
	"github.com/haihhhhh/ht_server_game_template/servers/game/internal/player/modules"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const MaxKinds = 256
const MaxCount int64 = 1000000

// 每条背包变更在保存副本中的估算字节数，数量降为零的待删条目也占一条。
const estimatedBytesPerItemChange = 64

var ErrItem = errors.New("inventory: invalid item or count")
var ErrInsufficient = errors.New("inventory: insufficient items")
var ErrLimit = errors.New("inventory: capacity exceeded")
var ErrLevel = errors.New("inventory: level requirement not met")

// RoleProgress 只声明背包使用的角色能力；装配时注入，不把整个玩家交给背包。
type RoleProgress interface {
	Level() int64
	Experience() int64
	PrepareAddExp(int64) (int64, error)
	ApplyExp(int64)
}
type Module struct {
	config  *gamedata.Data
	roleID  int64
	profile RoleProgress
	items   map[int32]int64
	// 每种道具首次改动前的数量。改回原数时即可移出，无需写库。
	previousCounts map[int32]int64
}

func New(roleID int64, profile RoleProgress) *Module {
	return &Module{
		roleID:         roleID,
		profile:        profile,
		items:          make(map[int32]int64),
		previousCounts: make(map[int32]int64),
	}
}
func (*Module) Name() string {
	return "inventory"
}
func (inventoryModule *Module) Restore(rows []model.PlayerItem) error {
	if len(rows) > MaxKinds {
		return ErrLimit
	}
	for _, row := range rows {
		if row.ItemID <= 0 || row.Count <= 0 || row.Count > MaxCount {
			return ErrItem
		}
		inventoryModule.items[row.ItemID] = row.Count
	}
	return nil
}
func (inventoryModule *Module) AfterLoad() error {
	if inventoryModule.roleID <= 0 || inventoryModule.profile == nil || inventoryModule.config == nil {
		return errors.New("inventory: missing owner or profile")
	}
	return nil
}
func (*Module) Enter(time.Time) {}
func (*Module) Leave(time.Time) {}
func (inventoryModule *Module) Count(id int32) int64 {
	return inventoryModule.items[id]
}
func (inventoryModule *Module) Add(id int32, count int64) error {
	if id <= 0 || count <= 0 || count > MaxCount {
		return ErrItem
	}
	current := inventoryModule.items[id]
	if current > MaxCount-count || (current == 0 && len(inventoryModule.items) >= MaxKinds) {
		return ErrLimit
	}
	inventoryModule.setCount(id, current+count)
	return nil
}
func (inventoryModule *Module) CheckConsume(id int32, count int64, minLevel int64) error {
	if id <= 0 || count <= 0 || count > MaxCount || minLevel < 1 {
		return ErrItem
	}
	if inventoryModule.profile.Level() < minLevel {
		return ErrLevel
	}
	if inventoryModule.items[id] < count {
		return ErrInsufficient
	}
	return nil
}
func (inventoryModule *Module) Consume(id int32, count int64, minLevel int64) error {
	if err := inventoryModule.CheckConsume(id, count, minLevel); err != nil {
		return err
	}
	inventoryModule.setCount(id, inventoryModule.items[id]-count)
	return nil
}

func (inventoryModule *Module) setCount(id int32, count int64) {
	if previous, changed := inventoryModule.previousCounts[id]; !changed {
		inventoryModule.previousCounts[id] = inventoryModule.items[id]
	} else if count == previous {
		delete(inventoryModule.previousCounts, id)
	}
	if count == 0 {
		delete(inventoryModule.items, id)
	} else {
		inventoryModule.items[id] = count
	}
}

// Items 返回有序值副本，网络编码不接触背包内部 map。
func (inventoryModule *Module) Items() []model.PlayerItem {
	rows := make([]model.PlayerItem, 0, len(inventoryModule.items))
	for id, count := range inventoryModule.items {
		rows = append(rows, model.PlayerItem{
			RoleID: inventoryModule.roleID,
			ItemID: id,
			Count:  count,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		return rows[i].ItemID < rows[j].ItemID
	})
	return rows
}
func (inventoryModule *Module) Snapshot() *modules.Save {
	if len(inventoryModule.previousCounts) == 0 {
		return nil
	}
	roleID := inventoryModule.roleID
	var rows []model.PlayerItem
	var removed []int32
	for id := range inventoryModule.previousCounts {
		if count := inventoryModule.items[id]; count > 0 {
			rows = append(rows, model.PlayerItem{RoleID: roleID, ItemID: id, Count: count})
		} else {
			removed = append(removed, id)
		}
	}
	return &modules.Save{
		ExtraBytes: (len(rows) + len(removed)) * estimatedBytesPerItemChange,
		Write: func(db *gorm.DB) error {
			var failures []error
			if len(rows) > 0 {
				// 保有的道具种类受 MaxKinds 限制，最多一条批量新增或更新。
				if err := db.Clauses(clause.OnConflict{DoUpdates: clause.AssignmentColumns([]string{"count"})}).Create(&rows).Error; err != nil {
					failures = append(failures, err)
				}
			}
			for ids := range slices.Chunk(removed, MaxKinds) {
				if err := db.Where("role_id = ? AND item_id IN ?", roleID, ids).Delete(&model.PlayerItem{}).Error; err != nil {
					failures = append(failures, err)
				}
			}
			return errors.Join(failures...)
		},
	}
}
func (inventoryModule *Module) Accepted() {
	clear(inventoryModule.previousCounts)
}
