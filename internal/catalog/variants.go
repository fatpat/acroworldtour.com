// Package catalog holds the trick catalog and the generation of unique tricks
// (direction x bonus combinations), spec 02 §2.4.
package catalog

import (
	"encoding/json"
	"fmt"
	"io"
	"slices"
	"strings"
)

// Bonus is a bonus that can be combined on a trick (spec 02 §2.4.1).
type Bonus struct {
	Name        string  `json:"name"`
	Bonus       float64 `json:"bonus"`
	SampleVideo string  `json:"sample_video"`
}

// UniqueTrick is one direction x bonus variant of a catalog trick (spec 02
// §2.4.3, without the technical marks which belong to scoring).
type UniqueTrick struct {
	Name                 string   `json:"name"`
	Acronym              string   `json:"acronym"`
	TechnicalCoefficient float64  `json:"technical_coefficient"`
	Bonus                float64  `json:"bonus"`
	BonusTypes           []string `json:"bonus_types"`
	BaseTrick            string   `json:"base_trick"`
	Uniqueness           []string `json:"uniqueness"`
	Bonuses              []Bonus  `json:"bonuses"`
	Synchro              bool     `json:"synchro"`
	Solo                 bool     `json:"solo"`
	SoloAWT              bool     `json:"solo_awt"`
	Types                []string `json:"types"`
}

// Trick is a catalog trick (spec 02 §2.4.2). Tricks holds the stored variants
// when present in the source JSON; Generate recomputes them.
type Trick struct {
	Name                 string        `json:"name"`
	Acronym              string        `json:"acronym"`
	Types                []string      `json:"types"`
	Solo                 bool          `json:"solo"`
	SoloAWT              bool          `json:"solo_awt"`
	Synchro              bool          `json:"synchro"`
	Directions           []string      `json:"directions"`
	TechnicalCoefficient float64       `json:"technical_coefficient"`
	Bonuses              []Bonus       `json:"bonuses"`
	BonusConstraints     [][]string    `json:"bonus_constraints"`
	TricksConstraints    []string      `json:"tricks_constraints"`
	FirstManeuver        int           `json:"first_maneuver"`
	NoFirstManeuver      int           `json:"no_first_maneuver"`
	LastManeuver         int           `json:"last_maneuver"`
	NoLastManeuver       int           `json:"no_last_maneuver"`
	Tricks               []UniqueTrick `json:"tricks"`
	Repeatable           bool          `json:"repeatable"`
	SampleVideo          string        `json:"sample_video"`
}

// UnmarshalJSON applies the spec defaults (solo_awt defaults to true) and
// ignores unknown fields such as description and criterias.
func (t *Trick) UnmarshalJSON(b []byte) error {
	type plain Trick
	aux := plain{SoloAWT: true}
	if err := json.Unmarshal(b, &aux); err != nil {
		return err
	}
	*t = Trick(aux)
	return nil
}

// LoadJSON reads a JSON array of catalog tricks and rejects unknown bonus names.
func LoadJSON(r io.Reader) ([]Trick, error) {
	var tricks []Trick
	if err := json.NewDecoder(r).Decode(&tricks); err != nil {
		return nil, fmt.Errorf("decode catalog: %w", err)
	}
	for _, t := range tricks {
		for _, b := range t.Bonuses {
			if _, ok := LookupBonus(b.Name); !ok {
				return nil, fmt.Errorf("trick %q: unknown bonus %q", t.Name, b.Name)
			}
		}
		for _, d := range t.Directions {
			if _, ok := directionAcronyms[d]; !ok {
				return nil, fmt.Errorf("trick %q: unknown direction %q", t.Name, d)
			}
		}
	}
	return tricks, nil
}

// Generate expands a trick into its unique tricks (spec 02 §2.4.5): the plain
// trick first, then every allowed bonus combination in catalog order, each one
// repeated per direction.
func Generate(t Trick) []UniqueTrick {
	out := generateOne(t, nil, nil)
	n := len(t.Bonuses)
	for k := 1; k <= n; k++ {
		forEachCombination(n, k, func(idx []int) {
			combo := make([]Bonus, k)
			for i, j := range idx {
				combo[i] = t.Bonuses[j]
			}
			if violatesConstraint(combo, t.BonusConstraints) {
				return
			}
			out = generateOne(t, combo, out)
		})
	}
	return out
}

// forEachCombination calls fn with each k-combination of 0..n-1 in
// lexicographic order (same as itertools.combinations). idx is reused.
func forEachCombination(n, k int, fn func(idx []int)) {
	idx := make([]int, k)
	var rec func(start, depth int)
	rec = func(start, depth int) {
		if depth == k {
			fn(idx)
			return
		}
		for i := start; i < n; i++ {
			idx[depth] = i
			rec(i+1, depth+1)
		}
	}
	rec(0, 0)
}

func violatesConstraint(combo []Bonus, constraints [][]string) bool {
	for i := 0; i < len(combo); i++ {
		for j := i + 1; j < len(combo); j++ {
			pair := []string{combo[i].Name, combo[j].Name}
			slices.Sort(pair)
			for _, c := range constraints {
				sorted := slices.Clone(c)
				slices.Sort(sorted)
				if slices.Equal(pair, sorted) {
					return true
				}
			}
		}
	}
	return false
}

func generateOne(t Trick, combo []Bonus, out []UniqueTrick) []UniqueTrick {
	var preNames, postNames []string
	var preAcr, postAcr strings.Builder
	total := 0.0
	bonusTypes := []string{}
	for _, b := range combo {
		def, _ := LookupBonus(b.Name)
		total += b.Bonus
		if def.Prefix {
			preAcr.WriteString(def.Acronym)
			preNames = append(preNames, b.Name)
		} else {
			postAcr.WriteString(def.Acronym)
			postNames = append(postNames, b.Name)
		}
		if !slices.Contains(bonusTypes, def.Type) {
			bonusTypes = append(bonusTypes, def.Type)
		}
	}
	baseUniqueness := []string{}
	if slices.Contains(bonusTypes, "reverse") {
		baseUniqueness = append(baseUniqueness, "reverse")
	}

	build := func(direction string, solo bool) UniqueTrick {
		parts := slices.Clone(preNames)
		acronym := preAcr.String()
		uniq := slices.Clone(baseUniqueness)
		if direction != "" {
			parts = append(parts, direction)
			acronym += directionAcronyms[direction]
			uniq = append(uniq, direction)
		}
		parts = append(parts, t.Name)
		parts = append(parts, postNames...)
		acronym += t.Acronym + postAcr.String()
		bonuses := slices.Clone(combo)
		if bonuses == nil {
			bonuses = []Bonus{}
		}
		return UniqueTrick{
			Name:                 strings.Join(parts, " "),
			Acronym:              acronym,
			TechnicalCoefficient: t.TechnicalCoefficient,
			Bonus:                total,
			BonusTypes:           slices.Clone(bonusTypes),
			BaseTrick:            t.Name,
			Uniqueness:           uniq,
			Bonuses:              bonuses,
			Synchro:              t.Synchro,
			Solo:                 solo,
			SoloAWT:              t.SoloAWT,
			Types:                nonNil(t.Types),
		}
	}

	if len(t.Directions) == 0 {
		return append(out, build("", t.Solo))
	}
	for _, d := range t.Directions {
		out = append(out, build(d, t.Solo))
	}
	if t.Solo && t.Synchro {
		out = append(out, build("opposite", false))
	}
	return out
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return slices.Clone(s)
}
