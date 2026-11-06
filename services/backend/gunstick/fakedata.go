// Copyright 2026 Tomas Machalek <tomas.machalek@gmail.com>
// Copyright 2026 Department of Linguistics,
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

package gunstick

// TEMPORARY HACK
//
// The Gunstick backend (versologie.cz) has been suspended by its authors
// due to a PHP vulnerability. To keep the WaG Gunstick tile working (e.g. for
// presentations), we serve a few hardcoded fake results here.
// Once the backend is back, set `gunstickBackendSuspended` to false
// (or remove this file altogether).
//
// Note: the verses below are made up (only authors and book titles are real).

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// gunstickBackendSuspended switches the service from proxying to the real
// Gunstick backend to serving the fake data below.
const gunstickBackendSuspended = true

const gunstickOfflineMsg = "Due to a PHP vulnerability, the Gunstick API is temporarily offline. " +
	"Only a few demo words are available."

type fakeBook struct {
	Author string
	ID     string
	Name   string
	Year   int
}

var (
	bkMacha     = fakeBook{"Mácha, Karel Hynek", "201", "Máj", 1836}
	bkErben     = fakeBook{"Erben, Karel Jaromír", "202", "Kytice", 1853}
	bkHalek     = fakeBook{"Hálek, Vítězslav", "203", "Večerní písně", 1859}
	bkNeruda    = fakeBook{"Neruda, Jan", "204", "Knihy veršů", 1868}
	bkVrchlicky = fakeBook{"Vrchlický, Jaroslav", "205", "Duch a svět", 1878}
	bkCech      = fakeBook{"Čech, Svatopluk", "206", "Jitřní písně", 1883}
	bkSova      = fakeBook{"Sova, Antonín", "207", "Květy intimních nálad", 1895}
	bkBrezina   = fakeBook{"Březina, Otokar", "208", "Svítání na západě", 1897}
	bkBezruc    = fakeBook{"Bezruč, Petr", "209", "Slezské písně", 1909}
)

// gunstickFakeDataSize is a fake corpus size (in tokens) per year
var gunstickFakeDataSize = map[int]int{
	1836: 28400, 1853: 35200, 1859: 30100, 1868: 33500, 1878: 41200,
	1883: 44700, 1895: 52100, 1897: 50800, 1909: 49900,
}

// fakeRhyme represents a number of occurrences (Count) of a rhyme pair
// in a book, along with a single example couplet.
type fakeRhyme struct {
	Book     fakeBook
	PoemName string
	Count    int
	Line1    string
	Line2    string
}

type fakeWordEntry struct {
	Ending string
	// partner word => occurrences
	Rhymes map[string][]fakeRhyme
}

