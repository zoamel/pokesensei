package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// VersionGroupInfo holds resolved version group data from the database and PokeAPI.
type VersionGroupInfo struct {
	VersionGroupID int
	VersionIDs     []int
	Name           string
	MaxPokedex     int
}

// LoadBadgeMap loads a badge-to-location mapping from a JSON file.
// Key: PokéAPI location slug, Value: badge number required.
// JSON files live in cmd/import/badges/<game>.json.
func LoadBadgeMap(gameGroup string) (map[string]int, error) {
	filename := fmt.Sprintf("cmd/import/badges/%s.json", gameGroup)
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("reading badge map %s: %w", filename, err)
	}
	var badges map[string]int
	if err := json.Unmarshal(data, &badges); err != nil {
		return nil, fmt.Errorf("parsing badge map %s: %w", filename, err)
	}
	return badges, nil
}
