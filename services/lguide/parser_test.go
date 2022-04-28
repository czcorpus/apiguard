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

func loadTestingFile(relativePath string, t *testing.T) string {
	_, filepath, _, _ := runtime.Caller(0)
	srcPath := path.Join(filepath, "..", "..", "..", relativePath)
	content, err := os.ReadFile(srcPath)
	if err != nil {
		t.Error(err)
	}
	return string(content)
}

// TODO to be removed
func TestNull(t *testing.T) {
	assert.Equal(t, 1, 1)
}

func TestParserNounResponse(t *testing.T) {
	content := loadTestingFile("testdata/lguide/noun_response.html", t)
	ans := Parse(content)
	assert.Equal(t, ans.heading, "okolnost")

	// položky
	assert.Equal(t, ans.division, "okol-nost")
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
	content := loadTestingFile("testdata/lguide/adjective_response.html", t)
	ans := Parse(content)
	assert.Equal(t, ans.heading, "modrý")

	// položky
	assert.Equal(t, ans.division, "mo-d-rý")
	assert.Equal(t, ans.comparison.comparative, "modřejší")
	assert.Equal(t, ans.comparison.superlative, "nejmodřejší")
	assert.Contains(t, ans.items, "příklady")
	assert.Equal(t, ans.items["příklady"], "modré oči; tmavě modré šaty; tmavomodré šaty")
}

func TestParserPronounResponse(t *testing.T) {
	content := loadTestingFile("testdata/lguide/pronoun_response.html", t)
	ans := Parse(content)
	assert.Equal(t, ans.heading, "se")

	// položky
	assert.Equal(t, ans.division, "se")
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
	content := loadTestingFile("testdata/lguide/numeral_response.html", t)
	ans := Parse(content)
	assert.Equal(t, ans.heading, "sto")

	// položky
	assert.Equal(t, ans.division, "sto")
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
	content := loadTestingFile("testdata/lguide/verb_response.html", t)
	ans := Parse(content)
	assert.Equal(t, ans.heading, "dělat")

	// položky
	assert.Equal(t, ans.division, "dě-lat")

	// tabulka
	assert.Equal(t, ans.conjugation.person.first.singular, "dělám")
	assert.Equal(t, ans.conjugation.person.first.plural, "děláme")
	assert.Equal(t, ans.conjugation.person.second.singular, "děláš")
	assert.Equal(t, ans.conjugation.person.second.plural, "děláte")
	assert.Equal(t, ans.conjugation.person.third.singular, "dělá")
	assert.Equal(t, ans.conjugation.person.third.plural, "dělají")

	assert.Equal(t, ans.conjugation.imperative.singular, "dělej")
	assert.Equal(t, ans.conjugation.imperative.plural, "dělejte")
	assert.Equal(t, ans.conjugation.participle.active, "dělal")
	assert.Equal(t, ans.conjugation.participle.passive, "dělán")
	assert.Equal(t, ans.conjugation.transgressive.present.m.singular, "dělaje")
	assert.Equal(t, ans.conjugation.transgressive.present.m.plural, "dělajíce")
	assert.Equal(t, ans.conjugation.transgressive.present.zs.singular, "dělajíc")
	assert.Equal(t, ans.conjugation.transgressive.present.zs.plural, "dělajíce")
	assert.Equal(t, ans.conjugation.verbalNoun, "dělání")
}

func TestParserAdverbResponse(t *testing.T) {
	content := loadTestingFile("testdata/lguide/adverb_response.html", t)
	ans := Parse(content)
	assert.Equal(t, ans.heading, "nahoře")

	// položky
	assert.Equal(t, ans.division, "na-ho-ře")
	assert.Contains(t, ans.items, "příklady")
	assert.Equal(t, ans.items["příklady"], "Spisy jsou uloženy nahoře na polici.(na rozdíl od: Stanul na hoře Říp/Řípu.)")
}

func TestParserPrepositionResponse(t *testing.T) {
	content := loadTestingFile("testdata/lguide/preposition_response.html", t)
	ans := Parse(content)
	assert.Equal(t, ans.heading, "vedle")

	// položky
	assert.Equal(t, ans.division, "ve-dle")
	assert.Contains(t, ans.items, "příklady")
	assert.Equal(t, ans.items["příklady"], "stáli vedle sebe; dům vedle se bude opravovat")
}

func TestParserConjunctionResponse(t *testing.T) {
	content := loadTestingFile("testdata/lguide/conjunction_response.html", t)
	ans := Parse(content)
	assert.Equal(t, ans.heading, "nebo")

	// položky
	assert.Equal(t, ans.division, "ne-bo")
	assert.Contains(t, ans.items, "příklady")
	assert.Equal(t, ans.items["příklady"], "Podejte nám zprávu písemně nebo telefonicky. Pospěšte si, nebo nám ujede vlak.")
}

func TestParserParticleResponse(t *testing.T) {
	content := loadTestingFile("testdata/lguide/particle_response.html", t)
	ans := Parse(content)
	assert.Equal(t, ans.heading, "ať")

	// položky
	assert.Equal(t, ans.division, "ať")
	assert.Contains(t, ans.items, "příklady")
	assert.Equal(t, ans.items["příklady"], "Ať \nuž však byl jeho úmysl jakýkoli, působil dojmem člověka, který to projel\n na celé čáře. Poradil nám, ať zajdeme za ředitelem. Ať se jde se svým \nnávrhem vycpat! Ať to byl, kdo chtěl, jasně vám dokazuji, že to měl \nudělat nějak jinak. Musí ho poslouchat všichni, ať jsou to kněží, nebo \nobchodníci.")
}

func TestParserInterjectionResponse(t *testing.T) {
	content := loadTestingFile("testdata/lguide/interjection_response.html", t)
	ans := Parse(content)
	assert.Equal(t, ans.heading, "haló")

	// položky
	assert.Equal(t, ans.division, "ha-ló")
	assert.Contains(t, ans.items, "jiné je")
	assert.Equal(t, ans.items["jiné je"], "haló, s.")
	assert.Contains(t, ans.items, "příklady")
	assert.Equal(t, ans.items["příklady"], "Haló, tady Jiřina!; Halo, právě volá vaše láska!")
}

func TestParseJavascript(t *testing.T) {
	content := loadTestingFile("testdata/lguide/adjective_response.html", t)
	ans := Parse(content)
	assert.Equal(t, 1, len(ans.scripts))
	assert.Equal(t, "/files/prirucka.js", ans.scripts[0])
	assert.Equal(t, 3, len(ans.cssLinks))
	assert.Equal(t, "/files/all1.css", ans.cssLinks[0])
	assert.Equal(t, "/files/screen1.css", ans.cssLinks[1])
	assert.Equal(t, "/files/print.css", ans.cssLinks[2])
}
