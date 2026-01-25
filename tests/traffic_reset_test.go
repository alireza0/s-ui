// Package tests contains unit tests for the traffic reset feature.
// These tests are designed to be run independently without the full project dependencies.
package tests

import (
	"testing"
	"time"
)

// Client mirrors the model.Client struct for testing purposes
type Client struct {
	Id              uint
	Name            string
	Enable          bool
	Volume          int64
	Up              int64
	Down            int64
	CreatedAt       int64
	ResetMode       int
	ResetDayOfMonth int
	ResetPeriodDays int
	LastResetAt     int64
}

// Reset mode constants (mirrors model constants)
const (
	ResetModeDisabled = 0
	ResetModeMonthly  = 1
	ResetModePeriodic = 2
)

// checkMonthlyReset checks if a client should have its traffic reset based on monthly schedule
// This is a copy of the logic from service/client.go for testing purposes
func checkMonthlyReset(client Client, now time.Time) (bool, int64) {
	today := now.Day()
	currentMonth := now.Month()
	currentYear := now.Year()

	// Determine reset day
	resetDay := client.ResetDayOfMonth
	if resetDay <= 0 {
		// Use creation day as default
		createdAt := time.Unix(client.CreatedAt, 0)
		resetDay = createdAt.Day()
	}

	// Handle months with fewer days
	daysInMonth := time.Date(currentYear, currentMonth+1, 0, 0, 0, 0, 0, now.Location()).Day()
	if resetDay > daysInMonth {
		resetDay = daysInMonth
	}

	// Check if today is the reset day
	if today != resetDay {
		return false, 0
	}

	// Check if already reset this month
	if client.LastResetAt > 0 {
		lastReset := time.Unix(client.LastResetAt, 0)
		if lastReset.Month() == currentMonth && lastReset.Year() == currentYear {
			return false, 0
		}
	}

	// Calculate period start time (last reset or creation time)
	periodStart := client.LastResetAt
	if periodStart == 0 {
		periodStart = client.CreatedAt
	}

	return true, periodStart
}

// checkPeriodicReset checks if a client should have its traffic reset based on N-day period
// This is a copy of the logic from service/client.go for testing purposes
func checkPeriodicReset(client Client, now time.Time) (bool, int64) {
	if client.ResetPeriodDays <= 0 {
		return false, 0
	}

	nowUnix := now.Unix()
	periodSeconds := int64(client.ResetPeriodDays) * 24 * 60 * 60

	// Determine the reference point (last reset or creation time)
	referenceTime := client.LastResetAt
	if referenceTime == 0 {
		referenceTime = client.CreatedAt
	}

	// Check if enough time has passed since last reset
	timeSinceReference := nowUnix - referenceTime
	if timeSinceReference < periodSeconds {
		return false, 0
	}

	return true, referenceTime
}

func TestCheckMonthlyReset(t *testing.T) {
	tests := []struct {
		name        string
		client      Client
		now         time.Time
		shouldReset bool
	}{
		{
			name: "Should reset on reset day",
			client: Client{
				ResetMode:       ResetModeMonthly,
				ResetDayOfMonth: 15,
				CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
				LastResetAt:     0,
			},
			now:         time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			shouldReset: true,
		},
		{
			name: "Should not reset on non-reset day",
			client: Client{
				ResetMode:       ResetModeMonthly,
				ResetDayOfMonth: 15,
				CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
				LastResetAt:     0,
			},
			now:         time.Date(2026, 1, 14, 10, 0, 0, 0, time.UTC),
			shouldReset: false,
		},
		{
			name: "Should not reset if already reset this month",
			client: Client{
				ResetMode:       ResetModeMonthly,
				ResetDayOfMonth: 15,
				CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
				LastResetAt:     time.Date(2026, 1, 15, 1, 0, 0, 0, time.UTC).Unix(),
			},
			now:         time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC),
			shouldReset: false,
		},
		{
			name: "Should use creation day when resetDayOfMonth is 0",
			client: Client{
				ResetMode:       ResetModeMonthly,
				ResetDayOfMonth: 0, // Use creation day
				CreatedAt:       time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC).Unix(),
				LastResetAt:     0,
			},
			now:         time.Date(2026, 2, 20, 10, 0, 0, 0, time.UTC),
			shouldReset: true,
		},
		{
			name: "Should handle month with fewer days (Feb 28)",
			client: Client{
				ResetMode:       ResetModeMonthly,
				ResetDayOfMonth: 31, // Will be adjusted to 28 for Feb
				CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
				LastResetAt:     0,
			},
			now:         time.Date(2026, 2, 28, 10, 0, 0, 0, time.UTC),
			shouldReset: true,
		},
		{
			name: "Should reset in next month after previous reset",
			client: Client{
				ResetMode:       ResetModeMonthly,
				ResetDayOfMonth: 1,
				CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
				LastResetAt:     time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
			},
			now:         time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC),
			shouldReset: true,
		},
		{
			name: "Should handle leap year Feb 29",
			client: Client{
				ResetMode:       ResetModeMonthly,
				ResetDayOfMonth: 29,
				CreatedAt:       time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
				LastResetAt:     0,
			},
			now:         time.Date(2024, 2, 29, 10, 0, 0, 0, time.UTC), // 2024 is a leap year
			shouldReset: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldReset, _ := checkMonthlyReset(tt.client, tt.now)
			if shouldReset != tt.shouldReset {
				t.Errorf("checkMonthlyReset() = %v, want %v", shouldReset, tt.shouldReset)
			}
		})
	}
}

