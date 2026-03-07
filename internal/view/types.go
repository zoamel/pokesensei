package view

import "zoamel/pokesensei/db/generated"

// StarterInfo holds display data for a starter Pokémon option.
type StarterInfo struct {
	PokemonID int
	Name      string
	TypeName  string
	SpriteURL string
}

// TypeInfo holds display data for a Pokémon type.
type TypeInfo struct {
	ID   int32
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
