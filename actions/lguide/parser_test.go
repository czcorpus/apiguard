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
	assert.Equal(t, ans[0], [2]string{"okolnost", "okolnosti"})
	assert.Equal(t, ans[1], [2]string{"okolnosti", "okolností"})
	assert.Equal(t, ans[2], [2]string{"okolnosti", "okolnostem"})
	assert.Equal(t, ans[3], [2]string{"okolnost", "okolnosti"})
	assert.Equal(t, ans[4], [2]string{"okolnosti", "okolnosti"})
	assert.Equal(t, ans[5], [2]string{"okolnosti", "okolnostech"})
	assert.Equal(t, ans[6], [2]string{"okolností", "okolnostmi"})
}
