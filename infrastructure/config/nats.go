package config

import (
	"errors"
	"net/url"
	"strconv"
	"strings"
)

// NATSConnection 将公开地址和部署秘密合成 Pitaya 原生 Connect；调用方不得记录结果。
func (c Base) NATSConnection() (string, error) {
	if c.NATSUser == "" || c.NATSPassword == "" {
		return "", errors.New("config: HT_NATS_USER and HT_NATS_PASSWORD required")
	}
	addresses := strings.Split(c.NATS, ",")
	if len(addresses) > 16 {
		return "", errors.New("config: too many NATS endpoints")
	}
	for i, address := range addresses {
		u, err := url.Parse(strings.TrimSpace(address))
		if err != nil {
			return "", errors.New("config: invalid NATS endpoint")
		}
		port, err := strconv.Atoi(u.Port())
		if u.Hostname() == "" || err != nil || port < 1 || port > 65535 || (u.Scheme != "nats" && u.Scheme != "tls") || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
			return "", errors.New("config: invalid NATS endpoint; supply credentials through environment")
		}
		u.User = url.UserPassword(c.NATSUser, c.NATSPassword)
		addresses[i] = u.String()
	}
	return strings.Join(addresses, ","), nil
}
