package view

import (
	"strings"

	"zoamel/pokesensei/db/generated"
)

// StarterInfo holds display data for a starter Pokémon option.
type StarterInfo struct {
	PokemonID   int
	Name        string
	TypeName    string
	SpriteURL   string
	Description string
}

// TypeInfo holds display data for a Pokémon type.
type TypeInfo struct {
	ID   int64
	Name string
	Slug string
}

// PokemonListItem combines a Pokemon with its type info for list display.
type PokemonListItem struct {
	Pokemon generated.Pokemon
	Types   []TypeInfo
}

// PokemonDetail holds all data for the Pokémon detail page.
type PokemonDetail struct {
	Pokemon    generated.Pokemon
	Types      []TypeInfo
	Encounters []generated.ListEncountersByPokemonRow
	EvoChain   []generated.GetEvolutionChainByPokemonRow
	Abilities  []generated.ListPokemonAbilitiesRow
	Moves      []generated.ListPokemonMovesRow
}

// TeamSlotData holds data for a single team slot display.
type TeamSlotData struct {
	Member *generated.ListTeamMembersRow
	Types  []TypeInfo
}

// CoverageCell represents one cell in the type coverage matrix.
type CoverageCell struct {
	TypeName     string
	TypeSlug     string
	Factor       string // "2x", "0.5x", "0x", "1x"
	FactorClass  string // "super-effective", "not-effective", "immune", "neutral"
}

var encounterMethods = map[string]string{
	"walk":           "Walking",
	"surf":           "Surfing",
	"fish-old-rod":   "Old Rod",
	"old-rod":        "Old Rod",
	"fish-good-rod":  "Good Rod",
	"good-rod":       "Good Rod",
	"fish-super-rod": "Super Rod",
	"super-rod":      "Super Rod",
	"gift":           "Gift",
	"headbutt":       "Headbutt",
	"rock-smash":     "Rock Smash",
}

func formatEncounterMethod(s string) string {
	if formatted, ok := encounterMethods[s]; ok {
		return formatted
	}
	return titleCase(s)
}

var learnMethods = map[string]string{
	"level-up": "Level Up",
	"machine":  "TM/HM",
	"tutor":    "Tutor",
	"egg":      "Egg",
}

func formatLearnMethod(s string) string {
	if formatted, ok := learnMethods[s]; ok {
		return formatted
	}
	return titleCase(s)
}

var learnMethodTips = map[string]string{
	"level-up": "Learned automatically when the Pokémon reaches this level",
	"machine":  "Taught using a Technical Machine (TM) or Hidden Machine (HM) item",
	"tutor":    "Taught by a special move tutor NPC in the game",
	"egg":      "Inherited from a parent Pokémon when hatched from an egg",
}

func learnMethodTip(s string) string {
	if tip, ok := learnMethodTips[s]; ok {
		return tip
	}
	return ""
}

func titleCase(s string) string {
	parts := strings.Split(s, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}
