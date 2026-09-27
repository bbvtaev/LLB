package bot

import (
	"slices"
	"testing"
)

func TestParseEntry(t *testing.T) {
	tests := []struct {
		line string
		want entry
		ok   bool
	}{
		{"apple - яблоко", entry{Word: "apple", Translation: "яблоко"}, true},
		{"well-known — известный", entry{Word: "well-known", Translation: "известный"}, true},
		{"猫 = кошка | ねこ #Животные #n5", entry{Word: "猫", Translation: "кошка", Note: "ねこ", Groups: []string{"животные", "n5"}}, true},
		{"#arch load balancer - балансировщик нагрузки | распределяет трафик", entry{Word: "load balancer", Translation: "балансировщик нагрузки", Note: "распределяет трафик", Groups: []string{"arch"}}, true},
		{"just a sentence", entry{}, false},
		{"apple - ", entry{}, false},
	}
	for _, tt := range tests {
		got, ok := parseEntry(tt.line)
		if ok != tt.ok {
			t.Errorf("parseEntry(%q) ok = %v, want %v", tt.line, ok, tt.ok)
			continue
		}
		if !ok {
			continue
		}
		if got.Word != tt.want.Word || got.Translation != tt.want.Translation || got.Note != tt.want.Note || !slices.Equal(got.Groups, tt.want.Groups) {
			t.Errorf("parseEntry(%q) = %+v, want %+v", tt.line, got, tt.want)
		}
	}
}

func TestCheckAnswer(t *testing.T) {
	tests := []struct {
		input, expected string
		want            bool
	}{
		{"Яблоко", "яблоко", true},
		{" ёлка! ", "елка", true},
		{"дом", "дом, здание", true},
		{"home", "house / home", true},
		{"замок", "замок (строение)", true},
		{"дом, здание", "дом, здание", true},
		{"дома", "дом", false},
		{"", "дом", false},
	}
	for _, tt := range tests {
		if got := checkAnswer(tt.input, tt.expected); got != tt.want {
			t.Errorf("checkAnswer(%q, %q) = %v, want %v", tt.input, tt.expected, got, tt.want)
		}
	}
}
