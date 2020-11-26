module github.com/temporalio/roadrunner-temporal

go 1.15

require (
	github.com/buger/goterm v0.0.0-20200322175922-2f3e71b85129
	github.com/cenkalti/backoff/v4 v4.1.0
	github.com/dustin/go-humanize v1.0.0
	github.com/fatih/color v1.10.0
	github.com/json-iterator/go v1.1.10
	github.com/mattn/go-runewidth v0.0.9
	github.com/nsf/termbox-go v0.0.0-20201124104050-ed494de23a00 // indirect
	github.com/olekukonko/tablewriter v0.0.4
	github.com/spf13/cobra v1.1.0
	github.com/spiral/endure v1.0.0-beta20
	github.com/spiral/errors v1.0.4
	github.com/spiral/goridge/v2 v2.4.6
	github.com/spiral/roadrunner/v2 v2.0.0-alpha21
	github.com/vbauerster/mpb/v5 v5.3.0
	go.temporal.io/api v1.2.0
	go.temporal.io/sdk v1.1.0
	go.uber.org/zap v1.16.0
)

replace go.temporal.io/sdk v1.1.0 => ../sdk-go
