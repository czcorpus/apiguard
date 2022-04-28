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
	assert.Equal(t, ans.Heading, "okolnost")

	// položky
	assert.Equal(t, ans.Division, "okol-nost")
	assert.Contains(t, ans.items, "rod")
	assert.Equal(t, ans.items["rod"], "ž.")

	// tabulka
	assert.Equal(t, ans.GrammarCase.nominative.singular, "okolnost")
	assert.Equal(t, ans.GrammarCase.nominative.plural, "okolnosti")
	assert.Equal(t, ans.GrammarCase.genitive.singular, "okolnosti")
	assert.Equal(t, ans.GrammarCase.genitive.plural, "okolností")
	assert.Equal(t, ans.GrammarCase.dative.singular, "okolnosti")
	assert.Equal(t, ans.GrammarCase.dative.plural, "okolnostem")
	assert.Equal(t, ans.GrammarCase.accusative.singular, "okolnost")
	assert.Equal(t, ans.GrammarCase.accusative.plural, "okolnosti")
	assert.Equal(t, ans.GrammarCase.vocative.singular, "okolnosti")
	assert.Equal(t, ans.GrammarCase.vocative.plural, "okolnosti")
	assert.Equal(t, ans.GrammarCase.locative.singular, "okolnosti")
	assert.Equal(t, ans.GrammarCase.locative.plural, "okolnostech")
	assert.Equal(t, ans.GrammarCase.instrumental.singular, "okolností")
	assert.Equal(t, ans.GrammarCase.instrumental.plural, "okolnostmi")
}

func TestParserAdjectiveResponse(t *testing.T) {
	content := loadTestingFile("testdata/lguide/adjective_response.html", t)
	ans := Parse(content)
	assert.Equal(t, ans.Heading, "modrý")

	// položky
	assert.Equal(t, ans.Division, "mo-d-rý")
	assert.Equal(t, ans.Comparison.comparative, "modřejší")
	assert.Equal(t, ans.Comparison.superlative, "nejmodřejší")
	assert.Contains(t, ans.items, "příklady")
	assert.Equal(t, ans.items["příklady"], "modré oči; tmavě modré šaty; tmavomodré šaty")
}

func TestParserPronounResponse(t *testing.T) {
	content := loadTestingFile("testdata/lguide/pronoun_response.html", t)
	ans := Parse(content)
	assert.Equal(t, ans.Heading, "se")

	// položky
	assert.Equal(t, ans.Division, "se")
	assert.Contains(t, ans.items, "jiné je")
	assert.Equal(t, ans.items["jiné je"], "se, předl.")
	assert.Contains(t, ans.items, "příklady")
	assert.Equal(t, ans.items["příklady"], "vzít s sebou; to se rozumí samo sebou; otevření (se) světu; vařící (se) voda; rozhodl se zúčastnit se; rozhodl se zúčastnit; rozhodl zúčastnit se")

	// tabulka
	assert.Equal(t, ans.GrammarCase.nominative.singular, "")
	assert.Equal(t, ans.GrammarCase.genitive.singular, "sebe")
	assert.Equal(t, ans.GrammarCase.dative.singular, "sobě, si")
	assert.Equal(t, ans.GrammarCase.accusative.singular, "sebe, se")
	assert.Equal(t, ans.GrammarCase.vocative.singular, "")
	assert.Equal(t, ans.GrammarCase.locative.singular, "sobě")
	assert.Equal(t, ans.GrammarCase.instrumental.singular, "sebou")

	assert.Equal(t, ans.GrammarCase.nominative.plural, "")
	assert.Equal(t, ans.GrammarCase.genitive.plural, "")
	assert.Equal(t, ans.GrammarCase.dative.plural, "")
	assert.Equal(t, ans.GrammarCase.accusative.plural, "")
	assert.Equal(t, ans.GrammarCase.vocative.plural, "")
	assert.Equal(t, ans.GrammarCase.locative.plural, "")
	assert.Equal(t, ans.GrammarCase.instrumental.plural, "")
}

func TestParserNumeralResponse(t *testing.T) {
	content := loadTestingFile("testdata/lguide/numeral_response.html", t)
	ans := Parse(content)
	assert.Equal(t, ans.Heading, "sto")

	// položky
	assert.Equal(t, ans.Division, "sto")
	assert.Contains(t, ans.items, "rod")
	assert.Equal(t, ans.items["rod"], "s.")
	assert.Contains(t, ans.items, "příklady")
	assert.Equal(t, ans.items["příklady"], "pět set; sto padesát tři; tři sta třicet tři tisíc; asi sto lidí se sešlo před magistrátem, aby protestovalo/protestovali proti plánované stavbě")
	assert.Contains(t, ans.items, "poznámky k heslu")
	assert.Equal(t, ans.items["poznámky k heslu"], "ve spojení s výrazem dvě má tvar 1. p. mn. č. podobu stě (dvě stě); ve spojení s počítaným předmětem může v j. č. zůstat výraz sto nesklonný (ke stu korun/korunám i ke sto korunám)")

	// tabulka
	assert.Equal(t, ans.GrammarCase.nominative.singular, "sto")
	assert.Equal(t, ans.GrammarCase.nominative.plural, "sta")
	assert.Equal(t, ans.GrammarCase.genitive.singular, "sta")
	assert.Equal(t, ans.GrammarCase.genitive.plural, "set")
	assert.Equal(t, ans.GrammarCase.dative.singular, "stu")
	assert.Equal(t, ans.GrammarCase.dative.plural, "stům")
	assert.Equal(t, ans.GrammarCase.accusative.singular, "sto")
	assert.Equal(t, ans.GrammarCase.accusative.plural, "sta")
	assert.Equal(t, ans.GrammarCase.vocative.singular, "sto")
	assert.Equal(t, ans.GrammarCase.vocative.plural, "sta")
	assert.Equal(t, ans.GrammarCase.locative.singular, "stu")
	assert.Equal(t, ans.GrammarCase.locative.plural, "stech")
	assert.Equal(t, ans.GrammarCase.instrumental.singular, "stem")
	assert.Equal(t, ans.GrammarCase.instrumental.plural, "sty")
}

