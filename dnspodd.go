package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/caiguanhao/dnspodd/vendor/diffmatchpatch"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

const (
	GIST_DNS_FILENAME   = "DNS"
	LINE_FORMAT         = "%-8s  %-20s  %-5s  %-6s  %-20s  %-15s  %s\n"
	DNSPODD_MAX_RETRIES = 2
	GIST_MAX_RETRIES    = 2
)

type (
	Status struct {
		Code      string `json:"code"`
		CreatedAt string `json:"created_at"`
		Message   string `json:"message"`
	}
	DomainList struct {
		Domains []struct {
			CnameSpeedup     string `json:"cname_speedup"`
			CreatedOn        string `json:"created_on"`
			ExtStatus        string `json:"ext_status"`
			Grade            string `json:"grade"`
			GradeTitle       string `json:"grade_title"`
			GroupID          string `json:"group_id"`
			ID               int    `json:"id"`
			IsMark           string `json:"is_mark"`
			IsVip            string `json:"is_vip"`
			Name             string `json:"name"`
			Owner            string `json:"owner"`
			Punycode         string `json:"punycode"`
			Records          string `json:"records"`
			Remark           string `json:"remark"`
			SearchenginePush string `json:"searchengine_push"`
			Status           string `json:"status"`
			TTL              string `json:"ttl"`
			UpdatedOn        string `json:"updated_on"`
		} `json:"domains"`
		Info struct {
			AllTotal      int `json:"all_total"`
			DomainTotal   int `json:"domain_total"`
			ErrorTotal    int `json:"error_total"`
			IsmarkTotal   int `json:"ismark_total"`
			LockTotal     int `json:"lock_total"`
			MineTotal     int `json:"mine_total"`
			PauseTotal    int `json:"pause_total"`
			ShareOutTotal int `json:"share_out_total"`
			ShareTotal    int `json:"share_total"`
			SpamTotal     int `json:"spam_total"`
			VipExpire     int `json:"vip_expire"`
			VipTotal      int `json:"vip_total"`
		} `json:"info"`
		Status Status `json:"status"`
	}

	RecordList struct {
		Domain struct {
			Grade    string `json:"grade"`
			ID       int    `json:"id"`
			Name     string `json:"name"`
			Owner    string `json:"owner"`
			Punycode string `json:"punycode"`
		} `json:"domain"`
		Info struct {
			RecordTotal string `json:"record_total"`
			SubDomains  string `json:"sub_domains"`
		} `json:"info"`
		Records []Record `json:"records"`
		Status  Status   `json:"status"`
	}

	Record struct {
		DomainName string

		Enabled       string `json:"enabled"`
		ID            string `json:"id"`
		Line          string `json:"line"`
		MonitorStatus string `json:"monitor_status"`
		Mx            string `json:"mx"`
		Name          string `json:"name"`
		Remark        string `json:"remark"`
		Status        string `json:"status"`
		TTL           string `json:"ttl"`
		Type          string `json:"type"`
		UpdatedOn     string `json:"updated_on"`
		UseAqb        string `json:"use_aqb"`
		Value         string `json:"value"`
	}

	ByNormal []Record
)

func (t ByNormal) Len() int      { return len(t) }
func (t ByNormal) Swap(i, j int) { t[i], t[j] = t[j], t[i] }
func (t ByNormal) Less(i, j int) bool {
	if t[i].Type == t[j].Type {
		if t[i].DomainName == t[j].DomainName {
			if t[i].Name == t[j].Name {
				if t[i].Value == t[j].Value {
					return t[i].UpdatedOn < t[j].UpdatedOn
				}
				return t[i].Value < t[j].Value
			}
			return t[i].Name < t[j].Name
		}
		return t[i].DomainName < t[j].DomainName
	}
	return t[i].Type < t[j].Type
}

