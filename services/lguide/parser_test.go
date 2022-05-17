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
	assert.NoError(t, ans.Error)
	assert.Equal(t, ans.Heading, "okolnost")

	// položky
	assert.Equal(t, ans.Syllabification, "okol-nost")
	assert.Equal(t, ans.Gender, "ž.")

	// tabulka
	assert.Equal(t, ans.GrammarCase.Nominative.Singular, "okolnost")
	assert.Equal(t, ans.GrammarCase.Nominative.Plural, "okolnosti")
	assert.Equal(t, ans.GrammarCase.Genitive.Singular, "okolnosti")
	assert.Equal(t, ans.GrammarCase.Genitive.Plural, "okolností")
	assert.Equal(t, ans.GrammarCase.Dative.Singular, "okolnosti")
	assert.Equal(t, ans.GrammarCase.Dative.Plural, "okolnostem")
	assert.Equal(t, ans.GrammarCase.Accusative.Singular, "okolnost")
	assert.Equal(t, ans.GrammarCase.Accusative.Plural, "okolnosti")
	assert.Equal(t, ans.GrammarCase.Vocative.Singular, "okolnosti")
	assert.Equal(t, ans.GrammarCase.Vocative.Plural, "okolnosti")
	assert.Equal(t, ans.GrammarCase.Locative.Singular, "okolnosti")
	assert.Equal(t, ans.GrammarCase.Locative.Plural, "okolnostech")
	assert.Equal(t, ans.GrammarCase.Instrumental.Singular, "okolností")
	assert.Equal(t, ans.GrammarCase.Instrumental.Plural, "okolnostmi")
}

func TestParserAdjectiveResponse(t *testing.T) {
	content := loadTestingFile("testdata/lguide/adjective_response.html", t)
	ans := Parse(content)
	assert.NoError(t, ans.Error)
	assert.Equal(t, ans.Heading, "modrý")

	// položky
	assert.Equal(t, ans.Syllabification, "mo-d-rý")
	assert.Equal(t, ans.Comparison.Comparative, "modřejší")
	assert.Equal(t, ans.Comparison.Superlative, "nejmodřejší")
	assert.ElementsMatch(t, ans.Examples, [3]string{
		"modré oči",
		"tmavě modré šaty",
		"tmavomodré šaty",
	})
}

func TestParserPronounResponse(t *testing.T) {
	content := loadTestingFile("testdata/lguide/pronoun_response.html", t)
	ans := Parse(content)
	assert.NoError(t, ans.Error)
	assert.Equal(t, ans.Heading, "se")

	// položky
	assert.Equal(t, ans.Syllabification, "se")
	assert.Contains(t, ans.items, "jiné je")
	assert.Equal(t, ans.items["jiné je"], "se, předl.")
	assert.ElementsMatch(t, ans.Examples, [7]string{
		"vzít s sebou",
		"to se rozumí samo sebou",
		"otevření (se) světu",
		"vařící (se) voda",
		"rozhodl se zúčastnit se",
		"rozhodl se zúčastnit",
		"rozhodl zúčastnit se",
	})

	// tabulka
	assert.Equal(t, ans.GrammarCase.Nominative.Singular, "")
	assert.Equal(t, ans.GrammarCase.Genitive.Singular, "sebe")
	assert.Equal(t, ans.GrammarCase.Dative.Singular, "sobě, si")
	assert.Equal(t, ans.GrammarCase.Accusative.Singular, "sebe, se")
	assert.Equal(t, ans.GrammarCase.Vocative.Singular, "")
	assert.Equal(t, ans.GrammarCase.Locative.Singular, "sobě")
	assert.Equal(t, ans.GrammarCase.Instrumental.Singular, "sebou")

	assert.Equal(t, ans.GrammarCase.Nominative.Plural, "")
	assert.Equal(t, ans.GrammarCase.Genitive.Plural, "")
	assert.Equal(t, ans.GrammarCase.Dative.Plural, "")
	assert.Equal(t, ans.GrammarCase.Accusative.Plural, "")
	assert.Equal(t, ans.GrammarCase.Vocative.Plural, "")
	assert.Equal(t, ans.GrammarCase.Locative.Plural, "")
	assert.Equal(t, ans.GrammarCase.Instrumental.Plural, "")
}

