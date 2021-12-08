package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"regexp"
)

var (
	portPtr = flag.String("port", "8081", "Port to listen on")
	re      = regexp.MustCompile(`(?m)^(\S{2}:){5}\S{2}$`)
)

func main() {
	http.HandleFunc("/", handler)
	log.Println("Listing on port: " + string(*portPtr))
	err := http.ListenAndServe(":"+*portPtr, nil)
	if err != nil {
		log.Fatalln("Can not listen: " + err.Error())
		return
	}
}

func handler(w http.ResponseWriter, r *http.Request) {

	keys, ok := r.URL.Query()["mac"]

	if !ok || len(keys[0]) < 1 {
		log.Println("Url Param 'mac' is missing")
		_, err := fmt.Fprintf(w, "Missing 'mac' parameter")
		if err != nil {
			return
		}
		return
	}

	// Query()["key"] will return an array of items,
	// we only want the single item.
	key := keys[0]

	if re.MatchString(string(key)) {
		cmd := exec.Command("/usr/bin/l2ping",
			"-t 1",
			"-c 2",
			key)
		exitCode := "0"
		out, err := cmd.Output()
		if werr, ok := err.(*exec.ExitError); ok {
			exitCode = werr.Error()
		}
		log.Printf("mac: %s | cmd exit code: %s | cmd output: %s\n", key, exitCode, out)

		if exitCode == "0" {
			httpResponse(w, 1, "in range")
		} else {
			httpResponse(w, 0, "not found")
		}
	} else {
		log.Printf("Wrong input\n")
		httpResponse(w, 0, "wrong input")
	}
}

func httpResponse(w http.ResponseWriter, result int, description string) {
	log.Printf("{\"description\":\"%s\",\"state\":%d}", description, result)
	_, err := fmt.Fprintf(w, "{\"description\":\"%s\",\"state\":%d}", description, result)
	if err != nil {
		log.Println(err)
	}
}
