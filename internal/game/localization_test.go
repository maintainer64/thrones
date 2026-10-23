package game

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"unicode"
)

func hasCyrillic(text string) bool {
	return strings.ContainsFunc(text, func(r rune) bool { return unicode.Is(unicode.Cyrillic, r) })
}

func TestLocalizedCatalogKeepsRulesAndIDs(t *testing.T) {
	ru, en := LoadCatalog(), LoadCatalogForLanguage("en-US")
	count := 0
	for _, houseID := range BaseHouses {
		house := ru.Houses[houseID]
		for id, card := range house.Cards {
			translated, ok := en.Houses[houseID].Cards[id]
			if !ok || translated.Name == "" || translated.Description == "" || hasCyrillic(translated.Name+translated.Description) {
				t.Fatalf("missing English card %s/%s: %+v", houseID, id, translated)
			}
			if translated.Name == card.Name || translated.Description == card.Description {
				t.Fatalf("untranslated card %s", id)
			}
			translated.Name, translated.Description = card.Name, card.Description
			if !reflect.DeepEqual(card, translated) {
				t.Fatalf("translation changed rules: %s", id)
			}
			count++
		}
	}
	if count != 42 || len(ruLocale.Cards) != count || len(enLocale.Cards) != count {
		t.Fatalf("incomplete card locales: %d cards", count)
	}
	if !reflect.DeepEqual(ru.Boards, en.Boards) || !reflect.DeepEqual(ru.Units, en.Units) {
		t.Fatal("translation changed the board or units")
	}
	for i, order := range OrdersForLanguage("en") {
		if order.Name == Orders[i].Name || hasCyrillic(order.Name) {
			t.Fatalf("untranslated order: %+v", order)
		}
		order.Name = Orders[i].Name
		if order != Orders[i] {
			t.Fatal("translation changed an order's rules")
		}
	}
}

func TestLocalizedViewPreservesSnapshotAndHumanText(t *testing.T) {
	r := NewRoom("test", "Мой Старк", "host", "Русе Болтон")
	r.Match = &Match{LastCombat: &CombatResult{Effects: []string{"Кейтилин Старк: бонус приказа обороны удвоен"}}}
	r.Match.Combats = []*CombatRecord{{ID: 1, Result: r.Match.LastCombat}}
	r.event("victory", "stark", "winterfell", "white-harbor", "Бой: 5 : 3. Победа дома Старк")
	view := r.View("host", LoadCatalog())
	before, _ := json.Marshal(view)
	en := LocalizeView(view, "en")
	after, _ := json.Marshal(view)
	if string(before) != string(after) {
		t.Fatal("localization mutated shared state")
	}
	if en.Name != "Мой Старк" || en.Seats[0].Name != "Русе Болтон" || en.ID != view.ID || en.Revision != view.Revision {
		t.Fatal("localization changed human text or state")
	}
	if !strings.HasPrefix(en.Seats[1].Name, "Bot · ") || en.Events[0].Text != "Combat: 5 : 3. House victory: Stark" || !strings.HasPrefix(en.Log[0], "Room created.") {
		t.Fatalf("missing localized messages: %+v", en)
	}
	if hasCyrillic(en.Match.LastCombat.Effects[0]) {
		t.Fatalf("untranslated combat effect: %s", en.Match.LastCombat.Effects[0])
	}
	if hasCyrillic(en.Match.Combats[0].Result.Effects[0]) {
		t.Fatal("history effects were not localized")
	}
}

func TestAllEngineErrorLiteralsHaveEnglishTranslations(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "New" {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if !ok || pkg.Name != "errors" {
				return true
			}
			literal, ok := call.Args[0].(*ast.BasicLit)
			if !ok {
				return true
			}
			text, err := strconv.Unquote(literal.Value)
			if err != nil {
				t.Fatal(err)
			}
			if hasCyrillic(TranslateSystemText(text, "en")) {
				t.Errorf("%s: missing English error %q", path, text)
			}
			return true
		})
	}
}

func TestNormalizeLanguage(t *testing.T) {
	for input, want := range map[string]string{"en": "en", "en-US,en;q=0.8": "en", " EN-gb ": "en", "ru,en;q=0.5": "ru", "fr": "ru", "": "ru"} {
		if got := NormalizeLanguage(input); got != want {
			t.Errorf("%q: %q, want %q", input, got, want)
		}
	}
}

func TestServerMessageKeysMatchAcrossLocales(t *testing.T) {
	if len(ruLocale.Messages) != len(enLocale.Messages) {
		t.Fatal("locale keys differ")
	}
	for key, text := range ruLocale.Messages {
		if !strings.Contains(key, ".") || hasCyrillic(key) {
			t.Errorf("non-semantic message key: %s", key)
		}
		if text == "" || enLocale.Messages[key] == "" || hasCyrillic(enLocale.Messages[key]) {
			t.Errorf("missing message translation: %s", key)
		}
	}
}
