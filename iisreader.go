package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/360EntSecGroup-Skylar/excelize/v2"
	uuid "github.com/satori/go.uuid"
)

// excel documentation: https://xuri.me/excelize/en/

type ipinfo struct {
	username  string
	norequest int
}

//var files = flag.String("f", "", "log files to be read")
var detail = flag.String("d", "page", "level of detail: page | ip | status | all")
var reqstr = flag.String("r", ".aspx", "request string")
var verbose = flag.Bool("v", true, "write report to screen")

func main() {
	flag.Parse()

	fmt.Printf("Detail:%s\n", *detail)
	fmt.Printf("Request string:%s\n", *reqstr)
	fmt.Printf("Verbose:%t", *verbose)
	files := flag.Args()
	fmt.Printf("%s\n", files)
	f := excelize.NewFile()

	for _, file := range files {
		fmt.Printf("%s\n", file)
		requestrows, iprows, statusrows, date := readLog(file, f)
		keys := make([]string, 0, len(requestrows))
		for k := range requestrows {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		reportPage(&requestrows, keys, *reqstr, date, f)

		reportIP(iprows, date+" Ip-info", f)

		reportStatus(statusrows, date+" Statuskoder", f)

		if *verbose {
			printPage(&requestrows, keys, *reqstr)
			printIP(iprows)
			printStatus(statusrows)
		}
	}
	f.DeleteSheet("Sheet1") // delete the default sheet
	if err := f.SaveAs("LogReport1.xlsx"); err != nil {
		fmt.Println(err)
	}

	os.Exit(0)
}

func printPage(requestrows *map[string][]int64, keys []string, request string) {
	for _, key := range keys {
		value := (*requestrows)[key]
		if strings.Contains(key, strings.ToLower(*reqstr)) {
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

			request := trimRequest(entry[4])
			//fmt.Printf("Last byte: %x\n", time[len(time)-1:])
			//time = time[0 : len(time)-1]
			//	fmt.Printf("%s\n", time)
			//	fmt.Printf("%s : %d\n", entry[13], len(entry[13]))
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
				//if i > 1000 {
				//	fmt.Printf("%s %s\n", entry[4], entry[13])
				//}
			}
		}

		//fmt.Printf("%v", requestrows)
	}

	return requestrows, iprows, statusrows, date
	/*
		for _, key := range keys {
			value := requestrows[key]
			if strings.Contains(key, "/api") {
				av := average(value)
				threshold := av
				fmt.Printf("%s:%d:%d,threshold:%d, max:%d, min:%d\n", key, len(value), av, reqthres(value, threshold), max(value), min(value))
			}
		}
	*/
	// for key, value := range requestrows {
	// 	if strings.Contains(key, ".aspx") {
	// 		fmt.Printf("%s:%d:%d, max:%d, min:%d\n", key, len(value), average(value), max(value), min(value))
	// 	}
	// }
}
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

func inUserName(currentUsers, user string) bool {
	users := strings.Split(currentUsers, " ")
	for _, cuser := range users {
		if user == cuser {
			return true
		}
	}
	return false
}

func reportPage(requestrows *map[string][]int64, keys []string, request, date string, f *excelize.File) {
	f.NewSheet(date)
	f.SetCellValue(date, "A1", "Request")
	f.SetCellValue(date, "B1", "Antal")
	f.SetCellValue(date, "C1", "Gennemsnit")
	f.SetCellValue(date, "D1", "Antal over gennemsnit")
	f.SetCellValue(date, "E1", "Maximum")
	f.SetCellValue(date, "F1", "Minimum")
	count := 2
	for _, key := range keys {
		if strings.Contains(key, request) {
			value := (*requestrows)[key]
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
