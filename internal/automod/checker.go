// Package automod implements the rule evaluation engine. Pure rule predicates
// are exported for unit testing; the Checker adds stateful spam tracking and a
// periodic cleanup goroutine.
package automod

import (
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/0xSalik/specter/internal/db/queries"
)

// Violation describes a triggered rule and the action that should be taken.
type Violation struct {
	Rule   string // "spam","invite","link","caps","badwords"
	Reason string
	Action string // resolved action: delete/warn/timeout/kick/ban
}

var inviteRe = regexp.MustCompile(`(?i)(discord\.gg/|discord\.com/invite/|discordapp\.com/invite/)`)
var urlRe = regexp.MustCompile(`(?i)\bhttps?://[^\s]+`)

// HasInvite reports whether content contains a Discord invite link.
func HasInvite(content string) bool {
	return inviteRe.MatchString(content)
}

// HasDisallowedLink reports whether content contains a URL whose host is not in
// the allowed-domains whitelist. An empty whitelist blocks all external links.
func HasDisallowedLink(content string, allowedDomains []string) bool {
	matches := urlRe.FindAllString(content, -1)
	if len(matches) == 0 {
		return false
	}
	allowed := make(map[string]struct{}, len(allowedDomains))
	for _, d := range allowedDomains {
		allowed[strings.ToLower(strings.TrimPrefix(d, "www."))] = struct{}{}
	}
	for _, raw := range matches {
		u, err := url.Parse(raw)
		if err != nil {
			return true
		}
		host := strings.ToLower(strings.TrimPrefix(u.Hostname(), "www."))
		if _, ok := allowed[host]; !ok {
			return true
		}
	}
	return false
}

// CapsRatio returns the fraction (0..1) of alphabetic characters that are
// uppercase. Messages with no letters return 0.
func CapsRatio(content string) float64 {
	var letters, upper int
	for _, r := range content {
		switch {
		case r >= 'A' && r <= 'Z':
			upper++
			letters++
		case r >= 'a' && r <= 'z':
			letters++
		}
	}
	if letters == 0 {
		return 0
	}
	return float64(upper) / float64(letters)
}

// ExceedsCaps reports whether a message of sufficient length exceeds the caps
// threshold percentage.
func ExceedsCaps(content string, thresholdPct int) bool {
	if len([]rune(content)) <= 10 {
		return false
	}
	return CapsRatio(content)*100 >= float64(thresholdPct)
}

// ContainsBadWord reports whether content matches any banned word using a
// case-insensitive substring match.
func ContainsBadWord(content string, badWords []string) bool {
	lc := strings.ToLower(content)
	for _, w := range badWords {
		w = strings.ToLower(strings.TrimSpace(w))
		if w == "" {
			continue
		}
		if strings.Contains(lc, w) {
			return true
		}
	}
	return false
}

// RuleAppliesToRoles reports whether a rule should apply to a member given its
// per-rule role scope. A member with an excluded role is never caught; if the
// include list is non-empty the member must hold one of those roles.
func RuleAppliesToRoles(scope queries.RuleScope, userRoles []string) bool {
	roleSet := make(map[string]struct{}, len(userRoles))
	for _, r := range userRoles {
		roleSet[r] = struct{}{}
	}
	for _, r := range scope.Exclude {
		if _, ok := roleSet[r]; ok {
			return false
		}
	}
	if len(scope.Include) > 0 {
		for _, r := range scope.Include {
			if _, ok := roleSet[r]; ok {
				return true
			}
		}
		return false
	}
	return true
}

// IsExempt reports whether a member's roles or the channel are exempt.
func IsExempt(userRoles []string, channelID string, exemptRoles, exemptChannels []string) bool {
	for _, c := range exemptChannels {
		if c == channelID {
			return true
		}
	}
	roleSet := make(map[string]struct{}, len(userRoles))
	for _, r := range userRoles {
		roleSet[r] = struct{}{}
	}
	for _, r := range exemptRoles {
		if _, ok := roleSet[r]; ok {
			return true
		}
	}
	return false
}

