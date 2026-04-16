package suggest

// Engine provides team suggestions based on type coverage analysis.
// It is stateless — all data is passed in per call.
type Engine struct{}

func New() *Engine {
	return &Engine{}
}

// Pokemon represents a candidate for the team.
type Pokemon struct {
	ID             int64
	Name           string
	SpriteURL      string
	Types          []int64 // type IDs
	TradeRequired  bool
	BadgeRequired  int64
	AvailableMoves []CandidateMove // learnable moves (populated only for picked Pokémon)
}

// CandidateMove represents a move in a Pokémon's learnset.
type CandidateMove struct {
	ID          int64
	Name        string
	Slug        string
	TypeID      int64
	TypeName    string
	TypeSlug    string
	Power       int64 // 0 for status moves
	DamageClass string
}

// SuggestedMove represents a move chosen for a suggested team slot.
type SuggestedMove struct {
	ID          int64
	Name        string
	Slug        string
	TypeID      int64
	TypeName    string
	TypeSlug    string
	Power       int64
	DamageClass string
}

// TeamSlot represents a current or suggested team member.
type TeamSlot struct {
	Pokemon          *Pokemon
	IsLocked         bool
	IsUtilityCarrier bool            // true for the designated field/HM move carrier
	Slot             int             // 1-6
	Moves            []SuggestedMove // up to 4 suggested moves
}

// SuggestionInput holds all data needed to generate suggestions.
type SuggestionInput struct {
	Starter           *Pokemon
	CurrentTeam       []TeamSlot // existing team (locked slots respected)
	Candidates        []Pokemon  // all available Pokémon
	BadgeCount        int64
	TradingEnabled    bool
	Efficacy          map[int64]map[int64]int64 // [attacker_type][defender_type] = factor
	UtilityPokemonIDs map[int64]bool            // Pokémon IDs that can learn a field/HM move
}

// fieldMoveSlugs is the canonical set of HM/field moves that define utility coverage.
var fieldMoveSlugs = map[string]bool{
	"cut":        true,
	"surf":       true,
	"fly":        true,
	"strength":   true,
	"rock-smash": true,
	"waterfall":  true,
	"flash":      true,
	"whirlpool":  true,
	"dive":       true,
}

// SuggestionResult holds a suggested team.
type SuggestionResult struct {
	Slots []TeamSlot
}

// SuggestCurrent returns the best team using only Pokémon available at the current badge level.
func (e *Engine) SuggestCurrent(input SuggestionInput) SuggestionResult {
	candidates := filterByBadge(input.Candidates, input.BadgeCount, input.TradingEnabled)
	return e.fillTeam(input, candidates)
}

// SuggestPlanned returns the ideal endgame team without badge restrictions.
func (e *Engine) SuggestPlanned(input SuggestionInput) SuggestionResult {
	candidates := input.Candidates
	if !input.TradingEnabled {
		candidates = filterNoTrade(candidates)
	}
	return e.fillTeam(input, candidates)
}

func (e *Engine) fillTeam(input SuggestionInput, candidates []Pokemon) SuggestionResult {
	result := SuggestionResult{
		Slots: make([]TeamSlot, 6),
	}

	// slotScores tracks the coverage score contributed when each slot was filled.
	// Locked slots and the starter use a sentinel (999) so they are never swapped out.
	slotScores := [6]int{}

	// Copy locked slots
	teamTypes := make([]int64, 0)
	for _, slot := range input.CurrentTeam {
		if slot.IsLocked && slot.Pokemon != nil {
			idx := slot.Slot - 1
			if idx >= 0 && idx < 6 {
				result.Slots[idx] = slot
				teamTypes = append(teamTypes, slot.Pokemon.Types...)
				slotScores[idx] = 999
			}
		}
	}

	// Place starter in slot 1 if not locked elsewhere
	if input.Starter != nil && result.Slots[0].Pokemon == nil {
		result.Slots[0] = TeamSlot{
			Pokemon: input.Starter,
			Slot:    1,
		}
		teamTypes = append(teamTypes, input.Starter.Types...)
		slotScores[0] = 999
	}

	// Greedy fill remaining slots
	used := make(map[int64]bool)
	for _, slot := range result.Slots {
		if slot.Pokemon != nil {
			used[slot.Pokemon.ID] = true
		}
	}

	for i := range result.Slots {
		if result.Slots[i].Pokemon != nil {
			continue
		}

		best, score := findBestCandidate(candidates, teamTypes, used, input.Efficacy)
		if best != nil {
			result.Slots[i] = TeamSlot{
				Pokemon: best,
				Slot:    i + 1,
			}
			teamTypes = append(teamTypes, best.Types...)
			used[best.ID] = true
			slotScores[i] = score
		}
	}

	ensureUtilityCarrier(&result, input, candidates, slotScores)

	return result
}

