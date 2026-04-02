package main

import (
	"encoding/json"
	"fmt"
	"os"
)

// VersionGroupInfo maps game group shorthand to PokéAPI IDs.
type VersionGroupInfo struct {
	VersionGroupID int
	VersionIDs     []int
	VersionSlugs   []string
	Name           string
}

// VersionGroups maps CLI flag values to PokéAPI version group info.
var VersionGroups = map[string]VersionGroupInfo{
	"frlg": {
		VersionGroupID: 7,
		VersionIDs:     []int{10, 11},
		VersionSlugs:   []string{"firered", "leafgreen"},
		Name:           "FireRed/LeafGreen",
	},
	"hgss": {
		VersionGroupID: 10,
		VersionIDs:     []int{15, 16},
		VersionSlugs:   []string{"heartgold", "soulsilver"},
		Name:           "HeartGold/SoulSilver",
	},
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
