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

package hex

// TEMPORARY HACK
//
// The Hex backend (versologie.cz) has been suspended by its authors
// due to a PHP vulnerability. To keep the WaG Hex tile working (e.g. for
// presentations), we serve a few hardcoded fake results here.
// Once the backend is back, set `hexBackendSuspended` to false
// (or remove this file altogether).

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"github.com/czcorpus/cnc-gokit/uniresp"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog/log"
)

// hexBackendSuspended switches the service from proxying to the real
// Hex backend to serving the fake data below.
const hexBackendSuspended = true

const hexOfflineMsg = "Due to a PHP vulnerability, the Hex API is temporarily offline. " +
	"Only a few demo words are available."

type fakePoemHit struct {
	Author   string
	BookID   int
	BookName string
	PoemID   int
	PoemName string
	Year     int
	AF       int     // absolute freq. of the word in the poem
	Phi      float64 // keyness/association score
}

// hexFakeCorpusSize is a fake size of the corpus per year
// (lemmas, lines, poems)
var hexFakeCorpusSize = map[int][3]int{
	1836: {28400, 4100, 120},
	1853: {35200, 5300, 160},
	1858: {31800, 4700, 190},
	1859: {30100, 4500, 210},
	1862: {29700, 4400, 175},
	1868: {33500, 4900, 205},
	1872: {38900, 5600, 230},
	1878: {41200, 6000, 260},
	1883: {44700, 6400, 280},
	1888: {47300, 6800, 310},
	1895: {52100, 7400, 340},
	1897: {50800, 7200, 330},
	1903: {54600, 7800, 350},
	1909: {49900, 7100, 300},
}

