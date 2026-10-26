package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/ranfish/pt-forward/internal/titleparser"
)

type row struct {
	InfoHash   string `json:"info_hash"`
	Title      string `json:"title"`
	MediaInfo  string `json:"media_info"`
	DetailJSON string `json:"detail_source_json"`
}

func main() {
	raw, _ := os.ReadFile("/tmp/opencode/ok_hashes.json")
	var hs []string
	json.Unmarshal(raw, &hs)
	set := map[string]bool{}
	for _, h := range hs {
		set[h] = true
	}
	rawRows, _ := os.ReadFile("/tmp/opencode/all_rows.json")
	var rows []row
	json.Unmarshal(rawRows, &rows)
	n := 0
	for _, r := range rows {
		if !set[r.InfoHash] || len(r.InfoHash) < 10 {
			continue
		}
		domM, domR, domV, domA := titleparser.DOMFieldsFromDetailSource(r.DetailJSON)
		tp := titleparser.BuildTechProfile(r.Title, r.MediaInfo, domM, domR, domV, domA)
		name := titleparser.ReassembleFromTechProfile(tp, titleparser.V105TitleFormat())
		fmt.Printf("%s ST=%-12q | %s\n", r.InfoHash[:10], tp.SourceType, name[:min(78, len(name))])
		n++
		if n >= 20 {
			break
		}
	}
	fmt.Println("shown:", n)
}