var gunstickFakeData = map[string]fakeWordEntry{
	"láska": {
		Ending: "áska",
		Rhymes: map[string][]fakeRhyme{
			"kráska": {
				{bkMacha, "Máj", 3, "a v duši zhasla jeho láska", "když zhasla jeho bledá kráska"},
				{bkHalek, "Ty jsi tak krásná", 7, "a v srdci zpívá stará láska", "jež kvete jako jarní kráska"},
				{bkSova, "Za tichých nocí", 4, "ó tichá, tajemná ty láska", "mé noci snivá, bledá kráska"},
			},
			"vráska": {
				{bkNeruda, "Matičce", 5, "v té tváři zbyla jenom láska", "a na čele ti sedí vráska"},
				{bkVrchlicky, "Láska", 3, "co zbude nám, když zmizí láska?", "jen v koutku oka tichá vráska"},
			},
			"páska": {
				{bkErben, "Svatební košile", 2, "ach, kde je moje drahá láska", "jen v kosách zbyla rudá páska"},
				{bkBrezina, "Láska", 2, "je věčná, svatá, tichá láska", "a duši váže zlatá páska"},
			},
		},
	},
	"noc": {
		Ending: "oc",
		Rhymes: map[string][]fakeRhyme{
			"moc": {
				{bkMacha, "Máj", 4, "byl pozdní večer, padla noc", "a nad jezerem tajná moc"},
				{bkErben, "Štědrý den", 5, "a půlnoc přišla, temná noc", "kdo zná tu jejich čarnou moc"},
				{bkVrchlicky, "Noc", 6, "nad světem tiše plyne noc", "a v hvězdách věčná, tichá moc"},
				{bkBezruc, "Noc", 3, "nad Beskydem je černá noc", "a nad námi je cizí moc"},
			},
			"pomoc": {
				{bkErben, "Svatební košile", 4, "a hrůzně plyne dlouhá noc", "ó Panno, dej mi svoji pomoc"},
				{bkCech, "Noc", 3, "kdo bdí, když kolem spí jen noc", "kdo přinese nám v bídě pomoc"},
			},
			"nemoc": {
				{bkNeruda, "Umírající", 3, "a za oknem je bledá noc", "a v hrudi tíží tichá nemoc"},
				{bkSova, "Za tichých nocí", 2, "a dlouhá, šedá, smutná noc", "jak tichá, nevyléčná nemoc"},
			},
		},
	},
	"svět": {
		Ending: "ět",
		Rhymes: map[string][]fakeRhyme{
			"květ": {
				{bkMacha, "Máj", 2, "tak krásný, tichý byl ten svět", "a na stromech se chvěl už květ"},
				{bkHalek, "Což je to divné", 6, "a jaro jde, a plane svět", "a z každé snítky voní květ"},
				{bkVrchlicky, "Duch a svět", 5, "jak širý, krásný je ten svět", "jenž rodí hvězdu, rodí květ"},
				{bkBrezina, "Svítání na západě", 3, "a nový vzejde z noci svět", "jak z rosy jitřní vzchází květ"},
			},
			"let": {
				{bkNeruda, "Jak lačné oči", 4, "jak malý je ten lidský svět", "a nad ním hvězd je věčný let"},
				{bkCech, "Jitřní píseň", 3, "a k slunci pne se celý svět", "jak ptáka volný, smělý let"},
			},
			"zpět": {
				{bkErben, "Vodník", 3, "a za ní zůstal celý svět", "a nevrátí se nikdy zpět"},
				{bkSova, "Měsíc nad řekou", 2, "tak cizí je mi náhle svět", "a čas se nevrací už zpět"},
				{bkBezruc, "Maryčka Magdónova", 2, "kdo pro nás stvořil tento svět", "kdo vrátí nám ty roky zpět"},
			},
		},
	},
	"les": {
		Ending: "es",
		Rhymes: map[string][]fakeRhyme{
			"nebes": {
				{bkMacha, "Máj", 3, "a tmavý, tichý šumí les", "a nad ním modrá klenba nebes"},
				{bkVrchlicky, "V lesích", 4, "jak chrám se tyčí starý les", "a sloupy nesou vrchol nebes"},
			},
			"dnes": {
				{bkHalek, "V přírodě", 5, "jak krásně voní jarní les", "a srdce zpívá, zpívá dnes"},
				{bkNeruda, "Kde že jsou ty pohádky", 2, "kam zmizel pohádkový les", "kde jsou ty báje? kde jsou dnes?"},
				{bkBezruc, "Ostrava", 4, "a padá starý, šedý les", "a kdo nás zastá, kdo nás dnes?"},
			},
			"ples": {
				{bkErben, "Polednice", 2, "a za vsí tiše dřímá les", "a v lese víly vedou ples"},
				{bkCech, "Lesní píseň", 3, "a jitřním světlem plane les", "a ptáci ranní slaví ples"},
			},
		},
	},
	"žal": {
		Ending: "ál",
		Rhymes: map[string][]fakeRhyme{
			"dál": {
				{bkMacha, "Máj", 5, "a v srdci tichý, hluboký žal", "a čas se valí dál a dál"},
				{bkHalek, "Večerní píseň", 3, "co z lásky zbývá? jenom žal", "a život plyne dál a dál"},
				{bkSova, "Za tichých nocí", 4, "a v duši tichne starý žal", "a cesta vede kamsi dál"},
				{bkBezruc, "Slezské písně", 5, "a v prsou zbyl nám jenom žal", "a pán nás žene dál a dál"},
			},
			"stál": {
				{bkErben, "Vrba", 3, "a v očích zrál mu těžký žal", "když nad tou vrbou tiše stál"},
				{bkNeruda, "Matičce", 2, "a v hrudi mi se zvedal žal", "když nad tvým hrobem jsem tu stál"},
			},
			"král": {
				{bkVrchlicky, "Hlasy v poušti", 3, "a v sále vládl tichý žal", "když sám a smutný seděl král"},
				{bkBrezina, "Ruce", 2, "a nad světem jak věčný žal", "tu trůní temný, tichý král"},
			},
		},
	},
	"srdce": {
		Ending: "dce",
		Rhymes: map[string][]fakeRhyme{
			"Hradec": {
				{bkHalek, "Mé srdce zpívá", 3, "tam zůstalo mé mladé srdce", "kde stojí věže Hradce"},
				{bkNeruda, "Matičce", 2, "a v dálce tluče věrné srdce", "jak zvony z dálného Hradce"},
				{bkCech, "Srdce", 4, "kde zvoní zvon, tam bije srdce", "nad bránou starého Hradce"},
			},
			"vrbce": {
				{bkErben, "Vrba", 2, "a v kmeni tiše bije srdce", "jak v štíhlé, plačící vrbce"},
				{bkSova, "Srdce jak píseň", 3, "a smutné, tiché moje srdce", "se chvěje jako list na vrbce"},
			},
		},
	},
	"měsíc": {
		Ending: "ěsíc",
		Rhymes: map[string][]fakeRhyme{
			"tisíc": {
				{bkMacha, "Máj", 4, "a nad jezerem bledý měsíc", "a hvězd se třpytí na tisíc"},
				{bkErben, "Svatební košile", 3, "a z mraků vyšel bledý měsíc", "a mrtvých stínů bylo tisíc"},
				{bkNeruda, "Měsíček", 5, "jak stříbrný se dívá měsíc", "na lidských osudů tisíc"},
				{bkSova, "Měsíc nad řekou", 3, "nad řekou plane tichý měsíc", "a vlnek blýská se tisíc"},
			},
			"visíc": {
				{bkVrchlicky, "Za měsíčné noci", 2, "nad strání bledne zlatý měsíc", "nad lesem jako lampa visíc"},
				{bkBrezina, "Měsíční svit", 2, "a v prázdnu plane tichý měsíc", "jak slza v nekonečnu visíc"},
			},
		},
	},
	"smrt": {
		Ending: "rt",
		Rhymes: map[string][]fakeRhyme{
			"chrt": {
				{bkErben, "Vodník", 2, "a za ní kráčí tichá smrt", "a za ní běží věrný chrt"},
				{bkCech, "Smrt", 3, "a v polích číhá bledá smrt", "a lovce sleduje jen chrt"},
			},
			"drť": {
				{bkNeruda, "Umírající", 3, "a za dveřmi už čeká smrt", "a z kostí zbude jenom drť"},
				{bkBrezina, "Smrt", 4, "a tiše přichází k nám smrt", "a tělo v prach se mění, v drť"},
				{bkBezruc, "Maryčka Magdónova", 5, "a v šachtě na nás čeká smrt", "a z těla zbude černá drť"},
			},
		},
	},
	"sláva": {
		Ending: "áva",
		Rhymes: map[string][]fakeRhyme{
			"hlava": {
				{bkErben, "Kytice", 2, "co zbylo z dávných časů? sláva", "a nad ní šedá, stará hlava"},
				{bkVrchlicky, "Sláva", 5, "jak prchavá je lidská sláva", "a bílou je už moje hlava"},
				{bkBezruc, "Slezské písně", 3, "kde zůstala ta naše sláva?", "jen sklání se nám k zemi hlava"},
			},
			"tráva": {
				{bkHalek, "V přírodě", 3, "co je mi zlato, co mi sláva", "když kolem voní jarní tráva"},
				{bkSova, "Vltava", 2, "a zapadá v nás dávná sláva", "a nad hroby nám roste tráva"},
			},
			"Vltava": {
				{bkCech, "Píseň otroka", 6, "ó vlasti má, ty naše sláva", "kde tiše šumí stará Vltava"},
				{bkNeruda, "Romance o Karlu IV.", 4, "a v Praze zněla královská sláva", "a pod hradem se třpytila Vltava"},
			},
		},
	},
}

