package main

// iisreader analyses and samples informaton from the iislog and generates various reports in excel
import (
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/360EntSecGroup-Skylar/excelize/v2"
	uuid "github.com/satori/go.uuid"
	"gopkg.in/gomail.v2"
)

// excel documentation: https://xuri.me/excelize/en/

type ipinfo struct {
	username  string
	norequest int
}

type PageInfo struct {
	requests map[string][]int64
	keys     []string
}

func (p *PageInfo) Init() {
	p.requests = make(map[string][]int64)
}

func (p *PageInfo) Add(url string, duration int64) {
	p.requests[url] = append(p.requests[url], duration)
}

func (p *PageInfo) Sort() {
	p.keys = make([]string, len(p.requests))
	for key := range p.requests {
		p.keys = append(p.keys, key)
	}
	sort.Strings(p.keys)
}

func (p *PageInfo) Print(filter string) {
	for _, key := range p.keys {
		value := p.requests[key]
		if contains(key, strings.ToLower(filter)) {
			av := average(value)
			threshold := av
			fmt.Printf("%s:%d:%d,threshold:%d, max:%d, min:%d\n", key, len(value), av, reqthres(value, threshold), max(value), min(value))
		}
	}
}

func (p *PageInfo) Report(filter, date string, f *excelize.File) {
	f.NewSheet(date)
	f.SetCellValue(date, "A1", "Request")
	f.SetCellValue(date, "B1", "Antal")
	f.SetCellValue(date, "C1", "Gennemsnit")
	f.SetCellValue(date, "D1", "Antal over gennemsnit")
	f.SetCellValue(date, "E1", "Maximum")
	f.SetCellValue(date, "F1", "Minimum")
	count := 2
	for _, key := range p.keys {
		if contains(key, filter) {
			value := p.requests[key]
			av := average(value)
			index := strconv.Itoa(count)
			f.SetCellValue(date, "A"+index, key)
			f.SetCellValue(date, "B"+index, len(value))
			f.SetCellValue(date, "C"+index, av)
			f.SetCellValue(date, "D"+index, reqthres(value, av))
			f.SetCellValue(date, "E"+index, max(value))
			f.SetCellValue(date, "F"+index, min(value))
			count++
		}
	}
	f.SetColWidth(date, "A", "A", 30)
}

/*
	type accessinfo struct {
		ressource string
		count     int
	}
*/
type UserInfo struct {
	requests map[string]map[string]int
	keys     []string
}

func (u *UserInfo) Init() {
	u.requests = make(map[string]map[string]int)
}

func (u *UserInfo) Sort() {
	u.keys = make([]string, len(u.requests))
	for key := range u.requests {
		u.keys = append(u.keys, key)
	}
	sort.Strings(u.keys)
}

func (u *UserInfo) Add(username, page string) {
	//u.requests[username]
	if val, ok := u.requests[username]; ok {
		val[page]++
	} else {
		ressource := make(map[string]int)
		//ressource[page]++
		u.requests[username] = ressource
	}
	u.requests[username][page]++
}

func (u *UserInfo) Print(filter string) {
	fmt.Println("Printing Userinfo................................................................" + filter)
	for _, key := range u.keys {
		value := u.requests[key]
		for res, count := range value {
			if contains(strings.ToLower(res), strings.ToLower(filter)) {
				fmt.Printf("%s\t %s: %d\n", key, res, count)
			}
		}
	}
}

func (u *UserInfo) Report(filter, name string, f *excelize.File) {
	f.NewSheet(name)
	f.SetCellValue(name, "A1", "Brugernavn")
	f.SetCellValue(name, "B1", "webside")
	f.SetCellValue(name, "C1", "Antal requests")
	//f.SetCellValue(name, "C1", "Antal requests")

	rowcount := 2
	for _, key := range u.keys {
		value := u.requests[key]
		for res, count := range value {
			if contains(strings.ToLower(res), strings.ToLower(filter)) {
				index := strconv.Itoa(rowcount)
				if len(key) > 0 {
					fmt.Printf("%s\t %s: %d\n", key, res, count)
					f.SetCellValue(name, "A"+index, key)
					f.SetCellValue(name, "B"+index, res)
					f.SetCellValue(name, "C"+index, count)
					rowcount++
				}
			}
		}
	}

}

type IpadrInfo struct {
	requests map[string]*ipinfo
	keys     []string
}

