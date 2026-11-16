// TODO(业务扩展)【R10】这里负责角色等级与经验规则。校验和修改分开，方便背包先确认所有条件，再一起修改内存。
// 编号对照：docs/business-request-flow.md；TODO(业务扩展) 表示项目续写位置，不表示现有功能未完成。
package profile

import "errors"

// MaxExperience 是存档数值安全界限；实际玩法上限来自本次等级配置。
const MaxExperience int64 = 1000000000000
const maxPreferencesBytes = 4096

var ErrExperience = errors.New("profile: experience out of range")
var ErrPreferencesTooLong = errors.New("profile: preferences too large")

func (profileModule *Module) Experience() int64 {
	return profileModule.data.Experience
}

// 等级由本次操作固定的配置决定，不修改已保存的累计经验。
func (profileModule *Module) Level() int64 {
	return profileModule.config.LevelForExp(profileModule.data.Experience)
}
func (profileModule *Module) PrepareAddExp(amount int64) (int64, error) {
	limit := profileModule.config.MaxExperience()
	if amount <= 0 || amount > limit || profileModule.data.Experience > limit-amount {
		return 0, ErrExperience
	}
	return profileModule.data.Experience + amount, nil
}

// ApplyExp 只接收同一玩家操作刚刚校验过的结果，不允许客户端直接设置经验。
func (profileModule *Module) ApplyExp(total int64) {
	profileModule.data.Experience = total
}
func (profileModule *Module) SetPreferences(value string) error {
	if len(value) > maxPreferencesBytes {
		return ErrPreferencesTooLong
	}
	profileModule.data.Preferences = value
	return nil
}
