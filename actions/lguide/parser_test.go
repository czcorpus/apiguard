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
	assert.Equal(t, ans.heading, "okolnost")

	// položky
	assert.Contains(t, ans.items, "dělení")
	assert.Equal(t, ans.items["dělení"], "okol-nost")
	assert.Contains(t, ans.items, "rod")
	assert.Equal(t, ans.items["rod"], "ž.")

	// tabulka
	assert.Equal(t, ans.grammarCase.nominative.singular, "okolnost")
	assert.Equal(t, ans.grammarCase.nominative.plural, "okolnosti")
	assert.Equal(t, ans.grammarCase.genitive.singular, "okolnosti")
	assert.Equal(t, ans.grammarCase.genitive.plural, "okolností")
	assert.Equal(t, ans.grammarCase.dative.singular, "okolnosti")
	assert.Equal(t, ans.grammarCase.dative.plural, "okolnostem")
	assert.Equal(t, ans.grammarCase.accusative.singular, "okolnost")
	assert.Equal(t, ans.grammarCase.accusative.plural, "okolnosti")
	assert.Equal(t, ans.grammarCase.vocative.singular, "okolnosti")
	assert.Equal(t, ans.grammarCase.vocative.plural, "okolnosti")
	assert.Equal(t, ans.grammarCase.locative.singular, "okolnosti")
	assert.Equal(t, ans.grammarCase.locative.plural, "okolnostech")
	assert.Equal(t, ans.grammarCase.instrumental.singular, "okolností")
	assert.Equal(t, ans.grammarCase.instrumental.plural, "okolnostmi")
}

func TestParserAdjectiveResponse(t *testing.T) {
	_, filepath, _, _ := runtime.Caller(0)
	srcPath := path.Join(filepath, "..", "..", "..", "testdata/lguide/adjective_response.html")
	content, err := os.ReadFile(srcPath)
	if err != nil {
		t.Error(err)
	}
	ans := Parse(string(content))
	assert.Equal(t, ans.heading, "modrý")

	// položky
	assert.Contains(t, ans.items, "dělení")
	assert.Equal(t, ans.items["dělení"], "mo-d-rý")
	assert.Contains(t, ans.items, "2. stupeň")
	assert.Equal(t, ans.items["2. stupeň"], "modřejší")
	assert.Contains(t, ans.items, "3. stupeň")
	assert.Equal(t, ans.items["3. stupeň"], "nejmodřejší")
	assert.Contains(t, ans.items, "příklady")
	assert.Equal(t, ans.items["příklady"], "modré oči; tmavě modré šaty; tmavomodré šaty")
}

func TestParserPronounResponse(t *testing.T) {
	_, filepath, _, _ := runtime.Caller(0)
	srcPath := path.Join(filepath, "..", "..", "..", "testdata/lguide/pronoun_response.html")
	content, err := os.ReadFile(srcPath)
	if err != nil {
		t.Error(err)
	}
	ans := Parse(string(content))
	assert.Equal(t, ans.heading, "se")

	// položky
	assert.Contains(t, ans.items, "dělení")
	assert.Equal(t, ans.items["dělení"], "se")
	assert.Contains(t, ans.items, "jiné je")
	assert.Equal(t, ans.items["jiné je"], "se, předl.")
	assert.Contains(t, ans.items, "příklady")
	assert.Equal(t, ans.items["příklady"], "vzít s sebou; to se rozumí samo sebou; otevření (se) světu; vařící (se) voda; rozhodl se zúčastnit se; rozhodl se zúčastnit; rozhodl zúčastnit se")

	// tabulka
	assert.Equal(t, ans.grammarCase.nominative.singular, "")
	assert.Equal(t, ans.grammarCase.genitive.singular, "sebe")
	assert.Equal(t, ans.grammarCase.dative.singular, "sobě, si")
	assert.Equal(t, ans.grammarCase.accusative.singular, "sebe, se")
	assert.Equal(t, ans.grammarCase.vocative.singular, "")
	assert.Equal(t, ans.grammarCase.locative.singular, "sobě")
	assert.Equal(t, ans.grammarCase.instrumental.singular, "sebou")

	assert.Equal(t, ans.grammarCase.nominative.plural, "")
	assert.Equal(t, ans.grammarCase.genitive.plural, "")
	assert.Equal(t, ans.grammarCase.dative.plural, "")
	assert.Equal(t, ans.grammarCase.accusative.plural, "")
	assert.Equal(t, ans.grammarCase.vocative.plural, "")
	assert.Equal(t, ans.grammarCase.locative.plural, "")
	assert.Equal(t, ans.grammarCase.instrumental.plural, "")
}

