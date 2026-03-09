package suggest

// Engine provides team suggestions based on type coverage analysis.
// It is stateless — all data is passed in per call.
type Engine struct{}

func New() *Engine {
	return &Engine{}
}

// Pokemon represents a candidate for the team.
type Pokemon struct {
	ID            int64
	Name          string
	SpriteURL     string
	Types         []int64 // type IDs
	TradeRequired bool
	BadgeRequired int64
}

// TeamSlot represents a current or suggested team member.
type TeamSlot struct {
	Pokemon  *Pokemon
	IsLocked bool
	Slot     int // 1-6
}

// SuggestionInput holds all data needed to generate suggestions.
type SuggestionInput struct {
	Starter        *Pokemon
	CurrentTeam    []TeamSlot // existing team (locked slots respected)
	Candidates     []Pokemon  // all available Pokémon
	BadgeCount     int64
	TradingEnabled bool
	Efficacy       map[int64]map[int64]int64 // [attacker_type][defender_type] = factor
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

	// Copy locked slots
	teamTypes := make([]int64, 0)
	for _, slot := range input.CurrentTeam {
		if slot.IsLocked && slot.Pokemon != nil {
			idx := slot.Slot - 1
			if idx >= 0 && idx < 6 {
				result.Slots[idx] = slot
				teamTypes = append(teamTypes, slot.Pokemon.Types...)
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

		best := findBestCandidate(candidates, teamTypes, used, input.Efficacy)
		if best != nil {
			result.Slots[i] = TeamSlot{
				Pokemon: best,
				Slot:    i + 1,
			}
			teamTypes = append(teamTypes, best.Types...)
			used[best.ID] = true
		}
	}

	return result
}

// findBestCandidate scores each candidate by coverage improvement.
func findBestCandidate(candidates []Pokemon, teamTypes []int64, used map[int64]bool, efficacy map[int64]map[int64]int64) *Pokemon {
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

	return best
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
