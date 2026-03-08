package main

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Importer handles fetching from PokéAPI and inserting into the database.
type Importer struct {
	pool   *pgxpool.Pool
	client *PokeAPIClient
	log    *slog.Logger

	// Caches to avoid re-fetching
	typeNameToID map[string]int
	movesSeen    map[int]bool
	abilitySeen  map[int]bool

	// Deferred pokemon_abilities inserts (populated during ImportPokemon, flushed after ImportAbilities)
	deferredAbilities []pokemonAbilityRow
}

type pokemonAbilityRow struct {
	PokemonID int
	AbilityID int
	IsHidden  bool
	Slot      int
}

func NewImporter(pool *pgxpool.Pool, client *PokeAPIClient, log *slog.Logger) *Importer {
	return &Importer{
		pool:         pool,
		client:       client,
		log:          log,
		typeNameToID: make(map[string]int),
		movesSeen:    make(map[int]bool),
		abilitySeen:  make(map[int]bool),
	}
}

func (imp *Importer) ImportTypes(ctx context.Context) error {
	// Truncate and reload
	if _, err := imp.pool.Exec(ctx, "TRUNCATE types CASCADE"); err != nil {
		return err
	}

	batch := &pgx.Batch{}
	for id := 1; id <= 18; id++ {
		t, err := imp.client.GetType(ctx, id)
		if err != nil {
			return fmt.Errorf("fetching type %d: %w", id, err)
		}
		imp.typeNameToID[t.Name] = t.ID
		batch.Queue("INSERT INTO types (id, name, slug) VALUES ($1, $2, $3)",
			t.ID, formatName(t.Name), t.Name)
		imp.log.Info("fetched type", "id", t.ID, "name", t.Name)
	}

	br := imp.pool.SendBatch(ctx, batch)
	defer br.Close()
	for i := 0; i < batch.Len(); i++ {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("inserting type: %w", err)
		}
	}
	imp.log.Info("imported types", "count", 18)
	return nil
}

func (imp *Importer) ImportTypeEfficacy(ctx context.Context) error {
	if _, err := imp.pool.Exec(ctx, "TRUNCATE type_efficacy"); err != nil {
		return err
	}

	batch := &pgx.Batch{}
	for id := 1; id <= 18; id++ {
		t, err := imp.client.GetType(ctx, id)
		if err != nil {
			return fmt.Errorf("fetching type %d for efficacy: %w", id, err)
		}

		// Build efficacy map for this attacking type
		efficacy := make(map[int]int)
		// Default all to 100 (normal)
		for defID := 1; defID <= 18; defID++ {
			efficacy[defID] = 100
		}
		for _, target := range t.DamageRelations.DoubleDamageTo {
			if tid, ok := imp.typeNameToID[target.Name]; ok {
				efficacy[tid] = 200
			}
		}
		for _, target := range t.DamageRelations.HalfDamageTo {
			if tid, ok := imp.typeNameToID[target.Name]; ok {
				efficacy[tid] = 50
			}
		}
		for _, target := range t.DamageRelations.NoDamageTo {
			if tid, ok := imp.typeNameToID[target.Name]; ok {
				efficacy[tid] = 0
			}
		}

		for defID := 1; defID <= 18; defID++ {
			batch.Queue("INSERT INTO type_efficacy (attacking_type_id, defending_type_id, damage_factor) VALUES ($1, $2, $3)",
				id, defID, efficacy[defID])
		}
	}

	br := imp.pool.SendBatch(ctx, batch)
	defer br.Close()
	for i := 0; i < batch.Len(); i++ {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("inserting type efficacy: %w", err)
		}
	}
	imp.log.Info("imported type efficacy matrix", "entries", 18*18)
	return nil
}

