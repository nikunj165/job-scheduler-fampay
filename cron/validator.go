package cron

import (
	"fmt"
	"strconv"
	"strings"
)

// ValidateField validates a single CRON field
func ValidateField(field string, min, max int, allowedNames []string) error {
	// Handle wildcards
	if field == "*" || field == "?" {
		return nil
	}

	// Handle lists (e.g., "1,3,5")
	if strings.Contains(field, ",") {
		parts := strings.Split(field, ",")
		for _, part := range parts {
			if err := ValidateField(strings.TrimSpace(part), min, max, allowedNames); err != nil {
				return err
			}
		}
		return nil
	}

	// Handle ranges (e.g., "10-15")
	if strings.Contains(field, "-") {
		parts := strings.Split(field, "-")
		if len(parts) != 2 {
			return fmt.Errorf("invalid range format: %s", field)
		}

		start := strings.TrimSpace(parts[0])
		end := strings.TrimSpace(parts[1])

		// Check if using names (e.g., MON-FRI)
		if allowedNames != nil {
			startIdx := findNameIndex(start, allowedNames)
			endIdx := findNameIndex(end, allowedNames)
			if startIdx >= 0 && endIdx >= 0 {
				if startIdx > endIdx {
					return fmt.Errorf("invalid range: %s-%s", start, end)
				}
				return nil
			}
		}

		// Numeric range
		startNum, err1 := strconv.Atoi(start)
		endNum, err2 := strconv.Atoi(end)
		if err1 != nil || err2 != nil {
			return fmt.Errorf("invalid range values: %s", field)
		}
		if startNum < min || startNum > max || endNum < min || endNum > max {
			return fmt.Errorf("range values must be between %d and %d", min, max)
		}
		if startNum > endNum {
			return fmt.Errorf("invalid range: start (%d) > end (%d)", startNum, endNum)
		}
		return nil
	}

	// Handle steps (e.g., "*/5" or "0-30/5")
	if strings.Contains(field, "/") {
		parts := strings.Split(field, "/")
		if len(parts) != 2 {
			return fmt.Errorf("invalid step format: %s", field)
		}

		// Validate the range/start part
		rangePart := strings.TrimSpace(parts[0])
		if rangePart != "*" {
			if strings.Contains(rangePart, "-") {
				if err := ValidateField(rangePart, min, max, allowedNames); err != nil {
					return err
				}
			} else {
				num, err := strconv.Atoi(rangePart)
				if err != nil || num < min || num > max {
					return fmt.Errorf("invalid step start value: %s", rangePart)
				}
			}
		}

		// Validate the step value
		step, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil || step <= 0 {
			return fmt.Errorf("step value must be a positive integer: %s", parts[1])
		}
		return nil
	}

	// Check if it's a named value (e.g., MON, JAN)
	if allowedNames != nil {
		if findNameIndex(field, allowedNames) >= 0 {
			return nil
		}
	}

	// Single numeric value
	num, err := strconv.Atoi(field)
	if err != nil {
		if allowedNames != nil {
			return fmt.Errorf("invalid value: %s (not a number or recognized name)", field)
		}
		return fmt.Errorf("invalid numeric value: %s", field)
	}

	if num < min || num > max {
		return fmt.Errorf("value %d must be between %d and %d", num, min, max)
	}

	return nil
}

// findNameIndex finds the index of a name in the allowed names list (case-insensitive)
func findNameIndex(name string, allowedNames []string) int {
	upperName := strings.ToUpper(name)
	for i, allowed := range allowedNames {
		if strings.ToUpper(allowed) == upperName {
			return i
		}
	}
	return -1
}

// ValidateCronExpression performs detailed validation of a 6-field CRON expression
func ValidateCronExpression(expression string) error {
	fields := strings.Fields(expression)
	if len(fields) != 6 {
		return fmt.Errorf("CRON expression must have exactly 6 fields (second minute hour day month weekday), got %d fields", len(fields))
	}

	// Define allowed names for months and weekdays
	monthNames := []string{"JAN", "FEB", "MAR", "APR", "MAY", "JUN", "JUL", "AUG", "SEP", "OCT", "NOV", "DEC"}
	weekdayNames := []string{"SUN", "MON", "TUE", "WED", "THU", "FRI", "SAT"}

	// Validate each field
	fieldSpecs := []struct {
		name         string
		min, max     int
		allowedNames []string
	}{
		{"second", 0, 59, nil},
		{"minute", 0, 59, nil},
		{"hour", 0, 23, nil},
		{"day of month", 1, 31, nil},
		{"month", 1, 12, monthNames},
		{"weekday", 0, 6, weekdayNames},
	}

	for i, spec := range fieldSpecs {
		if err := ValidateField(fields[i], spec.min, spec.max, spec.allowedNames); err != nil {
			return fmt.Errorf("%s field: %w", spec.name, err)
		}
	}

	return nil
}
