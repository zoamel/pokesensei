package suggest

import "testing"

// Type IDs used in tests (matches Pokémon type IDs from DB).
const (
	typeNormal   int64 = 1
	typeFire     int64 = 10
	typeWater    int64 = 11
	typeElectric int64 = 13
	typeGrass    int64 = 12
	typeIce      int64 = 15
	typeFighting int64 = 2
	typeGround   int64 = 5
	typeFlying   int64 = 3
	typePsychic  int64 = 14
	typeBug      int64 = 7
	typeRock     int64 = 6
	typeGhost    int64 = 8
	typeSteel    int64 = 9
	typeDragon   int64 = 16
	typeDark     int64 = 17
	typePoison   int64 = 4
)

// buildTestEfficacy returns a minimal efficacy map for the tests below.
func buildTestEfficacy() map[int64]map[int64]int64 {
	// [attacker][defender] = factor (200 == super effective)
	return map[int64]map[int64]int64{
		typeFire:     {typeGrass: 200, typeIce: 200, typeBug: 200, typeSteel: 200},
		typeWater:    {typeFire: 200, typeGround: 200, typeRock: 200},
		typeElectric: {typeWater: 200, typeFlying: 200},
		typeGrass:    {typeWater: 200, typeGround: 200, typeRock: 200},
		typeIce:      {typeGrass: 200, typeGround: 200, typeFlying: 200, typeDragon: 200},
		typeRock:     {typeFire: 200, typeIce: 200, typeFlying: 200, typeBug: 200},
		typeGround:   {typeFire: 200, typeElectric: 200, typeRock: 200, typeSteel: 200, typePoison: 200},
		typeFighting: {typeNormal: 200, typeIce: 200, typeRock: 200, typeDark: 200, typeSteel: 200},
		typePsychic:  {typeFighting: 200, typePoison: 200},
		typeGhost:    {typeGhost: 200, typePsychic: 200},
	}
}

func TestSelectMoves_PrioritizesSTAB(t *testing.T) {
	// Charmander (Fire) should pick a Fire move first due to STAB
	pokemon := &Pokemon{
		ID:    4,
		Name:  "Charmander",
		Types: []int64{typeFire},
		AvailableMoves: []CandidateMove{
			{ID: 52, Name: "Ember", TypeID: typeFire, Power: 40, DamageClass: "special"},
			{ID: 33, Name: "Tackle", TypeID: typeNormal, Power: 40, DamageClass: "physical"},
			{ID: 43, Name: "Leer", TypeID: typeNormal, Power: 0, DamageClass: "status"},
			{ID: 10, Name: "Scratch", TypeID: typeNormal, Power: 40, DamageClass: "physical"},
		},
	}

	moves := selectMoves(pokemon, buildTestEfficacy())

	if len(moves) == 0 {
		t.Fatal("expected at least one move, got none")
	}
	if moves[0].TypeID != typeFire {
		t.Errorf("expected first move to be Fire-type (STAB), got type ID %d", moves[0].TypeID)
	}
}

func TestSelectMoves_DiversifiesCoverage(t *testing.T) {
	// Given multiple Fire moves and one of each other type, we should NOT pick 4 Fire moves.
	pokemon := &Pokemon{
		ID:    4,
		Name:  "Charmander",
		Types: []int64{typeFire},
		AvailableMoves: []CandidateMove{
			{ID: 1, Name: "Ember", TypeID: typeFire, Power: 40, DamageClass: "special"},
			{ID: 2, Name: "Flamethrower", TypeID: typeFire, Power: 90, DamageClass: "special"},
			{ID: 3, Name: "Fire Blast", TypeID: typeFire, Power: 110, DamageClass: "special"},
			{ID: 4, Name: "Heat Wave", TypeID: typeFire, Power: 95, DamageClass: "special"},
			{ID: 5, Name: "Dig", TypeID: typeGround, Power: 80, DamageClass: "physical"},
			{ID: 6, Name: "Rock Slide", TypeID: typeRock, Power: 75, DamageClass: "physical"},
			{ID: 7, Name: "Brick Break", TypeID: typeFighting, Power: 75, DamageClass: "physical"},
		},
	}

	moves := selectMoves(pokemon, buildTestEfficacy())

	if len(moves) != 4 {
		t.Fatalf("expected 4 moves, got %d", len(moves))
	}

	// Count distinct types
	types := make(map[int64]bool)
	for _, m := range moves {
		types[m.TypeID] = true
	}
	if len(types) < 3 {
		t.Errorf("expected at least 3 distinct move types, got %d (types selected: %v)", len(types), types)
	}
}