func (i *IpadrInfo) Init() {
	i.requests = make(map[string]*ipinfo)
}

func (i *IpadrInfo) Sort() {
	i.keys = make([]string, len(i.requests))
	for key := range i.requests {
		i.keys = append(i.keys, key)
	}
	sort.Strings(i.keys)
}

func (i *IpadrInfo) Add(sourceip, username string) {
	if val, ok := i.requests[sourceip]; ok {
		val.norequest++
		//val.username = username
		if username != "-" && !inUserName(val.username, username) {
			val.username += " " + username
		}
	} else {
		ip := &ipinfo{"", 1}
		i.requests[sourceip] = ip
		//	ip.username = username

		if username != "-" {
			ip.username = username
		}
	}
}

func (i *IpadrInfo) Print() {
	for key, value := range i.requests {
		fmt.Printf("%s:%s, %d\n", key, value.username, value.norequest)
	}
}
func (i *IpadrInfo) Report(name string, f *excelize.File) {
	f.NewSheet(name)
	f.SetCellValue(name, "A1", "Ipadresse")
	f.SetCellValue(name, "B1", "Brugernavn")
	f.SetCellValue(name, "C1", "Antal requests")
	count := 2
	for key, value := range i.requests {
		index := strconv.Itoa(count)
		f.SetCellValue(name, "A"+index, key)
		f.SetCellValue(name, "B"+index, value.username)
		f.SetCellValue(name, "C"+index, value.norequest)
		count++
	}
}

type StatusInfo struct {
	requests map[string]int
	keys     []string
}

func (s *StatusInfo) Init() {
	s.requests = make(map[string]int)
}

func (s *StatusInfo) Add(status string) {
	s.requests[status]++
}

func (s *StatusInfo) Sort() {
	s.keys = make([]string, len(s.requests))
	for key := range s.requests {
		s.keys = append(s.keys, key)
	}
	sort.Strings(s.keys)
}

func (s *StatusInfo) Print() {
	for _, key := range s.keys {
		value := s.requests[key]
		fmt.Printf("%s: %d\n", key, value)
	}
}

func (s *StatusInfo) Report(name string, f *excelize.File) {
	f.NewSheet(name)
	f.SetCellValue(name, "A1", "HTTP returkode")
	f.SetCellValue(name, "B1", "Antal requests")
	count := 2
	for key, value := range s.requests {
		index := strconv.Itoa(count)
		f.SetCellValue(name, "A"+index, key)
		f.SetCellValue(name, "B"+index, value)
		count++
	}
}

type IntervalInfo struct {
	requests map[string]map[string][]int64
	keys     []string
}

func (i *IntervalInfo) Init() {
	i.requests = make(map[string]map[string][]int64)
}

func (i *IntervalInfo) Add(time, request string, duration int64) {
	interval := time[0:2]
	intervalrequestrows, ok := i.requests[interval]
	if !ok {
		intervalrequestrows = make(map[string][]int64)
	}
	intervalrequestrows[request] = append(intervalrequestrows[request], duration)
	i.requests[interval] = intervalrequestrows
}

func (i *IntervalInfo) Sort() {

	i.keys = make([]string, len(i.requests))
	for key := range i.requests {
		i.keys = append(i.keys, key)
	}
	sort.Strings(i.keys)
}
func (i *IntervalInfo) Report(filter, date string, f *excelize.File) {
	f.NewSheet(date)
	f.SetCellValue(date, "A1", "Request")
	f.SetCellValue(date, "B1", "Antal")
	f.SetCellValue(date, "C1", "Gennemsnit")
	f.SetCellValue(date, "D1", "Antal over gennemsnit")
	f.SetCellValue(date, "E1", "Maximum")
	f.SetCellValue(date, "F1", "Minimum")
	count := 2
	for _, key := range i.keys {
		value := i.requests[key]
		index := strconv.Itoa(count)
		if len(key) > 0 {
			f.SetCellValue(date, "A"+index, key)
			count++
		}
		for subkey, subvalue := range value {
			if contains(subkey, strings.ToLower(filter)) {
				av := average(subvalue)
				index := strconv.Itoa(count)
				f.SetCellValue(date, "A"+index, subkey)
				f.SetCellValue(date, "B"+index, len(subvalue))
				f.SetCellValue(date, "C"+index, av)
				f.SetCellValue(date, "D"+index, reqthres(subvalue, av))
				f.SetCellValue(date, "E"+index, max(subvalue))
				f.SetCellValue(date, "F"+index, min(subvalue))
				count++
			}
		}
	}
	f.SetColWidth(date, "A", "A", 30)
}

