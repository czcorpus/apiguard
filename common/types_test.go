// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package common

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSec2hms1(t *testing.T) {
	ans := Dur2Hms(9613 * time.Second)
	assert.Equal(t, "02:40:13", ans)
}

func TestSec2hms2(t *testing.T) {
	ans := Dur2Hms(362413 * time.Second)
	assert.Equal(t, "100:40:13", ans)
}

func TestSec2hms3(t *testing.T) {
	ans := Dur2Hms(1033 * time.Second)
	assert.Equal(t, "00:17:13", ans)
}

func TestSec2hmsZero(t *testing.T) {
	ans := Dur2Hms(0)
	assert.Equal(t, "00:00:00", ans)
}

func TestSec2hmsNegative(t *testing.T) {
	assert.Panics(t, func() {
		Dur2Hms(-10 * time.Second)
	})
}
