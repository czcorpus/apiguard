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
