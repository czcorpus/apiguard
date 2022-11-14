// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package assc

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

func TestParserAutoResponse(t *testing.T) {
	content := loadTestingFile("testdata/assc/auto.html", t)
	ans, err := parseData(content)
	assert.NoError(t, err)
	assert.Len(t, ans, 1)

	first := ans[0]
	assert.Equal(t, first.Key, "auto")
	assert.Equal(t, first.Pronunciation, "[ʔa͡uto]")
	assert.Equal(t, first.Quality, "")
	assert.Equal(t, first.POS, "podstatné jméno rodu středního")
	assert.Equal(t, first.Note, "")

	assert.Contains(t, first.Forms, "2. j.")
	assert.Equal(t, first.Forms["2. j."], "auta")
	assert.Contains(t, first.Forms, "2. j.")
	assert.Equal(t, first.Forms["6. j."], "autě, autu")

	assert.Len(t, first.Meaning, 1)
	assert.Equal(t, first.Meaning[0].Explanation, "motorové vozidlo (obvykle čtyřkolové) určené k přepravě osob nebo nákladů")
	assert.Equal(t, first.Meaning[0].MetaExplanation, "")
	assert.ElementsMatch(t, first.Meaning[0].Synonyms, [1]string{"automobil"})
	assert.ElementsMatch(t, first.Meaning[0].Examples, [16]string{"nákladní / osobní auto", "ojeté auto", "policejní / hasičské auto", "závodní auto", "řidič auta", "kolona aut", "půjčovna aut", "nastartovat / zaparkovat auto", "přijet autem", "nastoupit do auta", "vystoupit z auta", "Auto prudce zabrzdilo.", "Auto havarovalo.", "Ženu srazilo auto.", "Šéf naboural služební auto.", "Dáš si pivo? – Ne, jsem tu autem."})

	assert.Len(t, first.Phrasemes, 0)
}