func (imp *Importer) ImportNatures(ctx context.Context) error {
	if _, err := imp.pool.Exec(ctx, "TRUNCATE natures"); err != nil {
		return err
	}

	batch := &pgx.Batch{}
	for id := 1; id <= 25; id++ {
		n, err := imp.client.GetNature(ctx, id)
		if err != nil {
			return fmt.Errorf("fetching nature %d: %w", id, err)
		}

		var incStat, decStat *string
		if n.IncreasedStat != nil {
			s := n.IncreasedStat.Name
			incStat = &s
		}
		if n.DecreasedStat != nil {
			s := n.DecreasedStat.Name
			decStat = &s
		}

		batch.Queue("INSERT INTO natures (id, name, slug, increased_stat, decreased_stat) VALUES ($1, $2, $3, $4, $5)",
			n.ID, formatName(n.Name), n.Name, incStat, decStat)
	}

	br := imp.pool.SendBatch(ctx, batch)
	defer br.Close()
	for i := 0; i < batch.Len(); i++ {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("inserting nature: %w", err)
		}
	}
	imp.log.Info("imported natures", "count", 25)
	return nil
}

func (imp *Importer) ImportGameVersions(ctx context.Context) error {
	if _, err := imp.pool.Exec(ctx, "TRUNCATE game_versions CASCADE"); err != nil {
		return err
	}

	versions := []struct {
		id   int
		name string
		slug string
	}{
		{10, "FireRed", "firered"},
		{11, "LeafGreen", "leafgreen"},
		{15, "HeartGold", "heartgold"},
		{16, "SoulSilver", "soulsilver"},
	}

	batch := &pgx.Batch{}
	for _, v := range versions {
		batch.Queue("INSERT INTO game_versions (id, name, slug) VALUES ($1, $2, $3)",
			v.id, v.name, v.slug)
	}

	br := imp.pool.SendBatch(ctx, batch)
	defer br.Close()
	for i := 0; i < batch.Len(); i++ {
		if _, err := br.Exec(); err != nil {
			return fmt.Errorf("inserting game version: %w", err)
		}
	}
	imp.log.Info("imported game versions", "count", len(versions))
	return nil
}

