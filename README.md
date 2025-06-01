# aws-rds-top
A command line tool that retrieves AWS RDS Enhanced Monitoring statistics from CloudWatch and displays the information in a format similar to the Linux top command.

**Status: Alpha**

## Features

- Retrieves and displays RDS instance system, network, disk IO, and process statistics.
- Supports sorting by memory usage or CPU usage.
- Interactive mode with periodic data refresh (every 5 seconds).

## Prerequisites

- Go programming language installed (version 1.19 or higher)
- AWS CLI configured with the appropriate credentials and region
- [RDS Enhanced Monitoring](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_Monitoring.OS.html) enabled for your RDS instances

## Note on Processes and Threads

RDS Enhanced Monitoring displays a maximum of 100 processes and threads, which are a combination of the top CPU consuming and memory consuming processes and threads. If there are more than 50 processes and more than 50 threads, the console displays the top 50 consumers in each category. For more information, refer to the [official AWS documentation](https://docs.aws.amazon.com/AmazonRDS/latest/UserGuide/USER_Monitoring.OS.Viewing.html).


## Installation

```sh
git clone https://github.com/sherwind/aws-rds-top.git
cd aws-rds-top
go build -o rds-top
```

## Usage

By default, `rds-top` runs in an interactive mode, continuously fetching and refreshing the monitoring data for the specified RDS instance.

```sh
./rds-top [options] <rds-instance-name>
```

If you provide a `--start-time` option, `rds-top` will fetch data from that specific point in time and then exit, similar to the previous behavior. This is useful for scripting or examining historical data.

```sh
# Fetch data from a specific start time and exit
./rds-top --start-time=$(date -v-13d +%s) <rds-instance-name>

# Example with sorting and filtering (will also exit after one fetch due to --start-time)
./rds-top --sort-by-mem --start-time=$(date -j -f "%Y-%m-%dT%H:%M:%S%z" "2019-09-12T13:05:00+0000" +%s) <rds-instance-name> | grep -v 'idle$'
```

## Interactive Mode

When run without the `--start-time` option, `rds-top` enters interactive mode:

- The display automatically refreshes with the latest data every 5 seconds.
- To quit the application, press **q** followed by **Enter**, or press **Ctrl+D**.

## Options

```
--start-time=t           Optional: Specify the start time in seconds since the Unix epoch
--sort-by-mem            Optional: Sorts output by memory. Default is to sort by CPU
```

## License

This project is licensed under the MIT License.

## Contributing

Please feel free to submit issues, fork the repository, and send pull requests.
