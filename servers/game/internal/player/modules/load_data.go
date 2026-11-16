package modules

import model "github.com/haihhhhh/ht_server_game_template/common/model/database"

// LoadData 只传递本次加载的数据，组件不在初始化时自行查询数据库。
type LoadData struct {
	Profile                                              model.PlayerProfile
	OnlineStats                                          model.PlayerOnlineStats
	Items                                                []model.PlayerItem
	Challenge                                            model.PlayerChallenge
	ProfileMissing, OnlineStatsMissing, ChallengeMissing bool
}
