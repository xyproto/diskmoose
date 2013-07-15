package main

/*
 * Program to be run in the background for warning users
 * when partitions are out of disk space.
 */

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	MIN_MB         = 100
	CHECK_INTERVAL = 120
	COWTYPE        = "moose"
	VERSION        = 0.4

	MOUNTCMD  = "/usr/bin/mount"
	WHOCMD    = "/usr/bin/who"
	DFCMD     = "/usr/bin/df"
	COWSAYCMD = "/usr/bin/cowsay"
)

/*
   Starting point:
   1. Able to find all relevant mountpoints
   2. Able to write to all pts's
   3. Able to check the disk space of a mountpoint
   4. Able to use cowsay -f moose and warn users
*/

// Evaluates if the given mount point is relevant for our purposes
func isRelevant(mountpoint string) bool {
	switch mountpoint {
	case "/", "/tmp", "/var", "/var/log", "/var/cache", "/usr", "/home":
		return true
	}
	return false
}

// Get all relevant mount points by running MOUNTCMD and then parse the output
func getRelevantMountpoints() []string {
	r := make([]string, 0)
	cmd := exec.Command(MOUNTCMD)
	b, err := cmd.Output()
	if err != nil {
		log.Println("Could not run mount")
		return []string{"/"}
	}
	s := string(b)
	mountpoint := ""
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			mountpoint = getFields(line)[2]
			if isRelevant(mountpoint) {
				r = append(r, mountpoint)
			}
		}
	}
	return r
}

// Get all pts files we could wish to write to by running WHOCMD
func getPtsFiles() []string {
	r := make([]string, 0)
	cmd := exec.Command(WHOCMD)
	b, err := cmd.Output()
	if err != nil {
		log.Println("Could not run who")
		return []string{}
	}
	s := string(b)
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			if strings.Index(line, ":S.") != -1 {
				// skip pts's in screen
				continue
			}
			// Add the pts name to the string list
			r = append(r, getFields(line)[1])
		}
	}
	return r
}

// Write a message directly to a given pts device
func writeToPts(pts, msg string) {
	filename := "/dev/" + pts
	f, err := os.OpenFile(filename, os.O_RDWR|os.O_APPEND, 0666)
	defer f.Close()
	if err != nil {
		log.Println("Could not open", filename, "for append")
		return
	}
	f.WriteString("\n" + msg + "\n")
}

// Write a message to all pts devices (excluding screen)
func writeToAll(msg string) {
	for _, pts := range getPtsFiles() {
		writeToPts(pts, msg)
	}
}

/* Get the fields of a string
 * "a  b c    d     " gives ["a" "b" "c" "d"]
 * Can be made faster by allocating more space at the start
 */
func getFields(s string) []string {
	r := make([]string, 0)
	fields := strings.Split(s, " ")
	var f string
	for _, field := range fields {
		f = strings.TrimSpace(field)
		if f != "" {
			r = append(r, f)
		}
	}
	return r
}

// Get the number of free MB for a given mountpoint
func checkFreeSpaceMBytes(mountpoint string) (int, error) {
	cmd := exec.Command(DFCMD, "-BM", mountpoint)
	b, err := cmd.Output()
	if err != nil {
		log.Println("Could not run df")
		return 0, err
	}
	s := string(b)
	// Get the fields from the second line, not the headline
	fields := getFields(strings.Split(s, "\n")[1])
	if len(fields) < 5 {
		log.Println("Too little output from df")
		return 0, errors.New("Too little output from df")
	}
	df_mountpoint := fields[5]
	if df_mountpoint != mountpoint {
		log.Println("df could not check the given mountpoint: mismatch")
		log.Println("mountpoint from df:")
		log.Println(df_mountpoint)
		log.Println("mountpoint from diskmoose:")
		log.Println(mountpoint)
		return 0, errors.New("df could not check the given mountpoint")
	}
	sMBfree := fields[3]
	if strings.Index(sMBfree, "M") == -1 {
		log.Println("No \"M\" in output from df")
		return 0, errors.New("No \"M\" in output from df")
	}
	mbfree, err := strconv.Atoi(strings.Split(sMBfree, "M")[0])
	if err != nil {
		log.Println("Could not get MB free number from df")
		return 0, err
	}
	return mbfree, nil
}

// Uses cowsay to make a moose say the given message
func mooseSays(msg string) string {
	cmd := exec.Command(COWSAYCMD, "-f", COWTYPE, msg)
	b, err := cmd.Output()
	if err != nil {
		log.Println("Could not run cowsay")
		return msg
	}
	return string(b)
}

func main() {
	var (
		freeMBytes int
		msg        string
		err        error
	)
	msg = fmt.Sprintf("I'll let you know if there are less than %v MB free in /, /tmp, /var, /var/log, /var/cache, /usr or /home. Just let me run in the background.", MIN_MB)
	fmt.Println(mooseSays(msg))
	for {
		for _, mountpoint := range getRelevantMountpoints() {
			freeMBytes, err = checkFreeSpaceMBytes(mountpoint)
			if err != nil {
				log.Printf("Could not get free space for %s.\nAborting.", mountpoint)
				os.Exit(1)
			}
			if freeMBytes < MIN_MB { //freeMBytes > 0
				msg = fmt.Sprintf("Only %v MB free on %v", freeMBytes, mountpoint)
				writeToAll(mooseSays(msg))
			}
		}
		time.Sleep(CHECK_INTERVAL * 1e9)
	}
}
