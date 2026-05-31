package tags

import "fmt"

// fieldString converts a WritableFields value to its string representation.
// Integers are formatted with strconv; all other types use fmt.Sprintf.
func fieldString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case int:
		return fmt.Sprintf("%d", t)
	case int64:
		return fmt.Sprintf("%d", t)
	case float64:
		return fmt.Sprintf("%g", t)
	case float32:
		return fmt.Sprintf("%g", t)
	default:
		return fmt.Sprintf("%v", t)
	}
}