func TestCheckPeriodicReset(t *testing.T) {
	tests := []struct {
		name        string
		client      Client
		now         time.Time
		shouldReset bool
	}{
		{
			name: "Should reset after period elapsed",
			client: Client{
				ResetMode:       ResetModePeriodic,
				ResetPeriodDays: 7,
				CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
				LastResetAt:     0,
			},
			now:         time.Date(2026, 1, 8, 10, 0, 0, 0, time.UTC),
			shouldReset: true,
		},
		{
			name: "Should not reset before period elapsed",
			client: Client{
				ResetMode:       ResetModePeriodic,
				ResetPeriodDays: 7,
				CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
				LastResetAt:     0,
			},
			now:         time.Date(2026, 1, 5, 10, 0, 0, 0, time.UTC),
			shouldReset: false,
		},
		{
			name: "Should reset based on last reset time",
			client: Client{
				ResetMode:       ResetModePeriodic,
				ResetPeriodDays: 30,
				CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
				LastResetAt:     time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC).Unix(),
			},
			now:         time.Date(2026, 2, 14, 10, 0, 0, 0, time.UTC),
			shouldReset: true,
		},
		{
			name: "Should not reset with invalid period days (0)",
			client: Client{
				ResetMode:       ResetModePeriodic,
				ResetPeriodDays: 0,
				CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
				LastResetAt:     0,
			},
			now:         time.Date(2026, 12, 31, 10, 0, 0, 0, time.UTC),
			shouldReset: false,
		},
		{
			name: "Should not reset with negative period days",
			client: Client{
				ResetMode:       ResetModePeriodic,
				ResetPeriodDays: -1,
				CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
				LastResetAt:     0,
			},
			now:         time.Date(2026, 12, 31, 10, 0, 0, 0, time.UTC),
			shouldReset: false,
		},
		{
			name: "Should reset exactly at period boundary (daily)",
			client: Client{
				ResetMode:       ResetModePeriodic,
				ResetPeriodDays: 1, // Daily reset
				CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
				LastResetAt:     0,
			},
			now:         time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC),
			shouldReset: true,
		},
		{
			name: "Should handle weekly reset (7 days)",
			client: Client{
				ResetMode:       ResetModePeriodic,
				ResetPeriodDays: 7,
				CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
				LastResetAt:     time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC).Unix(),
			},
			now:         time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
			shouldReset: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shouldReset, _ := checkPeriodicReset(tt.client, tt.now)
			if shouldReset != tt.shouldReset {
				t.Errorf("checkPeriodicReset() = %v, want %v", shouldReset, tt.shouldReset)
			}
		})
	}
}

func TestCheckMonthlyResetPeriodStartTime(t *testing.T) {
	// Test that periodStartTime is correctly returned
	client := Client{
		ResetMode:       ResetModeMonthly,
		ResetDayOfMonth: 15,
		CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
		LastResetAt:     time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC).Unix(),
	}
	now := time.Date(2026, 2, 15, 10, 0, 0, 0, time.UTC)

	shouldReset, periodStart := checkMonthlyReset(client, now)
	if !shouldReset {
		t.Errorf("Expected shouldReset=true, got false")
	}
	if periodStart != client.LastResetAt {
		t.Errorf("Expected periodStart=%d, got %d", client.LastResetAt, periodStart)
	}
}

func TestCheckPeriodicResetPeriodStartTime(t *testing.T) {
	// Test that periodStartTime is correctly returned
	client := Client{
		ResetMode:       ResetModePeriodic,
		ResetPeriodDays: 7,
		CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
		LastResetAt:     time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC).Unix(),
	}
	now := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)

	shouldReset, periodStart := checkPeriodicReset(client, now)
	if !shouldReset {
		t.Errorf("Expected shouldReset=true, got false")
	}
	if periodStart != client.LastResetAt {
		t.Errorf("Expected periodStart=%d, got %d", client.LastResetAt, periodStart)
	}
}

func TestCheckMonthlyResetUsesCreatedAtWhenNoLastReset(t *testing.T) {
	client := Client{
		ResetMode:       ResetModeMonthly,
		ResetDayOfMonth: 1,
		CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
		LastResetAt:     0, // Never reset before
	}
	now := time.Date(2026, 2, 1, 10, 0, 0, 0, time.UTC)

	shouldReset, periodStart := checkMonthlyReset(client, now)
	if !shouldReset {
		t.Errorf("Expected shouldReset=true, got false")
	}
	if periodStart != client.CreatedAt {
		t.Errorf("Expected periodStart to be CreatedAt=%d, got %d", client.CreatedAt, periodStart)
	}
}

func TestCheckPeriodicResetUsesCreatedAtWhenNoLastReset(t *testing.T) {
	client := Client{
		ResetMode:       ResetModePeriodic,
		ResetPeriodDays: 30,
		CreatedAt:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC).Unix(),
		LastResetAt:     0, // Never reset before
	}
	now := time.Date(2026, 1, 31, 10, 0, 0, 0, time.UTC)

	shouldReset, periodStart := checkPeriodicReset(client, now)
	if !shouldReset {
		t.Errorf("Expected shouldReset=true, got false")
	}
	if periodStart != client.CreatedAt {
		t.Errorf("Expected periodStart to be CreatedAt=%d, got %d", client.CreatedAt, periodStart)
	}
}
