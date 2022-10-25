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

func TestParserNounResponse(t *testing.T) {
	content := loadTestingFile("testdata/assc/auto.html", t)
	ans, err := parseData(content)
	assert.NoError(t, err)
	assert.Len(t, ans, 1)

	first := ans[0]
	assert.Equal(t, first.Key, "auto")
	assert.Equal(t, first.Pronunciation, "[ʔa͡uto]")
	assert.Equal(t, first.Quality, "")

	assert.Contains(t, first.Forms, "2. j.")
	assert.Equal(t, first.Forms["2. j."], "auta")
	assert.Contains(t, first.Forms, "2. j.")
	assert.Equal(t, first.Forms["6. j."], "autě, autu")

	assert.Equal(t, first.POS, "podstatné jméno rodu středního")

	assert.Len(t, first.Meaning, 1)
	assert.Equal(t, first.Meaning[0].Explanation, "motorové vozidlo (obvykle čtyřkolové) určené k přepravě osob nebo nákladů")
	assert.Equal(t, first.Meaning[0].MetaExplanation, "")
	assert.ElementsMatch(t, first.Meaning[0].Synonyms, [1]string{"automobil"})
	assert.ElementsMatch(t, first.Meaning[0].Examples, [16]string{"nákladní / osobní auto", "ojeté auto", "policejní / hasičské auto", "závodní auto", "řidič auta", "kolona aut", "půjčovna aut", "nastartovat / zaparkovat auto", "přijet autem", "nastoupit do auta", "vystoupit z auta", "Auto prudce zabrzdilo.", "Auto havarovalo.", "Ženu srazilo auto.", "Šéf naboural služební auto.", "Dáš si pivo? – Ne, jsem tu autem."})

	assert.Equal(t, first.Note, "")
}
