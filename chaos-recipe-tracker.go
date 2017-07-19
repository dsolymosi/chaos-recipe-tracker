package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/spf13/viper"
)

// Data type to sort maps by value
type SortablePair struct {
	Key   int
	Value int
}
type SortablePairList []SortablePair

// funcs needed to implement sort.Sort
func (s SortablePairList) Swap(i, j int) {
	s[i], s[j] = s[j], s[i]
}
func (s SortablePairList) Len() int {
	return len(s)
}
func (s SortablePairList) Less(i, j int) bool {
	return s[i].Value < s[j].Value
}

// sort map by value
func sortMap(m map[int]int) SortablePairList {
	s := make(SortablePairList, len(m))
	i := 0
	for k, v := range m {
		s[i] = SortablePair{k, v}
		i++
	}
	sort.Sort(s)
	return s
}

type farmer struct {
	//stash id -> type -> number
	c map[string]map[int]int
}

//name of stash tab
var stashName string

//map from account name to w.e
var farmerMap map[string]*farmer

//slice of names of farmers for display purposes
var farmerSlice []string
var farmerSliceMux sync.RWMutex

var currentLeague string
var changeId string

func indexHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, `<!DOCTYPE html>
<html> 
<body> 
 
<h1> Chaos Recipe Tracker </h1> 
 
<div id="counts"> 
  Loading... 
</div> 
<br>

<div id="users">
<button onclick="add()">Add farmer</button>
<button onclick="del()">Remove farmer</button>
</div>
 
<script> 
function add() {
  var farmer = prompt("Please enter farmer's account name:");
  if (farmer != null && farmer != ""){
    var xhttp = new XMLHttpRequest();
    xhttp.open("GET", "/add/"+farmer, true); 
    xhttp.send();
  }
}

function del() {
  var farmer = prompt("Please enter farmer's account name:");
  if (farmer != null && farmer != ""){
    var xhttp = new XMLHttpRequest();
    xhttp.open("GET", "/del/"+farmer, true); 
    xhttp.send();
  }
}

function loadCount() { 
  var xhttp = new XMLHttpRequest(); 
  xhttp.onreadystatechange = function() { 
    if (this.readyState == 4 && this.status == 200) { 
      document.getElementById("counts").innerHTML = 
      this.responseText; 
    } 
  }; 
  xhttp.open("GET", "/count", true); 
  xhttp.send(); 
} 
 
setInterval(loadCount, 5000) 
</script> 
 
</body> 
</html> 
`)
}

func addHandler(w http.ResponseWriter, r *http.Request) {
	//remove '/add/' from url and add
	addFarmer(r.URL.Path[5:])
}

func delHandler(w http.ResponseWriter, r *http.Request) {
	//remove '/add/' from url and add
	delFarmer(r.URL.Path[5:])
}

var countPage string
var countMux sync.RWMutex

func countHandler(w http.ResponseWriter, r *http.Request) {
	// check mutex
	countMux.RLock()
	fmt.Fprint(w, countPage)
	countMux.RUnlock()
}

func iconToType(in string) int {
	//this is an awful hack but i don't want regex or anything slow
	//example : "http:\/\/web.poecdn.com\/image\/Art\/2DItems\/Amulets\/TurquoiseAmulet.png?..."
	// "http://web.poecdn.com/image/Art/2DItems/Armours/Hel"
	if len(in) < 50 {
		return 0
	}
	switch in[40:43] {
	case "Amu":
		return 1
	case "Rin":
		return 2
	case "Bel":
		return 3
	case "Wea":
		return 8
	case "Arm":
		switch in[48:51] {
		case "Hel":
			return 4
		case "Bod":
			return 5
		case "Glo":
			return 6
		case "Boo":
			return 7
		default:
			return 0
		}

	default:
		return 0
	}
}

func typeToName(in int) string {
	//TODO: don't hardcode colours, maybe move them to config
	switch in {
	case 1:
		return "<td bgcolor=\"#ff5b00\">Amulets</td>"
	case 2:
		return "<td bgcolor=\"#580056\">Ring Pairs</td>"
	case 3:
		return "<td bgcolor=\"#628000\">Belts</td>"
	case 4:
		return "<td bgcolor=\"#bf0000\">Helmets</td>"
	case 5:
		return "<td bgcolor=\"#0000ff\">Body Armours</td>"
	case 6:
		return "<td bgcolor=\"#feaa00\">Gloves</td>"
	case 7:
		return "<td bgcolor=\"#900053\">Boots</td>"
	case 8:
		return "<td bgcolor=\"#00bf00\">Weapon Pairs</td>"
	default:
		return "Unknown"
	}
}

// BEGIN json structs
type itemApi struct {
	Ilvl int
	//icon used to tell type of item
	Icon   string
	League string
	//we want frameType 2
	FrameType int
}

type stashApi struct {
	AccountName string
	Id          string
	//only look at stashes with 99exa b/o?  							-FEATURE1-
	Stash string
	Items []itemApi
}

type errStruct struct {
	message string
}

type gggApi struct {
	Next_change_id string
	Error          errStruct
	Stashes        []stashApi
}

// END json structs