func (imp *Importer) ImportPokemon(ctx context.Context, maxDex int) error {
	if _, err := imp.pool.Exec(ctx, "TRUNCATE pokemon CASCADE"); err != nil {
		return err
	}

	for id := 1; id <= maxDex; id++ {
		p, err := imp.client.GetPokemon(ctx, id)
		if err != nil {
			imp.log.Warn("skipping pokemon", "id", id, "error", err)
			continue
		}

		species, err := imp.client.GetSpecies(ctx, id)
		if err != nil {
			imp.log.Warn("skipping pokemon species", "id", id, "error", err)
			continue
		}

		gen := parseGeneration(species.Generation.Name)
		spriteURL := ""
		if p.Sprites.FrontDefault != "" {
			spriteURL = p.Sprites.FrontDefault
		}

		// Extract base stats
		stats := extractStats(p)

		batch := &pgx.Batch{}
		batch.Queue(`INSERT INTO pokemon (id, name, slug, generation, sprite_url,
			base_hp, base_attack, base_defense, base_sp_atk, base_sp_def, base_speed)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
			p.ID, formatName(p.Name), p.Name, gen, spriteURL,
			stats["hp"], stats["attack"], stats["defense"],
			stats["special-attack"], stats["special-defense"], stats["speed"])

		// Types
		for _, t := range p.Types {
			typeID := imp.typeNameToID[t.Type.Name]
			if typeID == 0 {
				typeID = extractID(t.Type.URL)
			}
			batch.Queue("INSERT INTO pokemon_types (pokemon_id, type_id, slot) VALUES ($1, $2, $3)",
				p.ID, typeID, t.Slot)
		}

		// Collect abilities (deferred until abilities table is populated)
		for _, a := range p.Abilities {
			abilityID := extractID(a.Ability.URL)
			imp.abilitySeen[abilityID] = true
			imp.deferredAbilities = append(imp.deferredAbilities, pokemonAbilityRow{
				PokemonID: p.ID,
				AbilityID: abilityID,
				IsHidden:  a.IsHidden,
				Slot:      a.Slot,
			})
		}

		br := imp.pool.SendBatch(ctx, batch)
		for i := 0; i < batch.Len(); i++ {
			if _, err := br.Exec(); err != nil {
				br.Close()
				return fmt.Errorf("inserting pokemon %d: %w", id, err)
			}
		}
		br.Close()

		if id%50 == 0 {
			imp.log.Info("importing pokemon", "progress", fmt.Sprintf("%d/%d", id, maxDex))
		}
	}

	imp.log.Info("imported pokemon", "count", maxDex)
	return nil
}

func (imp *Importer) ImportAbilities(ctx context.Context) error {
	if _, err := imp.pool.Exec(ctx, "TRUNCATE abilities CASCADE"); err != nil {
		return err
	}

	batch := &pgx.Batch{}
	count := 0
	for abilityID := range imp.abilitySeen {
		a, err := imp.client.GetAbility(ctx, abilityID)
		if err != nil {
			imp.log.Warn("skipping ability", "id", abilityID, "error", err)
			continue
		}

		desc := ""
		for _, e := range a.EffectEntries {
			if e.Language.Name == "en" {
				desc = e.Short
				break
			}
		}

		batch.Queue("INSERT INTO abilities (id, name, slug, description) VALUES ($1, $2, $3, $4)",
			a.ID, formatName(a.Name), a.Name, desc)
		count++

		// Send batch every 100 abilities
		if count%100 == 0 {
			br := imp.pool.SendBatch(ctx, batch)
			for i := 0; i < batch.Len(); i++ {
				if _, err := br.Exec(); err != nil {
					br.Close()
					return fmt.Errorf("inserting abilities: %w", err)
				}
			}
			br.Close()
			batch = &pgx.Batch{}
			imp.log.Info("importing abilities", "progress", count)
		}
	}

	if batch.Len() > 0 {
		br := imp.pool.SendBatch(ctx, batch)
		for i := 0; i < batch.Len(); i++ {
			if _, err := br.Exec(); err != nil {
				br.Close()
				return fmt.Errorf("inserting abilities: %w", err)
			}
		}
		br.Close()
	}

	imp.log.Info("imported abilities", "count", count)
	return nil
}

func (imp *Importer) FlushPokemonAbilities(ctx context.Context) error {
	if _, err := imp.pool.Exec(ctx, "TRUNCATE pokemon_abilities CASCADE"); err != nil {
		return err
	}

	batch := &pgx.Batch{}
	for i, row := range imp.deferredAbilities {
		batch.Queue(`INSERT INTO pokemon_abilities (pokemon_id, ability_id, is_hidden, slot)
			VALUES ($1, $2, $3, $4)
			ON CONFLICT (pokemon_id, slot) DO NOTHING`,
			row.PokemonID, row.AbilityID, row.IsHidden, row.Slot)

		if (i+1)%500 == 0 {
			br := imp.pool.SendBatch(ctx, batch)
			for j := 0; j < batch.Len(); j++ {
				if _, err := br.Exec(); err != nil {
					br.Close()
					return fmt.Errorf("inserting pokemon_abilities: %w", err)
				}
			}
			br.Close()
			batch = &pgx.Batch{}
		}
	}

	if batch.Len() > 0 {
		br := imp.pool.SendBatch(ctx, batch)
		for j := 0; j < batch.Len(); j++ {
			if _, err := br.Exec(); err != nil {
				br.Close()
				return fmt.Errorf("inserting pokemon_abilities: %w", err)
			}
		}
		br.Close()
	}

	imp.log.Info("flushed pokemon_abilities", "count", len(imp.deferredAbilities))
	return nil
}

func (imp *Importer) ImportMoves(ctx context.Context) error {
	// Only import moves once (shared across games)
	if len(imp.movesSeen) > 0 {
		return nil
	}

	if _, err := imp.pool.Exec(ctx, "TRUNCATE moves CASCADE"); err != nil {
		return err
	}

	// PokéAPI has ~900 moves, but we only need those learnable by Gen I-IV Pokémon.
	// We'll collect move IDs during learnset import and backfill.
	// For now, import all moves up to ~500 (covers Gen IV).
	batch := &pgx.Batch{}
	count := 0
	for id := 1; id <= 467; id++ { // Gen IV last move is ~467
		m, err := imp.client.GetMove(ctx, id)
		if err != nil {
			imp.log.Debug("skipping move", "id", id, "error", err)
			continue
		}

		typeID := extractID(m.Type.URL)

		var power, accuracy *int
		if m.Power != nil && *m.Power > 0 {
			power = m.Power
		}
		if m.Accuracy != nil {
			accuracy = m.Accuracy
		}

		effect := ""
		for _, e := range m.EffectEntries {
			if e.Language.Name == "en" {
				effect = e.Short
				break
			}
		}

		batch.Queue(`INSERT INTO moves (id, name, slug, type_id, power, accuracy, pp, damage_class, effect)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
			m.ID, formatName(m.Name), m.Name, typeID, power, accuracy, m.PP, m.DamageClass.Name, effect)
		imp.movesSeen[m.ID] = true
		count++

		if count%100 == 0 {
			br := imp.pool.SendBatch(ctx, batch)
			for i := 0; i < batch.Len(); i++ {
				if _, err := br.Exec(); err != nil {
					br.Close()
					return fmt.Errorf("inserting moves: %w", err)
				}
			}
			br.Close()
			batch = &pgx.Batch{}
			imp.log.Info("importing moves", "progress", count)
		}
	}

	if batch.Len() > 0 {
		br := imp.pool.SendBatch(ctx, batch)
		for i := 0; i < batch.Len(); i++ {
			if _, err := br.Exec(); err != nil {
				br.Close()
				return fmt.Errorf("inserting moves: %w", err)
			}
		}
		br.Close()
	}

	imp.log.Info("imported moves", "count", count)
	return nil
}

