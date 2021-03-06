// Copyright 2015 MongoDB, Inc. All rights reserved.
// Use of this source code is governed by the MIT
// license that can be found in the LICENSE file.

package main

import (
	"./model"
	"./util"
	"flag"
	"fmt"
	"github.com/fractalcat/nagiosplugin"
	"os"
	"time"
)

const (
	CredFile = ".mongodb_mms"
)

var groupId string
var hostname string
var metricName string
var dbName string
var server string
var warning string
var critical string
var timeout int
var maxAge int
var granularity string
var period string
var username string
var apiKey string

func main() {
	setupFlags()
	if hostname == "" || groupId == "" {
		flag.Usage()
		os.Exit(2)
		return
	}

	check := nagiosplugin.NewCheck()
	defer check.Finish()

	api, err := util.NewMMSAPI(server, timeout, username, apiKey)
	if err != nil {
		check.AddResultf(nagiosplugin.UNKNOWN, "Failed to create API. Error: %v", err)
		return
	}

	host, err := api.GetHostByName(groupId, hostname)
	if err != nil {
		check.AddResultf(nagiosplugin.UNKNOWN, "%v", err)
		return
	}

	if metricName == "" {
		doHostCheck(check, host)
	} else {
		doMetricCheck(check, api, host)
	}
}

func doHostCheck(check *nagiosplugin.Check, host *model.Host) {
	age := time.Since(host.LastPing)

	critRange, err := nagiosplugin.ParseRange(critical)
	if err != nil {
		check.AddResultf(nagiosplugin.UNKNOWN, "Error parsing critical range. Error: %v", err)
		return
	}

	if critRange.Check(age.Seconds()) {
		check.AddResultf(nagiosplugin.CRITICAL, fmt.Sprintf("Last ping was %v seconds ago", age.Seconds()))
		return
	}

	warnRange, err := nagiosplugin.ParseRange(warning)
	if err != nil {
		check.AddResultf(nagiosplugin.UNKNOWN, "Error parsing warning range. Error: %v", err)
		return
	}

	if warnRange.Check(age.Seconds()) {
		check.AddResultf(nagiosplugin.WARNING, fmt.Sprintf("Last ping was %v seconds ago", age.Seconds()))
		return
	}

	check.AddResultf(nagiosplugin.OK, fmt.Sprintf("Last ping was %v seconds ago", age.Seconds()))
}

func doMetricCheck(check *nagiosplugin.Check, api *util.MMSAPI, host *model.Host) {
	var metric *model.Metric
	var err error
	if dbName == "" {
		metric, err = api.GetHostMetric(groupId, host.Id, metricName, granularity, period)
	} else {
		metric, err = api.GetHostDBMetric(groupId, host.Id, metricName, dbName, granularity, period)
	}

	if err != nil {
		check.AddResultf(nagiosplugin.UNKNOWN, "%v", err)
		return
	}

	if len(metric.DataPoints) == 0 {
		check.AddResultf(nagiosplugin.UNKNOWN, "No data points found for %v", metricName)
		return
	}

	lastDataPoint := metric.DataPoints[len(metric.DataPoints)-1]
	age := time.Since(lastDataPoint.Timestamp)
	if int(age.Seconds()) > maxAge {
		check.AddResultf(nagiosplugin.CRITICAL, "Last data point for %v is %v seconds old.", metricName, int(age.Seconds()))
		return
	}

	check.AddPerfDatum(metricName, "", lastDataPoint.Value)

	critRange, err := nagiosplugin.ParseRange(critical)
	if err != nil {
		check.AddResultf(nagiosplugin.UNKNOWN, "Error parsing critical range. Error: %v", err)
		return
	}

	if critRange.Check(lastDataPoint.Value) {
		check.AddResultf(nagiosplugin.CRITICAL, metric.ToStringLastDataPoint())
		return
	}

	warnRange, err := nagiosplugin.ParseRange(warning)
	if err != nil {
		check.AddResultf(nagiosplugin.UNKNOWN, "Error parsing warning range. Error: %v", err)
		return
	}

	if warnRange.Check(lastDataPoint.Value) {
		check.AddResultf(nagiosplugin.WARNING, metric.ToStringLastDataPoint())
		return
	}

	check.AddResultf(nagiosplugin.OK, metric.ToStringLastDataPoint())
}