func TestParserNumeralResponse(t *testing.T) {
	content := loadTestingFile("testdata/lguide/numeral_response.html", t)
	ans := Parse(content)
	assert.NoError(t, ans.Error)
	assert.Equal(t, ans.Heading, "sto")

	// položky
	assert.Equal(t, ans.Syllabification, "sto")
	assert.Equal(t, ans.Gender, "s.")
	assert.ElementsMatch(t, ans.Examples, [4]string{
		"pět set",
		"sto padesát tři",
		"tři sta třicet tři tisíc",
		"asi sto lidí se sešlo před magistrátem, aby protestovalo/protestovali proti plánované stavbě",
	})
	assert.Contains(t, ans.items, "poznámky k heslu")
	assert.Equal(t, ans.items["poznámky k heslu"], "ve spojení s výrazem dvě má tvar 1. p. mn. č. podobu stě (dvě stě); ve spojení s počítaným předmětem může v j. č. zůstat výraz sto nesklonný (ke stu korun/korunám i ke sto korunám)")

	// tabulka
	assert.Equal(t, ans.GrammarCase.Nominative.Singular, "sto")
	assert.Equal(t, ans.GrammarCase.Nominative.Plural, "sta")
	assert.Equal(t, ans.GrammarCase.Genitive.Singular, "sta")
	assert.Equal(t, ans.GrammarCase.Genitive.Plural, "set")
	assert.Equal(t, ans.GrammarCase.Dative.Singular, "stu")
	assert.Equal(t, ans.GrammarCase.Dative.Plural, "stům")
	assert.Equal(t, ans.GrammarCase.Accusative.Singular, "sto")
	assert.Equal(t, ans.GrammarCase.Accusative.Plural, "sta")
	assert.Equal(t, ans.GrammarCase.Vocative.Singular, "sto")
	assert.Equal(t, ans.GrammarCase.Vocative.Plural, "sta")
	assert.Equal(t, ans.GrammarCase.Locative.Singular, "stu")
	assert.Equal(t, ans.GrammarCase.Locative.Plural, "stech")
	assert.Equal(t, ans.GrammarCase.Instrumental.Singular, "stem")
	assert.Equal(t, ans.GrammarCase.Instrumental.Plural, "sty")
}

func TestParserVerbResponse(t *testing.T) {
	content := loadTestingFile("testdata/lguide/verb_response.html", t)
	ans := Parse(content)
	assert.NoError(t, ans.Error)
	assert.Equal(t, ans.Heading, "dělat")

	// položky
	assert.Equal(t, ans.Syllabification, "dě-lat")

	// tabulka
	assert.Equal(t, ans.Conjugation.Person.First.Singular, "dělám")
	assert.Equal(t, ans.Conjugation.Person.First.Plural, "děláme")
	assert.Equal(t, ans.Conjugation.Person.Second.Singular, "děláš")
	assert.Equal(t, ans.Conjugation.Person.Second.Plural, "děláte")
	assert.Equal(t, ans.Conjugation.Person.Third.Singular, "dělá")
	assert.Equal(t, ans.Conjugation.Person.Third.Plural, "dělají")

	assert.Equal(t, ans.Conjugation.Imperative.Singular, "dělej")
	assert.Equal(t, ans.Conjugation.Imperative.Plural, "dělejte")
	assert.Equal(t, ans.Conjugation.Participle.Active, "dělal")
	assert.Equal(t, ans.Conjugation.Participle.Passive, "dělán")
	assert.Equal(t, ans.Conjugation.Transgressive.Present.M.Singular, "dělaje")
	assert.Equal(t, ans.Conjugation.Transgressive.Present.M.Plural, "dělajíce")
	assert.Equal(t, ans.Conjugation.Transgressive.Present.ZS.Singular, "dělajíc")
	assert.Equal(t, ans.Conjugation.Transgressive.Present.ZS.Plural, "dělajíce")
	assert.Equal(t, ans.Conjugation.VerbalNoun, "dělání")
}