func TestSelectMoves_FallsBackToStatusWhenFewDamaging(t *testing.T) {
	// Only 2 damaging moves available — the other 2 slots should fall back to status moves.
	pokemon := &Pokemon{
		ID:    4,
		Name:  "Charmander",
		Types: []int64{typeFire},
		AvailableMoves: []CandidateMove{
			{ID: 1, Name: "Ember", TypeID: typeFire, Power: 40, DamageClass: "special"},
			{ID: 2, Name: "Scratch", TypeID: typeNormal, Power: 40, DamageClass: "physical"},
			{ID: 3, Name: "Growl", TypeID: typeNormal, Power: 0, DamageClass: "status"},
			{ID: 4, Name: "Leer", TypeID: typeNormal, Power: 0, DamageClass: "status"},
		},
	}

	moves := selectMoves(pokemon, buildTestEfficacy())

	if len(moves) != 4 {
		t.Fatalf("expected 4 moves with status fallback, got %d", len(moves))
	}
}

func TestSelectMoves_EmptyLearnset(t *testing.T) {
	pokemon := &Pokemon{
		ID:             4,
		Name:           "Charmander",
		Types:          []int64{typeFire},
		AvailableMoves: []CandidateMove{},
	}

	moves := selectMoves(pokemon, buildTestEfficacy())
	if len(moves) != 0 {
		t.Errorf("expected 0 moves for empty learnset, got %d", len(moves))
	}
}

func TestSelectMoves_NoSTABStillPicksMoves(t *testing.T) {
	// Dual-typed Pokemon with no STAB moves in learnset (unusual but possible).
	pokemon := &Pokemon{
		ID:    81,
		Name:  "Magnemite",
		Types: []int64{typeElectric, typeSteel},
		AvailableMoves: []CandidateMove{
			{ID: 1, Name: "Tackle", TypeID: typeNormal, Power: 40, DamageClass: "physical"},
			{ID: 2, Name: "Water Gun", TypeID: typeWater, Power: 40, DamageClass: "special"},
			{ID: 3, Name: "Ice Beam", TypeID: typeIce, Power: 90, DamageClass: "special"},
		},
	}

	moves := selectMoves(pokemon, buildTestEfficacy())
	if len(moves) != 3 {
		t.Fatalf("expected 3 moves, got %d", len(moves))
	}
}

func TestFillTeam_EnsuresUtilityCarrier(t *testing.T) {
	// The greedy algorithm would never pick meowth (Normal-type, zero new coverage),
	// but since it's the only utility-capable Pokémon, it should replace the
	// lowest-scored non-starter slot and be marked IsUtilityCarrier = true.
	efficacy := buildTestEfficacy()

	meowth := Pokemon{
		ID:    52,
		Name:  "Meowth",
		Types: []int64{typeNormal},
	}
	// High-coverage candidates the greedy algorithm prefers
	gyarados := Pokemon{ID: 130, Name: "Gyarados", Types: []int64{typeWater, typeFlying}}
	raichu := Pokemon{ID: 26, Name: "Raichu", Types: []int64{typeElectric}}
	venusaur := Pokemon{ID: 3, Name: "Venusaur", Types: []int64{typeGrass, typePoison}}
	onix := Pokemon{ID: 95, Name: "Onix", Types: []int64{typeRock, typeGround}}
	machamp := Pokemon{ID: 68, Name: "Machamp", Types: []int64{typeFighting}}

	engine := New()
	result := engine.SuggestCurrent(SuggestionInput{
		Candidates: []Pokemon{gyarados, raichu, venusaur, onix, machamp, meowth},
		BadgeCount: 8,
		Efficacy:   efficacy,
		UtilityPokemonIDs: map[int64]bool{
			meowth.ID: true,
		},
	})

	foundCarrier := false
	for _, slot := range result.Slots {
		if slot.Pokemon != nil && slot.Pokemon.ID == meowth.ID {
			if !slot.IsUtilityCarrier {
				t.Errorf("meowth slot should be marked IsUtilityCarrier = true")
			}
			foundCarrier = true
		}
	}
	if !foundCarrier {
		t.Error("expected meowth (utility carrier) to appear in the suggested team")
	}
}