// ensureUtilityCarrier guarantees at least one slot can learn a field/HM move.
// If the team already contains a utility-capable Pokémon, it is marked and we return.
// Otherwise the weakest swappable slot is replaced with the best utility candidate.
func ensureUtilityCarrier(result *SuggestionResult, input SuggestionInput, candidates []Pokemon, slotScores [6]int) {
	if len(input.UtilityPokemonIDs) == 0 {
		return
	}

	// Mark existing utility carrier if one is already on the team
	for i, slot := range result.Slots {
		if slot.Pokemon != nil && input.UtilityPokemonIDs[slot.Pokemon.ID] {
			result.Slots[i].IsUtilityCarrier = true
			return
		}
	}

	// No carrier found — compute current team types for scoring the replacement
	teamTypes := collectTeamTypes(result)
	inTeam := usedSet(result)

	best := findBestUtilityCandidate(candidates, input.UtilityPokemonIDs, inTeam, teamTypes, input.Efficacy)
	if best == nil {
		return
	}

	// Find the weakest swappable slot (lowest score, not locked, sentinel not set)
	swapIdx, swapScore := -1, 999
	for i, slot := range result.Slots {
		if slot.Pokemon == nil || slot.IsLocked || slotScores[i] >= 999 {
			continue
		}
		if slotScores[i] < swapScore {
			swapScore = slotScores[i]
			swapIdx = i
		}
	}
	if swapIdx < 0 {
		return
	}

	result.Slots[swapIdx] = TeamSlot{
		Pokemon:          best,
		IsUtilityCarrier: true,
		Slot:             swapIdx + 1,
	}
}

func findBestUtilityCandidate(candidates []Pokemon, utilityIDs, inTeam map[int64]bool, teamTypes []int64, efficacy map[int64]map[int64]int64) *Pokemon {
	var best *Pokemon
	bestScore := -1
	for i := range candidates {
		c := &candidates[i]
		if inTeam[c.ID] || !utilityIDs[c.ID] {
			continue
		}
		score := scoreCoverage(c.Types, teamTypes, efficacy)
		if score > bestScore {
			bestScore = score
			best = c
		}
	}
	return best
}

func collectTeamTypes(result *SuggestionResult) []int64 {
	var types []int64
	for _, slot := range result.Slots {
		if slot.Pokemon != nil {
			types = append(types, slot.Pokemon.Types...)
		}
	}
	return types
}

func usedSet(result *SuggestionResult) map[int64]bool {
	used := make(map[int64]bool)
	for _, slot := range result.Slots {
		if slot.Pokemon != nil {
			used[slot.Pokemon.ID] = true
		}
	}
	return used
}

// findBestCandidate scores each candidate by coverage improvement.
// Returns the best candidate and its coverage score.
func findBestCandidate(candidates []Pokemon, teamTypes []int64, used map[int64]bool, efficacy map[int64]map[int64]int64) (*Pokemon, int) {
	var best *Pokemon
	bestScore := -1

	for i := range candidates {
		c := &candidates[i]
		if used[c.ID] {
			continue
		}

		score := scoreCoverage(c.Types, teamTypes, efficacy)

		// Penalize duplicate types
		for _, ct := range c.Types {
			for _, tt := range teamTypes {
				if ct == tt {
					score -= 2
				}
			}
		}

		if score > bestScore {
			bestScore = score
			best = c
		}
	}

	return best, bestScore
}

// scoreCoverage counts how many new super-effective matchups this candidate adds.
func scoreCoverage(candidateTypes []int64, teamTypes []int64, efficacy map[int64]map[int64]int64) int {
	// Find types the team can already hit super-effectively
	covered := make(map[int64]bool)
	for _, atkType := range teamTypes {
		if defenders, ok := efficacy[atkType]; ok {
			for defType, factor := range defenders {
				if factor >= 200 {
					covered[defType] = true
				}
			}
		}
	}

	// Count new super-effective coverages from candidate
	score := 0
	for _, atkType := range candidateTypes {
		if defenders, ok := efficacy[atkType]; ok {
			for defType, factor := range defenders {
				if factor >= 200 && !covered[defType] {
					score++
				}
			}
		}
	}

	return score
}

// SelectMovesForResult populates the Moves field for each slot in the result,
// using the Pokémon's AvailableMoves and the type efficacy map. Call this after
// the handler has loaded each suggested Pokémon's learnset.
func (e *Engine) SelectMovesForResult(result *SuggestionResult, efficacy map[int64]map[int64]int64) {
	for i := range result.Slots {
		p := result.Slots[i].Pokemon
		if p == nil || len(p.AvailableMoves) == 0 {
			continue
		}
		moves := selectMoves(p, efficacy)
		if result.Slots[i].IsUtilityCarrier {
			moves = ensureFieldMove(moves, p.AvailableMoves)
		}
		result.Slots[i].Moves = moves
	}
}

