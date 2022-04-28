// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package lguide

type GrammarNumber struct {
	singular string
	plural   string
}

type GrammarCase struct {
	nominative   GrammarNumber
	genitive     GrammarNumber
	dative       GrammarNumber
	accusative   GrammarNumber
	vocative     GrammarNumber
	locative     GrammarNumber
	instrumental GrammarNumber
}

type GrammarPerson struct {
	first  GrammarNumber
	second GrammarNumber
	third  GrammarNumber
}

type Participle struct {
	active  string
	passive string
}

type TransgressiveRow struct {
	m  GrammarNumber
	zs GrammarNumber
}

type Transgressives struct {
	past    TransgressiveRow
	present TransgressiveRow
}

type Comparison struct {
	comparative string
	superlative string
}

type NounData struct {
	grammarCase GrammarCase
}

type Conjugation struct {
	person        GrammarPerson
	imperative    GrammarNumber
	participle    Participle
	transgressive Transgressives
	verbalNoun    string
}