func TestFillTeam_MarksExistingUtilityCarrier(t *testing.T) {
	// If the greedy algorithm naturally picks a utility-capable Pokémon,
	// it should be marked IsUtilityCarrier without any swap.
	efficacy := buildTestEfficacy()

	// Gyarados can learn surf — it will be picked naturally for coverage.
	gyarados := Pokemon{ID: 130, Name: "Gyarados", Types: []int64{typeWater, typeFlying}}
	raichu := Pokemon{ID: 26, Name: "Raichu", Types: []int64{typeElectric}}

	engine := New()
	result := engine.SuggestCurrent(SuggestionInput{
		Candidates: []Pokemon{gyarados, raichu},
		BadgeCount: 8,
		Efficacy:   efficacy,
		UtilityPokemonIDs: map[int64]bool{
			gyarados.ID: true,
		},
	})

	for _, slot := range result.Slots {
		if slot.Pokemon != nil && slot.Pokemon.ID == gyarados.ID {
			if !slot.IsUtilityCarrier {
				t.Errorf("gyarados should be marked IsUtilityCarrier = true")
			}
			return
		}
	}
	t.Error("gyarados not found in team")
}

func TestSelectMoves_IncludesFieldMoveForUtilityCarrier(t *testing.T) {
	// Even though Cut has low power, it should replace the 4th (lowest-priority)
	// battle move when the slot is marked IsUtilityCarrier.
	efficacy := buildTestEfficacy()

	meowth := &Pokemon{
		ID:    52,
		Name:  "Meowth",
		Types: []int64{typeNormal},
		AvailableMoves: []CandidateMove{
			{ID: 1, Name: "Slash", Slug: "slash", TypeID: typeNormal, Power: 70, DamageClass: "physical"},
			{ID: 2, Name: "Bite", Slug: "bite", TypeID: typeDark, Power: 60, DamageClass: "physical"},
			{ID: 3, Name: "Dig", Slug: "dig", TypeID: typeGround, Power: 80, DamageClass: "physical"},
			{ID: 4, Name: "Shadow Ball", Slug: "shadow-ball", TypeID: typeGhost, Power: 80, DamageClass: "special"},
			{ID: 5, Name: "Cut", Slug: "cut", TypeID: typeNormal, Power: 50, DamageClass: "physical"},
		},
	}

	engine := New()
	result := &SuggestionResult{
		Slots: []TeamSlot{
			{Pokemon: meowth, IsUtilityCarrier: true, Slot: 1},
		},
	}
	engine.SelectMovesForResult(result, efficacy)

	moves := result.Slots[0].Moves
	if len(moves) == 0 {
		t.Fatal("expected moves to be selected")
	}
	hasCut := false
	for _, m := range moves {
		if m.Slug == "cut" {
			hasCut = true
			break
		}
	}
	if !hasCut {
		t.Errorf("expected Cut to be included for utility carrier, got: %v", moves)
	}
}

func TestSelectMoves_FieldMoveAlreadySelected_NoRedundantSwap(t *testing.T) {
	// If Surf is already selected via normal coverage logic, don't add another field move.
	efficacy := buildTestEfficacy()

	tentacruel := &Pokemon{
		ID:    73,
		Name:  "Tentacruel",
		Types: []int64{typeWater, typePoison},
		AvailableMoves: []CandidateMove{
			{ID: 1, Name: "Surf", Slug: "surf", TypeID: typeWater, Power: 95, DamageClass: "special"},
			{ID: 2, Name: "Ice Beam", Slug: "ice-beam", TypeID: typeIce, Power: 90, DamageClass: "special"},
			{ID: 3, Name: "Sludge Bomb", Slug: "sludge-bomb", TypeID: typePoison, Power: 90, DamageClass: "special"},
		},
	}

	engine := New()
	result := &SuggestionResult{
		Slots: []TeamSlot{
			{Pokemon: tentacruel, IsUtilityCarrier: true, Slot: 1},
		},
	}
	engine.SelectMovesForResult(result, efficacy)

	moves := result.Slots[0].Moves
	fieldCount := 0
	for _, m := range moves {
		if fieldMoveSlugs[m.Slug] {
			fieldCount++
		}
	}
	if fieldCount > 1 {
		t.Errorf("expected at most 1 field move, got %d", fieldCount)
	}
}

func TestSelectMoves_PrefersHigherPowerOnTie(t *testing.T) {
	// Two Fire moves with identical coverage — higher power should win.
	pokemon := &Pokemon{
		ID:    4,
		Name:  "Charmander",
		Types: []int64{typeFire},
		AvailableMoves: []CandidateMove{
			{ID: 1, Name: "Ember", TypeID: typeFire, Power: 40, DamageClass: "special"},
			{ID: 2, Name: "Flamethrower", TypeID: typeFire, Power: 90, DamageClass: "special"},
		},
	}

	moves := selectMoves(pokemon, buildTestEfficacy())
	if len(moves) == 0 {
		t.Fatal("expected at least one move")
	}
	if moves[0].Name != "Flamethrower" {
		t.Errorf("expected Flamethrower (power 90) to win over Ember (power 40), got %q", moves[0].Name)
	}
}
