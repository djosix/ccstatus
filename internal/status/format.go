package status

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	defaultMaxTokens   = 200_000
	defaultUsableRatio = 80 // 80% of max
	longMaxTokens      = 1_000_000
)

// WindowLimits holds resolved context window size information.
type WindowLimits struct {
	MaxTokens    int
	UsableTokens int
}

// FormatTokens formats a token count into a human-readable string.
// Examples: 500 -> "500", 1500 -> "1.5k", 1200000 -> "1.2M"
func FormatTokens(count int) string {
	if count >= 1_000_000 {
		return fmt.Sprintf("%.1fM", float64(count)/1_000_000)
	}
	if count >= 1000 {
		return fmt.Sprintf("%.1fk", float64(count)/1000)
	}
	return strconv.Itoa(count)
}

// ContextConfig resolves context window size.
// Primary: use context_window.context_window_size from JSON input.
// Fallback: heuristic based on model ID (for older Claude Code versions).
func ContextConfig(data *Session) WindowLimits {
	if data.ContextWindow != nil && data.ContextWindow.ContextWindowSize != nil {
		size := *data.ContextWindow.ContextWindowSize
		return WindowLimits{
			MaxTokens:    size,
			UsableTokens: size * defaultUsableRatio / 100,
		}
	}

	lower := strings.ToLower(data.Model.ID)
	if strings.Contains(lower, "claude-sonnet-4-5") && strings.Contains(lower, "[1m]") {
		return WindowLimits{
			MaxTokens:    longMaxTokens,
			UsableTokens: longMaxTokens * defaultUsableRatio / 100,
		}
	}
	return WindowLimits{
		MaxTokens:    defaultMaxTokens,
		UsableTokens: defaultMaxTokens * defaultUsableRatio / 100,
	}
}

// ContextPercentage returns the context usage percentage.
// Primary: use pre-calculated used_percentage from JSON input.
// Fallback: calculate from current_usage tokens and context_window_size.
// Returns (value, ok) where ok=false means no data available.
func ContextPercentage(data *Session) (float64, bool) {
	if data.ContextWindow != nil && data.ContextWindow.UsedPercentage != nil {
		return *data.ContextWindow.UsedPercentage, true
	}
	if data.ContextWindow != nil && data.ContextWindow.CurrentUsage != nil {
		cu := data.ContextWindow.CurrentUsage
		contextLength := cu.InputTokens + cu.CacheCreationInputTokens + cu.CacheReadInputTokens
		cfg := ContextConfig(data)
		if cfg.MaxTokens == 0 {
			return 0, false
		}
		pct := float64(contextLength) / float64(cfg.MaxTokens) * 100
		if pct > 100 {
			return 100, true
		}
		return pct, true
	}
	return 0, false
}

// RemainingPercentage returns the remaining context window percentage.
// Primary: use pre-calculated remaining_percentage from JSON input.
// Fallback: calculate as 100 - used_percentage.
// Returns (value, ok) where ok=false means no data available.
func RemainingPercentage(data *Session) (float64, bool) {
	if data.ContextWindow != nil && data.ContextWindow.RemainingPercentage != nil {
		return *data.ContextWindow.RemainingPercentage, true
	}
	used, ok := ContextPercentage(data)
	if !ok {
		return 0, false
	}
	remaining := 100 - used
	if remaining < 0 {
		return 0, true
	}
	return remaining, true
}

// CacheHitRate returns the cache read ratio as a percentage.
// Formula: cache_read_input_tokens / (input_tokens + cache_creation_input_tokens + cache_read_input_tokens) * 100
// Returns (value, ok) where ok=false means no data available (total tokens is 0).
func CacheHitRate(data *Session) (float64, bool) {
	total := ContextLength(data)
	if total == 0 {
		return 0, false
	}
	return float64(data.ContextWindow.CurrentUsage.CacheReadInputTokens) / float64(total) * 100, true
}

// FiveHourUsage returns the 5-hour rate limit usage percentage.
// Returns (value, ok) where ok=false means no data available.
func FiveHourUsage(data *Session) (float64, bool) {
	if data.RateLimits == nil || data.RateLimits.FiveHour == nil || data.RateLimits.FiveHour.UsedPercentage == nil {
		return 0, false
	}
	return *data.RateLimits.FiveHour.UsedPercentage, true
}

// SevenDayUsage returns the 7-day rate limit usage percentage.
// Returns (value, ok) where ok=false means no data available.
func SevenDayUsage(data *Session) (float64, bool) {
	if data.RateLimits == nil || data.RateLimits.SevenDay == nil || data.RateLimits.SevenDay.UsedPercentage == nil {
		return 0, false
	}
	return *data.RateLimits.SevenDay.UsedPercentage, true
}

// FiveHourRefill returns the time remaining until the 5-hour rate limit resets.
// Returns (duration, ok) where ok=false means no data available or already reset.
func FiveHourRefill(data *Session) (time.Duration, bool) {
	return rateLimitRefill(data.RateLimits, func(rl *RateLimits) *RateLimitWindow { return rl.FiveHour })
}

// SevenDayRefill returns the time remaining until the 7-day rate limit resets.
// Returns (duration, ok) where ok=false means no data available or already reset.
func SevenDayRefill(data *Session) (time.Duration, bool) {
	return rateLimitRefill(data.RateLimits, func(rl *RateLimits) *RateLimitWindow { return rl.SevenDay })
}

func rateLimitRefill(rl *RateLimits, window func(*RateLimits) *RateLimitWindow) (time.Duration, bool) {
	if rl == nil {
		return 0, false
	}
	w := window(rl)
	if w == nil || w.ResetsAt == nil {
		return 0, false
	}
	resetTime := time.Unix(*w.ResetsAt, 0)
	remaining := time.Until(resetTime)
	if remaining < 0 {
		return 0, true
	}
	return remaining, true
}

// FormatRefillDuration formats a duration as a human-readable refill time.
// Examples: "2h15m", "3d5h", "<1m", "45m"
func FormatRefillDuration(d time.Duration) string {
	if d <= 0 {
		return "<1m"
	}
	totalMinutes := int(d.Minutes())
	days := totalMinutes / (60 * 24)
	hours := (totalMinutes % (60 * 24)) / 60
	mins := totalMinutes % 60

	if days > 0 && hours > 0 {
		return fmt.Sprintf("%dd%dh", days, hours)
	}
	if days > 0 {
		return fmt.Sprintf("%dd", days)
	}
	if hours > 0 && mins > 0 {
		return fmt.Sprintf("%dh%dm", hours, mins)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh", hours)
	}
	if mins > 0 {
		return fmt.Sprintf("%dm", mins)
	}
	return "<1m"
}

// ContextLength returns the total input token count (context length).
// This is the sum of input_tokens + cache_creation_input_tokens + cache_read_input_tokens.
func ContextLength(data *Session) int {
	if data.ContextWindow == nil || data.ContextWindow.CurrentUsage == nil {
		return 0
	}
	cu := data.ContextWindow.CurrentUsage
	return cu.InputTokens + cu.CacheCreationInputTokens + cu.CacheReadInputTokens
}