func TestParserAdverbResponse(t *testing.T) {
	content := loadTestingFile("testdata/lguide/adverb_response.html", t)
	ans := Parse(content)
	assert.NoError(t, ans.Error)
	assert.Equal(t, ans.Heading, "nahoře")

	// položky
	assert.Equal(t, ans.Syllabification, "na-ho-ře")
	assert.ElementsMatch(t, ans.Examples, [1]string{"Spisy jsou uloženy nahoře na polici.(na rozdíl od: Stanul na hoře Říp/Řípu.)"})
}

func TestParserPrepositionResponse(t *testing.T) {
	content := loadTestingFile("testdata/lguide/preposition_response.html", t)
	ans := Parse(content)
	assert.NoError(t, ans.Error)
	assert.Equal(t, ans.Heading, "vedle")

	// položky
	assert.Equal(t, ans.Syllabification, "ve-dle")
	assert.ElementsMatch(t, ans.Examples, [2]string{
		"stáli vedle sebe",
		"dům vedle se bude opravovat",
	})
}

func TestParserConjunctionResponse(t *testing.T) {
	content := loadTestingFile("testdata/lguide/conjunction_response.html", t)
	ans := Parse(content)
	assert.NoError(t, ans.Error)
	assert.Equal(t, ans.Heading, "nebo")

	// položky
	assert.Equal(t, ans.Syllabification, "ne-bo")
	assert.ElementsMatch(t, ans.Examples, [1]string{"Podejte nám zprávu písemně nebo telefonicky. Pospěšte si, nebo nám ujede vlak."})
}

func TestParserParticleResponse(t *testing.T) {
	content := loadTestingFile("testdata/lguide/particle_response.html", t)
	ans := Parse(content)
	assert.NoError(t, ans.Error)
	assert.Equal(t, ans.Heading, "ať")

	// položky
	assert.Equal(t, ans.Syllabification, "ať")
	assert.ElementsMatch(
		t,
		ans.Examples,
		[1]string{"Ať\nuž však byl jeho úmysl jakýkoli, působil dojmem člověka, který to projel\n na celé čáře. Poradil nám, ať zajdeme za ředitelem. Ať se jde se svým\nnávrhem vycpat! Ať to byl, kdo chtěl, jasně vám dokazuji, že to měl\nudělat nějak jinak. Musí ho poslouchat všichni, ať jsou to kněží, nebo\nobchodníci."},
	)
}

func TestParserInterjectionResponse(t *testing.T) {
	content := loadTestingFile("testdata/lguide/interjection_response.html", t)
	ans := Parse(content)
	assert.NoError(t, ans.Error)
	assert.Equal(t, ans.Heading, "haló")

	// položky
	assert.Equal(t, ans.Syllabification, "ha-ló")
	assert.Contains(t, ans.items, "jiné je")
	assert.Equal(t, ans.items["jiné je"], "haló, s.")
	assert.ElementsMatch(t, ans.Examples, [2]string{
		"Haló, tady Jiřina!",
		"Halo, právě volá vaše láska!",
	})
}

func TestParseJavascript(t *testing.T) {
	content := loadTestingFile("testdata/lguide/adjective_response.html", t)
	ans := Parse(content)
	assert.NoError(t, ans.Error)
	assert.Equal(t, 1, len(ans.Scripts))
	assert.Equal(t, "/files/prirucka.js", ans.Scripts[0])
	assert.Equal(t, 3, len(ans.CSSLinks))
	assert.Equal(t, "/files/all1.css", ans.CSSLinks[0])
	assert.Equal(t, "/files/screen1.css", ans.CSSLinks[1])
	assert.Equal(t, "/files/print.css", ans.CSSLinks[2])
}
