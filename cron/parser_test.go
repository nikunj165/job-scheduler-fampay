package cron

import (
	"testing"
	"time"
)

func TestParser_Validate(t *testing.T) {
	parser := NewParser()

	tests := []struct {
		name       string
		expression string
		wantErr    bool
	}{
		{
			name:       "Valid expression with seconds",
			expression: "31 10-15 1 * * MON-FRI",
			wantErr:    false,
		},
		{
			name:       "Every 30 seconds",
			expression: "*/30 * * * * *",
			wantErr:    false,
		},
		{
			name:       "Every minute at 0 seconds",
			expression: "0 * * * * *",
			wantErr:    false,
		},
		{
			name:       "Complex expression",
			expression: "0 0,15,30,45 9-17 * * MON-FRI",
			wantErr:    false,
		},
		{
			name:       "Invalid - too few fields",
			expression: "0 0 * * *",
			wantErr:    true,
		},
		{
			name:       "Invalid - too many fields",
			expression: "0 0 0 * * * *",
			wantErr:    true,
		},
		{
			name:       "Invalid - empty expression",
			expression: "",
			wantErr:    true,
		},
		{
			name:       "Invalid - bad range",
			expression: "0 60 * * * *",
			wantErr:    true,
		},
		{
			name:       "Valid with month names",
			expression: "0 0 0 1 JAN-DEC *",
			wantErr:    false,
		},
		{
			name:       "Valid with weekday names",
			expression: "0 0 0 * * SUN",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := parser.Validate(tt.expression)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParser_GetNextRun(t *testing.T) {
	parser := NewParser()

	// Fixed time for testing: January 1, 2024, 00:00:00
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	tests := []struct {
		name       string
		expression string
		from       time.Time
		wantAfter  time.Time
	}{
		{
			name:       "Every minute at 30 seconds",
			expression: "30 * * * * *",
			from:       baseTime,
			wantAfter:  baseTime.Add(30 * time.Second),
		},
		{
			name:       "Every hour at 0 minutes 0 seconds",
			expression: "0 0 * * * *",
			from:       baseTime,
			wantAfter:  baseTime.Add(1 * time.Hour),
		},
		{
			name:       "Specific time daily",
			expression: "0 30 14 * * *", // 2:30 PM every day
			from:       baseTime,
			wantAfter:  time.Date(2024, 1, 1, 14, 30, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parser.GetNextRun(tt.expression, tt.from)
			if err != nil {
				t.Fatalf("GetNextRun() error = %v", err)
			}
			if !got.Equal(tt.wantAfter) {
				t.Errorf("GetNextRun() = %v, want %v", got, tt.wantAfter)
			}
		})
	}
}

func TestParser_GetNextNRuns(t *testing.T) {
	parser := NewParser()
	baseTime := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)

	// Test getting next 5 runs for "every 10 seconds"
	expression := "*/10 * * * * *"
	runs, err := parser.GetNextNRuns(expression, 5, baseTime)
	if err != nil {
		t.Fatalf("GetNextNRuns() error = %v", err)
	}

	if len(runs) != 5 {
		t.Fatalf("Expected 5 runs, got %d", len(runs))
	}

	// Check that runs are 10 seconds apart
	for i := 0; i < len(runs); i++ {
		expectedTime := baseTime.Add(time.Duration((i+1)*10) * time.Second)
		if !runs[i].Equal(expectedTime) {
			t.Errorf("Run %d: got %v, want %v", i, runs[i], expectedTime)
		}
	}
}

func TestValidateCronExpression(t *testing.T) {
	tests := []struct {
		name        string
		expression  string
		wantErr     bool
		errContains string
	}{
		{
			name:       "Valid basic expression",
			expression: "0 0 0 * * *",
			wantErr:    false,
		},
		{
			name:       "Valid with ranges",
			expression: "0 10-15 1 * * MON-FRI",
			wantErr:    false,
		},
		{
			name:       "Valid with lists",
			expression: "0,15,30,45 * * * * *",
			wantErr:    false,
		},
		{
			name:       "Valid with steps",
			expression: "*/5 * * * * *",
			wantErr:    false,
		},
		{
			name:        "Invalid second value",
			expression:  "60 * * * * *",
			wantErr:     true,
			errContains: "second field",
		},
		{
			name:        "Invalid minute value",
			expression:  "0 60 * * * *",
			wantErr:     true,
			errContains: "minute field",
		},
		{
			name:        "Invalid hour value",
			expression:  "0 0 24 * * *",
			wantErr:     true,
			errContains: "hour field",
		},
		{
			name:        "Invalid day value",
			expression:  "0 0 0 32 * *",
			wantErr:     true,
			errContains: "day of month field",
		},
		{
			name:        "Invalid month value",
			expression:  "0 0 0 * 13 *",
			wantErr:     true,
			errContains: "month field",
		},
		{
			name:        "Invalid weekday value",
			expression:  "0 0 0 * * 7",
			wantErr:     true,
			errContains: "weekday field",
		},
		{
			name:       "Valid month names",
			expression: "0 0 0 * JAN,FEB,MAR *",
			wantErr:    false,
		},
		{
			name:       "Valid weekday names",
			expression: "0 0 0 * * MON,WED,FRI",
			wantErr:    false,
		},
		{
			name:        "Invalid month name",
			expression:  "0 0 0 * INVALID *",
			wantErr:     true,
			errContains: "month field",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCronExpression(tt.expression)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateCronExpression() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil && tt.errContains != "" {
				if !contains(err.Error(), tt.errContains) {
					t.Errorf("Error message should contain '%s', got: %s", tt.errContains, err.Error())
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && s[:len(substr)] == substr ||
		len(s) > len(substr) && contains(s[1:], substr)
}