// Checker holds stateful automod logic (spam counters).
type Checker struct {
	mu    sync.Mutex
	spam  map[string][]time.Time // key: guild:user:channel -> message timestamps
	stop  chan struct{}
	clock func() time.Time
}

// NewChecker constructs a Checker and starts its background cleanup loop.
func NewChecker() *Checker {
	c := &Checker{
		spam:  make(map[string][]time.Time),
		stop:  make(chan struct{}),
		clock: time.Now,
	}
	go c.cleanupLoop()
	return c
}

// Close stops the cleanup goroutine.
func (c *Checker) Close() { close(c.stop) }

// RecordAndCheckSpam records a message timestamp and reports whether the user
// has exceeded the threshold within the window.
func (c *Checker) RecordAndCheckSpam(key string, threshold, windowSecs int) bool {
	now := c.clock()
	window := time.Duration(windowSecs) * time.Second

	c.mu.Lock()
	defer c.mu.Unlock()

	times := c.spam[key]
	cutoff := now.Add(-window)
	kept := times[:0]
	for _, t := range times {
		if t.After(cutoff) {
			kept = append(kept, t)
		}
	}
	kept = append(kept, now)
	c.spam[key] = kept
	return len(kept) >= threshold
}

func (c *Checker) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.stop:
			return
		case <-ticker.C:
			c.gc()
		}
	}
}

func (c *Checker) gc() {
	now := c.clock()
	c.mu.Lock()
	defer c.mu.Unlock()
	for k, times := range c.spam {
		cutoff := now.Add(-60 * time.Second)
		var any bool
		for _, t := range times {
			if t.After(cutoff) {
				any = true
				break
			}
		}
		if !any {
			delete(c.spam, k)
		}
	}
}

// Evaluate runs all enabled rules against a message and returns the first
// violation found, or nil. Spam state is mutated as a side effect.
func (c *Checker) Evaluate(cfg *queries.AutomodConfig, key, content string, userRoles []string, channelID string) *Violation {
	if cfg == nil || !cfg.Enabled {
		return nil
	}
	if IsExempt(userRoles, channelID, cfg.ExemptRoles, cfg.ExemptChannels) {
		return nil
	}

	// applies reports whether a rule is in effect for this member after applying
	// its optional per-rule role scope.
	applies := func(rule string) bool {
		if cfg.RuleRoleScopes == nil {
			return true
		}
		return RuleAppliesToRoles(cfg.RuleRoleScopes[rule], userRoles)
	}

	if cfg.AntiSpamEnabled && applies("spam") && c.RecordAndCheckSpam(key, cfg.AntiSpamThreshold, cfg.AntiSpamWindowSecs) {
		return &Violation{Rule: "spam", Reason: "Sending messages too quickly.", Action: cfg.Action}
	}
	if cfg.AntiInviteEnabled && applies("invite") && HasInvite(content) {
		return &Violation{Rule: "invite", Reason: "Posting Discord invite links is not allowed.", Action: cfg.Action}
	}
	if cfg.AntiLinkEnabled && applies("link") && HasDisallowedLink(content, cfg.AllowedLinkDomains) {
		return &Violation{Rule: "link", Reason: "Posting external links is not allowed.", Action: cfg.Action}
	}
	if cfg.AntiCapsEnabled && applies("caps") && ExceedsCaps(content, cfg.CapsThresholdPct) {
		return &Violation{Rule: "caps", Reason: "Excessive capitalization.", Action: cfg.Action}
	}
	if cfg.BadWordsEnabled && applies("badwords") && ContainsBadWord(content, cfg.BadWords) {
		return &Violation{Rule: "badwords", Reason: "Message contained a prohibited word.", Action: cfg.Action}
	}
	return nil
}
