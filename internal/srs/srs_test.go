package srs

import (
	"testing"
	"time"
)

func TestNext(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	tests := []struct {
		rating     Rating
		streak     int
		wantStreak int
		wantIn     time.Duration
	}{
		{Repeat, 3, 0, time.Minute},
		{Good, 3, 0, day},
		{Fluent, 0, 1, 3 * day},
		{Fluent, 1, 2, 6 * day},
		{Fluent, 2, 3, 12 * day},
		{Fluent, 6, 7, 192 * day},
		{Fluent, 20, 21, 192 * day},
	}
	for _, tt := range tests {
		streak, due := Next(tt.rating, tt.streak, now)
		if streak != tt.wantStreak || due.Sub(now) != tt.wantIn {
			t.Errorf("Next(%s, %d) = %d, +%v; want %d, +%v", tt.rating, tt.streak, streak, due.Sub(now), tt.wantStreak, tt.wantIn)
		}
	}
}
