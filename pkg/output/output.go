package output

import (
	"fmt"
	"os"
)

var stdout = os.Stdout

const (
	Reset   = "\033[0m"
	BoldOn  = "\033[1m"
	BoldOff = "\033[22m"
)

func Green(s string) string {
	return fmt.Sprintf("\033[92m%s%s%s", BoldOn, s, Reset)
}

func Red(s string) string {
	return fmt.Sprintf("\033[91m%s%s%s", BoldOn, s, Reset)
}

func Yellow(s string) string {
	return fmt.Sprintf("\033[93m%s%s%s", BoldOn, s, Reset)
}

func Cyan(s string) string {
	return fmt.Sprintf("\033[96m%s%s%s", BoldOn, s, Reset)
}

func Blue(s string) string {
	return fmt.Sprintf("\033[94m%s%s%s", BoldOn, s, Reset)
}

func Magenta(s string) string {
	return fmt.Sprintf("\033[95m%s%s%s", BoldOn, s, Reset)
}

func Gray(s string) string {
	return fmt.Sprintf("\033[37m%s%s", s, Reset)
}

func Bold(s string) string {
	return fmt.Sprintf("%s%s%s", BoldOn, s, BoldOff)
}

func StatusColor(status string) string {
	switch status {
	case "running", "Running", "RUNNING", "started", "Started":
		return Green(status)
	case "stopped", "Stopped", "STOPPED":
		return Red(status)
	case "starting", "Starting":
		return Yellow(status)
	default:
		return Cyan(status)
	}
}

func Print(a ...interface{}) {
	fmt.Fprint(stdout, a...)
}

func Println(a ...interface{}) {
	fmt.Fprintln(stdout, a...)
}

func Printf(format string, a ...interface{}) {
	fmt.Fprintf(stdout, format, a...)
}

func Successf(format string, a ...interface{}) {
	Print(Green(fmt.Sprintf(format, a...)))
}

func Errorf(format string, a ...interface{}) {
	Print(Red(fmt.Sprintf(format, a...)))
}

func Warningf(format string, a ...interface{}) {
	Print(Yellow(fmt.Sprintf(format, a...)))
}

func Infof(format string, a ...interface{}) {
	Print(Cyan(fmt.Sprintf(format, a...)))
}

func Grayf(format string, a ...interface{}) {
	Print(Gray(fmt.Sprintf(format, a...)))
}

func Boldf(format string, a ...interface{}) string {
	return fmt.Sprintf("%s%s%s", BoldOn, fmt.Sprintf(format, a...), BoldOff)
}

func Cyanf(format string, a ...interface{}) {
	Print(Cyan(fmt.Sprintf(format, a...)))
}

func Greenf(format string, a ...interface{}) {
	Print(Green(fmt.Sprintf(format, a...)))
}

func Redf(format string, a ...interface{}) {
	Print(Red(fmt.Sprintf(format, a...)))
}

func Yellowf(format string, a ...interface{}) {
	Print(Yellow(fmt.Sprintf(format, a...)))
}

func Success(a ...interface{}) {
	Print(Green(fmt.Sprint(a...)))
}

func Error(a ...interface{}) {
	Print(Red(fmt.Sprint(a...)))
}

func Warning(a ...interface{}) {
	Print(Yellow(fmt.Sprint(a...)))
}

func Info(a ...interface{}) {
	Print(Cyan(fmt.Sprint(a...)))
}

func Fprintln() {
	Println()
}

func IsTerminal() bool {
	return os.Getenv("TERM") != "dumb" && os.Getenv("TERM") != ""
}

var Banner = `
%s██████╗ ███████╗███████╗███████╗██████╗ ██╗     ██╗███╗   ██╗██╗  ██╗███████╗
██╔══██╗██╔════╝██╔════╝██╔════╝██╔══██╗██║     ██║████╗  ██║██║ ██╔╝██╔════╝
██████╔╝█████╗  ███████╗█████╗  ██████╔╝██║     ██║██╔██╗ ██║█████╔╝ █████╗  
██╔══██╗██╔══╝  ╚════██║██╔══╝  ██╔══██╗██║     ██║██║╚██╗██║██╔═██╗ ██╔══╝  
██║  ██║███████╗███████║███████╗██║  ██║███████╗██║██║ ╚████║██║  ██╗███████╗
██║  ██║╚══════╝╚══════╝╚══════╝╚═╝  ╚═╝╚══════╝╚═╝╚═╝  ╚═══╝╚═╝  ╚═╝╚══════╝
%s%s
`

func PrintBanner() {
	if IsTerminal() {
		fmt.Fprintf(stdout, Banner, Cyan(""), Reset, Gray("                        database provisioning for developers\n"))
	} else {
		fmt.Fprintf(stdout, "\n rexadb - database provisioning for developers\n\n")
	}
}