func apiInteraction() {
	// download json
	resp, err := http.Get("http://api.pathofexile.com/public-stash-tabs?id=" + changeId)
	if err != nil {
		fmt.Println("Error getting info:", err)
	} else {
		defer resp.Body.Close()
		jsonApi, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			fmt.Println("Error getting info:", err)
		} else {

			// parse json
			var apiRes gggApi
			if err := json.Unmarshal(jsonApi, &apiRes); err != nil {
				fmt.Println("Error unmarshaling json:", err)
			} else {
				if apiRes.Error.message != "" {
					time.Sleep(time.Second * time.Duration(10))
				} else {
					// for each farmer.ign from farmerMap found in json, update farmer.c
					for _, stashRes := range apiRes.Stashes {
						// check if true gamer
						// check stash name
						// i.e. stashRes.Stash == "~b\/o 99 exa"
						if farmerMap[stashRes.AccountName] != nil && len(stashRes.Stash) >= len(stashName) && stashRes.Stash[:len(stashName)] == stashName {
							// check if stash has been init, or is nil
							if farmerMap[stashRes.AccountName].c[stashRes.Id] == nil {
								farmerMap[stashRes.AccountName].c[stashRes.Id] = make(map[int]int)
							}
							fmt.Print("Found update.")
							// stash updated, reset counts to 0
							for i := 0; i < 9; i++ {
								farmerMap[stashRes.AccountName].c[stashRes.Id][i] = 0
							}
							for _, itemRes := range stashRes.Items {
								fmt.Print(".")
								//add everything up
								if itemRes.Ilvl >= 60 && itemRes.League == currentLeague && itemRes.FrameType == 2 {
									//update farmerMap[stashRes.accountName].c[stashRes.id][-TYPE-] count for each type
									farmerMap[stashRes.AccountName].c[stashRes.Id][iconToType(itemRes.Icon)]++
								}
							}
							fmt.Println(".done!")
						}

					}
					// update changeId we received
					changeId = apiRes.Next_change_id
				}
			}
		}
	}
}

func addFarmer(accName string) {
	//fmt.Println("adding " + accName)
	farmerMap[accName] = &farmer{}
	farmerMap[accName].c = make(map[string]map[int]int)

	farmerSliceMux.Lock()
	farmerSlice = append(farmerSlice, accName)
	farmerSliceMux.Unlock()
}

func delFarmer(accName string) {
	//fmt.Println("removing " + accName)
	if farmerMap[accName] != nil {
		countMux.Lock()
		delete(farmerMap, accName)
		countMux.Unlock()

		//find user in slice
		i := 0
		farmerSliceMux.RLock()
		for ; i < len(farmerSlice) && farmerSlice[i] != accName; i++ {
		}
		farmerSliceMux.RUnlock()

		farmerSliceMux.Lock()
		farmerSlice = append(farmerSlice[:i], farmerSlice[i+1:]...)
		farmerSliceMux.Unlock()
	}
}

func main() {

	viper.SetConfigName("chaos-config")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		fmt.Println("Configuration file error: ", err)
		fmt.Println("Continuing using defaults..")
	} else {
		fmt.Println("Configuration file successfully read!")
	}

	viper.SetDefault("sleeptime", 800)
	viper.SetDefault("league", "2 Week Mayhem HC Solo (JRE093)")
	// TODO: check poe.ninja/stats to get more fresh changeId
	viper.SetDefault("changeid", "70616293-74324832-69546610-80840272-75180862")
	viper.SetDefault("defaultuser", "coldie48")

	viper.SetDefault("stashname", "chaos")

	// INIT
	pauseTime := time.Millisecond * time.Duration(viper.GetInt("sleeptime"))
	currentLeague = viper.GetString("league")
	changeId = viper.GetString("changeid")
	stashName = viper.GetString("stashname")

	// create
	farmerMap = make(map[string]*farmer)
	addFarmer(viper.GetString("defaultuser"))

	// register pages
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/count", countHandler)
	http.HandleFunc("/add/", addHandler)
	http.HandleFunc("/del/", delHandler)
	go http.ListenAndServe(":8080", nil)

	fmt.Println("Successfully launched! Make your tabs public and name them \"" + stashName + "\". Items might not show up until you place a new item in the tab! If you did not update changeid in the config, they might never show up.")
	fmt.Println("Navigate to http://localhost:8080/ to view your chaos recipe numbers!")
	fmt.Println("Sorry about the colours, they match my filter, appropriate section: https://pastebin.com/cGM7CjSH")

	for { //loop
		countMux.Lock()
		// check api and update numbers
		apiInteraction()
		// update totals
		totalRares := make(map[int]int)
		//make totals all 0 to make display pretty
		for i := 1; i <= 8; i++ {
			totalRares[i] = 0
		}
		for _, farmer := range farmerMap {
			for _, stash := range farmer.c {
				for k, v := range stash {
					if k != 0 {
						totalRares[k] += v
					}
				}
			}
		}
		countMux.Unlock()
		//half the pairs
		totalRares[2] /= 2
		totalRares[8] /= 2

		//save -TODO-

		//fmt.Println(totalRares)
		//sort
		sortedRares := sortMap(totalRares)

		// update count
		countPageTmp := "<table>\n"
		//the rest
		for i := 0; i < 8 && i < len(sortedRares); i++ {
			countPageTmp += "<tr>\n<td>" + fmt.Sprint(sortedRares[i].Value) + "</td>\n" + typeToName(sortedRares[i].Key) + "\n</tr>\n"
		}
		countPageTmp += "</table>\n<br>\n<br>\n<h2>Current Farmers</h2>\n"

		//list current farmers
		countMux.RLock()
		for _, f := range farmerSlice {
			countPageTmp += f + "\n<br>\n"
		}
		countMux.RUnlock()

		countMux.Lock()
		countPage = countPageTmp
		countMux.Unlock()

		// sleep
		time.Sleep(pauseTime)
	} // loop end
}