func setupFlags() {
	const (
		groupIdDefault  = ""
		groupIdUsage    = "The MMS/Ops Manager group ID that contains the server"
		hostnameDefault = ""
		hostnameUsage   = "hostname:port of the mongod/s to check"
		metricDefault   = ""
		metricUsage     = "metric to query"
		dbNameDefault   = ""
		dbNameUsage     = "database name for DB_ metrics"
		serverDefault   = "https://mms.mongodb.com"
		serverUsage     = "hostname and port of the MMS/Ops Manager service"
		warningDefault  = "~:" // considered negative infinity to positive infinity (https://nagios-plugins.org/doc/guidelines.html#THRESHOLDFORMAT)
		warningUsage    = "warning threshold for given metric"
		criticalDefault = "~:"
		criticalUsage   = "critical threshold for given metric"
		timeoutDefault  = 10
		timeoutUsage    = "connection timeout connecting MMS/Ops Manager service"
		maxAgeDefault   = 360
		maxAgeUsage     = "the maximum number of seconds old a metric before it is considerd stale"
		granularityDefault	= "MINUTE"
		granularityUsage	= "the size of the epoch. Acceptable values are MINUTE HOUR DAY"
		periodDefault	= "1H"
		periodUsage		= "the ISO-8601 formatted time period that specifies how far back in the past to query."
		usernameDefault	= ""
		usernameUsage	= "the username for auth"
		apiKeyDefault	= ""
		apiKeyUsage	    = "the api key for the user"

	)

	flag.StringVar(&groupId, "groupid", groupIdDefault, groupIdUsage)
	flag.StringVar(&groupId, "g", groupIdDefault, groupIdUsage)

	flag.StringVar(&hostname, "hostname", hostnameDefault, hostnameUsage)
	flag.StringVar(&hostname, "H", hostnameDefault, hostnameUsage)

	flag.StringVar(&metricName, "metric", metricDefault, metricUsage)
	flag.StringVar(&metricName, "m", metricDefault, metricUsage)

	flag.StringVar(&dbName, "dbname", dbNameDefault, dbNameUsage)
	flag.StringVar(&dbName, "d", dbNameDefault, dbNameUsage)

	flag.IntVar(&maxAge, "maxage", maxAgeDefault, maxAgeUsage)
	flag.IntVar(&maxAge, "a", maxAgeDefault, maxAgeUsage)

	flag.StringVar(&server, "server", serverDefault, serverUsage)
	flag.StringVar(&server, "s", serverDefault, serverUsage)

	flag.StringVar(&warning, "warning", warningDefault, warningUsage)
	flag.StringVar(&warning, "w", warningDefault, warningUsage)

	flag.StringVar(&critical, "critical", criticalDefault, criticalUsage)
	flag.StringVar(&critical, "c", criticalDefault, criticalUsage)

	flag.IntVar(&timeout, "timeout", timeoutDefault, timeoutUsage)
	flag.IntVar(&timeout, "t", timeoutDefault, timeoutUsage)

	flag.StringVar(&granularity, "granularity", granularityDefault, granularityUsage)
	flag.StringVar(&granularity, "r", granularityDefault, granularityUsage)

	flag.StringVar(&period, "period", periodDefault, periodUsage)
	flag.StringVar(&period, "p", periodDefault, periodUsage)

	flag.StringVar(&username, "username", usernameDefault, usernameUsage)
	flag.StringVar(&username, "u", usernameDefault, usernameUsage)

	flag.StringVar(&apiKey, "apikey", apiKeyDefault, usernameUsage)
	flag.StringVar(&apiKey, "k", apiKeyDefault, apiKeyUsage)

	flag.Usage = func() {
		fmt.Fprintf(os.Stdout, "Usage: check_mongodb_mms  -g groupid -H hostname [-m metric] [-d dbname] [-a age] [-s server] [-t timeout] [-w warning_level] [-c critica_level] [-r granularity] [-p period] [-u username] [-k apikey]\n")
		fmt.Fprintf(os.Stdout, "     -g, --groupid  %v\n", groupIdUsage)
		fmt.Fprintf(os.Stdout, "     -H, --hostname %v\n", hostnameUsage)
		fmt.Fprintf(os.Stdout, "     -m, --metric (no metric means check last ping age in seconds) %v\n", metricUsage)
		fmt.Fprintf(os.Stdout, "     -d, --dbname (default %v) %v\n", dbNameDefault, dbNameUsage)
		fmt.Fprintf(os.Stdout, "     -a, --maxage (default %v) %v\n", maxAgeDefault, maxAgeUsage)
		fmt.Fprintf(os.Stdout, "     -s, --server (default: %v) %v\n", serverDefault, serverUsage)
		fmt.Fprintf(os.Stdout, "     -w, --warning (default: %v) %v\n", warningDefault, warningUsage)
		fmt.Fprintf(os.Stdout, "     -c, --critical (default: %v) %v\n", criticalDefault, criticalUsage)
		fmt.Fprintf(os.Stdout, "     -t, --timeout (default: %v) %v\n", timeoutDefault, timeoutUsage)
		fmt.Fprintf(os.Stdout, "     -r, --granularity (default: %v) %v\n", granularityDefault, granularityUsage)
		fmt.Fprintf(os.Stdout, "     -p, --period (default: %v) %v\n", periodDefault, periodUsage)
		fmt.Fprintf(os.Stdout, "     -u, --username (default: %v) %v\n", usernameDefault, usernameUsage)
		fmt.Fprintf(os.Stdout, "     -k, --apiKey (default: %v) %v\n", apiKeyDefault, apiKeyUsage)
		fmt.Fprintf(os.Stdout, "\n     -w and -c support the standard nagios threshold formats.\n"+
			"     See https://nagios-plugins.org/doc/guidelines.html#THRESHOLDFORMAT for more details.\n")
	}
	flag.Parse()
}
