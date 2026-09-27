// Package bot is the Telegram side of LLB: commands, adding words, review sessions and reminders.
package bot

import (
	"context"
	"fmt"
	"html"
	"log"
	"regexp"
	"strings"
	"sync"
	"time"

	tg "github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"

	"llb/internal/store"
)

type Config struct {
	Token    string
	OwnerID  int64    // the only Telegram user the bot answers; 0 makes it reply with the sender's id
	RemindAt []string // "HH:MM" in local time
}

type App struct {
	cfg Config
	st  *store.Store
	tg  *tg.Bot

	mu   sync.Mutex // serializes updates and reminders; guards sess
	sess *session
}

func New(cfg Config, st *store.Store) (*App, error) {
	a := &App{cfg: cfg, st: st}
	b, err := tg.New(cfg.Token, tg.WithDefaultHandler(a.handle))
	if err != nil {
		return nil, err
	}
	a.tg = b
	return a, nil
}

func (a *App) Run(ctx context.Context) {
	_, err := a.tg.SetMyCommands(ctx, &tg.SetMyCommandsParams{Commands: []models.BotCommand{
		{Command: "review", Description: "Review words"},
		{Command: "lang", Description: "Current language"},
		{Command: "dir", Description: "Card direction"},
		{Command: "list", Description: "List words [#group]"},
		{Command: "stats", Description: "Statistics"},
		{Command: "del", Description: "Delete a word"},
		{Command: "stop", Description: "Stop reviewing"},
		{Command: "help", Description: "How to use"},
	}})
	if err != nil {
		log.Printf("set commands: %v", err)
	}
	go a.remindLoop(ctx)
	a.tg.Start(ctx)
}

func (a *App) handle(ctx context.Context, _ *tg.Bot, u *models.Update) {
	var from int64
	switch {
	case u.Message != nil && u.Message.From != nil:
		from = u.Message.From.ID
	case u.CallbackQuery != nil:
		from = u.CallbackQuery.From.ID
	default:
		return
	}

	if a.cfg.OwnerID == 0 {
		if u.Message != nil {
			a.sendTo(ctx, u.Message.Chat.ID, fmt.Sprintf(
				"OWNER_ID is not set. Your id: <code>%d</code>. Add it to .env and restart the bot.", from), nil)
		}
		return
	}
	if from != a.cfg.OwnerID {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()
	if u.CallbackQuery != nil {
		a.onCallback(ctx, u.CallbackQuery)
		return
	}
	a.onMessage(ctx, u.Message)
}

func (a *App) onMessage(ctx context.Context, m *models.Message) {
	text := strings.TrimSpace(m.Text)
	if text == "" {
		return
	}
	if !strings.HasPrefix(text, "/") {
		if a.sess != nil && a.sess.card != nil {
			a.answer(ctx, text)
		} else {
			a.addWords(ctx, text)
		}
		return
	}

	cmd, arg, _ := strings.Cut(text, " ")
	cmd, _, _ = strings.Cut(strings.ToLower(cmd), "@")
	arg = strings.TrimSpace(arg)
	switch cmd {
	case "/start", "/help":
		a.send(ctx, a.helpText(ctx), nil)
	case "/review":
		a.reviewMenu(ctx)
	case "/stop":
		a.stopReview(ctx)
	case "/lang":
		a.cmdLang(ctx, arg)
	case "/dir":
		a.cmdDir(ctx, arg)
	case "/list":
		a.cmdList(ctx, arg)
	case "/stats":
		a.cmdStats(ctx)
	case "/del":
		a.cmdDel(ctx, arg)
	default:
		a.send(ctx, "Unknown command. /help", nil)
	}
}

func (a *App) onCallback(ctx context.Context, q *models.CallbackQuery) {
	_, _ = a.tg.AnswerCallbackQuery(ctx, &tg.AnswerCallbackQueryParams{CallbackQueryID: q.ID})
	msgID := 0
	if q.Message.Message != nil {
		msgID = q.Message.Message.ID
	}

	kind, arg, _ := strings.Cut(q.Data, ":")
	switch kind {
	case "rv": // rv:<groupID>:<d|a>
		a.onReviewChoice(ctx, arg)
	case "rt": // rt:<wordID>:<rating>
		a.onRate(ctx, arg)
	case "sk":
		a.onSkip(ctx)
	case "st":
		a.stopReview(ctx)
	case "lg": // lg:<lang>
		a.setLang(ctx, arg)
		a.edit(ctx, msgID, "Current language: "+langTag(arg), nil)
	case "rl": // rl:<lang>: reminder button, switch language and review what's due
		a.setLang(ctx, arg)
		a.startReview(ctx, 0, true)
	case "dir":
		a.setDir(ctx, arg)
		a.edit(ctx, msgID, "Direction: "+dirNames[arg], nil)
	}
}

// addWords saves every "word - translation" line of text into the current language.
func (a *App) addWords(ctx context.Context, text string) {
	lang := a.lang(ctx)
	var saved, bad []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		e, ok := parseEntry(line)
		if !ok {
			bad = append(bad, esc(line))
			continue
		}
		if err := a.st.AddWord(ctx, lang, e.Word, e.Translation, e.Note, e.Groups); err != nil {
			log.Printf("add word %q: %v", e.Word, err)
			bad = append(bad, esc(line)+" (database error)")
			continue
		}
		s := fmt.Sprintf("<b>%s</b> — %s", esc(e.Word), esc(e.Translation))
		for _, g := range e.Groups {
			s += " #" + esc(g)
		}
		saved = append(saved, s)
	}

	var b strings.Builder
	if len(saved) > 0 {
		fmt.Fprintf(&b, "✅ Saved to %s:\n%s", langTag(lang), strings.Join(saved, "\n"))
	}
	if len(bad) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		fmt.Fprintf(&b, "Couldn't parse:\n%s\n\nFormat: <code>word - translation | note #group</code>", strings.Join(bad, "\n"))
	}
	a.send(ctx, b.String(), nil)
}

