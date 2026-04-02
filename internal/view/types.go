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
	Pokemon        generated.Pokemon
	Types          []TypeInfo
	Abilities      []generated.ListPokemonAbilitiesRow
	EvolutionChain []generated.GetEvolutionChainByPokemonRow
	Moves          []generated.ListPokemonMovesRow
	Encounters     []generated.ListEncountersByPokemonRow
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


// TeamMemberDetailData holds all data for the team member detail page.
type TeamMemberDetailData struct {
	Member          generated.GetTeamMemberDetailRow
	Types           []TypeInfo
	Natures         []generated.Nature
	Abilities       []generated.ListPokemonAbilitiesRow
	Moves           []generated.ListTeamMemberMovesRow
	Available       []generated.ListAvailableMovesRow
	AssignedMoveIDs map[int64]bool
}

// MoveSlotData holds data for rendering a single move slot.
type MoveSlotData struct {
	SlotNum int
	Move    *generated.ListTeamMemberMovesRow
}

func filterMovesByMethod(moves []generated.ListPokemonMovesRow, method string) []generated.ListPokemonMovesRow {
	var result []generated.ListPokemonMovesRow
	for _, m := range moves {
		if m.LearnMethod == method {
			result = append(result, m)
		}
	}
	return result
}

func moveMethodLabel(method string) string {
	switch method {
	case "level-up":
		return "Level Up"
	case "machine":
		return "TM/HM"
	case "tutor":
		return "Move Tutor"
	case "egg":
		return "Egg Moves"
	default:
		return titleCase(method)
	}
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
