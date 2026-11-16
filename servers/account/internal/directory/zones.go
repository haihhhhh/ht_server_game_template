package directory

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Zone 是逻辑区，不绑定某个 game 进程或端口。
type Zone struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Open bool   `json:"open"`
}

func ValidText(s string, max int) bool {
	return s != "" && len(s) <= max && utf8.ValidString(s) && strings.TrimSpace(s) == s && strings.IndexFunc(s, unicode.IsControl) < 0
}

func Validate(zones []Zone) error {
	if len(zones) == 0 || len(zones) > 100 {
		return errors.New("directory: expected 1 to 100 zones")
	}
	seen := make(map[string]bool, len(zones))
	for _, z := range zones {
		if !ValidText(z.ID, 64) || !ValidText(z.Name, 128) || seen[z.ID] {
			return errors.New("directory: invalid or duplicate zone")
		}
		seen[z.ID] = true
	}
	return nil
}
