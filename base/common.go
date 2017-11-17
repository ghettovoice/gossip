package base

import (
	"strings"

	"github.com/ghettovoice/gossip/utils"
)

const RFC3261BranchMagicCookie = "z9hG4bK"

// GenerateBranch returns random unique branch ID.
func GenerateBranch() string {
	return strings.Join([]string{
		RFC3261BranchMagicCookie,
		utils.RandStr(16),
	}, "")
}
