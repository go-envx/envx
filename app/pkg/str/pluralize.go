package str

import "fmt"

// Pluralize renders a count with its noun, using the singular form only when the
// count is exactly one and the plural form otherwise.
func Pluralize(n int, singular, plural string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, singular)
	}
	return fmt.Sprintf("%d %s", n, plural)
}