// ensureFieldMove guarantees at least one move in the selected set is a field/HM
// move. If none of the selected moves are field moves but the Pokémon has one
// available, the last slot (lowest priority) is replaced with the highest-power
// field move available.
func ensureFieldMove(selected []SuggestedMove, available []CandidateMove) []SuggestedMove {
	for _, m := range selected {
		if fieldMoveSlugs[m.Slug] {
			return selected // already includes a field move
		}
	}

	var best *CandidateMove
	for i := range available {
		m := &available[i]
		if !fieldMoveSlugs[m.Slug] {
			continue
		}
		if best == nil || m.Power > best.Power {
			best = m
		}
	}
	if best == nil {
		return selected
	}

	if len(selected) < 4 {
		return append(selected, toSuggested(*best))
	}
	result := make([]SuggestedMove, len(selected))
	copy(result, selected)
	result[len(result)-1] = toSuggested(*best)
	return result
}

// selectMoves greedily picks up to 4 moves for a Pokémon, prioritising:
//  1. The highest-power STAB move (move type matches Pokémon's type)
//  2. Coverage: moves that hit types the chosen moves can't already hit super-effectively
//  3. Power as tiebreaker
//
// If fewer than 4 damaging moves are available, status moves are used as fallback.
func selectMoves(pokemon *Pokemon, efficacy map[int64]map[int64]int64) []SuggestedMove {
	if len(pokemon.AvailableMoves) == 0 {
		return nil
	}

	// Partition moves into damaging (power > 0) and status (power == 0).
	damaging := make([]CandidateMove, 0, len(pokemon.AvailableMoves))
	status := make([]CandidateMove, 0)
	for _, m := range pokemon.AvailableMoves {
		if m.Power > 0 {
			damaging = append(damaging, m)
		} else {
			status = append(status, m)
		}
	}

	selected := make([]SuggestedMove, 0, 4)
	used := make(map[int64]bool)
	coveredTypes := make(map[int64]bool) // defending types already hit super-effectively
	ownTypes := make(map[int64]bool)
	for _, t := range pokemon.Types {
		ownTypes[t] = true
	}

	// Slot 1: best STAB move (highest power among damaging moves whose type matches).
	if best := pickBestSTAB(damaging, ownTypes); best != nil {
		selected = append(selected, toSuggested(*best))
		used[best.ID] = true
		markCovered(best.TypeID, efficacy, coveredTypes)
	}

	// Remaining slots: greedy coverage + power among damaging moves.
	for len(selected) < 4 {
		best := pickBestCoverageMove(damaging, used, coveredTypes, efficacy)
		if best == nil {
			break
		}
		selected = append(selected, toSuggested(*best))
		used[best.ID] = true
		markCovered(best.TypeID, efficacy, coveredTypes)
	}

	// Fallback: fill remaining slots with status moves if we haven't reached 4.
	for _, m := range status {
		if len(selected) >= 4 {
			break
		}
		if used[m.ID] {
			continue
		}
		selected = append(selected, toSuggested(m))
		used[m.ID] = true
	}

	return selected
}

func pickBestSTAB(moves []CandidateMove, ownTypes map[int64]bool) *CandidateMove {
	var best *CandidateMove
	for i := range moves {
		m := &moves[i]
		if !ownTypes[m.TypeID] {
			continue
		}
		if best == nil || m.Power > best.Power {
			best = m
		}
	}
	return best
}

// pickBestCoverageMove picks the move that adds the most new super-effective
// coverage, breaking ties by raw power.
func pickBestCoverageMove(moves []CandidateMove, used map[int64]bool, covered map[int64]bool, efficacy map[int64]map[int64]int64) *CandidateMove {
	var best *CandidateMove
	bestScore := -1
	bestPower := int64(-1)

	for i := range moves {
		m := &moves[i]
		if used[m.ID] {
			continue
		}
		score := 0
		if defenders, ok := efficacy[m.TypeID]; ok {
			for defType, factor := range defenders {
				if factor >= 200 && !covered[defType] {
					score++
				}
			}
		}
		if score > bestScore || (score == bestScore && m.Power > bestPower) {
			bestScore = score
			bestPower = m.Power
			best = m
		}
	}
	return best
}

func markCovered(attackerType int64, efficacy map[int64]map[int64]int64, covered map[int64]bool) {
	if defenders, ok := efficacy[attackerType]; ok {
		for defType, factor := range defenders {
			if factor >= 200 {
				covered[defType] = true
			}
		}
	}
}

func toSuggested(m CandidateMove) SuggestedMove {
	return SuggestedMove{
		ID:          m.ID,
		Name:        m.Name,
		Slug:        m.Slug,
		TypeID:      m.TypeID,
		TypeName:    m.TypeName,
		TypeSlug:    m.TypeSlug,
		Power:       m.Power,
		DamageClass: m.DamageClass,
	}
}

func filterByBadge(candidates []Pokemon, badge int64, tradingEnabled bool) []Pokemon {
	var result []Pokemon
	for _, c := range candidates {
		if c.BadgeRequired > badge {
			continue
		}
		if c.TradeRequired && !tradingEnabled {
			continue
		}
		result = append(result, c)
	}
	return result
}

func filterNoTrade(candidates []Pokemon) []Pokemon {
	var result []Pokemon
	for _, c := range candidates {
		if !c.TradeRequired {
			result = append(result, c)
		}
	}
	return result
}