func TestParserDrakResponse(t *testing.T) {
	content := loadTestingFile("testdata/assc/drak.html", t)
	ans, err := parseData(content)
	assert.NoError(t, err)
	assert.Len(t, ans, 2)

	first := ans[0]
	assert.Equal(t, first.Key, "drak I")
	assert.Equal(t, first.Pronunciation, "[drak]")
	assert.Equal(t, first.Quality, "")
	assert.Equal(t, first.POS, "podstatné jméno rodu mužského životného")
	assert.Equal(t, first.Note, "")

	assert.Contains(t, first.Forms, "2. j.")
	assert.Equal(t, first.Forms["2. j."], "draka")
	assert.Contains(t, first.Forms, "3., 6. j.")
	assert.Equal(t, first.Forms["3., 6. j."], "drakovi, draku")
	assert.Contains(t, first.Forms, "5. j.")
	assert.Equal(t, first.Forms["5. j."], "draku")
	assert.Contains(t, first.Forms, "1. mn.")
	assert.Equal(t, first.Forms["1. mn."], "draci, drakové")
	assert.Contains(t, first.Forms, "6. mn.")
	assert.Equal(t, first.Forms["6. mn."], "dracích")

	assert.Len(t, first.Meaning, 2)
	assert.Equal(t, first.Meaning[0].Explanation, "pohádkový netvor znázorňovaný v podobě okřídleného ještěra, často s několika hlavami")
	assert.Equal(t, first.Meaning[0].MetaExplanation, "")
	assert.Len(t, first.Meaning[0].Synonyms, 0)
	assert.ElementsMatch(t, first.Meaning[0].Examples, [5]string{"pohádkový / sedmihlavý / strašlivý drak", "souboj s drakem", "přemoct / zabít draka", "utnout drakovi hlavu", "Princ bojoval s drakem."})
	assert.Equal(t, first.Meaning[1].Explanation, "dětská hračka s dřevěnou kostrou potaženou papírem nebo plátnem, pouštěná po větru do vzduchu")
	assert.Equal(t, first.Meaning[1].MetaExplanation, "")
	assert.Len(t, first.Meaning[1].Synonyms, 0)
	assert.ElementsMatch(t, first.Meaning[1].Examples, [5]string{"papírový drak", "soutěž o nejhezčího draka", "pouštět draka", "Nakonec se počasí nad našimi draky slitovalo a vítr začal vát.", "Draci nám létali pěkně vysoko."})

	assert.Len(t, first.Phrasemes, 4)
	assert.Equal(t, first.Phrasemes[0].Phraseme, "bojovat jak(o) drak")
	assert.Equal(t, first.Phrasemes[0].Explanation, "bojovat s velkým nasazením, elánem, velmi intenzivně")
	assert.ElementsMatch(t, first.Phrasemes[0].Examples, [1]string{"Bojovala jako drak a v závodě si dojela pro krásné druhé místo."})
	assert.Equal(t, first.Phrasemes[1].Phraseme, "být do práce jak(o) drak")
	assert.Equal(t, first.Phrasemes[1].Explanation, "být velmi pracovitý")
	assert.ElementsMatch(t, first.Phrasemes[1].Examples, [0]string{})
	assert.Equal(t, first.Phrasemes[2].Phraseme, "být na draka")
	assert.Equal(t, first.Phrasemes[2].Explanation, "být špatný, neradostný, bezcenný, neužitečný")
	assert.ElementsMatch(t, first.Phrasemes[2].Examples, [0]string{})
	assert.Equal(t, first.Phrasemes[3].Phraseme, "jet jak(o) drak")
	assert.Equal(t, first.Phrasemes[3].Explanation, "jet velmi rychle")
	assert.ElementsMatch(t, first.Phrasemes[3].Examples, [0]string{})

	second := ans[1]
	assert.Equal(t, second.Key, "dráček")
	assert.Equal(t, second.Pronunciation, "[draːček]")
	assert.Equal(t, second.Quality, "")
	assert.Equal(t, second.POS, "podstatné jméno rodu mužského životného")
	assert.Equal(t, second.Note, "")

	assert.Contains(t, second.Forms, "2. j.")
	assert.Equal(t, second.Forms["2. j."], "-čka")

	assert.Len(t, second.Meaning, 2)
	assert.Equal(t, second.Meaning[0].Explanation, "")
	assert.Equal(t, second.Meaning[0].MetaExplanation, "zdrob., zprav. expr. k 1")
	assert.Len(t, second.Meaning[0].Synonyms, 0)
	assert.ElementsMatch(t, second.Meaning[0].Examples, [3]string{"pohádkový dráček", "Na prapor vyšila zeleného dráčka.", "Z tajemného vajíčka se vyklubal hodný dráček."})
	assert.Equal(t, second.Meaning[1].Explanation, "")
	assert.Equal(t, second.Meaning[1].MetaExplanation, "zdrob., zprav. expr. k 2")
	assert.Len(t, second.Meaning[1].Synonyms, 0)
	assert.ElementsMatch(t, second.Meaning[1].Examples, [1]string{"Děti krásně vyzdobily papírové dráčky."})

	assert.Len(t, second.Phrasemes, 0)
}

func TestParserCenovkaResponse(t *testing.T) {
	content := loadTestingFile("testdata/assc/cenovka.html", t)
	ans, err := parseData(content)
	assert.NoError(t, err)
	assert.Len(t, ans, 1)

	first := ans[0]
	assert.Equal(t, first.Key, "cenovka")
	assert.Equal(t, first.Pronunciation, "[cenofka], 2. mn. [cenovek]")
	assert.Equal(t, first.AudioFile, "cenovka_soubory/14474.ogg")
	assert.Equal(t, first.Quality, "")
	assert.Equal(t, first.POS, "podstatné jméno rodu ženského")
	assert.Equal(t, first.Note, "")

	assert.Contains(t, first.Forms, "2. j.")
	assert.Equal(t, first.Forms["2. j."], "-vky")
	assert.Contains(t, first.Forms, "2. mn.")
	assert.Equal(t, first.Forms["2. mn."], "-vek")

	assert.Len(t, first.Meaning, 1)
	assert.Equal(t, first.Meaning[0].Explanation, "štítek, visačka s cenou zboží (často i s dalšími údaji)")
	assert.Equal(t, first.Meaning[0].MetaExplanation, "")
	assert.Len(t, first.Meaning[0].Synonyms, 0)
	assert.ElementsMatch(t, first.Meaning[0].Examples, [5]string{
		"papírové / elektronické cenovky",
		"regálová cenovka",
		"cenovka s popisem a čárovým kódem",
		"lepit cenovky na zboží",
		"označit výrobek cenovkou",
	})

	assert.Len(t, first.Phrasemes, 0)
}
