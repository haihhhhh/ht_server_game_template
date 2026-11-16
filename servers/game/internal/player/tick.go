package player

import (
	"context"
	"time"
)

// 从进入游戏起计时，Tick 和存盘只比较经过的时长。GM 回拨日期不会推迟下一次执行。
func (onlinePlayer *Player) startTimers() {
	onlinePlayer.enteredAt = time.Now()
	onlinePlayer.nextTick = onlinePlayer.playerManager.TickInterval
	onlinePlayer.nextSave = onlinePlayer.playerManager.SaveInterval
}

func (onlinePlayer *Player) onlineDuration() time.Duration {
	if onlinePlayer.enteredAt.IsZero() {
		return 0
	}
	return time.Since(onlinePlayer.enteredAt)
}

// update 在玩家自己的循环里运行，传给玩法的是当前业务日期。
func (playerEntry *onlinePlayerEntry) update(_ context.Context, onlinePlayer *Player) error {
	if !onlinePlayer.ready || playerEntry.leaving.Load() {
		return nil
	}
	playerManager := onlinePlayer.playerManager
	elapsed := onlinePlayer.onlineDuration()
	if elapsed >= onlinePlayer.nextTick {
		onlinePlayer.components.Tick(playerManager.Clock.Now())
		// 从原定时间推进。慢请求跨过多个周期时只执行一次，再等下一个周期。
		period := playerManager.TickInterval
		lateBy := elapsed - onlinePlayer.nextTick
		onlinePlayer.nextTick = elapsed + (period - lateBy%period)
	}

	// 【S01】保存从本次尝试起另等一个完整周期，入队失败也不会在下一次 Tick 立即重试。
	if elapsed < onlinePlayer.nextSave {
		return nil
	}
	err := playerManager.save(onlinePlayer, false)
	onlinePlayer.nextSave = onlinePlayer.onlineDuration() + playerManager.SaveInterval
	return err
}
