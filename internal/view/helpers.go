package view

import "strings"

// bulbapediaURL builds a Bulbapedia wiki URL for the given name and category.
// category is typically "Pokémon" or "Ability".
// Spaces are replaced with underscores; all other characters preserved as-is.
func bulbapediaURL(name, category string) string {
	slug := strings.ReplaceAll(name, " ", "_")
	return "https://bulbapedia.bulbagarden.net/wiki/" + slug + "_(" + category + ")"
}
