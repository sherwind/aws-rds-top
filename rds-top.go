// rds-top is a tool to get AWS RDS Enhanced Monitoring statistics
// from CloudWatch and show something similar to the Linux top command.
//
// based on https://gist.github.com/sherwind/962fabb187769517e93a0ac57bf88f4e
//
// Usage:
//
//	./rds-top rds-instance
//	./rds-top --start-time=$(date -v-13d +%s) rds-instance
//	./rds-top --sort-by-mem --start-time=$(date -j -f "%Y-%m-%dT%H:%M:%S%z" "2019-09-12T13:05:00+0000" +%s) rds-instance | grep -v 'idle$'
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/rds"
	"github.com/tidwall/gjson"
)

// RDSTopOptions contains command-line options for the rds-top tool.
type RDSTopOptions struct {
	startTime  int64
	sortByMem  bool
	instanceID string
}

// Process represents a single process running on an RDS instance.
type Process struct {
	ID           int     `json:"id"`
	ParentID     int     `json:"parentID"`
	VSS          int     `json:"vss"`
	RSS          int     `json:"rss"`
	CPUUsedPc    float64 `json:"cpuUsedPc"`
	MemoryUsedPc float64 `json:"memoryUsedPc"`
	Name         string  `json:"name"`
}

func main() {
	options, err := parseFlags()
	if err != nil {
		fmt.Println(err)
		usage()
		os.Exit(1)
	}

	sess, err := session.NewSessionWithOptions(session.Options{SharedConfigState: session.SharedConfigEnable})
	if err != nil {
		fmt.Println("Error creating AWS session:", err)
		os.Exit(1)
	}

	rdsSvc := rds.New(sess)
	cloudWatchLogsSvc := cloudwatchlogs.New(sess)

	resourceID, err := getResourceID(options.instanceID, rdsSvc)
	if err != nil {
		fmt.Println("Error getting resource ID:", err)
		os.Exit(1)
	}

	params := buildLogsParameters(resourceID, options.startTime)

	messageJSON, err := getLogEvents(params, cloudWatchLogsSvc)
	if err != nil {
		fmt.Println("Error getting log events:", err)
		os.Exit(1)
	}

	printSystemStats(messageJSON)
	fmt.Println()

	printNetworkStats(messageJSON)
	printDiskIOStats(messageJSON)

	fmt.Println()
	printProcessList(messageJSON, options.sortByMem)

}

func clearScreen() {
	cmd := "clear"
	if runtime.GOOS == "windows" {
		cmd = "cls"
	}

	c := exec.Command(cmd)
	c.Stdout = os.Stdout
	err := c.Run()
	if err != nil {
		fmt.Println("Error clearing screen:", err)
	}
}

func parseFlags() (RDSTopOptions, error) {
	startTimeFlag := flag.String("start-time", "", "Optional: Specify the start time in seconds since the Unix epoch")
	sortByMemFlag := flag.Bool("sort-by-mem", false, "Optional: Sorts output by memory. Default is to sort by CPU")

	flag.Parse()

	if flag.NArg() != 1 {
		return RDSTopOptions{}, errors.New("invalid number of arguments")
	}
	instanceID := flag.Arg(0)
	if *startTimeFlag != "" {
		startTime, err := strconv.ParseInt(*startTimeFlag, 10, 64)
		if err != nil {
			return RDSTopOptions{}, errors.New("invalid start time format")
		}
		return RDSTopOptions{startTime: startTime, sortByMem: *sortByMemFlag, instanceID: instanceID}, nil
	}

	return RDSTopOptions{startTime: 0, sortByMem: *sortByMemFlag, instanceID: instanceID}, nil
}

func usage() {
	fmt.Println(`Usage: rds-top [options] rds_instance_id
OPTIONS:
	--start-time=t           Optional: Specify the start time in seconds since the Unix epoch
	--sort-by-mem            Optional: Sorts output by memory. Default is to sort by CPU`)
}

func getResourceID(instanceID string, rdsSvc *rds.RDS) (string, error) {
	input := &rds.DescribeDBInstancesInput{
		DBInstanceIdentifier: aws.String(instanceID),
	}
	result, err := rdsSvc.DescribeDBInstances(input)
	if err != nil {
		return "", err
	}
	if len(result.DBInstances) == 0 {
		return "", errors.New("no DB instances found")
	}

	return *result.DBInstances[0].DbiResourceId, nil
}

func buildLogsParameters(resourceID string, startTime int64) *cloudwatchlogs.GetLogEventsInput {
	params := &cloudwatchlogs.GetLogEventsInput{
		LogGroupName:  aws.String("RDSOSMetrics"),
		LogStreamName: aws.String(resourceID),
		Limit:         aws.Int64(1),
	}
	if startTime > 0 {
		params.StartTime = aws.Int64(startTime * 1000)
		params.StartFromHead = aws.Bool(true)
	}
	return params
}

