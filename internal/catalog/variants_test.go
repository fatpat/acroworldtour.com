package catalog

import (
	"os"
	"reflect"
	"strings"
	"testing"
)

func loadFile(t *testing.T, path string) []Trick {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	tricks, err := LoadJSON(f)
	if err != nil {
		t.Fatal(err)
	}
	return tricks
}

func TestGenerateMatchesProductionCatalog(t *testing.T) {
	tricks := loadFile(t, "../../testdata/catalog/2026.json")
	if len(tricks) == 0 {
		t.Fatal("empty catalog")
	}
	for _, tr := range tricks {
		got := Generate(tr)
		if len(got) != len(tr.Tricks) {
			t.Errorf("%s: %d variants, stored %d", tr.Name, len(got), len(tr.Tricks))
			continue
		}
		for i := range got {
			if !reflect.DeepEqual(got[i], tr.Tricks[i]) {
				t.Errorf("%s variant %d:\n got  %+v\n want %+v", tr.Name, i, got[i], tr.Tricks[i])
			}
		}
	}
}

func TestGenerateMistyFlip(t *testing.T) {
	tr := Trick{
		Name: "Misty Flip", Acronym: "M", Solo: true, SoloAWT: true, Synchro: true,
		Directions:           []string{"left", "right"},
		TechnicalCoefficient: 1.65,
		Bonuses: []Bonus{
			{Name: "twisted", Bonus: 2.5},
			{Name: "twisted exit", Bonus: 3.5},
			{Name: "full twisted", Bonus: 6},
		},
		BonusConstraints: [][]string{{"twisted", "full twisted"}, {"twisted exit", "full twisted"}},
	}
	want := []string{
		"LM", "RM", "OM", "/LM", "/RM", "/OM", "LM/", "RM/", "OM/",
		"LM\\", "RM\\", "OM\\", "/LM/", "/RM/", "/OM/",
	}
	got := Generate(tr)
	var acr []string
	for _, u := range got {
		acr = append(acr, u.Acronym)
	}
	if !reflect.DeepEqual(acr, want) {
		t.Fatalf("acronyms %v, want %v", acr, want)
	}
	if got[2].Solo || !got[1].Solo || got[2].Name != "opposite Misty Flip" {
		t.Errorf("opposite variant wrong: %+v", got[2])
	}
	last := got[12]
	if last.Name != "twisted left Misty Flip twisted exit" || last.Bonus != 6.0 ||
		!reflect.DeepEqual(last.BonusTypes, []string{"twist"}) ||
		!reflect.DeepEqual(last.Uniqueness, []string{"left"}) {
		t.Errorf("/LM/ wrong: %+v", last)
	}
}

func TestLoadJSONDefaultsAndErrors(t *testing.T) {
	tr, err := LoadJSON(strings.NewReader(`[{"name":"a","acronym":"A","description":"x"}]`))
	if err != nil || !tr[0].SoloAWT {
		t.Fatalf("solo_awt should default to true: %v %+v", err, tr)
	}
	if _, err := LoadJSON(strings.NewReader(`[{"name":"a","bonuses":[{"name":"nope","bonus":1}]}]`)); err == nil {
		t.Error("expected unknown bonus error")
	}
}

func TestLegacyCatalogsLoadAndGenerate(t *testing.T) {
	for _, y := range []string{"2023", "2024", "2025"} {
		for _, tr := range loadFile(t, "../../testdata/catalog/"+y+".json") {
			if len(Generate(tr)) == 0 {
				t.Errorf("%s %s: no variants", y, tr.Name)
			}
		}
	}
}

// The 2022 files contain the direction "mirror", which the legacy API could
// never import (no acronym configured); LoadJSON must reject it.
func TestLegacy2022RejectsMirrorDirection(t *testing.T) {
	f, err := os.Open("../../testdata/catalog/2022.json")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := LoadJSON(f); err == nil || !strings.Contains(err.Error(), "mirror") {
		t.Fatalf("expected mirror direction error, got %v", err)
	}
}