var hexFakeData = map[string][]fakePoemHit{
	"láska": {
		{"Mácha, Karel Hynek", 101, "Máj", 1011, "Máj", 1836, 9, 0.412},
		{"Hálek, Vítězslav", 102, "Večerní písně", 1021, "Ty jsi tak krásná", 1859, 6, 0.385},
		{"Hálek, Vítězslav", 102, "Večerní písně", 1022, "Což je to divné", 1859, 4, 0.311},
		{"Neruda, Jan", 103, "Knihy veršů", 1031, "Matičce", 1868, 3, 0.204},
		{"Vrchlický, Jaroslav", 104, "Eklogy a písně", 1041, "Jsi jako píseň", 1872, 5, 0.297},
		{"Vrchlický, Jaroslav", 105, "Duch a svět", 1051, "Láska", 1878, 7, 0.356},
		{"Sova, Antonín", 106, "Květy intimních nálad", 1061, "Za tichých nocí", 1895, 4, 0.233},
		{"Březina, Otokar", 107, "Svítání na západě", 1071, "Láska", 1897, 3, 0.189},
	},
	"srdce": {
		{"Mácha, Karel Hynek", 101, "Máj", 1011, "Máj", 1836, 5, 0.268},
		{"Erben, Karel Jaromír", 108, "Kytice", 1081, "Svatební košile", 1853, 4, 0.221},
		{"Hálek, Vítězslav", 102, "Večerní písně", 1023, "Mé srdce zpívá", 1859, 6, 0.344},
		{"Neruda, Jan", 109, "Písně kosmické", 1091, "Jak lačné oči", 1878, 3, 0.176},
		{"Čech, Svatopluk", 110, "Jitřní písně", 1101, "Srdce", 1883, 5, 0.287},
		{"Vrchlický, Jaroslav", 111, "Hlasy v poušti", 1111, "Mé srdce", 1888, 4, 0.243},
		{"Sova, Antonín", 112, "Ještě jednou se vrátíme", 1121, "Srdce jak píseň", 1903, 3, 0.162},
	},
	"noc": {
		{"Mácha, Karel Hynek", 101, "Máj", 1011, "Máj", 1836, 8, 0.437},
		{"Erben, Karel Jaromír", 108, "Kytice", 1082, "Štědrý den", 1853, 5, 0.302},
		{"Erben, Karel Jaromír", 108, "Kytice", 1081, "Svatební košile", 1853, 6, 0.329},
		{"Neruda, Jan", 109, "Písně kosmické", 1092, "Kde že jsou ty pohádky", 1878, 4, 0.255},
		{"Vrchlický, Jaroslav", 111, "Hlasy v poušti", 1112, "Noc na Karlštejně", 1888, 5, 0.281},
		{"Sova, Antonín", 106, "Květy intimních nálad", 1061, "Za tichých nocí", 1895, 7, 0.364},
		{"Bezruč, Petr", 113, "Slezské písně", 1131, "Noc", 1909, 3, 0.171},
	},
	"měsíc": {
		{"Mácha, Karel Hynek", 101, "Máj", 1011, "Máj", 1836, 4, 0.318},
		{"Erben, Karel Jaromír", 108, "Kytice", 1081, "Svatební košile", 1853, 5, 0.354},
		{"Neruda, Jan", 109, "Písně kosmické", 1093, "Měsíček", 1878, 6, 0.402},
		{"Vrchlický, Jaroslav", 104, "Eklogy a písně", 1042, "Za měsíčné noci", 1872, 3, 0.214},
		{"Sova, Antonín", 106, "Květy intimních nálad", 1062, "Měsíc nad řekou", 1895, 4, 0.276},
		{"Březina, Otokar", 114, "Ruce", 1141, "Měsíční svit", 1903, 2, 0.143},
	},
	"smrt": {
		{"Mácha, Karel Hynek", 101, "Máj", 1011, "Máj", 1836, 6, 0.371},
		{"Erben, Karel Jaromír", 108, "Kytice", 1083, "Vodník", 1853, 3, 0.198},
		{"Erben, Karel Jaromír", 108, "Kytice", 1084, "Vrba", 1853, 4, 0.236},
		{"Neruda, Jan", 115, "Hřbitovní kvítí", 1151, "Hřbitovní kvítí", 1858, 7, 0.425},
		{"Neruda, Jan", 103, "Knihy veršů", 1032, "Umírající", 1868, 3, 0.187},
		{"Čech, Svatopluk", 110, "Jitřní písně", 1102, "Smrt", 1883, 4, 0.249},
		{"Březina, Otokar", 107, "Svítání na západě", 1072, "Smrt", 1897, 5, 0.312},
		{"Bezruč, Petr", 113, "Slezské písně", 1132, "Maryčka Magdónova", 1909, 6, 0.341},
	},
	"svět": {
		{"Mácha, Karel Hynek", 101, "Máj", 1011, "Máj", 1836, 5, 0.297},
		{"Hálek, Vítězslav", 102, "Večerní písně", 1022, "Což je to divné", 1859, 4, 0.226},
		{"Neruda, Jan", 109, "Písně kosmické", 1091, "Jak lačné oči", 1878, 8, 0.418},
		{"Vrchlický, Jaroslav", 105, "Duch a svět", 1052, "Duch a svět", 1878, 6, 0.383},
		{"Čech, Svatopluk", 110, "Jitřní písně", 1103, "Jitřní píseň", 1883, 4, 0.241},
		{"Březina, Otokar", 107, "Svítání na západě", 1073, "Svítání", 1897, 5, 0.322},
		{"Bezruč, Petr", 113, "Slezské písně", 1133, "Ostrava", 1909, 3, 0.168},
	},
	"les": {
		{"Mácha, Karel Hynek", 101, "Máj", 1011, "Máj", 1836, 4, 0.284},
		{"Erben, Karel Jaromír", 108, "Kytice", 1085, "Polednice", 1853, 3, 0.207},
		{"Hálek, Vítězslav", 116, "V přírodě", 1161, "V přírodě", 1872, 7, 0.431},
		{"Vrchlický, Jaroslav", 111, "Hlasy v poušti", 1113, "V lesích", 1888, 5, 0.316},
		{"Čech, Svatopluk", 110, "Jitřní písně", 1104, "Lesní píseň", 1883, 4, 0.262},
		{"Bezruč, Petr", 113, "Slezské písně", 1133, "Ostrava", 1909, 4, 0.239},
	},
	"žal": {
		{"Mácha, Karel Hynek", 101, "Máj", 1011, "Máj", 1836, 7, 0.448},
		{"Erben, Karel Jaromír", 108, "Kytice", 1084, "Vrba", 1853, 3, 0.214},
		{"Hálek, Vítězslav", 102, "Večerní písně", 1024, "Večerní píseň", 1859, 4, 0.259},
		{"Neruda, Jan", 103, "Knihy veršů", 1031, "Matičce", 1868, 3, 0.193},
		{"Vrchlický, Jaroslav", 111, "Hlasy v poušti", 1114, "Hlasy v poušti", 1888, 4, 0.247},
		{"Sova, Antonín", 106, "Květy intimních nálad", 1061, "Za tichých nocí", 1895, 5, 0.331},
		{"Bezruč, Petr", 113, "Slezské písně", 1134, "Slezské písně", 1909, 6, 0.376},
	},
	"sláva": {
		{"Erben, Karel Jaromír", 108, "Kytice", 1086, "Kytice", 1853, 3, 0.231},
		{"Neruda, Jan", 117, "Balady a romance", 1171, "Romance o Karlu IV.", 1883, 6, 0.402},
		{"Vrchlický, Jaroslav", 111, "Hlasy v poušti", 1115, "Sláva", 1888, 5, 0.347},
		{"Čech, Svatopluk", 118, "Písně otroka", 1181, "Píseň otroka", 1895, 7, 0.455},
		{"Sova, Antonín", 112, "Ještě jednou se vrátíme", 1122, "Vltava", 1903, 3, 0.188},
		{"Bezruč, Petr", 113, "Slezské písně", 1134, "Slezské písně", 1909, 4, 0.263},
	},
}

