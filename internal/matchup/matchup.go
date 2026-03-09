package matchup

// Engine calculates battle matchups between team members and opponents.
type Engine struct{}

func New() *Engine {
	return &Engine{}
}

// Pokemon represents a Pokémon in a matchup.
type Pokemon struct {
	ID        int64
	Name      string
	SpriteURL string
	Types     []int64
	Level     int64
}

// Move represents a move a Pokémon can use.
type Move struct {
	ID          int64
	Name        string
	TypeID      int64
	Power       int64  // 0 for status moves
	DamageClass string // physical, special, status
}

// MatchupResult ranks one team member against an opponent.
type MatchupResult struct {
	TeamMember     Pokemon
	BestMove       *Move
	Effectiveness  float64 // combined score
	EffectivenessLabel string  // "Super Effective", "Neutral", etc.
}

// RankTeam scores each team member against a single opponent Pokémon.
func (e *Engine) RankTeam(team []Pokemon, teamMoves map[int64][]Move, opponent Pokemon, efficacy map[int64]map[int64]int64) []MatchupResult {
	var results []MatchupResult

	for _, member := range team {
		moves := teamMoves[member.ID]
		result := MatchupResult{
			TeamMember: member,
		}

		bestScore := 0.0
		for i := range moves {
			m := &moves[i]
			if m.Power == 0 {
				continue // Skip status moves
			}

			score := calcMoveScore(m, member.Types, opponent.Types, efficacy)
			if score > bestScore {
				bestScore = score
				result.BestMove = m
				result.Effectiveness = score
			}
		}

		result.EffectivenessLabel = labelForScore(bestScore)
		results = append(results, result)
	}

	// Sort by effectiveness descending
	sortResults(results)
	return results
}

func calcMoveScore(move *Move, attackerTypes []int64, defenderTypes []int64, efficacy map[int64]map[int64]int64) float64 {
	// Type effectiveness (multiply across defender types)
	effectiveness := 1.0
	for _, defType := range defenderTypes {
		if factor, ok := efficacy[move.TypeID][defType]; ok {
			effectiveness *= float64(factor) / 100.0
		}
	}

	// STAB bonus
	stab := 1.0
	for _, atkType := range attackerTypes {
		if atkType == move.TypeID {
			stab = 1.5
			break
		}
	}

	return effectiveness * stab * float64(move.Power)
}

func labelForScore(score float64) string {
	if score == 0 {
		return "No effective moves"
	}
	// Base power 80 * 2.0 effectiveness * 1.5 STAB = 240 is strong
	// Base power 80 * 1.0 * 1.0 = 80 is neutral
	switch {
	case score >= 160:
		return "Super Effective"
	case score >= 80:
		return "Neutral"
	case score > 0:
		return "Not Very Effective"
	default:
		return "No effective moves"
	}
}

func sortResults(results []MatchupResult) {
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].Effectiveness > results[j-1].Effectiveness; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}
}
