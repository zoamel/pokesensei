package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const baseURL = "https://pokeapi.co/api/v2"

// PokeAPIClient fetches data from the PokéAPI with rate limiting and caching.
type PokeAPIClient struct {
	client  *http.Client
	limiter <-chan time.Time
	cache   sync.Map
	log     *slog.Logger
}

func NewPokeAPIClient(log *slog.Logger) *PokeAPIClient {
	return &PokeAPIClient{
		client:  &http.Client{Timeout: 30 * time.Second},
		limiter: time.Tick(100 * time.Millisecond), // 10 req/s
		log:     log,
	}
}

func (c *PokeAPIClient) fetch(ctx context.Context, url string, target any) error {
	// Check cache
	if cached, ok := c.cache.Load(url); ok {
		data := cached.([]byte)
		return json.Unmarshal(data, target)
	}

	<-c.limiter // Rate limit

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetching %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading body: %w", err)
	}

	c.cache.Store(url, data)
	return json.Unmarshal(data, target)
}

func (c *PokeAPIClient) GetPokemon(ctx context.Context, id int) (*APIPokemon, error) {
	var p APIPokemon
	if err := c.fetch(ctx, fmt.Sprintf("%s/pokemon/%d", baseURL, id), &p); err != nil {
		return nil, err
	}
	return &p, nil
}

