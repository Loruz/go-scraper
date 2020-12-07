package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/gorilla/mux"
	"github.com/mitchellh/mapstructure"
)

// Group of competion struct
type Group struct {
	Title     string     `json:"group_title"`
	SubGroups []SubGroup `json:"sub_groups"`
}

// Competition struct
type Competition struct {
	Title       string  `json:"title"`
	Description string  `json:"description"`
	Attendees   int32   `json:"attendees"`
	Groups      []Group `json:"groups"`
}

// AttendeeResult struct
type AttendeeResult struct {
	Place      string `json:"place"`
	Name       string `json:"name"`
	ID         int32  `json:"id"`
	Country    string `json:"country"`
	Club       string `json:"club"`
	FinishTime string `json:"finish_time"`
	Speed      string `json:"speed"`
}

// SubGroup struct
type SubGroup struct {
	Title   string           `json:"title"`
	Results []AttendeeResult `json:"results"`
}

// Result  struct
type Result struct {
	Competitions []Competition `json:"competitions"`
}

// Load html utility returns, document to query
func loadHTML(URL string) *goquery.Document {
	// Request the HTML page.
	res, err := http.Get(URL)
	if err != nil {
		log.Fatal(err)
	}

	defer res.Body.Close()
	if res.StatusCode != 200 {
		log.Fatalf("status code error: %d %s", res.StatusCode, res.Status)
	}

	// Load the HTML document
	doc, err := goquery.NewDocumentFromReader(res.Body)
	if err != nil {
		log.Fatal(err)
	}
	return doc
}

func parseInt(str string) int32 {
	var a int32

	if n, err := strconv.Atoi(strings.TrimSpace(str)); err == nil {
		a = int32(n)
	}
	return a
}

// Convert attendees count to number utility
func parseAttendeesCount(attendeesText string) int32 {
	i := strings.Index(attendeesText, ":")
	return parseInt(strings.TrimSpace(attendeesText[i+1:]))
}

// Extracts contender id
func extractContenderID(contenderTd *goquery.Selection) int32 {
	var ID int32
	contenderLink, exists := contenderTd.Find("a").Attr("href")
	if exists && strings.Contains(contenderLink, "dal") {
		i := strings.Index(contenderLink, "l/")
		ID = parseInt(contenderLink[i+2:])
	}
	return ID
}

// Extract sub groups in each group like M M12 V V12 etc.
func extractSubGroups(group *Group, t *goquery.Selection) {
	subGroup := SubGroup{Title: strings.Trim(t.Find("a").Text(), "\n")}
	// Request the HTML page.
	if href, exists := t.Find("a").Attr("href"); exists {
		resultDoc := loadHTML("https://dbsportas.lt/" + href)
		resultTable := resultDoc.Find(".tbl")

		resultTable.Find("tr").Each(func(i int, s *goquery.Selection) {
			if i > 0 { // Skip first `tr` because its 'th'
				result := AttendeeResult{}
				scructFieldMap := map[int]string{0: "Place", 1: "Name", 2: "Country", 3: "Club", 4: "FinishTime", 5: "Speed"}
				dataMap := make(map[string]interface{})

				s.Find("td").Each(func(j int, t *goquery.Selection) {
					if j == 1 { // Only check second `td` because its Contender
						result.ID = extractContenderID(t)
					}

					dataMap[scructFieldMap[j]] = strings.Trim(t.Text(), "\n")
				})
				mapstructure.Decode(dataMap, &result)
				subGroup.Results = append(subGroup.Results, result)
			}
		})
		group.SubGroups = append(group.SubGroups, subGroup)
	}
}

// Extract group from each event. Mostly its Male , Female
func extractGroups(URL string, results []Competition) []byte {

	doc := loadHTML(URL)

	content := doc.Find("#turinys")

	var attendees int32
	attendeesText := content.Find("h2 + b").Text()

	attendees = parseAttendeesCount(attendeesText)

	competition := Competition{
		Title:       content.Find("h1").Text(),
		Description: content.Find("small").Text(),
		Attendees:   attendees,
	}

	table := content.Find("h4 + table:first-of-type")

	table.Find("tr").Each(func(i int, s *goquery.Selection) {
		group := Group{}
		s.Find("td").Each(func(j int, t *goquery.Selection) {
			if j == 0 {
				group.Title = t.Text()
			} else {
				extractSubGroups(&group, t)
			}
		})

		competition.Groups = append(competition.Groups, group)
	})

	results = append(results, competition)

	prettyJSON, err := json.MarshalIndent(results, "", "    ")
	if err != nil {
		log.Fatal("Failed to generate json", err)
	}
	return prettyJSON
}

// Writes JSON in response
func writeJSON(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	URL := "https://dbsportas.lt/lt/varz/" + vars["id"]
	results := []Competition{}

	bytes := extractGroups(URL, results)

	w.Header().Set("Content-Type", "application/json")
	w.Write(bytes)
}

func defaultPath(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("Query params requered: Provide competition id /{id}"))
}

func main() {
	tmpl := template.Must(template.ParseFiles("example.html"))
	r := mux.NewRouter()
	r.HandleFunc("/scrape/{id}", writeJSON)
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		data := struct {
			Examples []int
		}{
			[]int{2020154, 2020153, 2020150},
		}
		tmpl.Execute(w, data)
	})

	fmt.Println("Server is running on :8888")
	err := http.ListenAndServe(":8888", r)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}

}
