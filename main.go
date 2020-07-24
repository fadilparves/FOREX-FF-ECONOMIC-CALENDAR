package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/carlescere/scheduler"
	"github.com/gorilla/mux"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"golang.org/x/net/html/charset"
)

type Events struct {
	Name     string `json:"title"`
	Country  string `json:"country"`
	Date     string `json:"date"`
	Time     string `json:"time"`
	Impact   string `json:"impact"`
	Forecast string `json:"forecast"`
	Previous string `json:"previous"`
}

type TodayEvents struct {
	Name     string `json:"title"`
	Country  string `json:"country"`
	Date     string `json:"date"`
	Time     string `json:"time"`
	Impact   string `json:"impact"`
	Forecast string `json:"forecast"`
	Previous string `json:"previous"`
}

type WeeklyEvents struct {
	XMLName xml.Name `xml:"weeklyevents"`
	Event   []Event  `xml:"event"`
}

type Event struct {
	XMLName  xml.Name `xml:"event"`
	Name     string   `xml:"title"`
	Country  string   `xml:"country"`
	Date     Date     `xml:"date"`
	Time     Time     `xml:"time"`
	Impact   Impact   `xml:"impact"`
	Forecast Forecast `xml:"forecast"`
	Previous Previous `xml:"previous"`
}

type Date struct {
	XMLName xml.Name `xml:"date"`
	Date    string   `xml:",cdata"`
}

type Time struct {
	XMLName xml.Name `xml:"time"`
	Time    string   `xml:",cdata"`
}

type Impact struct {
	XMLName xml.Name `xml:"impact"`
	Impact  string   `xml:",cdata"`
}

type Forecast struct {
	XMLName  xml.Name `xml:"forecast"`
	Forecast string   `xml:",cdata"`
}

type Previous struct {
	XMLName  xml.Name `xml:"previous"`
	Previous string   `xml:",cdata"`
}

func dbConnect(dbType, connectionURI string) *gorm.DB {
	db, err := gorm.Open(dbType, connectionURI)
	if err != nil {
		fmt.Println(err)
	}

	return db
}

func fetchURL(url string) []byte {
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalf("unable to GET '%s': %s", url, err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatalf("unable to read body '%s': %s", url, err)
	}
	return body
}

func parseXML(xmlDoc []byte, target interface{}) {
	reader := bytes.NewReader(xmlDoc)
	decoder := xml.NewDecoder(reader)
	decoder.CharsetReader = charset.NewReaderLabel
	if err := decoder.Decode(target); err != nil {
		log.Fatalf("unable to parse XML '%s':\n%s", err, xmlDoc)
	}
}

func storeData(db *gorm.DB, title, country, date, time, impact, forecast, previous string) (string, error) {
	err := db.Create(&Events{Name: title, Country: strings.ToLower(country), Time: time, Date: date, Impact: strings.ToLower(impact), Forecast: forecast, Previous: previous})
	if err != nil {
		return "", nil
	}
	return "Data saved", nil
}

func GetTimeDate() (string, string) {
	loc, err := time.LoadLocation("Asia/Kuala_Lumpur")
	if err != nil {
		panic(err)
	}
	t := time.Now().In(loc)
	current_date := t.Format("01-02-2006")
	current_time := t.Format(time.Kitchen)
	current_time = strings.ToLower(current_time)
	return current_date, current_time
}

func pullnstore() {
	db := dbConnect("postgres", "host= user= dbname= sslmode=disable password=")
	db.AutoMigrate(Events{})
	var events WeeklyEvents
	xmlDoc := fetchURL("https://www.forexfactory.com/ffcal_week_this.xml")
	parseXML(xmlDoc, &events)

	for _, d := range events.Event {
		if strings.Contains(d.Name, "Meeting") || strings.Contains(d.Name, "Speaks") || strings.Contains(d.Name, "Statement") || strings.Contains(d.Name, "Conference") || strings.Contains(d.Name, "Assessment") {
			continue
		} else {
			storeData(db, d.Name, d.Country, d.Date.Date, d.Time.Time, d.Impact.Impact, d.Forecast.Forecast, d.Previous.Previous)
		}
	}
}

func ServiceGetFilteredData(impact string, country string, date string) []Events {
	db := dbConnect("", "host= user= dbname= sslmode=disable password=")
	var events []Events
	if impact != "" {
		db.Raw("SELECT * FROM events WHERE impact = ?", impact).Scan(&events)
	}
	if country != "" {
		db.Raw("SELECT * FROM events WHERE country = ?", country).Scan(&events)
	}
	if date != "" {
		db.Raw("SELECT * FROM events WHERE date = ?", date).Scan(&events)
	}
	return events
}

func TodayData() []Events {
	db := dbConnect("", "host= user= dbname= sslmode=disable password=")
	var events []Events
	curr_date, _ := GetTimeDate()
	db.Raw("SELECT * FROM events WHERE impact = ? AND date = ?", "high", curr_date).Scan(&events)
	return events
}

func GetNewsFilteredByImpact(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	events := ServiceGetFilteredData(params["impact"], "", "")
	json.NewEncoder(w).Encode(events)
}

func GetNewsFilteredByCountry(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	events := ServiceGetFilteredData("", params["country"], "")
	json.NewEncoder(w).Encode(events)
}

func GetNewsFilteredByDate(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	events := ServiceGetFilteredData("", "", params["date"])
	json.NewEncoder(w).Encode(events)
}

func GetTodayNews(w http.ResponseWriter, r *http.Request) {
	events := TodayData()
	json.NewEncoder(w).Encode(events)
}

func TodayNews(date string) []Events {
	db := dbConnect("", "host= user= dbname= sslmode=disable password=")
	var events []Events
	db.Raw("SELECT * FROM events WHERE date = ? AND impact = ?", date, "high").Scan(&events)
	for _, v := range events {
		db.Create(&TodayEvents{Name: v.Name, Country: strings.ToLower(v.Country), Time: v.Time, Date: v.Date, Impact: strings.ToLower(v.Impact), Forecast: v.Forecast, Previous: v.Previous})
	}
	return events
}

func ClearTodayEvents() {
	db := dbConnect("", "host= user= dbname= sslmode=disable password=")
	db.Raw("TRUNCATE ONLY today_events RESTART IDENTITY")
}

func today() {
	curr_date, _ := GetTimeDate()
	ClearTodayEvents()
	TodayNews(curr_date)
}

func main() {
	scheduler.Every().Monday().At("05:00").Run(pullnstore)
	scheduler.Every().Day().Run(today)
	router := mux.NewRouter()
	router.HandleFunc("/economic/calendar/impact/{impact}", GetNewsFilteredByImpact).Methods("GET")
	router.HandleFunc("/economic/calendar/country/{country}", GetNewsFilteredByCountry).Methods("GET")
	router.HandleFunc("/economic/calendar/date/{date}", GetNewsFilteredByDate).Methods("GET")
	router.HandleFunc("/economic/today/news", GetTodayNews).Methods("GET")
	fmt.Println("Starting server at port 8080")
	log.Fatal(http.ListenAndServe(":8080", router))
}
