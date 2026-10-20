package community

import "regexp"

const TestCommunityCode = "test-community"

var communityCodePattern = regexp.MustCompile(`^[a-z0-9]+([a-z0-9-]*[a-z0-9])?$`)

func ValidCode(code string) bool {
	return len(code) >= 1 && len(code) <= 63 && communityCodePattern.MatchString(code)
}

type Community struct {
	ID       string `json:"id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	IsActive bool   `json:"is_active"`
}
