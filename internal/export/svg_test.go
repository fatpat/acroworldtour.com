package export

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestRenderOverlayEscapes(t *testing.T) {
	svg := string(RenderOverlay(Overlay{Run: 2, Name: `A<b> & "C" %{RUN}`, Country: "fr", Rank: 3, Score: "12.500"}))
	for _, want := range []string{"RUN 2", "A&lt;B&gt; &amp; &#34;C&#34; %{RUN}", "#3", "12.500", ">FR<"} {
		if !strings.Contains(svg, want) {
			t.Errorf("overlay lacks %q", want)
		}
	}
	if strings.Contains(svg, "<B>") {
		t.Error("unescaped markup")
	}
	if err := xml.Unmarshal([]byte(svg), new(struct{ XMLName xml.Name })); err != nil {
		t.Errorf("not well-formed XML: %v", err)
	}
}