/*
	func SortMap(elements map[string]interface{}) []string {
		keys := make([]string, len(elements))
		for key := range elements {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return keys
	}
*/
func (i *IntervalInfo) print(filter string) {
	for _, key := range i.keys {
		value := i.requests[key]
		fmt.Printf("%s:\n", key)
		for subkey, subvalue := range value {
			if contains(subkey, strings.ToLower(filter)) {
				av := average(subvalue)
				threshold := av
				fmt.Printf("%s:%d:%d,threshold:%d, max:%d, min:%d\n", subkey, len(subvalue), av, reqthres(subvalue, threshold), max(subvalue), min(subvalue))
			}
		}
	}
}

var (
	//	files      = flag.String("f", "", "log files to be read")
	detail     = flag.String("detail", "page", "level of detail: page | ip | status | all")
	reqstr     = flag.String("filter", ".aspx", "request URL filter")
	verbose    = flag.Bool("v", true, "write report to screen")
	nodays     = flag.Int("days", 0, "Number of days that report should be generated for starting from current day")
	reportname = flag.String("name", "Logreport.xlsx", "Excel report filename")
	mail       = flag.Bool("m", false, "send mail")
	hostname   = flag.String("host", "localhost", "host mail server")
	port       = flag.Int("p", 25, "mail port")
	mailto     = flag.String("to", "", "mail to")
	mailfrom   = flag.String("from", "logreport@kimik.it", "mail from")
	period     = flag.String("period", "", "date interval of log files split by ,")
	wrkdir     = flag.String("wrkdir", "", "working directory")
)

func main() {
	flag.Parse()
	fmt.Printf("Detail:%s\n", *detail)
	fmt.Printf("Request string:%s\n", *reqstr)
	fmt.Printf("Verbose:%t\n", *verbose)
	fmt.Printf("Excel report:%s\n", *reportname)
	files := flag.Args()
	fmt.Printf("mail: %t\n", *mail)
	fmt.Printf("%s\n", files)
	fmt.Printf("period:%s\n", *period)
	handleWorkingDirectory()

	// subtract 1 day, ie. start with the previous day
	// if days is used as a parameter ignore possible filenames on the command line
	// and reset files to contain the generated filenames
	//file := "u_ex" + strconv.Itoa(year)[2:] + fmt.Sprintf("%02d", month) + fmt.Sprintf("%02d", day) + ".log"
	files = handleDays(files)

	files = handlePeriod(files)

	f := excelize.NewFile()

	global_pagerows := PageInfo{}
	global_pagerows.Init()
	for _, file := range files {
		fmt.Printf("%s\n", file)
		/*
			var (
				userrows     *UserInfo
				iprows       *IpadrInfo
				statusrows   *StatusInfo
				intervalrows *IntervalInfo
				pagerows     *PageInfo
				date         string
			)
		*/
		userrows, iprows, statusrows, intervalrows, pagerows, date := readLog(file, &global_pagerows)
		/*
				for _, key := range p.keys {
					if contains(key, filter) {
						value := p.requests[key]
						av := average(value)
			for _, key := range pagerows.keys {
				value := pagerows.requests[key]
				av := average(value)
				threshold := av
				fmt.Printf("GLOBAL %s:%d:%d,threshold:%d, max:%d, min:%d\n", key, len(value), av, reqthres(value, threshold), max(value), min(value))
				global_pagerows.Add(key, av)
			}
		*/
		if date == "" {
			continue
		}
		pagerows.Sort()
		iprows.Sort()
		statusrows.Sort()
		userrows.Sort()
		intervalrows.Sort()
		if contains(*detail, "page all") {
			pagerows.Report(*reqstr /*date*/, date+" Websider", f)
			//reportPage(requestrows, keys, *reqstr, date, f)
		}
		if contains(*detail, "ip") {
			iprows.Report(date+" Ip-info", f)
		}
		if contains(*detail, "status all") {
			statusrows.Report(date+" Statuskoder", f)
		}
		if contains(*detail, "user all") {
			userrows.Report(*reqstr, date+" Brugersider", f)
		}
		if contains(*detail, "interval") {
			intervalrows.Report(*reqstr, date+" Websider - interval", f)
		}
		if *verbose {

			if contains(*detail, "page all") {
				pagerows.Print(*reqstr)
			}
			if contains(*detail, "ip all") {
				iprows.Print()
			}
			if contains(*detail, "status all") {
				statusrows.Print()
			}

			if contains(*detail, "user all") {
				userrows.Print(*reqstr)
			}
			if contains(*detail, "interval") {
				fmt.Println("Printing intervalrows:")
				intervalrows.print(*reqstr)
			}
		}
	}
	global_pagerows.Sort()
	fmt.Println("Pinrting global --------------------------------------------------------------")
	global_pagerows.Print(*reqstr)
	global_pagerows.Report(*reqstr, "Alle websider", f)
	f.DeleteSheet("Sheet1") // delete the default sheet
	if err := f.SaveAs(*reportname); err != nil {
		log.Fatal(err) // Ensure exit with non-zero exitcode on error.
	}
	if *mail {
		sendMail(*reportname, *hostname, *mailto, *mailfrom, *port)
	}
	os.Exit(0)
}