func getLogEvents(params *cloudwatchlogs.GetLogEventsInput, cloudWatchLogsSvc *cloudwatchlogs.CloudWatchLogs) (string, error) {
	result, err := cloudWatchLogsSvc.GetLogEvents(params)
	if err != nil {
		return "", err
	}

	var logMessages []string
	for _, event := range result.Events {
		logMessages = append(logMessages, *event.Message)
	}
	return strings.Join(logMessages, ""), nil
}

func printSystemStats(messageJSON string) {
	systemStats := gjson.GetMany(messageJSON, "instanceID", "timestamp", "uptime", "loadAverageMinute.one", "loadAverageMinute.five", "loadAverageMinute.fifteen", "tasks.total", "tasks.running", "tasks.sleeping", "tasks.stopped", "tasks.zombie", "cpuUtilization.user", "cpuUtilization.system", "cpuUtilization.nice", "cpuUtilization.idle", "cpuUtilization.wait", "cpuUtilization.steal", "memory.total", "memory.free", "memory.cached", "memory.buffers", "swap.total", "swap.free", "swap.cached")

	timestampStr := systemStats[1].String()
	timestamp, err := time.Parse(time.RFC3339, timestampStr)
	if err != nil {
		fmt.Println("Error parsing timestamp:", err)
		return
	}

	fmt.Printf("%s - %s - %s up, load average: %.2f, %.2f, %.2f\n", systemStats[0].String(), timestamp.Format(time.RFC3339), systemStats[2].String(), systemStats[3].Float(), systemStats[4].Float(), systemStats[5].Float())
	fmt.Printf("Tasks: %d total, %d running, %d sleeping, %d stopped, %d zombie\n", systemStats[6].Int(), systemStats[7].Int(), systemStats[8].Int(), systemStats[9].Int(), systemStats[10].Int())
	fmt.Printf("%%Cpu(s): %.2f us, %.2f sy, %.2f ni, %.2f id, %.2f wa, %.2f st\n", systemStats[11].Float(), systemStats[12].Float(), systemStats[13].Float(), systemStats[14].Float(), systemStats[15].Float(), systemStats[16].Float())
	fmt.Printf("MiB Mem: %.2f total, %.2f free, %.2f used, %.2f buff/cache\n", systemStats[17].Float()/1024, systemStats[18].Float()/1024, (systemStats[17].Float()-systemStats[18].Float())/1024, (systemStats[19].Float()+systemStats[20].Float())/1024)
	fmt.Printf("MiB Swap: %.2f total, %.2f free, %.2f cached\n", systemStats[21].Float()/1024, systemStats[22].Float()/1024, systemStats[23].Float())
}

func printNetworkStats(messageJSON string) {
	networkStats := gjson.Get(messageJSON, "network").Array()
	for _, networkStat := range networkStats {
		fmt.Printf("Net %s: %d rx, %d tx\n", networkStat.Get("interface").String(), networkStat.Get("rx").Int(), networkStat.Get("tx").Int())
	}
}

func printDiskIOStats(messageJSON string) {
	diskIOStats := gjson.Get(messageJSON, "diskIO").Array()
	for _, diskIOStat := range diskIOStats {
		fmt.Printf("Disk %s: %.2f tps, %.2f rrqm/s, %.2f wrqm/s, %.2f wKB/S, %.2f rKB/S, %.2f avgrq-sz, %.2f avgqu-sz, %.2f await, %.2f %%util\n", diskIOStat.Get("device").String(), diskIOStat.Get("tps").Float(), diskIOStat.Get("rrqmPS").Float(), diskIOStat.Get("wrqmPS").Float(), diskIOStat.Get("writeKbPS").Float(), diskIOStat.Get("readKbPS").Float(),
			diskIOStat.Get("avgReqSz").Float(), diskIOStat.Get("avgQueueLen").Float(), diskIOStat.Get("await").Float(), diskIOStat.Get("util").Float())
	}
}

func printProcessList(messageJSON string, sortByMem bool) {
	processes := gjson.Get(messageJSON, "processList").Array()
	var processList []Process

	for _, process := range processes {
		var p Process
		err := json.Unmarshal([]byte(process.Raw), &p)
		if err != nil {
			fmt.Println("Error unmarshaling JSON:", err)
			continue
		}
		processList = append(processList, p)
	}

	sort.Slice(processList, func(i, j int) bool {
		if sortByMem {
			return processList[i].MemoryUsedPc > processList[j].MemoryUsedPc
		}
		return processList[i].CPUUsedPc > processList[j].CPUUsedPc
	})

	format := "%-6s %-6s %-8s %-8s %-6s %-6s %s\n"
	fmt.Printf(format, "PID", "PPID", "VSS", "RSS", "%CPU", "%MEM", "COMMAND")

	for _, process := range processList {
		fmt.Printf(format, fmt.Sprintf("%d", process.ID), fmt.Sprintf("%d", process.ParentID), fmt.Sprintf("%d", process.VSS), fmt.Sprintf("%d", process.RSS), fmt.Sprintf("%.2f", process.CPUUsedPc), fmt.Sprintf("%.2f", process.MemoryUsedPc), process.Name)
	}
}
