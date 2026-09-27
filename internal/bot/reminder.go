package bot

import (
	"context"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"
)

// remindLoop sends a reminder at each Config.RemindAt time if some words are due.
func (a *App) remindLoop(ctx context.Context) {
	if len(a.cfg.RemindAt) == 0 {
		return
	}
	ticker := time.NewTicker(20 * time.Second)
	defer ticker.Stop()
	sent := map[string]bool{} // "2006-01-02 15:04" slots already handled
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			hm := now.Format("15:04")
			slot := now.Format("2006-01-02 ") + hm
			if !slices.Contains(a.cfg.RemindAt, hm) || sent[slot] {
				continue
			}
			sent[slot] = true
			a.remind(ctx)
		}
	}
}

func (a *App) remind(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.sess != nil {
		return // already reviewing
	}
	langs, err := a.st.LanguageCounts(ctx)
	if err != nil {
		log.Printf("reminder: %v", err)
		return
	}
	var lines []string
	var rows [][]models.InlineKeyboardButton
	for _, l := range langs {
		if l.Due == 0 {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s: %d", langTag(l.Name), l.Due))
		rows = append(rows, []models.InlineKeyboardButton{{
			Text:         fmt.Sprintf("Review %s (%d)", l.Name, l.Due),
			CallbackData: "rl:" + l.Name,
		}})
	}
	if len(lines) == 0 {
		return
	}
	a.send(ctx, "⏰ Time to review:\n"+strings.Join(lines, "\n"), keyboard(rows))
}
