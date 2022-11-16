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

func TestParserBytResponse(t *testing.T) {
	content := loadTestingFile("testdata/assc/byt.html", t)
	ans, err := parseData(content)
	assert.NoError(t, err)
	assert.Len(t, ans, 3)

	first := ans[0]
	assert.Equal(t, first.Key, "být")
	assert.Equal(t, first.Pronunciation, "[biːt]")
	assert.Equal(t, first.AudioFile, "")
	assert.Equal(t, first.Quality, "")
	assert.Equal(t, first.POS, "sloveso nedokonavé")
	assert.Equal(t, first.Note, "")

	// --- 1

	assert.Equal(t, first.Meaning[0].Explanation, "trvat, vyskytovat se v prostoru a čase")
	assert.Equal(t, first.Meaning[0].MetaExplanation, "")
	assert.Equal(t, first.Meaning[0].Attachement, "(kdo, co je ~; kde)")
	assert.ElementsMatch(t, first.Meaning[0].Synonyms, [1]string{"existovat"})
	assert.ElementsMatch(t, first.Meaning[0].Examples, [8]string{
		"Na světě je spousta krásných věcí.",
		"Dříve nebylo tolik aut.",
		"Takové slovo není.",
		"Je na Marsu život?",
		"Strašidla jsou jen v pohádkách.",
		"Jsou lidé, kteří nemají smysl pro humor.",
		"Descartes řekl: myslím, tedy jsem.",
		"Byl jednou jeden král…, Žili, byli…, Bylo nebylo… (úvodní formule v pohádkách)",
	})

	assert.Equal(t, first.Collocations[0].Collocation, "nebýt toho")
	assert.Equal(t, first.Collocations[0].Explanation, "")
	assert.ElementsMatch(t, first.Collocations[0].Examples, [0]string{})
	assert.Equal(t, first.Collocations[1].Collocation, "nebýt někoho / něčeho")
	assert.Equal(t, first.Collocations[1].Explanation, "pokud by nenastala určitá skutečnost (něco by se uskutečnilo)")
	assert.ElementsMatch(t, first.Collocations[1].Examples, [3]string{
		"Nebýt toho, že informace vynesli hackeři, zůstalo by všechno tajemstvím.",
		"Nebýt války, mohla z ní být klavírní virtuoska.",
		"Nebýt maminky, vůbec bych to nezvládla.",
	})

	// --- 2

	assert.Equal(t, first.Meaning[1].Explanation, "nacházet se na určitém místě")
	assert.Equal(t, first.Meaning[1].MetaExplanation, "")
	assert.Equal(t, first.Meaning[1].Attachement, "(kdo, co je kde)")
	assert.ElementsMatch(t, first.Meaning[1].Synonyms, [0]string{})
	assert.ElementsMatch(t, first.Meaning[1].Examples, [10]string{
		"Postel je u zdi.",
		"Kde je vypínač?",
		"Matka byla v kuchyni.",
		"Večer budu doma.",
		"Šanony jsou na polici.",
		"Arktida je na severu.",
		"Silnice není na mapě.",
		"Zlín je blízko hranic se Slovenskem.",
		"Okna jsou na jih. (vedou)",
		"„Kde jsi?“ – „V práci.“",
	})

	// --- 3

	assert.Equal(t, first.Meaning[2].Explanation, "trvat, uskutečňovat se v určitém čase")
	assert.Equal(t, first.Meaning[2].MetaExplanation, "")
	assert.Equal(t, first.Meaning[2].Attachement, "(co je kdy) (~ je kdy)")
	assert.ElementsMatch(t, first.Meaning[2].Synonyms, [0]string{})
	assert.ElementsMatch(t, first.Meaning[2].Examples, [10]string{
		"Dnes je úterý.",
		"Brzy bude jaro.",
		"Kdy je Kateřiny? (má svátek)",
		"Konečně už jsou prázdniny.",
		"Ordinační doba je od 13 do 18 hodin.",
		"V roce 2002 byly povodně.",
		"Začátek majálesu bude v poledne.",
		"Tréninky jsou v pátek.",
		"Výstava je až do neděle.",
		"Je to hodina, co se to stalo.",
	})

	// --- 4

	assert.Equal(t, first.Meaning[3].Explanation, "patřit, náležet do určité třídy věcí, jevů, skupiny ap. • mít stejnou platnost, jevit se totožným, odpovídat, rovnat se")
	assert.Equal(t, first.Meaning[3].MetaExplanation, "")
	assert.Equal(t, first.Meaning[3].Attachement, "(kdo, co je kdo, co; kým, čím; kde)")
	assert.ElementsMatch(t, first.Meaning[3].Synonyms, [0]string{})
	assert.ElementsMatch(t, first.Meaning[3].Examples, [16]string{
		"být učitel / pekař",
		"být u policie (pracovat)",
		"Otec byl skaut.",
		"Jsem občanem ČR.",
		"Táta nebyl ve straně.",
		"Sestra je lékařkou v krajské nemocnici.",
		"Kytovci jsou savci.",
		"Rakytník je dvoudomá rostlina.",
		"Epilepsie je onemocnění nervového systému.",
		"Její manžel je hlupák.",
		"Genetika je věda o dědičnosti.",
		"Já jsem Petr Novák.",
		"Dvě a dvě jsou čtyři.",
		"Paříž je hlavním městem módy.",
		"P  je chemická značka fosforu.", // TODO here are two spaces
		"Ukázalo se, že opak je pravdou.",
	})

	assert.Equal(t, first.Collocations[2].Collocation, "to jest")
	assert.Equal(t, first.Collocations[2].Explanation, "to znamená, zkr. tj.")
	assert.ElementsMatch(t, first.Collocations[2].Examples, [0]string{})

	// TODO here can be much more tests
}