func TestParserNumeralResponse(t *testing.T) {
	_, filepath, _, _ := runtime.Caller(0)
	srcPath := path.Join(filepath, "..", "..", "..", "testdata/lguide/numeral_response.html")
	content, err := os.ReadFile(srcPath)
	if err != nil {
		t.Error(err)
	}
	ans := Parse(string(content))
	assert.Equal(t, ans.heading, "sto")

	// položky
	assert.Contains(t, ans.items, "dělení")
	assert.Equal(t, ans.items["dělení"], "sto")
	assert.Contains(t, ans.items, "rod")
	assert.Equal(t, ans.items["rod"], "s.")
	assert.Contains(t, ans.items, "příklady")
	assert.Equal(t, ans.items["příklady"], "pět set; sto padesát tři; tři sta třicet tři tisíc; asi sto lidí se sešlo před magistrátem, aby protestovalo/protestovali proti plánované stavbě")
	assert.Contains(t, ans.items, "poznámky k heslu")
	assert.Equal(t, ans.items["poznámky k heslu"], "ve spojení s výrazem dvě má tvar 1. p. mn. č. podobu stě (dvě stě); ve spojení s počítaným předmětem může v j. č. zůstat výraz sto nesklonný (ke stu korun/korunám i ke sto korunám)")

	// tabulka
	assert.Equal(t, ans.grammarCase.nominative.singular, "sto")
	assert.Equal(t, ans.grammarCase.nominative.plural, "sta")
	assert.Equal(t, ans.grammarCase.genitive.singular, "sta")
	assert.Equal(t, ans.grammarCase.genitive.plural, "set")
	assert.Equal(t, ans.grammarCase.dative.singular, "stu")
	assert.Equal(t, ans.grammarCase.dative.plural, "stům")
	assert.Equal(t, ans.grammarCase.accusative.singular, "sto")
	assert.Equal(t, ans.grammarCase.accusative.plural, "sta")
	assert.Equal(t, ans.grammarCase.vocative.singular, "sto")
	assert.Equal(t, ans.grammarCase.vocative.plural, "sta")
	assert.Equal(t, ans.grammarCase.locative.singular, "stu")
	assert.Equal(t, ans.grammarCase.locative.plural, "stech")
	assert.Equal(t, ans.grammarCase.instrumental.singular, "stem")
	assert.Equal(t, ans.grammarCase.instrumental.plural, "sty")
}

func TestParserVerbResponse(t *testing.T) {
	_, filepath, _, _ := runtime.Caller(0)
	srcPath := path.Join(filepath, "..", "..", "..", "testdata/lguide/verb_response.html")
	content, err := os.ReadFile(srcPath)
	if err != nil {
		t.Error(err)
	}
	ans := Parse(string(content))
	assert.Equal(t, ans.heading, "dělat")

	// položky
	assert.Contains(t, ans.items, "dělení")
	assert.Equal(t, ans.items["dělení"], "dě-lat")

	// tabulka
	assert.Equal(t, ans.verbData.person.first.singular, "dělám")
	assert.Equal(t, ans.verbData.person.first.plural, "děláme")
	assert.Equal(t, ans.verbData.person.second.singular, "děláš")
	assert.Equal(t, ans.verbData.person.second.plural, "děláte")
	assert.Equal(t, ans.verbData.person.third.singular, "dělá")
	assert.Equal(t, ans.verbData.person.third.plural, "dělají")

	assert.Equal(t, ans.verbData.imperative.singular, "dělej")
	assert.Equal(t, ans.verbData.imperative.plural, "dělejte")
	assert.Equal(t, ans.verbData.participle.active, "dělal")
	assert.Equal(t, ans.verbData.participle.passive, "dělán")
	assert.Equal(t, ans.verbData.transgressive.present.m.singular, "dělaje")
	assert.Equal(t, ans.verbData.transgressive.present.m.plural, "dělajíce")
	assert.Equal(t, ans.verbData.transgressive.present.zs.singular, "dělajíc")
	assert.Equal(t, ans.verbData.transgressive.present.zs.plural, "dělajíce")
	assert.Equal(t, ans.verbData.verbalNoun, "dělání")
}

