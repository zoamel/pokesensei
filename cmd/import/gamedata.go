package main

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

// BadgeRouteAccess maps badge number to maximum accessible route/location area IDs
// per game group. This is hardcoded because PokéAPI doesn't provide badge gating info.
//
// badge_required = 0 means available from the start.
// These are PokéAPI location-area IDs.
//
// For FRLG:
//
//	Badge 0: Route 1, Pallet Town, Viridian City (up to Viridian Forest)
//	Badge 1 (Boulder): Route 2-4, Mt. Moon, Cerulean City
//	Badge 2 (Cascade): Route 5-10, Vermilion City, SS Anne
//	Badge 3 (Thunder): Route 11-12, Rock Tunnel, Lavender Town
//	Badge 4 (Rainbow): Route 13-15, Celadon City, Saffron City
//	Badge 5 (Soul): Route 16-18, Fuchsia City
//	Badge 6 (Marsh): Route 19-20, Seafoam Islands
//	Badge 7 (Volcano): Route 21, Cinnabar Island
//	Badge 8 (Earth): Route 22-23, Victory Road, Indigo Plateau
//
// For HGSS:
//
//	Badge 0: New Bark Town, Route 29-30, Cherrygrove City
//	Badge 1 (Zephyr): Sprout Tower, Route 31-32, Violet City
//	Badge 2 (Hive): Ilex Forest, Route 33-34, Azalea Town
//	Badge 3 (Plain): Route 35-36, National Park, Goldenrod City
//	Badge 4 (Fog): Route 37-39, Ecruteak City, Olivine City
//	Badge 5 (Storm): Route 40-41, Cianwood City, Whirl Islands
//	Badge 6 (Mineral): Route 42-44, Mahogany Town, Lake of Rage
//	Badge 7 (Glacier): Route 45-46, Ice Path, Blackthorn City
//	Badge 8 (Rising): Route 26-27, Victory Road, Indigo Plateau

// BadgeForRoute provides a rough badge-to-location mapping.
// Key: PokéAPI location slug, Value: badge number required.
// This is a simplified mapping — exact route gating varies by game.
var BadgeForRouteFRLG = map[string]int{
	"pallet-town":       0,
	"kanto-route-1":     0,
	"viridian-city":     0,
	"kanto-route-22":    0,
	"kanto-route-2":     0,
	"viridian-forest":   0,
	"pewter-city":       0,
	"kanto-route-3":     1,
	"mt-moon":           1,
	"kanto-route-4":     1,
	"cerulean-city":     1,
	"kanto-route-24":    1,
	"kanto-route-25":    1,
	"kanto-route-5":     2,
	"kanto-route-6":     2,
	"vermilion-city":    2,
	"kanto-route-11":    2,
	"digletts-cave":     2,
	"kanto-route-9":     2,
	"kanto-route-10":    2,
	"rock-tunnel":       2,
	"power-plant":       2,
	"kanto-route-7":     3,
	"kanto-route-8":     3,
	"lavender-town":     3,
	"pokemon-tower":     3,
	"celadon-city":      3,
	"kanto-route-16":    4,
	"kanto-route-17":    4,
	"kanto-route-18":    4,
	"fuchsia-city":      4,
	"kanto-safari-zone": 4,
	"saffron-city":      4,
	"kanto-route-12":    4,
	"kanto-route-13":    4,
	"kanto-route-14":    4,
	"kanto-route-15":    4,
	"kanto-route-19":    5,
	"kanto-route-20":    5,
	"seafoam-islands":   5,
	"kanto-route-21":    6,
	"cinnabar-island":   6,
	"pokemon-mansion":   6,
	"kanto-route-23":    7,
	"victory-road":      7,
	"cerulean-cave":     8,
}

var BadgeForRouteHGSS = map[string]int{
	"new-bark-town":     0,
	"johto-route-29":    0,
	"cherrygrove-city":  0,
	"johto-route-30":    0,
	"johto-route-31":    0,
	"violet-city":       0,
	"sprout-tower":      0,
	"johto-route-32":    1,
	"ruins-of-alph":     1,
	"union-cave":        1,
	"johto-route-33":    1,
	"azalea-town":       1,
	"slowpoke-well":     1,
	"ilex-forest":       2,
	"johto-route-34":    2,
	"goldenrod-city":    2,
	"johto-route-35":    3,
	"national-park":     3,
	"johto-route-36":    3,
	"johto-route-37":    3,
	"ecruteak-city":     3,
	"burned-tower":      3,
	"bell-tower":        3,
	"johto-route-38":    4,
	"johto-route-39":    4,
	"olivine-city":      4,
	"johto-route-40":    4,
	"johto-route-41":    4,
	"cianwood-city":     4,
	"whirl-islands":     5,
	"johto-route-42":    5,
	"mt-mortar":         5,
	"mahogany-town":     5,
	"lake-of-rage":      5,
	"johto-route-43":    5,
	"johto-route-44":    6,
	"ice-path":          6,
	"blackthorn-city":   6,
	"dragons-den":       7,
	"johto-route-45":    7,
	"johto-route-46":    7,
	"johto-route-26":    8,
	"johto-route-27":    8,
	"tohjo-falls":       8,
	"johto-victory-road": 8,
	"mt-silver":         8,
}
