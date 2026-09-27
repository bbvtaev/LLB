// Package srs holds the spaced-repetition rules: how a rating moves a word's next review.
package srs

import "time"

type Rating string

const (
	Repeat Rating = "repeat"
	Good   Rating = "good"
	Fluent Rating = "fluent"
)

// maxFluentDoublings caps the fluent interval at 3 * 2^6 = 192 days.
const maxFluentDoublings = 6

func Parse(s string) (Rating, bool) {
	switch r := Rating(s); r {
	case Repeat, Good, Fluent:
		return r, true
	}
	return "", false
}

// Next returns the word's new fluent streak and when it is due again.
// repeat → 1 minute, good → 1 day, fluent → 3, 6, 12, … days for consecutive fluent answers.
func Next(r Rating, streak int, now time.Time) (int, time.Time) {
	switch r {
	case Repeat:
		return 0, now.Add(time.Minute)
	case Good:
		return 0, now.Add(24 * time.Hour)
	default:
		days := 3 << min(streak, maxFluentDoublings)
		return streak + 1, now.Add(time.Duration(days) * 24 * time.Hour)
	}
}