func (imp *Importer) ImportLearnsets(ctx context.Context, maxDex int, versionGroupID int) error {
	// Delete existing learnsets for this version group
	if _, err := imp.pool.Exec(ctx, "DELETE FROM pokemon_moves WHERE version_group_id = $1", versionGroupID); err != nil {
		return err
	}

	vgIDStr := strconv.Itoa(versionGroupID)
	count := 0

	for pokemonID := 1; pokemonID <= maxDex; pokemonID++ {
		p, err := imp.client.GetPokemon(ctx, pokemonID)
		if err != nil {
			continue
		}

		batch := &pgx.Batch{}
		for _, m := range p.Moves {
			moveID := extractID(m.Move.URL)
			if !imp.movesSeen[moveID] {
				continue // Skip moves we haven't imported
			}

			for _, vgd := range m.VersionGroupDetails {
				// Match version group ID from URL
				vgURL := vgd.VersionGroup.URL
				if !strings.Contains(vgURL, "/"+vgIDStr+"/") {
					continue
				}

				batch.Queue(`INSERT INTO pokemon_moves (pokemon_id, move_id, version_group_id, learn_method, level_learned_at)
					VALUES ($1, $2, $3, $4, $5)
					ON CONFLICT DO NOTHING`,
					pokemonID, moveID, versionGroupID, vgd.MoveLearnMethod.Name, vgd.LevelLearnedAt)
				count++
			}
		}

		if batch.Len() > 0 {
			br := imp.pool.SendBatch(ctx, batch)
			for i := 0; i < batch.Len(); i++ {
				if _, err := br.Exec(); err != nil {
					br.Close()
					return fmt.Errorf("inserting learnset for pokemon %d: %w", pokemonID, err)
				}
			}
			br.Close()
		}

		if pokemonID%50 == 0 {
			imp.log.Info("importing learnsets", "version_group", versionGroupID, "progress", fmt.Sprintf("%d/%d", pokemonID, maxDex))
		}
	}

	imp.log.Info("imported learnsets", "version_group", versionGroupID, "entries", count)
	return nil
}

