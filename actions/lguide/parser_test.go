// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package lguide

import (
	"os"
	"path"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TODO to be removed
func TestNull(t *testing.T) {
	assert.Equal(t, 1, 1)
}

func TestParserNounResponse(t *testing.T) {
	_, filepath, _, _ := runtime.Caller(0)
	srcPath := path.Join(filepath, "..", "..", "..", "testdata/lguide/noun_response.html")
	content, err := os.ReadFile(srcPath)
	if err != nil {
		t.Error(err)
	}
	ans := Parse(string(content))
	assert.Contains(t, ans, "hlavička")
	assert.Equal(t, ans["hlavička"], "okolnost")

	// položky
	assert.Contains(t, ans, "dělení")
	assert.Equal(t, ans["dělení"], "okol-nost")
	assert.Contains(t, ans, "rod")
	assert.Equal(t, ans["rod"], "ž.")

	// tabulka
	assert.Contains(t, ans, "1. pád:jednotné číslo")
	assert.Contains(t, ans, "1. pád:množné číslo")
	assert.Equal(t, ans["1. pád:jednotné číslo"], "okolnost")
	assert.Equal(t, ans["1. pád:množné číslo"], "okolnosti")
	assert.Contains(t, ans, "2. pád:jednotné číslo")
	assert.Contains(t, ans, "2. pád:množné číslo")
	assert.Equal(t, ans["2. pád:jednotné číslo"], "okolnosti")
	assert.Equal(t, ans["2. pád:množné číslo"], "okolností")
	assert.Contains(t, ans, "3. pád:jednotné číslo")
	assert.Contains(t, ans, "3. pád:množné číslo")
	assert.Equal(t, ans["3. pád:jednotné číslo"], "okolnosti")
	assert.Equal(t, ans["3. pád:množné číslo"], "okolnostem")
	assert.Contains(t, ans, "4. pád:jednotné číslo")
	assert.Contains(t, ans, "4. pád:množné číslo")
	assert.Equal(t, ans["4. pád:jednotné číslo"], "okolnost")
	assert.Equal(t, ans["4. pád:množné číslo"], "okolnosti")
	assert.Contains(t, ans, "5. pád:jednotné číslo")
	assert.Contains(t, ans, "5. pád:množné číslo")
	assert.Equal(t, ans["5. pád:jednotné číslo"], "okolnosti")
	assert.Equal(t, ans["5. pád:množné číslo"], "okolnosti")
	assert.Contains(t, ans, "6. pád:jednotné číslo")
	assert.Contains(t, ans, "6. pád:množné číslo")
	assert.Equal(t, ans["6. pád:jednotné číslo"], "okolnosti")
	assert.Equal(t, ans["6. pád:množné číslo"], "okolnostech")
	assert.Contains(t, ans, "7. pád:jednotné číslo")
	assert.Contains(t, ans, "7. pád:množné číslo")
	assert.Equal(t, ans["7. pád:jednotné číslo"], "okolností")
	assert.Equal(t, ans["7. pád:množné číslo"], "okolnostmi")
}

func TestParserAdjectiveResponse(t *testing.T) {
	_, filepath, _, _ := runtime.Caller(0)
	srcPath := path.Join(filepath, "..", "..", "..", "testdata/lguide/adjective_response.html")
	content, err := os.ReadFile(srcPath)
	if err != nil {
		t.Error(err)
	}
	ans := Parse(string(content))
	assert.Contains(t, ans, "hlavička")
	assert.Equal(t, ans["hlavička"], "modrý")

	// položky
	assert.Contains(t, ans, "dělení")
	assert.Equal(t, ans["dělení"], "mo-d-rý")
	assert.Contains(t, ans, "2. stupeň")
	assert.Equal(t, ans["2. stupeň"], "modřejší")
	assert.Contains(t, ans, "3. stupeň")
	assert.Equal(t, ans["3. stupeň"], "nejmodřejší")
	assert.Contains(t, ans, "příklady")
	assert.Equal(t, ans["příklady"], "modré oči; tmavě modré šaty; tmavomodré šaty")
}

func TestParserPronounResponse(t *testing.T) {
	_, filepath, _, _ := runtime.Caller(0)
	srcPath := path.Join(filepath, "..", "..", "..", "testdata/lguide/pronoun_response.html")
	content, err := os.ReadFile(srcPath)
	if err != nil {
		t.Error(err)
	}
	ans := Parse(string(content))
	assert.Contains(t, ans, "hlavička")
	assert.Equal(t, ans["hlavička"], "se")

	// položky
	assert.Contains(t, ans, "dělení")
	assert.Equal(t, ans["dělení"], "se")
	assert.Contains(t, ans, "jiné je")
	assert.Equal(t, ans["jiné je"], "se, předl.")
	assert.Contains(t, ans, "příklady")
	assert.Equal(t, ans["příklady"], "vzít s sebou; to se rozumí samo sebou; otevření (se) světu; vařící (se) voda; rozhodl se zúčastnit se; rozhodl se zúčastnit; rozhodl zúčastnit se")

	// tabulka
	assert.Contains(t, ans, "2. pád:jednotné číslo")
	assert.Equal(t, ans["2. pád:jednotné číslo"], "sebe")
	assert.Contains(t, ans, "3. pád:jednotné číslo")
	assert.Equal(t, ans["3. pád:jednotné číslo"], "sobě, si")
	assert.Contains(t, ans, "4. pád:jednotné číslo")
	assert.Equal(t, ans["4. pád:jednotné číslo"], "sebe, se")
	assert.Contains(t, ans, "6. pád:jednotné číslo")
	assert.Equal(t, ans["6. pád:jednotné číslo"], "sobě")
	assert.Contains(t, ans, "7. pád:jednotné číslo")
	assert.Equal(t, ans["7. pád:jednotné číslo"], "sebou")

	assert.NotContains(t, ans, "1. pád:jednotné číslo")
	assert.NotContains(t, ans, "1. pád:množné číslo")
	assert.NotContains(t, ans, "2. pád:množné číslo")
	assert.NotContains(t, ans, "3. pád:množné číslo")
	assert.NotContains(t, ans, "4. pád:množné číslo")
	assert.NotContains(t, ans, "5. pád:jednotné číslo")
	assert.NotContains(t, ans, "5. pád:množné číslo")
	assert.NotContains(t, ans, "6. pád:množné číslo")
	assert.NotContains(t, ans, "7. pád:množné číslo")
}

func TestParserNumeralResponse(t *testing.T) {
	_, filepath, _, _ := runtime.Caller(0)
	srcPath := path.Join(filepath, "..", "..", "..", "testdata/lguide/numeral_response.html")
	content, err := os.ReadFile(srcPath)
	if err != nil {
		t.Error(err)
	}
	ans := Parse(string(content))
	assert.Contains(t, ans, "hlavička")
	assert.Equal(t, ans["hlavička"], "sto")

	// položky
	assert.Contains(t, ans, "dělení")
	assert.Equal(t, ans["dělení"], "sto")
	assert.Contains(t, ans, "rod")
	assert.Equal(t, ans["rod"], "s.")
	assert.Contains(t, ans, "příklady")
	assert.Equal(t, ans["příklady"], "pět set; sto padesát tři; tři sta třicet tři tisíc; asi sto lidí se sešlo před magistrátem, aby protestovalo/protestovali proti plánované stavbě")
	assert.Contains(t, ans, "poznámky k heslu")
	assert.Equal(t, ans["poznámky k heslu"], "ve spojení s výrazem dvě má tvar 1. p. mn. č. podobu stě (dvě stě); ve spojení s počítaným předmětem může v j. č. zůstat výraz sto nesklonný (ke stu korun/korunám i ke sto korunám)")

	// tabulka
	assert.Contains(t, ans, "1. pád:jednotné číslo")
	assert.Contains(t, ans, "1. pád:množné číslo")
	assert.Equal(t, ans["1. pád:jednotné číslo"], "sto")
	assert.Equal(t, ans["1. pád:množné číslo"], "sta")
	assert.Contains(t, ans, "2. pád:jednotné číslo")
	assert.Contains(t, ans, "2. pád:množné číslo")
	assert.Equal(t, ans["2. pád:jednotné číslo"], "sta")
	assert.Equal(t, ans["2. pád:množné číslo"], "set")
	assert.Contains(t, ans, "3. pád:jednotné číslo")
	assert.Contains(t, ans, "3. pád:množné číslo")
	assert.Equal(t, ans["3. pád:jednotné číslo"], "stu")
	assert.Equal(t, ans["3. pád:množné číslo"], "stům")
	assert.Contains(t, ans, "4. pád:jednotné číslo")
	assert.Contains(t, ans, "4. pád:množné číslo")
	assert.Equal(t, ans["4. pád:jednotné číslo"], "sto")
	assert.Equal(t, ans["4. pád:množné číslo"], "sta")
	assert.Contains(t, ans, "5. pád:jednotné číslo")
	assert.Contains(t, ans, "5. pád:množné číslo")
	assert.Equal(t, ans["5. pád:jednotné číslo"], "sto")
	assert.Equal(t, ans["5. pád:množné číslo"], "sta")
	assert.Contains(t, ans, "6. pád:jednotné číslo")
	assert.Contains(t, ans, "6. pád:množné číslo")
	assert.Equal(t, ans["6. pád:jednotné číslo"], "stu")
	assert.Equal(t, ans["6. pád:množné číslo"], "stech")
	assert.Contains(t, ans, "7. pád:jednotné číslo")
	assert.Contains(t, ans, "7. pád:množné číslo")
	assert.Equal(t, ans["7. pád:jednotné číslo"], "stem")
	assert.Equal(t, ans["7. pád:množné číslo"], "sty")
}

func TestParserVerbResponse(t *testing.T) {
	_, filepath, _, _ := runtime.Caller(0)
	srcPath := path.Join(filepath, "..", "..", "..", "testdata/lguide/verb_response.html")
	content, err := os.ReadFile(srcPath)
	if err != nil {
		t.Error(err)
	}
	ans := Parse(string(content))
	assert.Contains(t, ans, "hlavička")
	assert.Equal(t, ans["hlavička"], "dělat")

	// položky
	assert.Contains(t, ans, "dělení")
	assert.Equal(t, ans["dělení"], "dě-lat")

	// tabulka
	assert.Contains(t, ans, "1. osoba:jednotné číslo")
	assert.Contains(t, ans, "1. osoba:množné číslo")
	assert.Equal(t, ans["1. osoba:jednotné číslo"], "dělám")
	assert.Equal(t, ans["1. osoba:množné číslo"], "děláme")
	assert.Contains(t, ans, "2. osoba:jednotné číslo")
	assert.Contains(t, ans, "2. osoba:množné číslo")
	assert.Equal(t, ans["2. osoba:jednotné číslo"], "děláš")
	assert.Equal(t, ans["2. osoba:množné číslo"], "děláte")
	assert.Contains(t, ans, "3. osoba:jednotné číslo")
	assert.Contains(t, ans, "3. osoba:množné číslo")
	assert.Equal(t, ans["3. osoba:jednotné číslo"], "dělá")
	assert.Equal(t, ans["3. osoba:množné číslo"], "dělají")
	assert.Contains(t, ans, "rozkazovací způsob:jednotné číslo")
	assert.Contains(t, ans, "rozkazovací způsob:množné číslo")
	assert.Equal(t, ans["rozkazovací způsob:jednotné číslo"], "dělej")
	assert.Equal(t, ans["rozkazovací způsob:množné číslo"], "dělejte")
	assert.Contains(t, ans, "příčestí činné")
	assert.Equal(t, ans["příčestí činné"], "dělal")
	assert.Contains(t, ans, "příčestí trpné")
	assert.Equal(t, ans["příčestí trpné"], "dělán")
	assert.Contains(t, ans, "přechodník přítomný, m.:jednotné číslo")
	assert.Contains(t, ans, "přechodník přítomný, m.:množné číslo")
	assert.Equal(t, ans["přechodník přítomný, m.:jednotné číslo"], "dělaje")
	assert.Equal(t, ans["přechodník přítomný, m.:množné číslo"], "dělajíce")
	assert.Contains(t, ans, "přechodník přítomný, ž. + s.:jednotné číslo")
	assert.Contains(t, ans, "přechodník přítomný, ž. + s.:množné číslo")
	assert.Equal(t, ans["přechodník přítomný, ž. + s.:jednotné číslo"], "dělajíc")
	assert.Equal(t, ans["přechodník přítomný, ž. + s.:množné číslo"], "dělajíce")
	assert.Contains(t, ans, "verbální substantivum")
	assert.Equal(t, ans["verbální substantivum"], "dělání")
}

func TestParserAdverbResponse(t *testing.T) {
	_, filepath, _, _ := runtime.Caller(0)
	srcPath := path.Join(filepath, "..", "..", "..", "testdata/lguide/adverb_response.html")
	content, err := os.ReadFile(srcPath)
	if err != nil {
		t.Error(err)
	}
	ans := Parse(string(content))
	assert.Contains(t, ans, "hlavička")
	assert.Equal(t, ans["hlavička"], "nahoře")

	// položky
	assert.Contains(t, ans, "dělení")
	assert.Equal(t, ans["dělení"], "na-ho-ře")
	assert.Contains(t, ans, "příklady")
	assert.Equal(t, ans["příklady"], "Spisy jsou uloženy nahoře na polici.(na rozdíl od: Stanul na hoře Říp/Řípu.)")
}

func TestParserPrepositionResponse(t *testing.T) {
	_, filepath, _, _ := runtime.Caller(0)
	srcPath := path.Join(filepath, "..", "..", "..", "testdata/lguide/preposition_response.html")
	content, err := os.ReadFile(srcPath)
	if err != nil {
		t.Error(err)
	}
	ans := Parse(string(content))
	assert.Contains(t, ans, "hlavička")
	assert.Equal(t, ans["hlavička"], "vedle")

	// položky
	assert.Contains(t, ans, "dělení")
	assert.Equal(t, ans["dělení"], "ve-dle")
	assert.Contains(t, ans, "příklady")
	assert.Equal(t, ans["příklady"], "stáli vedle sebe; dům vedle se bude opravovat")
}

func TestParserConjunctionResponse(t *testing.T) {
	_, filepath, _, _ := runtime.Caller(0)
	srcPath := path.Join(filepath, "..", "..", "..", "testdata/lguide/conjunction_response.html")
	content, err := os.ReadFile(srcPath)
	if err != nil {
		t.Error(err)
	}
	ans := Parse(string(content))
	assert.Contains(t, ans, "hlavička")
	assert.Equal(t, ans["hlavička"], "nebo")

	// položky
	assert.Contains(t, ans, "dělení")
	assert.Equal(t, ans["dělení"], "ne-bo")
	assert.Contains(t, ans, "příklady")
	assert.Equal(t, ans["příklady"], "Podejte nám zprávu písemně nebo telefonicky. Pospěšte si, nebo nám ujede vlak.")
}

func TestParserParticleResponse(t *testing.T) {
	_, filepath, _, _ := runtime.Caller(0)
	srcPath := path.Join(filepath, "..", "..", "..", "testdata/lguide/particle_response.html")
	content, err := os.ReadFile(srcPath)
	if err != nil {
		t.Error(err)
	}
	ans := Parse(string(content))
	assert.Contains(t, ans, "hlavička")
	assert.Equal(t, ans["hlavička"], "ať")

	// položky
	assert.Contains(t, ans, "dělení")
	assert.Equal(t, ans["dělení"], "ať")
	assert.Contains(t, ans, "příklady")
	assert.Equal(t, ans["příklady"], `Ať 
už však byl jeho úmysl jakýkoli, působil dojmem člověka, který to projel
 na celé čáře. Poradil nám, ať zajdeme za ředitelem. Ať se jde se svým 
návrhem vycpat! Ať to byl, kdo chtěl, jasně vám dokazuji, že to měl 
udělat nějak jinak. Musí ho poslouchat všichni, ať jsou to kněží, nebo 
obchodníci.`)
}

func TestParserInterjectionResponse(t *testing.T) {
	_, filepath, _, _ := runtime.Caller(0)
	srcPath := path.Join(filepath, "..", "..", "..", "testdata/lguide/interjection_response.html")
	content, err := os.ReadFile(srcPath)
	if err != nil {
		t.Error(err)
	}
	ans := Parse(string(content))
	assert.Contains(t, ans, "hlavička")
	assert.Equal(t, ans["hlavička"], "haló")

	// položky
	assert.Contains(t, ans, "dělení")
	assert.Equal(t, ans["dělení"], "ha-ló")
	assert.Contains(t, ans, "jiné je")
	assert.Equal(t, ans["jiné je"], "haló, s.")
	assert.Contains(t, ans, "příklady")
	assert.Equal(t, ans["příklady"], "Haló, tady Jiřina!; Halo, právě volá vaše láska!")
}