func (c *PokeAPIClient) GetSpecies(ctx context.Context, id int) (*APISpecies, error) {
	var s APISpecies
	if err := c.fetch(ctx, fmt.Sprintf("%s/pokemon-species/%d", baseURL, id), &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func (c *PokeAPIClient) GetType(ctx context.Context, id int) (*APIType, error) {
	var t APIType
	if err := c.fetch(ctx, fmt.Sprintf("%s/type/%d", baseURL, id), &t); err != nil {
		return nil, err
	}
	return &t, nil
}

func (c *PokeAPIClient) GetNature(ctx context.Context, id int) (*APINature, error) {
	var n APINature
	if err := c.fetch(ctx, fmt.Sprintf("%s/nature/%d", baseURL, id), &n); err != nil {
		return nil, err
	}
	return &n, nil
}

func (c *PokeAPIClient) GetMove(ctx context.Context, id int) (*APIMove, error) {
	var m APIMove
	if err := c.fetch(ctx, fmt.Sprintf("%s/move/%d", baseURL, id), &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func (c *PokeAPIClient) GetAbility(ctx context.Context, id int) (*APIAbility, error) {
	var a APIAbility
	if err := c.fetch(ctx, fmt.Sprintf("%s/ability/%d", baseURL, id), &a); err != nil {
		return nil, err
	}
	return &a, nil
}

func (c *PokeAPIClient) GetEvolutionChain(ctx context.Context, id int) (*APIEvolutionChain, error) {
	var ec APIEvolutionChain
	if err := c.fetch(ctx, fmt.Sprintf("%s/evolution-chain/%d", baseURL, id), &ec); err != nil {
		return nil, err
	}
	return &ec, nil
}

func (c *PokeAPIClient) GetPokemonEncounters(ctx context.Context, id int) ([]APIEncounterLocation, error) {
	var enc []APIEncounterLocation
	if err := c.fetch(ctx, fmt.Sprintf("%s/pokemon/%d/encounters", baseURL, id), &enc); err != nil {
		return nil, err
	}
	return enc, nil
}

func (c *PokeAPIClient) GetLocationArea(ctx context.Context, id int) (*APILocationArea, error) {
	var la APILocationArea
	if err := c.fetch(ctx, fmt.Sprintf("%s/location-area/%d", baseURL, id), &la); err != nil {
		return nil, err
	}
	return &la, nil
}

// --- PokéAPI response types ---

type NamedResource struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type APIPokemon struct {
	ID      int    `json:"id"`
	Name    string `json:"name"`
	Sprites struct {
		FrontDefault string `json:"front_default"`
	} `json:"sprites"`
	Types []struct {
		Slot int           `json:"slot"`
		Type NamedResource `json:"type"`
	} `json:"types"`
	Stats []struct {
		BaseStat int           `json:"base_stat"`
		Stat     NamedResource `json:"stat"`
	} `json:"stats"`
	Abilities []struct {
		Ability  NamedResource `json:"ability"`
		IsHidden bool          `json:"is_hidden"`
		Slot     int           `json:"slot"`
	} `json:"abilities"`
	Moves []struct {
		Move                NamedResource `json:"move"`
		VersionGroupDetails []struct {
			LevelLearnedAt  int           `json:"level_learned_at"`
			MoveLearnMethod NamedResource `json:"move_learn_method"`
			VersionGroup    NamedResource `json:"version_group"`
		} `json:"version_group_details"`
	} `json:"moves"`
	Species NamedResource `json:"species"`
}

type APISpecies struct {
	ID             int            `json:"id"`
	Name           string         `json:"name"`
	Generation     NamedResource  `json:"generation"`
	EvolutionChain APIResource    `json:"evolution_chain"`
	Varieties      []APIVariety   `json:"varieties"`
}

type APIResource struct {
	URL string `json:"url"`
}

type APIVariety struct {
	IsDefault bool          `json:"is_default"`
	Pokemon   NamedResource `json:"pokemon"`
}

type APIType struct {
	ID              int    `json:"id"`
	Name            string `json:"name"`
	DamageRelations struct {
		DoubleDamageTo   []NamedResource `json:"double_damage_to"`
		DoubleDamageFrom []NamedResource `json:"double_damage_from"`
		HalfDamageTo     []NamedResource `json:"half_damage_to"`
		HalfDamageFrom   []NamedResource `json:"half_damage_from"`
		NoDamageTo       []NamedResource `json:"no_damage_to"`
		NoDamageFrom     []NamedResource `json:"no_damage_from"`
	} `json:"damage_relations"`
}

type APINature struct {
	ID            int            `json:"id"`
	Name          string         `json:"name"`
	IncreasedStat *NamedResource `json:"increased_stat"`
	DecreasedStat *NamedResource `json:"decreased_stat"`
}

type APIMove struct {
	ID          int           `json:"id"`
	Name        string        `json:"name"`
	Accuracy    *int          `json:"accuracy"`
	Power       *int          `json:"power"`
	PP          int           `json:"pp"`
	Type        NamedResource `json:"type"`
	DamageClass NamedResource `json:"damage_class"`
	EffectEntries []struct {
		Effect   string        `json:"effect"`
		Short    string        `json:"short_effect"`
		Language NamedResource `json:"language"`
	} `json:"effect_entries"`
}

type APIAbility struct {
	ID            int    `json:"id"`
	Name          string `json:"name"`
	EffectEntries []struct {
		Effect   string        `json:"effect"`
		Short    string        `json:"short_effect"`
		Language NamedResource `json:"language"`
	} `json:"effect_entries"`
}

type APIEvolutionChain struct {
	ID    int          `json:"id"`
	Chain APIChainLink `json:"chain"`
}

type APIChainLink struct {
	Species          NamedResource       `json:"species"`
	EvolutionDetails []APIEvolutionDetail `json:"evolution_details"`
	EvolvesTo        []APIChainLink      `json:"evolves_to"`
}

type APIEvolutionDetail struct {
	Trigger  NamedResource  `json:"trigger"`
	MinLevel *int           `json:"min_level"`
	Item     *NamedResource `json:"item"`
}

type APIEncounterLocation struct {
	LocationArea   NamedResource `json:"location_area"`
	VersionDetails []struct {
		Version          NamedResource `json:"version"`
		MaxChance        int           `json:"max_chance"`
		EncounterDetails []struct {
			Method   NamedResource `json:"method"`
			Chance   int           `json:"chance"`
			MinLevel int           `json:"min_level"`
			MaxLevel int           `json:"max_level"`
		} `json:"encounter_details"`
	} `json:"version_details"`
}

type APILocationArea struct {
	ID       int           `json:"id"`
	Name     string        `json:"name"`
	Location NamedResource `json:"location"`
}