func TestParserVerbResponse(t *testing.T) {
	content := loadTestingFile("testdata/lguide/verb_response.html", t)
	ans := Parse(content)
	assert.Equal(t, ans.Heading, "dělat")

	// položky
	assert.Equal(t, ans.Division, "dě-lat")

	// tabulka
	assert.Equal(t, ans.Conjugation.person.first.singular, "dělám")
	assert.Equal(t, ans.Conjugation.person.first.plural, "děláme")
	assert.Equal(t, ans.Conjugation.person.second.singular, "děláš")
	assert.Equal(t, ans.Conjugation.person.second.plural, "děláte")
	assert.Equal(t, ans.Conjugation.person.third.singular, "dělá")
	assert.Equal(t, ans.Conjugation.person.third.plural, "dělají")

	assert.Equal(t, ans.Conjugation.imperative.singular, "dělej")
	assert.Equal(t, ans.Conjugation.imperative.plural, "dělejte")
	assert.Equal(t, ans.Conjugation.participle.active, "dělal")
	assert.Equal(t, ans.Conjugation.participle.passive, "dělán")
	assert.Equal(t, ans.Conjugation.transgressive.present.m.singular, "dělaje")
	assert.Equal(t, ans.Conjugation.transgressive.present.m.plural, "dělajíce")
	assert.Equal(t, ans.Conjugation.transgressive.present.zs.singular, "dělajíc")
	assert.Equal(t, ans.Conjugation.transgressive.present.zs.plural, "dělajíce")
	assert.Equal(t, ans.Conjugation.verbalNoun, "dělání")
}

func TestParserAdverbResponse(t *testing.T) {
	content := loadTestingFile("testdata/lguide/adverb_response.html", t)
	ans := Parse(content)
	assert.Equal(t, ans.Heading, "nahoře")

	// položky
	assert.Equal(t, ans.Division, "na-ho-ře")
	assert.Contains(t, ans.items, "příklady")
	assert.Equal(t, ans.items["příklady"], "Spisy jsou uloženy nahoře na polici.(na rozdíl od: Stanul na hoře Říp/Řípu.)")
}

func TestParserPrepositionResponse(t *testing.T) {
	content := loadTestingFile("testdata/lguide/preposition_response.html", t)
	ans := Parse(content)
	assert.Equal(t, ans.Heading, "vedle")

	// položky
	assert.Equal(t, ans.Division, "ve-dle")
	assert.Contains(t, ans.items, "příklady")
	assert.Equal(t, ans.items["příklady"], "stáli vedle sebe; dům vedle se bude opravovat")
}

func TestParserConjunctionResponse(t *testing.T) {
	content := loadTestingFile("testdata/lguide/conjunction_response.html", t)
	ans := Parse(content)
	assert.Equal(t, ans.Heading, "nebo")

	// položky
	assert.Equal(t, ans.Division, "ne-bo")
	assert.Contains(t, ans.items, "příklady")
	assert.Equal(t, ans.items["příklady"], "Podejte nám zprávu písemně nebo telefonicky. Pospěšte si, nebo nám ujede vlak.")
}

func TestParserParticleResponse(t *testing.T) {
	content := loadTestingFile("testdata/lguide/particle_response.html", t)
	ans := Parse(content)
	assert.Equal(t, ans.Heading, "ať")

	// položky
	assert.Equal(t, ans.Division, "ať")
	assert.Contains(t, ans.items, "příklady")
	assert.Equal(
		t,
		"Ať\nuž však byl jeho úmysl jakýkoli, působil dojmem člověka, který to projel\n na celé čáře. Poradil nám, ať zajdeme za ředitelem. Ať se jde se svým\nnávrhem vycpat! Ať to byl, kdo chtěl, jasně vám dokazuji, že to měl\nudělat nějak jinak. Musí ho poslouchat všichni, ať jsou to kněží, nebo\nobchodníci.",
		ans.items["příklady"],
	)
}

func TestParserInterjectionResponse(t *testing.T) {
	content := loadTestingFile("testdata/lguide/interjection_response.html", t)
	ans := Parse(content)
	assert.Equal(t, ans.Heading, "haló")

	// položky
	assert.Equal(t, ans.Division, "ha-ló")
	assert.Contains(t, ans.items, "jiné je")
	assert.Equal(t, ans.items["jiné je"], "haló, s.")
	assert.Contains(t, ans.items, "příklady")
	assert.Equal(t, ans.items["příklady"], "Haló, tady Jiřina!; Halo, právě volá vaše láska!")
}

func TestParseJavascript(t *testing.T) {
	content := loadTestingFile("testdata/lguide/adjective_response.html", t)
	ans := Parse(content)
	assert.Equal(t, 1, len(ans.Scripts))
	assert.Equal(t, "/files/prirucka.js", ans.Scripts[0])
	assert.Equal(t, 3, len(ans.CSSLinks))
	assert.Equal(t, "/files/all1.css", ans.CSSLinks[0])
	assert.Equal(t, "/files/screen1.css", ans.CSSLinks[1])
	assert.Equal(t, "/files/print.css", ans.CSSLinks[2])
}