// gunstickRawTableItem mirrors `RawDataTableItem` from WaG's gunstick tile
type gunstickRawTableItem struct {
	Author   string `json:"author"`
	PoemName string `json:"poemName"`
	BookID   string `json:"bookId"`
	BookName string `json:"bookName"`
	Year     string `json:"year"`
	Line1    string `json:"line1"`
	Line2    string `json:"line2"`
	Ending   string `json:"ending"`
}

type gunstickResponse struct {
	Count    int                               `json:"count"`
	CountRY  map[string]map[string]string      `json:"countRY"`
	DataSize map[string]string                 `json:"dataSize"`
	Table    map[string][]gunstickRawTableItem `json:"table"`
}

func mkFakeGunstickResponse(entry fakeWordEntry) gunstickResponse {
	ans := gunstickResponse{
		CountRY:  make(map[string]map[string]string),
		DataSize: make(map[string]string),
		Table:    make(map[string][]gunstickRawTableItem),
	}
	for partner, rhymes := range entry.Rhymes {
		countY := make(map[int]int)
		for _, r := range rhymes {
			ans.Count += r.Count
			countY[r.Book.Year] += r.Count
			ans.DataSize[strconv.Itoa(r.Book.Year)] = strconv.Itoa(gunstickFakeDataSize[r.Book.Year])
			ans.Table[partner] = append(ans.Table[partner], gunstickRawTableItem{
				Author:   r.Book.Author,
				PoemName: r.PoemName,
				BookID:   r.Book.ID,
				BookName: r.Book.Name,
				Year:     strconv.Itoa(r.Book.Year),
				Line1:    r.Line1,
				Line2:    r.Line2,
				Ending:   entry.Ending,
			})
		}
		ans.CountRY[partner] = make(map[string]string, len(countY))
		for y, c := range countY {
			ans.CountRY[partner][strconv.Itoa(y)] = strconv.Itoa(c)
		}
	}
	return ans
}

// FakeDataHandler serves hardcoded Gunstick results for a few words
// and an error for any other word.
func FakeDataHandler(ctx *gin.Context) {
	word := strings.ToLower(strings.TrimSpace(ctx.Query("q")))
	entry, ok := gunstickFakeData[word]
	if !ok {
		log.Warn().
			Str("word", word).
			Msg("Gunstick backend suspended and no fake data available for the word")
		uniresp.RespondWithErrorJSON(
			ctx, fmt.Errorf("%s", gunstickOfflineMsg), http.StatusServiceUnavailable)
		return
	}
	ctx.JSON(http.StatusOK, mkFakeGunstickResponse(entry))
}
