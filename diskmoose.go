/*
 * Program to be run in the background for warning users
 * when partitions are out of disk space
 */

package main

import (
	"fmt"
	"log"
	"exec" // os/exec for newer versions of Go
	"bytes"
	"strings"
	"strconv"
	"time"
	"os"
)

const (
	WAIT_SEC = 120
	COWTYPE = "moose"
	MIN_MB = 100
	VERSION = 0.2
)

/*
   Starting point:
   1. Able to find all relevant mountpoints
   2. Able to write to all pts's
   3. Able to check the disk space of a mountpoint
   4. Able to use cowsay -f moose and warn users
 */

func isRelevant(mountpoint string) bool {
	switch mountpoint {
		case "/", "/tmp", "/var", "/var/log", "/var/cache", "/usr", "/home":
			return true
	}
	return false
}

func getRelevantMountpoints() []string {
	r := make([]string, 0)
	cmd := exec.Command("/bin/mount")
	b, err := cmd.Output()
	if err != nil {
		log.Println("Could not run mount")
		return []string{"/"}
	}
	s := bytes.NewBuffer(b).String()
	var mountpoint string
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

func getPtsFiles() []string {
	r := make([]string, 0)
	cmd := exec.Command("/usr/bin/who")
	b, err := cmd.Output()
	if err != nil {
		log.Println("Could not run who")
		return []string{}
	}
	s := bytes.NewBuffer(b).String()
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

// Write a message to a particular pts device
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
func checkFreeSpaceMBytes(mountpoint string) (int, os.Error) {
	cmd := exec.Command("/bin/df", "-BM", mountpoint)
	b, err := cmd.Output()
	if err != nil {
		log.Println("Could not run df")
		return 0, err
	}
	s := bytes.NewBuffer(b).String()
	// Get the fields from the second line, not the headline
	fields := getFields(strings.Split(s, "\n")[1])
	if len(fields) < 5 {
		log.Println("Too little output from df")
		return 0, os.NewError("Too little output from df")
	}
	df_mountpoint := fields[5]
	if df_mountpoint != mountpoint {
		log.Println("df could not check the given mountpoint: mismatch")
		log.Println("mountpoint from df:")
		log.Println(df_mountpoint)
		log.Println("mountpoint from diskmoose:")
		log.Println(mountpoint)
		return 0, os.NewError("df could not check the given mountpoint")
	}
	sMBfree := fields[3]
	if strings.Index(sMBfree, "M") == -1 {
		log.Println("No \"M\" in output from df")
		return 0, os.NewError("No \"M\" in output from df")
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
	cmd := exec.Command("/usr/bin/cowsay", "-f", COWTYPE, msg)
	b, err := cmd.Output()
	if err != nil {
		log.Println("Could not run cowsay")
		return msg
	}
	return bytes.NewBuffer(b).String()
}

func main() {
	var freeMBytes int
	var msg string
	var err os.Error
	fmt.Println(mooseSays(fmt.Sprintf("I'll let you know if there are less than %v MB free on /, /tmp, /var, /var/log, /var/cache, /usr or /home, just keep me running in the backround.", MIN_MB)))
	for {
		for _, mountpoint := range getRelevantMountpoints() {
			freeMBytes, err = checkFreeSpaceMBytes(mountpoint)
			if err != nil {
				log.Println("Could not get free space for", mountpoint)
				log.Println("Aborting.")
				os.Exit(1)
			}
			if freeMBytes < MIN_MB { //freeMBytes > 0
				msg = fmt.Sprintf("Only %v MB free on %v", freeMBytes, mountpoint)
				writeToAll(mooseSays(msg))
			}
		}
		time.Sleep(WAIT_SEC * 1e9)
	}
}