func (imp *Importer) ImportEncounters(ctx context.Context, vg VersionGroupInfo) error {
	// Delete existing encounters for these versions
	for _, versionID := range vg.VersionIDs {
		if _, err := imp.pool.Exec(ctx, "DELETE FROM encounters WHERE game_version_id = $1", versionID); err != nil {
			return err
		}
	}
	// Delete locations for these versions
	for _, versionID := range vg.VersionIDs {
		if _, err := imp.pool.Exec(ctx, "DELETE FROM locations WHERE game_version_id = $1", versionID); err != nil {
			return err
		}
	}

	// Build badge lookup based on game group
	badgeLookup := BadgeForRouteFRLG
	for group, info := range VersionGroups {
		if info.VersionGroupID == vg.VersionGroupID && group == "hgss" {
			badgeLookup = BadgeForRouteHGSS
		}
	}

	locationCache := make(map[string]int32) // "pokeapi_id:version_id:area" -> location row id
	count := 0

	// Fetch encounters per Pokémon (only Gen I-IV)
	for pokemonID := 1; pokemonID <= 493; pokemonID++ {
		encounters, err := imp.client.GetPokemonEncounters(ctx, pokemonID)
		if err != nil {
			continue
		}

		for _, enc := range encounters {
			locationAreaID := extractID(enc.LocationArea.URL)

			for _, vd := range enc.VersionDetails {
				versionID := extractID(vd.Version.URL)

				// Only process versions in this game group
				found := false
				for _, vid := range vg.VersionIDs {
					if vid == versionID {
						found = true
						break
					}
				}
				if !found {
					continue
				}

				// Get or create location
				cacheKey := fmt.Sprintf("%d:%d:%s", locationAreaID, versionID, enc.LocationArea.Name)
				locID, exists := locationCache[cacheKey]
				if !exists {
					// Fetch location area details for the parent location
					la, err := imp.client.GetLocationArea(ctx, locationAreaID)
					if err != nil {
						imp.log.Debug("skipping location area", "id", locationAreaID, "error", err)
						continue
					}

					locationSlug := la.Location.Name

					var newID int32
					err = imp.pool.QueryRow(ctx,
						`INSERT INTO locations (pokeapi_id, name, slug, game_version_id, area_name)
						VALUES ($1, $2, $3, $4, $5)
						ON CONFLICT (pokeapi_id, game_version_id, area_name) DO UPDATE SET name = EXCLUDED.name
						RETURNING id`,
						locationAreaID, formatName(la.Location.Name), locationSlug, versionID, formatName(la.Name)).Scan(&newID)
					if err != nil {
						return fmt.Errorf("inserting location: %w", err)
					}
					locID = newID
					locationCache[cacheKey] = locID
				}

				// Get badge for this location's parent
				la, _ := imp.client.GetLocationArea(ctx, locationAreaID)
				locationSlug := ""
				if la != nil {
					locationSlug = la.Location.Name
				}
				badge := lookupBadge(locationSlug, badgeLookup)

				for _, ed := range vd.EncounterDetails {
					_, err := imp.pool.Exec(ctx,
						`INSERT INTO encounters (pokemon_id, location_id, game_version_id, method, chance, min_level, max_level, badge_required)
						VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
						pokemonID, locID, versionID, ed.Method.Name, ed.Chance, ed.MinLevel, ed.MaxLevel, badge)
					if err != nil {
						return fmt.Errorf("inserting encounter: %w", err)
					}
					count++
				}
			}
		}

		if pokemonID%50 == 0 {
			imp.log.Info("importing encounters", "progress", fmt.Sprintf("%d/493", pokemonID))
		}
	}

	imp.log.Info("imported encounters", "game", vg.Name, "count", count)
	return nil
}

func (imp *Importer) ImportEvolutionChains(ctx context.Context, maxDex int) error {
	if _, err := imp.pool.Exec(ctx, "TRUNCATE evolution_steps CASCADE"); err != nil {
		return err
	}
	if _, err := imp.pool.Exec(ctx, "TRUNCATE evolution_chains CASCADE"); err != nil {
		return err
	}

	// Collect unique chain IDs from species data
	chainIDs := make(map[int]bool)
	for id := 1; id <= maxDex; id++ {
		species, err := imp.client.GetSpecies(ctx, id)
		if err != nil {
			continue
		}
		chainID := extractID(species.EvolutionChain.URL)
		if chainID > 0 {
			chainIDs[chainID] = true
		}
	}

	count := 0
	for chainID := range chainIDs {
		ec, err := imp.client.GetEvolutionChain(ctx, chainID)
		if err != nil {
			imp.log.Debug("skipping evolution chain", "id", chainID, "error", err)
			continue
		}

		// Insert chain
		if _, err := imp.pool.Exec(ctx, "INSERT INTO evolution_chains (id) VALUES ($1) ON CONFLICT DO NOTHING", ec.ID); err != nil {
			return fmt.Errorf("inserting evolution chain: %w", err)
		}

		// Flatten the recursive chain
		steps := flattenChain(ec.Chain, nil, 0)
		batch := &pgx.Batch{}
		for _, step := range steps {
			pokemonID := extractIDFromSpeciesURL(step.speciesURL)
			if pokemonID > maxDex {
				continue // Skip Pokémon outside our range
			}

			var evolvesFromID *int
			if step.evolvesFromURL != "" {
				efID := extractIDFromSpeciesURL(step.evolvesFromURL)
				if efID <= maxDex {
					evolvesFromID = &efID
				}
			}

			batch.Queue(`INSERT INTO evolution_steps (chain_id, pokemon_id, evolves_from_id, evolution_trigger, min_level, trigger_item, trade_required, position)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
				ec.ID, pokemonID, evolvesFromID, step.trigger, step.minLevel, step.triggerItem, step.tradeRequired, step.position)
			count++
		}

		if batch.Len() > 0 {
			br := imp.pool.SendBatch(ctx, batch)
			for i := 0; i < batch.Len(); i++ {
				if _, err := br.Exec(); err != nil {
					br.Close()
					return fmt.Errorf("inserting evolution step: %w", err)
				}
			}
			br.Close()
		}
	}

	imp.log.Info("imported evolution chains", "steps", count)
	return nil
}

// --- Helpers ---

type evolutionStep struct {
	speciesURL     string
	evolvesFromURL string
	trigger        *string
	minLevel       *int
	triggerItem    *string
	tradeRequired  bool
	position       int
}

func flattenChain(link APIChainLink, parentURL *string, position int) []evolutionStep {
	var steps []evolutionStep

	evolvesFrom := ""
	if parentURL != nil {
		evolvesFrom = *parentURL
	}

	var trigger *string
	var minLevel *int
	var triggerItem *string
	tradeRequired := false

	if len(link.EvolutionDetails) > 0 {
		ed := link.EvolutionDetails[0]
		if ed.Trigger.Name != "" {
			t := ed.Trigger.Name
			trigger = &t
		}
		if ed.MinLevel != nil {
			minLevel = ed.MinLevel
		}
		if ed.Item != nil {
			ti := ed.Item.Name
			triggerItem = &ti
		}
		if ed.Trigger.Name == "trade" {
			tradeRequired = true
		}
	}

	speciesURL := link.Species.URL
	steps = append(steps, evolutionStep{
		speciesURL:     speciesURL,
		evolvesFromURL: evolvesFrom,
		trigger:        trigger,
		minLevel:       minLevel,
		triggerItem:    triggerItem,
		tradeRequired:  tradeRequired,
		position:       position,
	})

	for _, child := range link.EvolvesTo {
		childSteps := flattenChain(child, &speciesURL, position+1)
		steps = append(steps, childSteps...)
	}

	return steps
}

var idFromURLRe = regexp.MustCompile(`/(\d+)/?$`)

func extractID(url string) int {
	matches := idFromURLRe.FindStringSubmatch(url)
	if len(matches) < 2 {
		return 0
	}
	id, _ := strconv.Atoi(matches[1])
	return id
}

func extractIDFromSpeciesURL(url string) int {
	return extractID(url)
}

func extractSlugFromURL(url string) string {
	// URL like https://pokeapi.co/api/v2/location/234/
	// We extract the ID and use it as-is (the location name comes from the API response)
	url = strings.TrimSuffix(url, "/")
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return ""
}

func extractStats(p *APIPokemon) map[string]int {
	stats := make(map[string]int)
	for _, s := range p.Stats {
		stats[s.Stat.Name] = s.BaseStat
	}
	return stats
}

func formatName(slug string) string {
	// Convert "fire-red" to "Fire Red", "thunderbolt" to "Thunderbolt"
	parts := strings.Split(slug, "-")
	for i, p := range parts {
		if len(p) > 0 {
			parts[i] = strings.ToUpper(p[:1]) + p[1:]
		}
	}
	return strings.Join(parts, " ")
}

func parseGeneration(name string) int {
	// "generation-i" -> 1, "generation-iv" -> 4
	parts := strings.Split(name, "-")
	if len(parts) < 2 {
		return 1
	}
	roman := parts[len(parts)-1]
	switch roman {
	case "i":
		return 1
	case "ii":
		return 2
	case "iii":
		return 3
	case "iv":
		return 4
	case "v":
		return 5
	case "vi":
		return 6
	case "vii":
		return 7
	case "viii":
		return 8
	case "ix":
		return 9
	default:
		return 1
	}
}

func lookupBadge(locationSlug string, badgeMap map[string]int) int {
	if badge, ok := badgeMap[locationSlug]; ok {
		return badge
	}
	return 0 // Default to available from start
}