func handlePeriod(files []string) []string {
	if len(*period) > 0 {
		const layout = "2006-01-02"
		dates := strings.Split(*period, ",")
		startdate, err := time.Parse(layout, dates[0])
		if err != nil {
			fmt.Printf("Error in dateformat: %s\n", dates[0])
			os.Exit(2)
		}
		enddate, err := time.Parse(layout, dates[1])

		enddate = enddate.AddDate(0, 0, 1)
		if err != nil {
			fmt.Printf("Error in dateformat: %s\n", dates[1])
			os.Exit(2)
		}
		for startdate != enddate {
			year := startdate.Year()
			month := int(startdate.Month())
			day := startdate.Day()
			file := fmt.Sprintf("u_ex%02d%02d%02d.log", year%100, month, day)
			files = append(files, file)
			startdate = startdate.AddDate(0, 0, 1)
		}

	}
	return files
}

func handleDays(files []string) []string {
	now := time.Now().AddDate(0, 0, -1)
	if *nodays > 0 {

		files = []string{}
	}
	for i := *nodays; i > 0; i-- {
		file := generateLogfilename(now)

		fmt.Printf("%s\n", file)
		files = append(files, file)
		now = now.AddDate(0, 0, -1)
	}
	return files
}

func handleWorkingDirectory() {
	if *wrkdir != "" {
		os.Chdir(*wrkdir)
		newDir, err := os.Getwd()
		if err != nil {
			fmt.Printf("Invalid working directory\n")
		}
		fmt.Printf("Current Working Directory: %s\n", newDir)
	}
}

func generateLogfilename(now time.Time) string {
	year := now.Year()
	month := int(now.Month())
	day := now.Day()
	return fmt.Sprintf("u_ex%02d%02d%02d.log", year%100, month, day)
}

func readLog(filename string, global_pagerows *PageInfo) (*UserInfo, *IpadrInfo, *StatusInfo, *IntervalInfo, *PageInfo, string) {
	var fields []string
	var date string
	statusrows := StatusInfo{}
	statusrows.Init()
	ipadrrows := IpadrInfo{}
	ipadrrows.Init()
	pagerows := PageInfo{}
	pagerows.Init()
	intervalrows := IntervalInfo{}
	intervalrows.Init()
	userrows := UserInfo{}
	userrows.Init()

	// TODO: ReadFile reads entire file into memory. Consider reading it line-by-line with bufio.Reader.
	data, err := os.ReadFile(filename)
	if err != nil {
		fmt.Printf("Error in read file\n")
		return nil, nil, nil, nil, nil, ""
		//os.Exit(0)
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.Trim(line, "\r")
		entry := strings.Split(line, " ")

		if len(entry) > 1 && entry[0] == "#Date:" {
			date = entry[1]
		}
		if len(entry) > 1 && entry[0] == "#Fields:" {
			fields = make([]string, len(entry)-1)
			copy(fields, entry[1:])
			// remove new line char in last field
			//for i, str := range fields {
			//	fields[i] = strings.Trim(str, "\r")
			//}
			//fmt.Println("")

		}
		if len(entry) > 1 && entry[0][0] != '#' {
			// date = entry[0]
			// time = entry[1]
			// targetip = entry[2]
			// method = entry[3]
			// requesst = entry[4]
			// urlparms = entry[5]
			// protocol = entry[6]
			// username = entry[7]
			// sourceip = entry[8]
			// browser = entry[9]
			// status = entry[10]
			// substatus = entry[11]
			// win32status = entry[12]
			// duration = entry[13]
			time := entry[slices.Index(fields, "time")]
			username := entry[slices.Index(fields, "cs-username")]
			sourceip := entry[slices.Index(fields, "c-ip")]
			status := entry[slices.Index(fields, "sc-status")]
			//duration := strings.Trim(entry[slices.Index(fields, "time-taken")], "\r") // remove Carriage return from the time entry
			duration := entry[slices.Index(fields, "time-taken")]
			request := trimRequestExt(entry[slices.Index(fields, "cs-uri-stem")])
			statusrows.Add(status)
			ipadrrows.Add(sourceip, username)
			userrows.Add(username, request)
			i, err := strconv.ParseInt(duration, 10, 64)
			if err == nil {
				pagerows.Add(request, i)
				global_pagerows.Add(request, i)
				intervalrows.Add(time, request, i)
			}
		}
	}
	return &userrows, &ipadrrows, &statusrows, &intervalrows, &pagerows, date
}

