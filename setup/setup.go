// Copyright (C) 2014 Space Monkey, Inc.

package setup

import (
	"flag"
	"fmt"
	"log"
	"log/syslog"
	"math"
	"os"
	"regexp"
	"strings"
	"text/template"

	"github.com/SpaceMonkeyGo/spacelog"
)

var (
	output = flag.String("log.output", "stderr", "log output")
	level  = flag.String("log.level", spacelog.DefaultLevel.Name(),
		"base logger level")
	filter = flag.String("log.filter", "",
		"logger prefix to set level to the lowest level")
	format       = flag.String("log.format", "", "Format string to use")
	stdlog_level = flag.String("log.stdlevel", "warn",
		"logger level for stdlog integration")
	subproc = flag.String("log.subproc", "",
		"process to run for stdout/stderr-captured logging. If set (usually to "+
			"/usr/bin/logger), will redirect stdout and stderr to the given "+
			"process. process should take --priority <facility>.<level> and "+
			"--tag <name> options")
	buffer = flag.Int("log.buffer", 0, "the number of messages to buffer. "+
		"0 for no buffer")

	stdlog  = spacelog.GetLoggerNamed("stdlog")
	funcmap = template.FuncMap{"ColorizeLevel": spacelog.ColorizeLevel}
)

// SetFormatMethod should be called (if at all) before Setup
func SetFormatMethod(name string, fn interface{}) {
	funcmap[name] = fn
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func MustSetup(procname string) {
	must(Setup(procname))
}

func Setup(procname string) error {
	return SetupWithFacility(procname, syslog.LOG_USER)
}

func MustSetupWithFacility(procname string, facility syslog.Priority) {
	must(SetupWithFacility(procname, facility))
}

func SetupWithFacility(procname string, facility syslog.Priority) error {
	if *subproc != "" {
		err := spacelog.CaptureOutputToProcess("/usr/bin/setsid", *subproc,
			"--tag", procname, "--priority", fmt.Sprintf("%d.%d", facility,
				syslog.LOG_CRIT))
		if err != nil {
			return err
		}
	}
	level_val, err := spacelog.LevelFromString(*level)
	if err != nil {
		return err
	}
	if level_val != spacelog.DefaultLevel {
		spacelog.SetLevel(nil, level_val)
	}
	if *filter != "" {
		re, err := regexp.Compile(*filter)
		if err != nil {
			return err
		}
		spacelog.SetLevel(re, spacelog.LogLevel(math.MinInt32))
	}
	var t *template.Template
	if *format != "" {
		var err error
		t, err = template.New("user").Funcs(funcmap).Parse(*format)
		if err != nil {
			return err
		}
	}
	var textout spacelog.TextOutput
	switch strings.ToLower(*output) {
	case "syslog":
		w, err := spacelog.NewSyslogOutput(facility, procname)
		if err != nil {
			return err
		}
		if t == nil {
			t = spacelog.SyslogTemplate
		}
		textout = w
	case "stdout":
		if t == nil {
			t = spacelog.ColorTemplate
		}
		textout = spacelog.NewWriterOutput(os.Stdout)
	case "stderr":
		if t == nil {
			t = spacelog.ColorTemplate
		}
		textout = spacelog.NewWriterOutput(os.Stderr)
	default:
		if t == nil {
			t = spacelog.StandardTemplate
		}
		fh, err := os.OpenFile(*output,
			os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		textout = spacelog.NewWriterOutput(fh)
	}
	if *buffer > 0 {
		textout = spacelog.NewBufferedOutput(textout, *buffer)
	}
	spacelog.SetHandler(nil, spacelog.NewTextHandler(t, textout))
	log.SetFlags(log.Lshortfile)
	stdlog_level_val, err := spacelog.LevelFromString(*stdlog_level)
	if err != nil {
		return err
	}
	log.SetOutput(stdlog.WriterWithoutCaller(stdlog_level_val))
	return nil
}