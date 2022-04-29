// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Charles University - Faculty of Arts,
//                Institute of the Czech National Corpus
// All rights reserved.

package lguide

type GrammarNumber struct {
	Singular string `json:"singular"`
	Plural   string `json:"plural"`
}

type GrammarCase struct {
	Nominative   GrammarNumber `json:"nominative"`
	Genitive     GrammarNumber `json:"genitive"`
	Dative       GrammarNumber `json:"dative"`
	Accusative   GrammarNumber `json:"accusative"`
	Vocative     GrammarNumber `json:"vocative"`
	Locative     GrammarNumber `json:"locative"`
	Instrumental GrammarNumber `json:"instrumental"`
}

type GrammarPerson struct {
	First  GrammarNumber `json:"first"`
	Second GrammarNumber `json:"second"`
	Third  GrammarNumber `json:"third"`
}

type Participle struct {
	Active  string `json:"active"`
	Passive string `json:"passive"`
}

type TransgressiveRow struct {
	M  GrammarNumber `json:"m"`
	ZS GrammarNumber `json:"zs"`
}

type Transgressives struct {
	Past    TransgressiveRow `json:"past"`
	Present TransgressiveRow `json:"present"`
}

type Comparison struct {
	Comparative string `json:"comparative"`
	Superlative string `json:"superlative"`
}

type Conjugation struct {
	Person        GrammarPerson  `json:"person"`
	Imperative    GrammarNumber  `json:"imperative"`
	Participle    Participle     `json:"participle"`
	Transgressive Transgressives `json:"transgressive"`
	VerbalNoun    string         `json:"verbalNoun"`
}
