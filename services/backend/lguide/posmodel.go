// Copyright 2022 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2022 Martin Zimandl <martin.zimandl@gmail.com>
// Copyright 2022 Department of Linguistics,
//                Faculty of Arts, Charles University
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

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
