package main

import (
	"flag"
	"fmt"
	cmap "github.com/orcaman/concurrent-map"
	"io"
	"log"
	"net/http"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/vharitonsky/iniflags"
)

var (
	portPtr             = flag.String("port", "8081", "Port to listen on")
	macPtr              = flag.String("mac", "", "Bluetooth MAC addresses separated with comma")
	intervalPtr         = flag.Int("scan-sleep", 60, "Check bluetooth devices every X seconds")
	pingCountPtr        = flag.Int("ping-count", 2, "Number of pings per check")
	oldDataThresholdPtr = flag.Int64("old-data-threshold", 120, "Old data threshold in seconds")
	re                  = regexp.MustCompile(`(?m)^(\S{2}:){5}\S{2}$`)
	devices             cmap.ConcurrentMap
)

type device struct {
	mac         string
	resultJson  string
	lastChecked time.Time
}

func init() {
	devices = cmap.New()
}

func main() {
	iniflags.Parse()
	macAddresses := getMacAddresses()
	quit := make(chan bool)
	go btChecker(macAddresses, quit)
	http.HandleFunc("/", handler)
	log.Println("Listening on port:", *portPtr)
	err := http.ListenAndServe(":"+*portPtr, nil)
	if err != nil {
		log.Fatalln("Can not listen:", err.Error())
		return
	}
	quit <- true
}

func saveBluetoothDevice(mac string, exitCode int) {
	lastChecked := time.Now()
	resultJson := ""
	if exitCode == 0 {
		resultJson = getJson(1, "in range")
	} else {
		resultJson = getJson(0, "not found")
	}
	devices.Set(mac, device{mac: mac, resultJson: resultJson, lastChecked: lastChecked})
}

func getMac(mac string) string {
	if devices.Has(mac) == false {
		return getJson(0, "Unknown MAC")
	}
	d, _ := devices.Get(mac)
	device, _ := d.(device)
	if time.Now().Unix()-device.lastChecked.Unix() > *oldDataThresholdPtr {
		return getJson(0, "Last checked is too old "+strconv.FormatInt(time.Now().Unix()-device.lastChecked.Unix(), 10))
	}

	return device.resultJson
}

func checkDevice(mac string) int {
	cmd := exec.Command("/usr/bin/l2ping",
		"-t 1",
		"-c "+strconv.Itoa(*pingCountPtr),
		mac)
	exitCode := "0"
	out, err := cmd.Output()
	if werr, ok := err.(*exec.ExitError); ok {
		exitCode = werr.Error()
	}
	log.Printf("mac: %s | cmd exit code: %s | cmd output: %s\n", mac, exitCode, out)

	exitCodeInt, err := strconv.Atoi(exitCode)
	if err != nil {
		return -99
	}

	return exitCodeInt
}

func btChecker(macAddresses []string, done chan bool) {
	checkDevices(macAddresses)
	ticker := time.NewTicker(time.Duration(*intervalPtr) * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-done:
			return
		case t := <-ticker.C:
			fmt.Println("Tick at", t)
			checkDevices(macAddresses)
		}
	}
}

func checkDevices(macAddresses []string) {
	for _, mac := range macAddresses {
		exitCode := checkDevice(mac)
		saveBluetoothDevice(mac, exitCode)
	}
}

func getMacAddresses() []string {
	if *macPtr == "" {
		log.Fatalln("No Bluetooth MAC addresses have been defined (use --mac parameter)")
	}
	macAddresses := strings.Split(*macPtr, ",")

	for _, mac := range macAddresses {
		if re.MatchString(mac) == false {
			log.Fatalln(mac, "is not valid MAC address.")
		}
	}

	return macAddresses
}

func handler(w http.ResponseWriter, r *http.Request) {
	keys, ok := r.URL.Query()["mac"]

	if !ok || len(keys[0]) < 1 {
		log.Println("Url Param 'mac' is missing")
		httpResponse(w, "Missing 'mac' parameter", "")
		return
	}

	key := keys[0]
	if re.MatchString(key) {
		httpResponse(w, getMac(key), key)
	} else {
		log.Println("Wrong input: " + key)
		httpResponse(w, getJson(0, "wrong input"), key)
	}
}

func httpResponse(w io.Writer, response string, mac string) {
	log.Printf("[%s]: %s", mac, response)
	_, err := fmt.Fprintf(w, response)
	if err != nil {
		log.Println("Could not write HTTP response:", err.Error())
		return
	}
}

func getJson(result int, description string) string {
	return fmt.Sprintf("{\"description\":\"%s\",\"state\":%d}", description, result)
}
