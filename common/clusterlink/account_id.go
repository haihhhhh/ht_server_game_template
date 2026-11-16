package clusterlink

import (
	"strconv"

	"github.com/haihhhhh/ht_server_frame/access"
)

// AccountUID 将账号 ID 转为 Pitaya 绑定和归属登记使用的十进制字符串。
func AccountUID(accountID int64) string {
	return strconv.FormatInt(accountID, 10)
}

// ParseAccountID 读取绑定中的账号 ID，只接受唯一的正整数写法，避免同一账号有多个归属键。
func ParseAccountID(uid string) (int64, error) {
	if len(uid) == 0 || uid[0] < '1' || uid[0] > '9' {
		return 0, access.ErrUnauthenticated
	}
	accountID, err := strconv.ParseInt(uid, 10, 64)
	if err != nil {
		return 0, access.ErrUnauthenticated
	}
	return accountID, nil
}