var langRe = regexp.MustCompile(`^[\p{L}-]{1,16}$`)

func (a *App) cmdLang(ctx context.Context, arg string) {
	if arg != "" {
		if !langRe.MatchString(arg) {
			a.send(ctx, "Language code: letters only, up to 16, e.g. <code>/lang ja</code>", nil)
			return
		}
		a.setLang(ctx, arg)
		a.send(ctx, "Current language: "+langTag(strings.ToLower(arg)), nil)
		return
	}

	current := a.lang(ctx)
	langs, err := a.st.LanguageCounts(ctx)
	if err != nil {
		a.fail(ctx, "languages", err)
		return
	}
	var rows [][]models.InlineKeyboardButton
	for _, l := range langs {
		rows = append(rows, []models.InlineKeyboardButton{{
			Text:         fmt.Sprintf("%s · %d words", l.Name, l.Total),
			CallbackData: "lg:" + l.Name,
		}})
	}
	a.send(ctx, fmt.Sprintf("Current language: %s\nSwitch: <code>/lang code</code>, e.g. <code>/lang ja</code>", langTag(current)),
		keyboard(rows))
}

var dirNames = map[string]string{
	"fwd": "word → translation",
	"rev": "translation → word",
	"mix": "mixed",
}

func (a *App) cmdDir(ctx context.Context, arg string) {
	if _, ok := dirNames[arg]; ok {
		a.setDir(ctx, arg)
		a.send(ctx, "Direction: "+dirNames[arg], nil)
		return
	}
	var row []models.InlineKeyboardButton
	for _, d := range []string{"fwd", "rev", "mix"} {
		row = append(row, models.InlineKeyboardButton{Text: dirNames[d], CallbackData: "dir:" + d})
	}
	a.send(ctx, "Current direction: "+dirNames[a.dir(ctx)], keyboard([][]models.InlineKeyboardButton{row}))
}

const listLimit = 100