func TestParserAdverbResponse(t *testing.T) {
	_, filepath, _, _ := runtime.Caller(0)
	srcPath := path.Join(filepath, "..", "..", "..", "testdata/lguide/adverb_response.html")
	content, err := os.ReadFile(srcPath)
	if err != nil {
		t.Error(err)
	}
	ans := Parse(string(content))
	assert.Equal(t, ans.heading, "nahoře")

	// položky
	assert.Contains(t, ans.items, "dělení")
	assert.Equal(t, ans.items["dělení"], "na-ho-ře")
	assert.Contains(t, ans.items, "příklady")
	assert.Equal(t, ans.items["příklady"], "Spisy jsou uloženy nahoře na polici.(na rozdíl od: Stanul na hoře Říp/Řípu.)")
}

func TestParserPrepositionResponse(t *testing.T) {
	_, filepath, _, _ := runtime.Caller(0)
	srcPath := path.Join(filepath, "..", "..", "..", "testdata/lguide/preposition_response.html")
	content, err := os.ReadFile(srcPath)
	if err != nil {
		t.Error(err)
	}
	ans := Parse(string(content))
	assert.Equal(t, ans.heading, "vedle")

	// položky
	assert.Contains(t, ans.items, "dělení")
	assert.Equal(t, ans.items["dělení"], "ve-dle")
	assert.Contains(t, ans.items, "příklady")
	assert.Equal(t, ans.items["příklady"], "stáli vedle sebe; dům vedle se bude opravovat")
}

func TestParserConjunctionResponse(t *testing.T) {
	_, filepath, _, _ := runtime.Caller(0)
	srcPath := path.Join(filepath, "..", "..", "..", "testdata/lguide/conjunction_response.html")
	content, err := os.ReadFile(srcPath)
	if err != nil {
		t.Error(err)
	}
	ans := Parse(string(content))
	assert.Equal(t, ans.heading, "nebo")

	// položky
	assert.Contains(t, ans.items, "dělení")
	assert.Equal(t, ans.items["dělení"], "ne-bo")
	assert.Contains(t, ans.items, "příklady")
	assert.Equal(t, ans.items["příklady"], "Podejte nám zprávu písemně nebo telefonicky. Pospěšte si, nebo nám ujede vlak.")
}

func TestParserParticleResponse(t *testing.T) {
	_, filepath, _, _ := runtime.Caller(0)
	srcPath := path.Join(filepath, "..", "..", "..", "testdata/lguide/particle_response.html")
	content, err := os.ReadFile(srcPath)
	if err != nil {
		t.Error(err)
	}
	ans := Parse(string(content))
	assert.Equal(t, ans.heading, "ať")

	// položky
	assert.Contains(t, ans.items, "dělení")
	assert.Equal(t, ans.items["dělení"], "ať")
	assert.Contains(t, ans.items, "příklady")
	assert.Equal(t, ans.items["příklady"], "Ať \nuž však byl jeho úmysl jakýkoli, působil dojmem člověka, který to projel\n na celé čáře. Poradil nám, ať zajdeme za ředitelem. Ať se jde se svým \nnávrhem vycpat! Ať to byl, kdo chtěl, jasně vám dokazuji, že to měl \nudělat nějak jinak. Musí ho poslouchat všichni, ať jsou to kněží, nebo \nobchodníci.")
}

func TestParserInterjectionResponse(t *testing.T) {
	_, filepath, _, _ := runtime.Caller(0)
	srcPath := path.Join(filepath, "..", "..", "..", "testdata/lguide/interjection_response.html")
	content, err := os.ReadFile(srcPath)
	if err != nil {
		t.Error(err)
	}
	ans := Parse(string(content))
	assert.Equal(t, ans.heading, "haló")

	// položky
	assert.Contains(t, ans.items, "dělení")
	assert.Equal(t, ans.items["dělení"], "ha-ló")
	assert.Contains(t, ans.items, "jiné je")
	assert.Equal(t, ans.items["jiné je"], "haló, s.")
	assert.Contains(t, ans.items, "příklady")
	assert.Equal(t, ans.items["příklady"], "Haló, tady Jiřina!; Halo, právě volá vaše láska!")
}