func getListOfDomains() (*DomainList, error) {
	data := url.Values{
		"login_email":    {DNSPOD_EMAIL},
		"login_password": {DNSPOD_PASSWORD},
		"format":         {"json"},
	}
	resp, err := http.PostForm("https://dnsapi.cn/Domain.List", data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	list := DomainList{}
	err = json.Unmarshal(body, &list)
	if err != nil {
		return nil, err
	}
	if list.Status.Code != "1" {
		return nil, errors.New(list.Status.Message)
	}
	return &list, nil
}

func getDomainRecordInfoById(id int) (*RecordList, error) {
	data := url.Values{
		"login_email":    {DNSPOD_EMAIL},
		"login_password": {DNSPOD_PASSWORD},
		"format":         {"json"},
		"domain_id":      {fmt.Sprintf("%d", id)},
	}
	resp, err := http.PostForm("https://dnsapi.cn/Record.List", data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	list := RecordList{}
	err = json.Unmarshal(body, &list)
	if err != nil {
		return nil, err
	}
	if list.Status.Code != "1" {
		return nil, errors.New(list.Status.Message)
	}
	return &list, nil
}

func isEnabled(input string) string {
	if input == "1" {
		return "Yes"
	}
	return "No"
}

func generateDNSTable() (*string, error) {
	list, err := getListOfDomains()
	if err != nil {
		return nil, err
	}

	var allRecords []Record
	var wg sync.WaitGroup

	for _, domain := range list.Domains {
		wg.Add(1)
		go func(id int, name string) {
			defer wg.Done()
			records, _ := getDomainRecordInfoById(id)
			for _, record := range records.Records {
				record.DomainName = name
				allRecords = append(allRecords, record)
			}
			if isVerbose {
				log.Printf("fetched %d records from %s", len(records.Records), name)
			}
		}(domain.ID, domain.Name)
	}
	wg.Wait()

	sort.Sort(ByNormal(allRecords))

	var ret string
	ret += fmt.Sprintf(LINE_FORMAT,
		"Enabled",
		"Updated At",
		"TTL",
		"Type",
		"Domain",
		"Name",
		"Value",
	)

	for _, record := range allRecords {
		ret += fmt.Sprintf(LINE_FORMAT,
			isEnabled(record.Enabled),
			record.UpdatedOn,
			record.TTL,
			record.Type,
			record.DomainName,
			record.Name,
			record.Value,
		)
	}

	return &ret, nil
}

func getOldDNSTable() (*string, error) {
	if isVerbose {
		log.Print("fetching list from github")
	}
	var gist *github.Gist
	var err error
	for tried := 0; tried < GIST_MAX_RETRIES; tried++ {
		gist, _, err = githubClient.Gists.Get(GIST_ID)
		if err == nil {
			break
		}
		if isVerbose {
			log.Print("retrying fetching list")
		}
	}
	if err != nil {
		return nil, err
	}
	if isVerbose {
		log.Printf("fetched list from github (%s)", *gist.HTMLURL)
	}
	return gist.Files[GIST_DNS_FILENAME].Content, nil
}

func makeDNSTableDiff(oldTable, newTable string) (ret string, count int) {
	dmp := diffmatchpatch.New()
	oldTableLines, newTableLines, lines := dmp.DiffLinesToChars(oldTable, newTable)
	lineBasedDiffs := dmp.DiffCleanupSemantic(dmp.DiffMain(oldTableLines, newTableLines, false))
	lineBasedDiffs = dmp.DiffCharsToLines(lineBasedDiffs, lines)
	line := 1
	for _, diff := range lineBasedDiffs {
		switch diff.Type {
		case diffmatchpatch.DiffInsert:
			ret += fmt.Sprintf("line %d added:\n", line)
			ret += diff.Text + "\n"
			count++
		case diffmatchpatch.DiffDelete:
			ret += fmt.Sprintf("line %d deleted:\n", line)
			ret += diff.Text + "\n"
			count++
		default:
			line += strings.Count(diff.Text, "\n")
		}
	}
	return
}

var githubClient *github.Client
var isVerbose bool

func init() {
	flag.BoolVar(&isVerbose, "v", false, "")
	flag.BoolVar(&isVerbose, "verbose", false, "")
	flag.Usage = func() {
		fmt.Println(path.Base(os.Args[0]), "- Print DNSPOD table changes")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Println("    -v, --verbose    Show more output")
		fmt.Println()
		fmt.Println("Source: https://github.com/caiguanhao/dnspodd")
	}
	flag.Parse()

	githubClient = github.NewClient(oauth2.NewClient(oauth2.NoContext, oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: string(GITHUB_TOKEN)},
	)))

	if PROXY_URL != "" {
		proxy, err := url.Parse(PROXY_URL)
		if err == nil {
			http.DefaultTransport = &http.Transport{
				Proxy: func(req *http.Request) (*url.URL, error) {
					if req.URL.Host == "api.github.com" {
						return proxy, nil
					}
					return nil, nil
				},
			}
		}
	}
}

func main() {
	var oldTable *string
	var newTable *string
	var oldTableErr error
	var newTableErr error

	for tried := 0; tried < DNSPODD_MAX_RETRIES; tried++ {
		if isVerbose {
			log.Print("fetching dns table")
		}

		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			oldTable, oldTableErr = getOldDNSTable()
		}()
		go func() {
			defer wg.Done()
			newTable, newTableErr = generateDNSTable()

		}()
		wg.Wait()

		if oldTableErr != nil {
			log.Fatalln(oldTableErr)
		}

		if newTableErr != nil {
			log.Fatalln(newTableErr)
		}

		if strings.Compare(*oldTable, *newTable) == 0 {
			if isVerbose {
				log.Printf("found no changes to the DNS table")
			}
			return
		}

		if isVerbose {
			log.Print("retrying fetching dns table")
		}

		time.Sleep(time.Second * 5)
	}

	diff, diffCount := makeDNSTableDiff(*oldTable, *newTable)

	fmt.Printf("found %d changes to the DNS table:\n", diffCount)
	fmt.Println()
	fmt.Println(strings.TrimSpace(diff))

	if isVerbose {
		log.Printf("updating table")
	}
	var gist *github.Gist
	var err error

	updated := github.Gist{
		Files: map[github.GistFilename]github.GistFile{
			GIST_DNS_FILENAME: github.GistFile{
				Content: newTable,
			},
		},
	}

	for tried := 0; tried < GIST_MAX_RETRIES; tried++ {
		gist, _, err = githubClient.Gists.Edit(GIST_ID, &updated)
		if err == nil {
			break
		}
		if isVerbose {
			log.Print("retrying updating table")
		}
	}

	if err != nil {
		log.Fatalln(err)
	}

	if isVerbose {
		log.Printf("table updated (%s)", *gist.HTMLURL)
	} else {
		fmt.Println()
		fmt.Printf("For more info, visit %s\n", *gist.HTMLURL)
	}
	os.Exit(1)
}
