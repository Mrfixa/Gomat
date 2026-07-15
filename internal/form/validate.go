package form

import (
"fmt"
"html"
"html/template"
"regexp"
"strings"

"github.com/go-playground/validator/v10"
"github.com/gobugger/globalize"
currency_service "github.com/gobugger/gomarket/internal/service/currency"
"github.com/gobugger/gomarket/internal/service/product"
)

var (
// SECURE: Reasonable size limits for various input types
MaxUsernameLength     = 20
MaxPasswordLength     = 100
MaxMessageLength      = 5000
MaxSubjectLength      = 100
MaxCommentLength      = 2000
MaxDescriptionLength  = 2000
MaxTitleLength        = 50
MaxTOSLength          = 5000
MaxLetterLength       = 2000

// SECURE: Regex patterns for validation
usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9_]+$`)
// SECURE: Monero address pattern (95 characters for standard address)
xmrAddressPattern = regexp.MustCompile(`^4[0-9AB][1-9A-HJ-NP-Za-km-z]{93}$`)
)

// SECURE: Validate username format and length
func validateUsername(username string) bool {
if len(username) < 3 || len(username) > MaxUsernameLength {
return false
}
return usernamePattern.MatchString(username)
}

func location(fl validator.FieldLevel) bool {
location := fl.Field().String()
return product.IsLocationSupported(location)
}

// SECURE: Validate Monero address properly with regex
func xmrAddress(f1 validator.FieldLevel) bool {
address := f1.Field().String()
if len(address) != 95 {
return false
}
return xmrAddressPattern.MatchString(address)
}

func locale(f1 validator.FieldLevel) bool {
locale := fl.Field().String()
return globalize.ValidLocale(locale)
}

func currency(f1 validator.FieldLevel) bool {
currency := f1.Field().String()
return currency_service.Currency(currency).IsSupported()
}

// SECURE: Sanitize HTML to prevent XSS
func SanitizeHTML(input string) string {
// HTML escape special characters
return html.EscapeString(input)
}

// SECURE: Sanitize text for safe display (removes HTML tags)
func SanitizeText(input string) string {
// Remove all HTML tags
re := regexp.MustCompile(`<[^>]*>`)
return html.EscapeString(re.ReplaceAllString(input, ""))
}

// SECURE: Sanitize for use in JavaScript strings
func SanitizeJS(input string) string {
var sb strings.Builder
for _, r := range input {
switch r {
case '\'':
sb.WriteString(`\'`)
case '"':
sb.WriteString(`\"`)
case '\\':
sb.WriteString(`\\`)
case '<':
sb.WriteString(`\u003C`)
case '>':
sb.WriteString(`\u003E`)
case '&':
sb.WriteString(`\u0026`)
default:
sb.WriteRune(r)
}
}
return sb.String()
}

// SECURE: Validate that input doesn't contain null bytes or control characters
func isSafeInput(input string) bool {
for _, r := range input {
// Allow printable characters, newlines, and tabs
if r < 32 && r != '\t' && r != '\n' && r != '\r' {
return false
}
// Disallow null byte
if r == 0 {
return false
}
}
return true
}

// SECURE: Validate all text inputs against injection patterns
func ValidateTextInput(input string, maxLen int) bool {
if len(input) == 0 || len(input) > maxLen {
return false
}
return isSafeInput(input)
}

// SECURE: Validate username input
func ValidateUsername(username string) bool {
return validateUsername(username)
}

// SECURE: Truncate text with ellipsis
func TruncateText(input string, maxLen int) string {
if len(input) <= maxLen {
return input
}
return input[:maxLen-3] + "..."
}

// SECURE: Template function for safe HTML rendering
func HTMLEscape(input interface{}) template.HTML {
return template.HTML(html.EscapeString(fmt.Sprintf("%v", input)))
}