// hexRawTableItem mirrors `RawDataTableItem` from WaG's hex tile
// (all values are encoded as strings, just like the original PHP backend does).
type hexRawTableItem struct {
	AF       string `json:"af"`
	Author   string `json:"author"`
	BookID   string `json:"bookId"`
	BookName string `json:"bookName"`
	Phi      string `json:"phi"`
	PoemID   string `json:"poemId"`
	PoemName string `json:"poemName"`
	RF       string `json:"rf"`
	Year     string `json:"year"`
}

type hexSize struct {
	Lemma map[int]string `json:"lemma"`
	Line  map[int]string `json:"line"`
	Poem  map[int]string `json:"poem"`
}

type hexSorting struct {
	Author   map[string]int `json:"author"`
	BookName map[string]int `json:"bookName"`
	PoemName map[string]int `json:"poemName"`
}

type hexResponse struct {
	Count   int               `json:"count"`
	Size    hexSize           `json:"size"`
	CountY  map[int]int       `json:"countY"`
	Table   []hexRawTableItem `json:"table"`
	Sorting hexSorting        `json:"sorting"`
}

func mkSortingIndex(values []string) map[string]int {
	uniq := make(map[string]struct{})
	for _, v := range values {
		uniq[v] = struct{}{}
	}
	sorted := make([]string, 0, len(uniq))
	for v := range uniq {
		sorted = append(sorted, v)
	}
	sort.Strings(sorted)
	ans := make(map[string]int, len(sorted))
	for i, v := range sorted {
		ans[v] = i
	}
	return ans
}

func mkFakeHexResponse(hits []fakePoemHit) hexResponse {
	ans := hexResponse{
		Size: hexSize{
			Lemma: make(map[int]string),
			Line:  make(map[int]string),
			Poem:  make(map[int]string),
		},
		CountY: make(map[int]int),
		Table:  make([]hexRawTableItem, 0, len(hits)),
	}
	authors := make([]string, 0, len(hits))
	books := make([]string, 0, len(hits))
	poems := make([]string, 0, len(hits))
	for _, hit := range hits {
		ans.Count += hit.AF
		ans.CountY[hit.Year] += hit.AF
		size := hexFakeCorpusSize[hit.Year]
		ans.Size.Lemma[hit.Year] = strconv.Itoa(size[0])
		ans.Size.Line[hit.Year] = strconv.Itoa(size[1])
		ans.Size.Poem[hit.Year] = strconv.Itoa(size[2])
		// relative freq. per million lemmas of the respective year
		var rf int
		if size[0] > 0 {
			rf = hit.AF * 1000000 / size[0]
		}
		ans.Table = append(ans.Table, hexRawTableItem{
			AF:       strconv.Itoa(hit.AF),
			Author:   hit.Author,
			BookID:   strconv.Itoa(hit.BookID),
			BookName: hit.BookName,
			Phi:      strconv.FormatFloat(hit.Phi, 'f', 3, 64),
			PoemID:   strconv.Itoa(hit.PoemID),
			PoemName: hit.PoemName,
			RF:       strconv.Itoa(rf),
			Year:     strconv.Itoa(hit.Year),
		})
		authors = append(authors, hit.Author)
		books = append(books, hit.BookName)
		poems = append(poems, hit.PoemName)
	}
	ans.Sorting = hexSorting{
		Author:   mkSortingIndex(authors),
		BookName: mkSortingIndex(books),
		PoemName: mkSortingIndex(poems),
	}
	return ans
}

// FakeDataHandler serves hardcoded Hex results for a few words
// and an error for any other word. The output format mimics
// the original Hex backend (after being processed by `Interceptor`),
// i.e. a JS snippet `var hex = {...};`.
func FakeDataHandler(ctx *gin.Context) {
	word := strings.ToLower(strings.TrimSpace(ctx.Query("q")))
	hits, ok := hexFakeData[word]
	if !ok {
		log.Warn().
			Str("word", word).
			Msg("Hex backend suspended and no fake data available for the word")
		// In streaming mode, WaG requests Hex data as base64-encoded
		// text, so the status code and the `error` key are not checked
		// by WaG's streaming layer. Older WaG builds would then crash on
		// missing fields (`resp.size.lemma`). So we send a valid empty
		// result along with the `error` key: newer WaG (with the error
		// check in hex/api.ts) shows the message, older ones show "no data".
		ctx.JSON(
			http.StatusServiceUnavailable,
			struct {
				Error string `json:"error"`
				Code  int    `json:"code"`
				hexResponse
			}{
				Error:       hexOfflineMsg,
				Code:        http.StatusServiceUnavailable,
				hexResponse: mkFakeHexResponse(nil),
			},
		)
		return
	}
	data, err := json.Marshal(mkFakeHexResponse(hits))
	if err != nil {
		uniresp.RespondWithErrorJSON(ctx, err, http.StatusInternalServerError)
		return
	}
	// note: WaG extracts data using /var hex = (\{.+\});/ so the JSON
	// must stay on a single line (which json.Marshal guarantees)
	ctx.Data(
		http.StatusOK,
		"text/html; charset=utf-8",
		[]byte(fmt.Sprintf("<script>\nvar hex = %s;\n</script>\n", data)),
	)
}
