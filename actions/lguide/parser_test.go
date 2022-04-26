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

	assert.Contains(t, ans, "dělení")
	assert.Equal(t, ans["dělení"], "okol-nost")
	assert.Contains(t, ans, "rod")
	assert.Equal(t, ans["rod"], "ž.")

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
