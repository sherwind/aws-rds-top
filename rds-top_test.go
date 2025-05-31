package main

import (
	"flag"
	"os"
	"reflect"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
)

func TestParseFlags(t *testing.T) {
	originalArgs := os.Args
	originalCommandLine := flag.CommandLine
	defer func() {
		os.Args = originalArgs
		flag.CommandLine = originalCommandLine
	}()

	tests := []struct {
		name        string
		args        []string
		wantOptions RDSTopOptions // For successful parsing or specific partial options on error
		wantErr     bool
	}{
		{
			name:        "no arguments (only command name)",
			args:        []string{"cmd"},
			wantOptions: RDSTopOptions{}, // Expect zero struct on this error
			wantErr:     true,
		},
		{
			name:        "valid instance ID only",
			args:        []string{"cmd", "test-instance"},
			wantOptions: RDSTopOptions{instanceID: "test-instance", sortByMem: false, startTime: 0},
			wantErr:     false,
		},
		{
			name:        "sort-by-mem flag then instance ID",
			args:        []string{"cmd", "--sort-by-mem", "test-instance"},
			wantOptions: RDSTopOptions{instanceID: "test-instance", sortByMem: true, startTime: 0},
			wantErr:     false,
		},
		{
			name:        "start-time flag then instance ID",
			args:        []string{"cmd", "--start-time=1678886400", "test-instance"},
			wantOptions: RDSTopOptions{instanceID: "test-instance", sortByMem: false, startTime: 1678886400},
			wantErr:     false,
		},
		{
			name:        "sort-by-mem and start-time flags then instance ID",
			args:        []string{"cmd", "--sort-by-mem", "--start-time=1678886400", "test-instance"},
			wantOptions: RDSTopOptions{instanceID: "test-instance", sortByMem: true, startTime: 1678886400},
			wantErr:     false,
		},
		{
			name:        "start-time and sort-by-mem flags then instance ID (order of flags different)",
			args:        []string{"cmd", "--start-time=1678886400", "--sort-by-mem", "test-instance"},
			wantOptions: RDSTopOptions{instanceID: "test-instance", sortByMem: true, startTime: 1678886400},
			wantErr:     false,
		},
		{
			name:        "invalid startTime format",
			args:        []string{"cmd", "--start-time=not-a-number", "test-instance"},
			// Expect instanceID and sortByMem to be parsed and set before the startTime conversion error
			wantOptions: RDSTopOptions{instanceID: "test-instance", sortByMem: false, startTime: 0},
			wantErr:     true,
		},
		{
			name:        "too many positional arguments",
			args:        []string{"cmd", "test-instance", "another-arg"},
			wantOptions: RDSTopOptions{}, // Expect zero struct on this error
			wantErr:     true,
		},
		{
			name:        "flag after positional argument (instance ID)",
			args:        []string{"cmd", "test-instance", "--sort-by-mem"},
			wantOptions: RDSTopOptions{}, // Expect zero struct on this error (invalid number of args)
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			os.Args = tt.args
			currentTestFlagSet := flag.NewFlagSet(tt.args[0], flag.PanicOnError)
			flag.CommandLine = currentTestFlagSet

			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Test panicked: %v for args %v", r, tt.args)
				}
			}()

			gotOptions, err := parseFlags()

			if (err != nil) != tt.wantErr {
				t.Errorf("parseFlags() for args %v: error = %v, wantErr %v", tt.args, err, tt.wantErr)
				return
			}

			// If an error was expected and occurred, or if no error was expected and none occurred,
			// compare gotOptions with tt.wantOptions.
			// For error cases, tt.wantOptions should represent the expected state of options on error.
			if !reflect.DeepEqual(gotOptions, tt.wantOptions) {
				t.Errorf("parseFlags() for args %v: \ngotOptions = %#v \nwantOptions= %#v", tt.args, gotOptions, tt.wantOptions)
			}
		})
	}
}

func TestBuildLogsParameters(t *testing.T) {
	tests := []struct {
		name         string
		resourceID   string
		startTime    int64
		wantParams   *cloudwatchlogs.GetLogEventsInput
	}{
		{
			name:       "startTime is 0",
			resourceID: "test-resource",
			startTime:  0,
			wantParams: &cloudwatchlogs.GetLogEventsInput{
				LogGroupName:  aws.String("RDSOSMetrics"),
				LogStreamName: aws.String("test-resource"),
				Limit:         aws.Int64(1),
			},
		},
		{
			name:       "startTime greater than 0",
			resourceID: "test-resource-2",
			startTime:  1678886400,
			wantParams: &cloudwatchlogs.GetLogEventsInput{
				LogGroupName:  aws.String("RDSOSMetrics"),
				LogStreamName: aws.String("test-resource-2"),
				Limit:         aws.Int64(1),
				StartTime:     aws.Int64(1678886400000),
				StartFromHead: aws.Bool(true),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotParams := buildLogsParameters(tt.resourceID, tt.startTime)
			if !reflect.DeepEqual(gotParams, tt.wantParams) {
				t.Errorf("buildLogsParameters() mismatch:\ngotParams = %#v\nwantParams = %#v", gotParams, tt.wantParams)
			}
		})
	}
}
