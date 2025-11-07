package cron

import (
	"fmt"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// Parser handles CRON expression parsing with seconds support
type Parser struct {
	parser cron.Parser
}

// NewParser creates a new CRON parser that supports 6-field expressions with seconds
func NewParser() *Parser {
	// Configure parser to handle seconds (6 fields)
	// Format: second minute hour day month weekday
	parser := cron.NewParser(
		cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor,
	)
	return &Parser{
		parser: parser,
	}
}

// Validate checks if a CRON expression is valid
func (p *Parser) Validate(expression string) error {
	if expression == "" {
		return fmt.Errorf("CRON expression cannot be empty")
	}

	// Check field count
	fields := strings.Fields(expression)
	if len(fields) != 6 {
		return fmt.Errorf("CRON expression must have exactly 6 fields (second minute hour day month weekday), got %d fields", len(fields))
	}

	// Try to parse the expression
	_, err := p.parser.Parse(expression)
	if err != nil {
		return fmt.Errorf("invalid CRON expression: %w", err)
	}

	return nil
}

// Parse parses a CRON expression and returns a Schedule
func (p *Parser) Parse(expression string) (cron.Schedule, error) {
	return p.parser.Parse(expression)
}

// GetNextRun calculates the next run time for a CRON expression
func (p *Parser) GetNextRun(expression string, from time.Time) (time.Time, error) {
	schedule, err := p.Parse(expression)
	if err != nil {
		return time.Time{}, err
	}

	return schedule.Next(from), nil
}

// GetNextNRuns calculates the next N run times for a CRON expression
func (p *Parser) GetNextNRuns(expression string, n int, from time.Time) ([]time.Time, error) {
	if n <= 0 {
		return nil, fmt.Errorf("n must be positive")
	}

	schedule, err := p.Parse(expression)
	if err != nil {
		return nil, err
	}

	runs := make([]time.Time, 0, n)
	current := from
	for i := 0; i < n; i++ {
		next := schedule.Next(current)
		if next.IsZero() {
			break
		}
		runs = append(runs, next)
		current = next
	}

	return runs, nil
}

// ExplainExpression provides a human-readable explanation of a CRON expression
func (p *Parser) ExplainExpression(expression string) (string, error) {
	if err := p.Validate(expression); err != nil {
		return "", err
	}

	fields := strings.Fields(expression)
	if len(fields) != 6 {
		return "", fmt.Errorf("invalid field count")
	}

	parts := []string{
		fmt.Sprintf("Second: %s", explainField(fields[0], "second", 0, 59)),
		fmt.Sprintf("Minute: %s", explainField(fields[1], "minute", 0, 59)),
		fmt.Sprintf("Hour: %s", explainField(fields[2], "hour", 0, 23)),
		fmt.Sprintf("Day: %s", explainField(fields[3], "day", 1, 31)),
		fmt.Sprintf("Month: %s", explainField(fields[4], "month", 1, 12)),
		fmt.Sprintf("Weekday: %s", explainField(fields[5], "weekday", 0, 6)),
	}

	return strings.Join(parts, "\n"), nil
}

// explainField provides a human-readable explanation for a single CRON field
func explainField(field, name string, min, max int) string {
	if field == "*" {
		return fmt.Sprintf("every %s", name)
	}
	if field == "?" {
		return fmt.Sprintf("any %s", name)
	}
	if strings.Contains(field, "-") {
		return fmt.Sprintf("%s in range %s", name, field)
	}
	if strings.Contains(field, ",") {
		return fmt.Sprintf("%s at %s", name, field)
	}
	if strings.Contains(field, "/") {
		parts := strings.Split(field, "/")
		if len(parts) == 2 {
			return fmt.Sprintf("every %s %s starting at %s", parts[1], name, parts[0])
		}
	}
	return fmt.Sprintf("%s at %s", name, field)
}

// IsWithinTimeWindow checks if the current time is within a job's execution window
func (p *Parser) IsWithinTimeWindow(expression string, window time.Duration) (bool, error) {
	nextRun, err := p.GetNextRun(expression, time.Now())
	if err != nil {
		return false, err
	}

	timeUntilNext := time.Until(nextRun)
	return timeUntilNext <= window && timeUntilNext >= 0, nil
}
