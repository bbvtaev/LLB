package bot

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"strconv"
	"strings"
	"time"

	"github.com/go-telegram/bot/models"

	"llb/internal/srs"
	"llb/internal/store"
)

// session is the review in progress. There is at most one, since the bot has a single user.
type session struct {
	queue []int64
	card  *card // the card waiting for a rating, nil between cards
	rated map[srs.Rating]int
}

type card struct {
	word    store.Word
	reverse bool   // ask for the word by its translation
	msgID   int    // the card's Telegram message
	typed   string // feedback on a typed answer, if any
}

func (a *App) reviewMenu(ctx context.Context) {
	lang := a.lang(ctx)
	langs, err := a.st.LanguageCounts(ctx)
	if err != nil {
		a.fail(ctx, "review menu", err)
		return
	}
	var all store.Count
	for _, l := range langs {
		if l.Name == lang {
			all = l
		}
	}
	if all.Total == 0 {
		a.send(ctx, "No words in "+langTag(lang)+" yet. Add one: <code>perro - dog</code>", nil)
		return
	}
	groups, err := a.st.GroupCounts(ctx, lang)
	if err != nil {
		a.fail(ctx, "review menu", err)
		return
	}

	rows := [][]models.InlineKeyboardButton{reviewButtons("All", 0, all)}
	for _, g := range groups {
		rows = append(rows, reviewButtons("#"+g.Name, g.ID, g))
	}
	a.send(ctx, fmt.Sprintf("%s What to review?\n⏰: only words whose timer is up\n📚: every word", langTag(lang)), keyboard(rows))
}

func reviewButtons(label string, groupID int64, c store.Count) []models.InlineKeyboardButton {
	id := strconv.FormatInt(groupID, 10)
	return []models.InlineKeyboardButton{
		{Text: fmt.Sprintf("%s ⏰ %d", label, c.Due), CallbackData: "rv:" + id + ":d"},
		{Text: fmt.Sprintf("%s 📚 %d", label, c.Total), CallbackData: "rv:" + id + ":a"},
	}
}

func (a *App) onReviewChoice(ctx context.Context, arg string) {
	idStr, mode, _ := strings.Cut(arg, ":")
	groupID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		return
	}
	a.startReview(ctx, groupID, mode == "d")
}

func (a *App) startReview(ctx context.Context, groupID int64, dueOnly bool) {
	if a.sess != nil {
		a.closeCard(ctx, "")
	}
	lang := a.lang(ctx)
	ids, err := a.st.ReviewIDs(ctx, lang, groupID, dueOnly)
	if err != nil {
		a.fail(ctx, "start review", err)
		return
	}
	if len(ids) == 0 {
		a.sess = nil
		a.send(ctx, "🎉 Nothing to review right now. All timers are still running.", nil)
		return
	}
	a.sess = &session{queue: ids, rated: map[srs.Rating]int{}}
	a.send(ctx, fmt.Sprintf("Let's go: %d words. Type your answer or just tap a button.", len(ids)), nil)
	a.nextCard(ctx)
}

// nextCard shows the next word of the session, or finishes it when the queue is empty.
func (a *App) nextCard(ctx context.Context) {
	s := a.sess
	for len(s.queue) > 0 {
		id := s.queue[0]
		s.queue = s.queue[1:]
		w, err := a.st.Word(ctx, id)
		if errors.Is(err, store.ErrNotFound) {
			continue // deleted mid-session
		}
		if err != nil {
			a.fail(ctx, "next card", err)
			a.sess = nil
			return
		}
		c := &card{word: w, reverse: a.pickReverse(ctx)}
		c.msgID = a.send(ctx, renderCard(c, false, ""), rateKeyboard(w.ID))
		s.card = c
		return
	}
	a.finishReview(ctx)
}

func (a *App) pickReverse(ctx context.Context) bool {
	switch a.dir(ctx) {
	case "rev":
		return true
	case "mix":
		return rand.IntN(2) == 0
	}
	return false
}