// converts a string of space separated substrings, eg. "str1 str2 str3", to an array of strings
// and tests individually whether the string is contained in source
func contains(source, target string) bool {
	targetarr := strings.Split(target, " ")
	for _, t := range targetarr {
		if strings.Contains(source, t) {
			return true
		}
	}
	return false
}

// removes parameters from the end of api calls using the http protocol in the form of guids
// in order to sample over the same request
/*
func trimRequest(request string) string {
	reqkom := strings.Split(request, "/")
	if strings.Contains(request, "/api") {
		_, err := uuid.FromString(reqkom[len(reqkom)-1])
		if err == nil {
			return strings.ToLower(strings.Join(reqkom[0:len(reqkom)-2], "/"))
		}
	}
	return strings.ToLower(request)
}
*/
// removes parameters from the api calls using the http protocol, either the form of ints or guids
// starts from the beginning of the request until it finds a value between slashes that are eiter are guid or an int
func trimRequestExt(request string) string {
	reqkom := strings.Split(request, "/")
	if strings.Contains(request, "/api") {
		for i, str := range reqkom {
			_, err := uuid.FromString(str)
			if err == nil {
				return strings.ToLower(strings.Join(reqkom[0:i], "/"))
			}
			_, err = strconv.ParseUint(str, 10, 64)
			if err == nil {
				return strings.ToLower(strings.Join(reqkom[0:i], "/"))
			}
		}
	}
	return strings.ToLower(request)
}
func inUserName(currentUsers, user string) bool {
	users := strings.Split(currentUsers, " ")
	for _, cuser := range users {
		if user == cuser {
			return true
		}
	}
	return false
}

// no of requests exceeding the threshold
func reqthres(request []int64, threshold int64) int {
	var count int
	for _, val := range request {
		if val > threshold {
			count++
		}
	}
	return count
}

func average(request []int64) int64 {
	var sum int64
	var ign int // ignored 302 request with 0 duration
	for _, val := range request {
		if val > 0 {
			sum += val
		} else {
			ign++
		}

	}
	res := float64(sum) / (float64(len(request) - ign))
	return int64(math.Trunc(res))
}

func min(request []int64) int64 {
	min := int64(math.MaxInt64)
	for _, val := range request {
		if val > 0 && val < min {
			min = val
		}
	}
	return min
}

func max(request []int64) int64 {
	max := int64(0)
	for _, val := range request {
		if val > max {
			max = val
		}
	}
	return max
}

func sendMail(reportname, hostname, mailto, mailfrom string, port int) {
	m := gomail.NewMessage()
	m.SetHeader("From", mailfrom)
	m.SetHeader("To", mailto)
	//	m.SetAddressHeader("Cc", "dan@example.com", "Dan")
	m.SetHeader("Subject", "Iis log report")
	m.SetBody("text/html", "Here is the log report")
	m.Attach(reportname)

	d := gomail.NewDialer(hostname, 25, "", "")
	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	// Send the email to Torsten :)
	if err := d.DialAndSend(m); err != nil {
		panic(err)
	}
}
