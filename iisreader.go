package main

// iisreader analyses and samples informaton from the iislog and generates various reports in excel
import (
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"math"
	"os"
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
	mailfrom   = flag.String("from", "logreport@kimik-it.gl", "mail from")
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
	now := time.Now().AddDate(0, 0, -1) // subtract 1 day, ie. start with the previous day
	if *nodays > 0 {
		// if days is used as a parameter ignore possible filenames on the command line
		// and reset files to contain the generated filenames
		files = []string{}
	}
	for i := *nodays; i > 0; i-- {
		file := generateLogfilename(now)
		//file := "u_ex" + strconv.Itoa(year)[2:] + fmt.Sprintf("%02d", month) + fmt.Sprintf("%02d", day) + ".log"
		fmt.Printf("%s\n", file)
		files = append(files, file)
		now = now.AddDate(0, 0, -1)
	}

	f := excelize.NewFile()

	for _, file := range files {
		fmt.Printf("%s\n", file)
		requestrows, iprows, statusrows, date := readLog(file, f)
		keys := make([]string, 0, len(requestrows))
		for k := range requestrows {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		if contains(*detail, "page all") {
			reportPage(requestrows, keys, *reqstr, date, f)
		}
		if contains(*detail, "ip all") {
			reportIP(iprows, date+" Ip-info", f)
		}
		if contains(*detail, "status all") {
			reportStatus(statusrows, date+" Statuskoder", f)
		}
		if *verbose {
			if contains(*detail, "page all") {
				printPage(requestrows, keys, *reqstr)
			}
			if contains(*detail, "ip all") {
				printIP(iprows)
			}
			if contains(*detail, "status all") {
				printStatus(statusrows)
			}
		}
	}
	f.DeleteSheet("Sheet1") // delete the default sheet
	if err := f.SaveAs(*reportname); err != nil {
		log.Fatal(err) // Ensure exit with non-zero exitcode on error.
	}
	if *mail {
		sendMail(*reportname, *hostname, *mailto, *mailfrom, *port)
	}
	os.Exit(0)
}
func generateLogfilename(now time.Time) string {
	year := now.Year()
	month := int(now.Month())
	day := now.Day()
	return fmt.Sprintf("u_ex%02d%02d%02d.log", year%100, month, day)
}

func printPage(requestrows map[string][]int64, keys []string, request string) {
	for _, key := range keys {
		value := requestrows[key]
		if contains(key, strings.ToLower(*reqstr)) {
			av := average(value)
			threshold := av
			fmt.Printf("%s:%d:%d,threshold:%d, max:%d, min:%d\n", key, len(value), av, reqthres(value, threshold), max(value), min(value))
		}
	}
}

func printIP(iprows map[string]*ipinfo) {
	for key, value := range iprows {
		fmt.Printf("%s:%s, %d\n", key, value.username, value.norequest)
	}
}

func printStatus(statusrows map[string]int) {
	for key, value := range statusrows {
		fmt.Printf("%s: %d\n", key, value)
	}
}

func readLog(filename string, f *excelize.File) (map[string][]int64, map[string]*ipinfo, map[string]int, string) {

	var date string
	//fmt.Println("Hallo")
	statusrows := make(map[string]int)
	iprows := make(map[string]*ipinfo)
	requestrows := make(map[string][]int64)
	// TODO: ReadFile reads entire file into memory. Consider reading it line-by-line with bufio.Reader.
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		fmt.Printf("Error in read file")
		os.Exit(0)
	}
	for _, line := range strings.Split(string(data), "\n") {
		entry := strings.Split(line, " ")
		if len(entry) > 1 && entry[0] == "#Date:" {
			date = entry[1]
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
			username := entry[7]
			sourceip := entry[8]
			status := entry[10]
			duration := strings.Trim(entry[13], "\r") // remove Carriage return from the time entry

			request := trimRequestExt(entry[4])
			statusrows[status]++
			if val, ok := iprows[sourceip]; ok {
				val.norequest++
				//val.username = username
				if username != "-" && !inUserName(val.username, username) {
					val.username += " " + username
				}
			} else {
				ip := &ipinfo{"", 1}
				iprows[sourceip] = ip
				//	ip.username = username

				if username != "-" {
					ip.username = username
				}
			}
			i, err := strconv.ParseInt(duration, 10, 64)
			if err == nil {
				requestrows[request] = append(requestrows[request], i)
			}
		}
	}
	return requestrows, iprows, statusrows, date
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

func reportPage(requestrows map[string][]int64, keys []string, request, date string, f *excelize.File) {
	f.NewSheet(date)
	f.SetCellValue(date, "A1", "Request")
	f.SetCellValue(date, "B1", "Antal")
	f.SetCellValue(date, "C1", "Gennemsnit")
	f.SetCellValue(date, "D1", "Antal over gennemsnit")
	f.SetCellValue(date, "E1", "Maximum")
	f.SetCellValue(date, "F1", "Minimum")
	count := 2
	for _, key := range keys {
		if contains(key, request) {
			value := requestrows[key]
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
	//	f.SetSheetName(date, date)
	// if err := f.SaveAs("LogReport1.xlsx"); err != nil {
	// 	fmt.Println(err)
	// }
}

func reportIP(iprows map[string]*ipinfo, name string, f *excelize.File) {
	f.NewSheet(name)
	f.SetCellValue(name, "A1", "Ipadresse")
	f.SetCellValue(name, "B1", "Brugernavn")
	f.SetCellValue(name, "C1", "Antal requests")
	count := 2
	for key, value := range iprows {
		index := strconv.Itoa(count)
		f.SetCellValue(name, "A"+index, key)
		f.SetCellValue(name, "B"+index, value.username)
		f.SetCellValue(name, "C"+index, value.norequest)
		count++
	}
}

func reportStatus(statusrows map[string]int, name string, f *excelize.File) {
	f.NewSheet(name)
	f.SetCellValue(name, "A1", "HTTP returkode")
	f.SetCellValue(name, "B1", "Antal requests")
	count := 2
	for key, value := range statusrows {
		index := strconv.Itoa(count)
		f.SetCellValue(name, "A"+index, key)
		f.SetCellValue(name, "B"+index, value)
		count++
	}
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
