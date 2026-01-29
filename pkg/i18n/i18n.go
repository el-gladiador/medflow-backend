package i18n

import (
	"context"
	"embed"
	"encoding/json"
	"strings"
	"sync"
)

//go:embed messages/*.json
var messagesFS embed.FS

// Supported locales
const (
	LocaleEnglish = "en"
	LocaleGerman  = "de"
	DefaultLocale = LocaleEnglish
)

// Context key for locale
type localeKey struct{}

var (
	messages     map[string]map[string]interface{}
	messagesOnce sync.Once
)

// loadMessages loads all message files from embedded filesystem
func loadMessages() {
	messagesOnce.Do(func() {
		messages = make(map[string]map[string]interface{})

		locales := []string{LocaleEnglish, LocaleGerman}
		for _, locale := range locales {
			data, err := messagesFS.ReadFile("messages/" + locale + ".json")
			if err != nil {
				continue
			}

			var msg map[string]interface{}
			if err := json.Unmarshal(data, &msg); err != nil {
				continue
			}

			messages[locale] = msg
		}
	})
}

// Localizer handles message localization
type Localizer struct {
	locale string
}

// NewLocalizer creates a new localizer for the given locale
func NewLocalizer(locale string) *Localizer {
	loadMessages()

	// Validate locale, default to English
	if locale != LocaleEnglish && locale != LocaleGerman {
		locale = DefaultLocale
	}

	return &Localizer{locale: locale}
}

// LocalizerFromContext creates a localizer from context
func LocalizerFromContext(ctx context.Context) *Localizer {
	locale := GetLocaleFromContext(ctx)
	return NewLocalizer(locale)
}

// T translates a message key with optional parameters
func (l *Localizer) T(key string, params ...map[string]string) string {
	loadMessages()

	// Get message from locale, fallback to default
	msg := l.getMessage(key, l.locale)
	if msg == "" {
		msg = l.getMessage(key, DefaultLocale)
	}
	if msg == "" {
		return key // Return key if not found
	}

	// Replace parameters
	if len(params) > 0 {
		for k, v := range params[0] {
			msg = strings.ReplaceAll(msg, "{"+k+"}", v)
		}
	}

	return msg
}

// getMessage retrieves a nested message by dot-notation key
func (l *Localizer) getMessage(key string, locale string) string {
	localeMessages, ok := messages[locale]
	if !ok {
		return ""
	}

	parts := strings.Split(key, ".")
	current := localeMessages

	for i, part := range parts {
		if i == len(parts)-1 {
			// Last part, should be a string
			if str, ok := current[part].(string); ok {
				return str
			}
			return ""
		}

		// Navigate deeper
		if nested, ok := current[part].(map[string]interface{}); ok {
			current = nested
		} else {
			return ""
		}
	}

	return ""
}

// GetLocale returns the current locale
func (l *Localizer) GetLocale() string {
	return l.locale
}

// WithLocale adds locale to context
func WithLocale(ctx context.Context, locale string) context.Context {
	return context.WithValue(ctx, localeKey{}, locale)
}

// GetLocaleFromContext retrieves locale from context
func GetLocaleFromContext(ctx context.Context) string {
	if locale, ok := ctx.Value(localeKey{}).(string); ok && locale != "" {
		return locale
	}
	return DefaultLocale
}

// ParseAcceptLanguage parses the Accept-Language header and returns the best matching locale
func ParseAcceptLanguage(header string) string {
	if header == "" {
		return DefaultLocale
	}

	// Simple parsing - check for supported locales
	header = strings.ToLower(header)

	// Check for German first (de, de-DE, de-AT, de-CH)
	if strings.Contains(header, "de") {
		return LocaleGerman
	}

	// Default to English
	return LocaleEnglish
}

// Global convenience functions

// T translates using the default locale
func T(key string, params ...map[string]string) string {
	return NewLocalizer(DefaultLocale).T(key, params...)
}

// TWithLocale translates using the specified locale
func TWithLocale(locale, key string, params ...map[string]string) string {
	return NewLocalizer(locale).T(key, params...)
}

// TFromContext translates using locale from context
func TFromContext(ctx context.Context, key string, params ...map[string]string) string {
	return LocalizerFromContext(ctx).T(key, params...)
}
