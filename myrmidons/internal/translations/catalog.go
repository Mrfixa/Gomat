package translations

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// Locale supported locales
type Locale string

const (
	LocaleEN Locale = "en" // English
	LocaleRU Locale = "ru" // Russian
	LocaleDE Locale = "de" // German
	LocaleFR Locale = "fr" // French
	LocaleES Locale = "es" // Spanish
	LocaleZH Locale = "zh" // Chinese
	LocaleJA Locale = "ja" // Japanese
	LocalePT Locale = "pt" // Portuguese
	LocaleAR Locale = "ar" // Arabic
	LocaleIT Locale = "it" // Italian
)

// Catalog manages all translations
type Catalog struct {
	locales       map[Locale]map[string]string
	defaultLocale Locale
	mu            sync.RWMutex
}

// Global catalog instance
var catalog *Catalog

// Init initializes the translation catalog
func Init(localesDir string) error {
	catalog = &Catalog{
		locales:       make(map[Locale]map[string]string),
		defaultLocale: LocaleEN,
	}

	// Load each locale file
	files, err := os.ReadDir(localesDir)
	if err != nil {
		return err
	}

	for _, f := range files {
		if filepath.Ext(f.Name()) != ".json" {
			continue
		}

		locale := Locale(strings.TrimSuffix(f.Name(), ".json"))
		data, err := os.ReadFile(filepath.Join(localesDir, f.Name()))
		if err != nil {
			return err
		}

		var translations map[string]string
		if err := json.Unmarshal(data, &translations); err != nil {
			return err
		}

		catalog.locales[locale] = translations
	}

	return nil
}

// T returns the translation for the given key in the specified locale
func (c *Catalog) T(locale Locale, key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if translations, ok := c.locales[locale]; ok {
		if v, ok := translations[key]; ok {
			return v
		}
	}

	// Fallback to default locale
	if locale != c.defaultLocale {
		if translations, ok := c.locales[c.defaultLocale]; ok {
			if v, ok := translations[key]; ok {
				return v
			}
		}
	}

	return key // Return key if not found
}

// LocaleFromString converts a string to Locale
func LocaleFromString(s string) Locale {
	s = strings.ToLower(s)
	switch s {
	case "en":
		return LocaleEN
	case "ru":
		return LocaleRU
	case "de":
		return LocaleDE
	case "fr":
		return LocaleFR
	case "es":
		return LocaleES
	case "zh":
		return LocaleZH
	case "ja":
		return LocaleJA
	case "pt":
		return LocalePT
	case "ar":
		return LocaleAR
	case "it":
		return LocaleIT
	default:
		return LocaleEN
	}
}

// SupportedLocales returns all supported locale codes
func SupportedLocales() []string {
	return []string{"en", "ru", "de", "fr", "es", "zh", "ja", "pt", "ar", "it"}
}

// LocaleContextKey is the context key for locale
type LocaleContextKey struct{}

// WithLocale adds locale to context
func WithLocale(ctx context.Context, locale Locale) context.Context {
	return context.WithValue(ctx, LocaleContextKey{}, locale)
}

// FromContext gets locale from context
func FromContext(ctx context.Context) Locale {
	if locale, ok := ctx.Value(LocaleContextKey{}).(Locale); ok {
		return locale
	}
	return LocaleEN
}

// TranslationFunc type for template functions
type TranslationFunc func(key string) string

// T creates a translation function for the given locale
func T(locale Locale) TranslationFunc {
	return func(key string) string {
		return catalog.T(locale, key)
	}
}
