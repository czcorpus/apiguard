// Copyright 2025 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2025 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2025 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package wagstream

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFindErrorInStream(t *testing.T) {
	s := "event: DataTile-1\n" +
		"data: {\"foo\": 1000}\n\n" +
		"event: DataTile-2\n" +
		"data: {\"foo\": 1000, \"error\":\n" +
		"data:\"data resource not found\"}\n\n" +
		"event: DataTile-3\n" +
		"data: {\"bar\": \"lorem ipsum\"}\n\n"
	fmt.Println(s)
	errMsg, event := findErrorMsgInStream([]byte(s))
	assert.Equal(t, "data resource not found", errMsg)
	assert.Equal(t, "DataTile-2", event)
}