func (a *App) cmdList(ctx context.Context, arg string) {
	lang := a.lang(ctx)
	var groupID int64
	if name := strings.TrimPrefix(arg, "#"); name != "" {
		id, err := a.st.GroupID(ctx, name)
		if err != nil {
			a.send(ctx, "No such group: "+esc(name), nil)
			return
		}
		groupID = id
	}
	words, err := a.st.ListWords(ctx, lang, groupID, listLimit)
	if err != nil {
		a.fail(ctx, "list", err)
		return
	}
	if len(words) == 0 {
		a.send(ctx, "Nothing here yet. Add a word: <code>perro - dog</code>", nil)
		return
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s, latest %d:\n", langTag(lang), len(words))
	for _, w := range words {
		line := fmt.Sprintf("%s <b>%s</b> — %s\n", statusIcons[w.Status], esc(w.Word), esc(w.Translation))
		if b.Len()+len(line) > 4000 {
			break
		}
		b.WriteString(line)
	}
	a.send(ctx, b.String(), nil)
}

func (a *App) cmdStats(ctx context.Context) {
	lang := a.lang(ctx)
	counts, err := a.st.StatusCounts(ctx, lang)
	if err != nil {
		a.fail(ctx, "stats", err)
		return
	}
	due := 0
	if langs, err := a.st.LanguageCounts(ctx); err == nil {
		for _, l := range langs {
			if l.Name == lang {
				due = l.Due
			}
		}
	}
	total := 0
	var b strings.Builder
	for _, s := range []string{"new", "repeat", "good", "fluent"} {
		fmt.Fprintf(&b, "%s %s: %d\n", statusIcons[s], s, counts[s])
		total += counts[s]
	}
	a.send(ctx, fmt.Sprintf("%s: %d words, %d due for review\n\n%s", langTag(lang), total, due, b.String()), nil)
}

func (a *App) cmdDel(ctx context.Context, arg string) {
	if arg == "" {
		a.send(ctx, "Usage: <code>/del word</code>", nil)
		return
	}
	ok, err := a.st.DeleteWord(ctx, a.lang(ctx), arg)
	switch {
	case err != nil:
		a.fail(ctx, "delete", err)
	case !ok:
		a.send(ctx, "Not found: “"+esc(arg)+"” in "+langTag(a.lang(ctx)), nil)
	default:
		a.send(ctx, "🗑 Deleted: "+esc(arg), nil)
	}
}

func (a *App) helpText(ctx context.Context) string {
	return `<b>LLB: Learning Language Bot</b>

To add a word, just send it:
<code>perro - dog</code>
<code>el equilibrador de carga - load balancer | spreads traffic #architecture</code>
You can send several lines at once. A note or example goes after <code>|</code>, groups after <code>#</code>.

/review: review words whose timer is up, all words, or one group
/lang: language (now ` + langTag(a.lang(ctx)) + `)
/dir: direction (word → translation, reverse, mixed)
/list [#group]: list words
/stats: statistics
/del word: delete a word
/stop: stop reviewing

The answer on a card is hidden under a spoiler. Type your answer and the bot checks it, then rate yourself:
🔁 repeat: again in 1 min
👍 good: again in 1 day
🚀 fluent: again in 3 days, then 6, 12 and so on`
}

// Settings.

func (a *App) lang(ctx context.Context) string {
	v, err := a.st.Setting(ctx, "lang", "en")
	if err != nil {
		log.Print(err)
		return "en"
	}
	return v
}

func (a *App) setLang(ctx context.Context, lang string) {
	if err := a.st.SetSetting(ctx, "lang", strings.ToLower(lang)); err != nil {
		log.Printf("set lang: %v", err)
	}
}

func (a *App) dir(ctx context.Context) string {
	v, err := a.st.Setting(ctx, "dir", "fwd")
	if err != nil {
		log.Print(err)
		return "fwd"
	}
	return v
}

func (a *App) setDir(ctx context.Context, dir string) {
	if _, ok := dirNames[dir]; !ok {
		return
	}
	if err := a.st.SetSetting(ctx, "dir", dir); err != nil {
		log.Printf("set dir: %v", err)
	}
}

// Telegram helpers. All messages go to the owner's private chat.

func (a *App) send(ctx context.Context, text string, kb *models.InlineKeyboardMarkup) int {
	return a.sendTo(ctx, a.cfg.OwnerID, text, kb)
}

func (a *App) sendTo(ctx context.Context, chatID int64, text string, kb *models.InlineKeyboardMarkup) int {
	p := &tg.SendMessageParams{ChatID: chatID, Text: text, ParseMode: models.ParseModeHTML}
	if kb != nil {
		p.ReplyMarkup = kb
	}
	m, err := a.tg.SendMessage(ctx, p)
	if err != nil {
		log.Printf("send message: %v", err)
		return 0
	}
	return m.ID
}

// edit replaces a message's text; a nil kb removes its buttons.
func (a *App) edit(ctx context.Context, msgID int, text string, kb *models.InlineKeyboardMarkup) {
	if msgID == 0 {
		a.send(ctx, text, kb)
		return
	}
	p := &tg.EditMessageTextParams{ChatID: a.cfg.OwnerID, MessageID: msgID, Text: text, ParseMode: models.ParseModeHTML}
	if kb != nil {
		p.ReplyMarkup = kb
	}
	if _, err := a.tg.EditMessageText(ctx, p); err != nil {
		log.Printf("edit message: %v", err)
	}
}

func (a *App) fail(ctx context.Context, what string, err error) {
	log.Printf("%s: %v", what, err)
	a.send(ctx, "⚠️ Something went wrong, see the logs", nil)
}

func keyboard(rows [][]models.InlineKeyboardButton) *models.InlineKeyboardMarkup {
	if len(rows) == 0 {
		return nil
	}
	return &models.InlineKeyboardMarkup{InlineKeyboard: rows}
}

var statusIcons = map[string]string{
	"new":    "🆕",
	"repeat": "🔁",
	"good":   "👍",
	"fluent": "🚀",
}

func esc(s string) string { return html.EscapeString(s) }

func langTag(lang string) string { return "<code>[" + esc(lang) + "]</code>" }

// humanDuration formats d roughly: "1 min", "5 h", "12 days".
func humanDuration(d time.Duration) string {
	switch {
	case d < time.Hour:
		return fmt.Sprintf("%d min", max(1, int(d.Round(time.Minute).Minutes())))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d h", int(d.Round(time.Hour).Hours()))
	default:
		return fmt.Sprintf("%d days", int(d.Round(24*time.Hour).Hours()/24))
	}
}