// answer checks a typed answer against the current card and reveals it.
func (a *App) answer(ctx context.Context, text string) {
	c := a.sess.card
	_, expected := c.sides()
	if checkAnswer(text, expected) {
		c.typed = "✅ Correct"
	} else {
		c.typed = "❌ Your answer: <s>" + esc(text) + "</s>"
	}
	a.edit(ctx, c.msgID, renderCard(c, true, "How well did you know it?"), rateKeyboard(c.word.ID))
}

func (a *App) onRate(ctx context.Context, arg string) {
	idStr, ratingStr, _ := strings.Cut(arg, ":")
	id, _ := strconv.ParseInt(idStr, 10, 64)
	rating, ok := srs.Parse(ratingStr)
	if !ok {
		return
	}
	if a.sess == nil || a.sess.card == nil || a.sess.card.word.ID != id {
		a.send(ctx, "This card is outdated. /review", nil)
		return
	}

	c := a.sess.card
	now := time.Now()
	streak, due := srs.Next(rating, c.word.Streak, now)
	if err := a.st.Rate(ctx, id, string(rating), streak, due); err != nil {
		a.fail(ctx, "rate", err)
		return
	}
	a.sess.rated[rating]++
	a.closeCard(ctx, fmt.Sprintf("%s %s · next review in %s", statusIcons[string(rating)], rating, humanDuration(due.Sub(now))))
	a.nextCard(ctx)
}

func (a *App) onSkip(ctx context.Context) {
	if a.sess == nil || a.sess.card == nil {
		return
	}
	a.closeCard(ctx, "⏭ skipped")
	a.nextCard(ctx)
}

func (a *App) stopReview(ctx context.Context) {
	if a.sess == nil {
		a.send(ctx, "No review in progress. /review", nil)
		return
	}
	a.closeCard(ctx, "⏹ stopped")
	a.finishReview(ctx)
}

// closeCard reveals the current card with a result line and removes its buttons.
func (a *App) closeCard(ctx context.Context, result string) {
	c := a.sess.card
	if c == nil {
		return
	}
	a.edit(ctx, c.msgID, renderCard(c, true, result), nil)
	a.sess.card = nil
}

func (a *App) finishReview(ctx context.Context) {
	s := a.sess
	a.sess = nil
	total := s.rated[srs.Repeat] + s.rated[srs.Good] + s.rated[srs.Fluent]
	text := fmt.Sprintf("Done! Words rated: %d\n🔁 %d · 👍 %d · 🚀 %d",
		total, s.rated[srs.Repeat], s.rated[srs.Good], s.rated[srs.Fluent])
	if s.rated[srs.Repeat] > 0 {
		text += "\n\nWords marked 🔁 are due again in a minute: /review"
	}
	a.send(ctx, text, nil)
}

// sides returns the card's question and expected answer.
func (c *card) sides() (question, answer string) {
	if c.reverse {
		return c.word.Translation, c.word.Word
	}
	return c.word.Word, c.word.Translation
}

// renderCard draws a card. Unrevealed cards hide the answer and note under a spoiler.
func renderCard(c *card, revealed bool, footer string) string {
	q, ans := c.sides()
	body := esc(ans)
	if c.word.Note != "" {
		body += "\n<i>" + esc(c.word.Note) + "</i>"
	}
	if !revealed {
		body = "<tg-spoiler>" + body + "</tg-spoiler>"
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s %s\n<b>%s</b>\n\n%s", langTag(c.word.Language), statusIcons[c.word.Status], esc(q), body)
	for _, line := range []string{c.typed, footer} {
		if line != "" {
			b.WriteString("\n\n" + line)
		}
	}
	return b.String()
}

func rateKeyboard(wordID int64) *models.InlineKeyboardMarkup {
	id := strconv.FormatInt(wordID, 10)
	return keyboard([][]models.InlineKeyboardButton{
		{
			{Text: "🔁 Repeat", CallbackData: "rt:" + id + ":repeat"},
			{Text: "👍 Good", CallbackData: "rt:" + id + ":good"},
			{Text: "🚀 Fluent", CallbackData: "rt:" + id + ":fluent"},
		},
		{
			{Text: "⏭ Skip", CallbackData: "sk"},
			{Text: "⏹ Stop", CallbackData: "st"},
		},
	})
}
