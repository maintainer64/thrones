package game

import (
	"encoding/json"
	"sort"
	"strings"
)

type localeData struct {
	Cards map[string]struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	} `json:"cards"`
	Messages map[string]string `json:"messages"`
}

func parseLocale(data []byte) localeData {
	var result localeData
	if err := json.Unmarshal(data, &result); err != nil {
		panic(err)
	}
	return result
}

var ruLocale = parseLocale(russianJSON)
var enLocale = parseLocale(englishJSON)

func NormalizeLanguage(value string) string {
	value = strings.ToLower(strings.TrimSpace(strings.Split(strings.Split(value, ",")[0], ";")[0]))
	if value == "en" || strings.HasPrefix(value, "en-") {
		return "en"
	}
	return "ru"
}
func localeForLanguage(language string) localeData {
	if NormalizeLanguage(language) == "en" {
		return enLocale
	}
	return ruLocale
}

var EnglishHouseNames = map[string]string{"stark": "Stark", "greyjoy": "Greyjoy", "lannister": "Lannister", "baratheon": "Baratheon", "tyrell": "Tyrell", "martell": "Martell", "arryn": "Arryn", "targaryen": "Targaryen"}

func HouseNamesForLanguage(language string) map[string]string {
	if NormalizeLanguage(language) == "en" {
		return EnglishHouseNames
	}
	return HouseNames
}
func OrdersForLanguage(language string) []Order {
	orders := append([]Order{}, Orders...)
	if NormalizeLanguage(language) != "en" {
		return orders
	}
	names := map[string]string{"march": "March", "defend": "Defense", "support": "Support", "raid": "Raid", "power": "Consolidate Power"}
	for i, order := range orders {
		name := names[order.Kind]
		switch order.ID {
		case "march-minus":
			name += " −1"
		case "march-star":
			name += " +1"
		case "defend-1", "defend-2":
			name += " +1"
		case "defend-star":
			name += " +2"
		case "support-star":
			name += " +1"
		}
		if order.Star {
			name += " ★"
		}
		orders[i].Name = name
	}
	return orders
}

// Translate only engine-authored messages. Names/titles supplied by humans are
// not passed through this function. Saved snapshots remain canonical Russian;
// localized HTTP/WS projections are independent for each participant.
var englishMessageReplacer = makeEnglishMessageReplacer()

func makeEnglishMessageReplacer() *strings.Replacer {
	entries := map[string]string{}
	for key, text := range enLocale.Messages {
		source := ruLocale.Messages[key]
		if source == "" {
			panic("missing Russian system message: " + key)
		}
		entries[source] = text
	}
	for id, text := range ruLocale.Cards {
		if translated, ok := enLocale.Cards[id]; ok {
			entries[text.Name] = translated.Name
		}
	}
	for id, name := range HouseNames {
		entries[name] = EnglishHouseNames[id]
	}
	keys := make([]string, 0, len(entries))
	for source := range entries {
		keys = append(keys, source)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) == len(keys[j]) {
			return keys[i] < keys[j]
		}
		return len(keys[i]) > len(keys[j])
	})
	pairs := []string{}
	for _, key := range keys {
		pairs = append(pairs, key, entries[key])
	}
	return strings.NewReplacer(pairs...)
}
func TranslateSystemText(text, language string) string {
	if NormalizeLanguage(language) == "en" {
		return englishMessageReplacer.Replace(text)
	}
	return text
}
func LocalizeView(view View, language string) View {
	if NormalizeLanguage(language) != "en" {
		return view
	}
	view.Seats = append([]SeatView{}, view.Seats...)
	for i, seat := range view.Seats {
		if seat.Bot {
			view.Seats[i].Name = "Bot · " + EnglishHouseNames[seat.House]
		}
	}
	view.Log = append([]string{}, view.Log...)
	for i, text := range view.Log {
		view.Log[i] = TranslateSystemText(text, language)
	}
	view.Events = append([]Event{}, view.Events...)
	for i, event := range view.Events {
		view.Events[i].Text = TranslateSystemText(event.Text, language)
	}
	if view.Match != nil {
		match := *view.Match
		translateResult := func(source *CombatResult) *CombatResult {
			if source == nil {
				return nil
			}
			combat := *source
			combat.Effects = append([]string{}, combat.Effects...)
			for i, text := range combat.Effects {
				combat.Effects[i] = TranslateSystemText(text, language)
			}
			return &combat
		}
		match.LastCombat = translateResult(match.LastCombat)
		if len(match.Combats) > 0 {
			match.Combats = make([]*CombatRecord, len(view.Match.Combats))
			for i, source := range view.Match.Combats {
				record := *source
				record.Result = translateResult(record.Result)
				match.Combats[i] = &record
			}
		}
		view.Match = &match
	}
	return view
}
