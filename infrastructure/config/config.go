// config 只读取和校验运行参数，不创建数据库、网络或业务对象。
package config

import (
	"encoding/json"
	"errors"
	"flag"
	"io"
	"net/url"
	"os"
	"strings"

	"github.com/go-sql-driver/mysql"
	"github.com/haihhhhh/ht_server_game_template/infrastructure/logging"
)

type Base struct {
	IDNode         int64          `json:"id_node"`
	NATS           string         `json:"nats"`
	Etcd           string         `json:"etcd"`
	Prefix         string         `json:"prefix"`
	PrivateNetwork bool           `json:"private_network"`
	Tables         string         `json:"tables"`
	DBQueues       int            `json:"db_queues"`
	Instance       string         `json:"instance"`
	Log            logging.Config `json:"log"`
	// 凭据只允许从明确的环境变量读取，不接受 JSON 内嵌秘密。
	DSN                string `json:"-"`
	NATSUser           string `json:"-"`
	NATSPassword       string `json:"-"`
	DevTicket          string `json:"-"`
	DiagnosticsAddress string `json:"-"`
	DiagnosticsToken   string `json:"-"`
	Pprof              bool   `json:"-"`
	Trace              bool   `json:"-"`
}

func Default() Base {
	return Base{
		IDNode:   -1,
		NATS:     "nats://127.0.0.1:14222",
		Etcd:     "http://127.0.0.1:12379",
		Prefix:   "ht_game_dev",
		Tables:   "tables/exported",
		DBQueues: 4,
		Log:      logging.Default(),
	}
}
func Path() string {
	p := flag.String("config", "", "项目 JSON 配置路径")
	flag.Parse()
	return *p
}
func Read(path string, target any) error {
	if path == "" {
		return nil
	}
	f, err := os.Open(path)
	if err != nil {
		return errors.New("config: cannot open file")
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.Size() > 65536 {
		return errors.New("config: file too large or unreadable")
	}
	d := json.NewDecoder(io.LimitReader(f, 65537))
	d.DisallowUnknownFields()
	if err = d.Decode(target); err != nil {
		return errors.New("config: invalid JSON or unknown field")
	}
	var extra any
	if d.Decode(&extra) != io.EOF {
		return errors.New("config: trailing JSON")
	}
	return nil
}
func (c *Base) Environment() {
	for name, dest := range map[string]*string{
		"HT_NATS":              &c.NATS,
		"HT_NATS_USER":         &c.NATSUser,
		"HT_NATS_PASSWORD":     &c.NATSPassword,
		"HT_ETCD":              &c.Etcd,
		"HT_INSTANCE":          &c.Instance,
		"HT_MYSQL_DSN":         &c.DSN,
		"HT_DEV_TICKET":        &c.DevTicket,
		"HT_DIAGNOSTICS_ADDR":  &c.DiagnosticsAddress,
		"HT_DIAGNOSTICS_TOKEN": &c.DiagnosticsToken,
		"HT_LOG_LEVEL":         &c.Log.Level,
	} {
		if value, ok := os.LookupEnv(name); ok {
			*dest = value
		}
	}
	c.Pprof = os.Getenv("HT_PPROF") == "1"
	c.Trace = os.Getenv("HT_TRACE") == "1"
}
func (c Base) Validate() error {
	if c.Prefix == "" || len(c.Prefix) > 64 || strings.ContainsAny(c.Prefix, "/\\") || len(c.Instance) > 128 {
		return errors.New("config: invalid base limits")
	}
	if _, err := c.NATSConnection(); err != nil {
		return err
	}
	for _, value := range []string{c.Etcd} {
		u, e := url.Parse(value)
		if e != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
			return errors.New("config: invalid cluster endpoint")
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return errors.New("config: unsupported cluster endpoint scheme")
		}
	}
	if c.DiagnosticsAddress != "" && c.DiagnosticsToken == "" {
		return errors.New("config: diagnostic token required")
	}
	return c.Log.Validate()
}

// ValidateDatabase 只由实际使用数据库的服务调用。
func (c Base) ValidateDatabase() error {
	if c.DBQueues < 1 || c.DBQueues > 8 {
		return errors.New("config: invalid database queue count")
	}
	if c.DSN == "" {
		return errors.New("config: HT_MYSQL_DSN required")
	}
	if parsed, err := mysql.ParseDSN(c.DSN); err != nil || parsed.DBName == "" {
		return errors.New("config: invalid MySQL DSN or missing database name")
	}
	return nil
}
