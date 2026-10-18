package catalog

// BonusDef describes a configured bonus (spec 02 §2.4.4). Prefix bonuses put
// their acronym before the trick acronym, the others after it.
type BonusDef struct {
	Name    string
	Acronym string
	Prefix  bool
	Type    string
}

// bonusDefs mirrors legacy api/core/config.py. When a name appears twice, the
// first definition wins (the second "wing touch"/"WT" entry is unreachable and
// therefore omitted).
var bonusDefs = []BonusDef{
	{"reverse", "R", false, "reverse"},
	{"twisted", "/", true, "twist"},
	{"twisted exit", "/", false, "twist"},
	{"full twisted", "\\", false, "twist"},
	{"devil twist", "X", false, "twist"},
	{"devil twist stall", "XX", false, "twist"},
	{"to twisted sat", "S", false, "twist"},
	{"cab slide", "C", false, "twist"},
	{"hardcore", "HC", false, "twist"},
	{"hardcore entry", "HC", false, "twist"},
	{"flip", "F", false, "flip"},
	{"double flip", "FF", false, "flip"},
	{"wing touch", "T", false, "other"},
}

// directionAcronyms maps a direction name to its acronym.
var directionAcronyms = map[string]string{
	"right":    "R",
	"left":     "L",
	"opposite": "O",
}

// LookupBonus returns the configured definition of the named bonus.
func LookupBonus(name string) (BonusDef, bool) {
	for _, d := range bonusDefs {
		if d.Name == name {
			return d, true
		}
	}
	return BonusDef{}, false
}

// BonusNames returns the configured bonus names in definition order.
func BonusNames() []string {
	out := make([]string, len(bonusDefs))
	for i, d := range bonusDefs {
		out[i] = d.Name
	}
	return out
}
