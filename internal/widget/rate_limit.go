package widget

import (
	"fmt"
	"time"

	"github.com/moond4rk/ccstatus/internal/config"
	"github.com/moond4rk/ccstatus/internal/status"
)

// rateLimitRefillExtractor extracts a refill duration from status Data.
type rateLimitRefillExtractor func(data *status.Session) (time.Duration, bool)

// rateLimitRefillWidget displays the time remaining until a rate limit resets.
type rateLimitRefillWidget struct {
	extract       rateLimitRefillExtractor
	displayName   string
	description   string
	defaultPrefix string
	defaultColor  string
}

// Render returns the formatted refill time.
func (w *rateLimitRefillWidget) Render(item *config.WidgetItem, ctx RenderContext, _ *config.Settings) string {
	if ctx.Data == nil {
		return ""
	}
	d, ok := w.extract(ctx.Data)
	if !ok {
		return ""
	}
	if item.RawValue {
		return fmt.Sprintf("%.0f", d.Seconds())
	}
	return status.FormatRefillDuration(d)
}

// DefaultColor returns the default foreground color.
func (w *rateLimitRefillWidget) DefaultColor() string {
	if w.defaultColor != "" {
		return w.defaultColor
	}
	return defaultDimColor
}

// DisplayName returns the human-readable name.
func (w *rateLimitRefillWidget) DisplayName() string { return w.displayName }

// Description returns what this widget shows.
func (w *rateLimitRefillWidget) Description() string { return w.description }

// SupportsRawValue returns true since this widget supports raw seconds output.
func (w *rateLimitRefillWidget) SupportsRawValue() bool { return true }

// DefaultPrefix returns the default prefix.
func (w *rateLimitRefillWidget) DefaultPrefix() string { return w.defaultPrefix }

// DefaultSuffix returns the default suffix.
func (w *rateLimitRefillWidget) DefaultSuffix() string { return "" }
