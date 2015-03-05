/**
eISCP Gateway
A system to allow control of a home receiver running the eISCP protocol, via a RESTful interface.

Gateway
*/

package main

import (
	"flag"
	"fmt"
	"github.com/gorilla/mux"
	"net"
	"net/http"
	"strconv"
	"time"
)

// Command line flags
var debug bool
var defaultDevice string
var devicePort int
var defaultPort int
var pollingRate int

// Global socket
var globalSocket *net.TCPConn

// Disconnection flag to end read loop
var disconnectFlag bool

// Device properties
var properties = make(map[string]string)

// Send recv stats
var sendCount int
var recvCount int

func main() {
	fmt.Println("Starting eISCP (ethernet Integra Serial Communication Protocol) Gateway")
	// Command line options
	flag.BoolVar(&debug, "debug", false, "enable verbose debugging")
	flag.StringVar(&defaultDevice, "device", "127.0.0.1", "IP address of device to connect to")
	flag.IntVar(&devicePort, "port", 60128, "port on device to commmunicate with")
	flag.IntVar(&defaultPort, "serve", 3000, "port to host REST API on")
	// Now that we've defined our flags, parse them
	flag.Parse()

	if debug {
		fmt.Println("Displaying debug output.")
	}

	fmt.Println("Searching for device on port", devicePort, "at", defaultDevice)

	// Attempt to connect to default device
	go connectDevice()

	r := mux.NewRouter()
	r.HandleFunc("/kill", HandleKill) //Debug Function
	r.HandleFunc("/status/", GetStatus).Methods("GET")
	r.HandleFunc("/device/", DeleteDevice).Methods("DELETE")
	r.HandleFunc("/device/{ip}/{port}", PutDevice).Methods("PUT")
	r.HandleFunc("/device/{property}", GetProperty).Methods("GET")
	r.HandleFunc("/device/{property}/{value}", PostProperty).Methods("POST")
	http.Handle("/", r)

	fmt.Println("REST API listening on port", strconv.Itoa(defaultPort))
	http.ListenAndServe(":" + strconv.Itoa(defaultPort), nil)
}

func connectDevice() {
	deviceSocket, err := net.DialTCP("tcp4", nil, &net.TCPAddr{
		IP:   net.ParseIP(defaultDevice),
		Port: devicePort,
	})

	// Check for error
	if err != nil {
		fmt.Println("Error connecting to device", defaultDevice)
		fmt.Println(err)
	}

	// We seem to have succeeded. Continue.
	globalSocket = deviceSocket

	for !disconnectFlag {
		data := make([]byte, 1024)
		deviceSocket.SetReadDeadline(time.Now().Add(time.Millisecond * 51))
		read, err := deviceSocket.Read(data)
		switch err := err.(type) {
		case net.Error:
			if err.Timeout() {
				// Timeout error, we expect this.
			} else {
				// Unexpected net error
			}
		default:
			// No error?
		}
		packet, valid := processISCP(data[:read])
		if valid {
			packetType := packet[2:5]
			packetData := packet[5:]
			properties[packetType] = packetData
			if debug {
				fmt.Println(time.Now().Format(time.StampMilli), "DEBUG: Packet:", packet, "received")
			}
			recvCount++
		}
	}
	disconnectFlag = false
}

func message(command string) {
	count, err := globalSocket.Write(packageISCP(command, modelBytes["TX-NR616"]))
	if err != nil {
		fmt.Println("Write failed", count)
	}
	sendCount++
}

func getProperty(property string) string {
	if _, present := properties[property]; !present {
		fmt.Println("Property not found, trying to acquire")
		timeout := time.Now().Add(time.Second)
		message("!1" + property + "QSTN")
		valid := false
		for !valid && time.Now().Before(timeout) {
			time.Sleep(time.Millisecond * 25)
			_, present := properties[property]
			valid = present
		}
		if valid {
			fmt.Println("We got the property")
			return properties[property]
		}
	} else {
		fmt.Println("The property already existed")
		return properties[property]
	}
	fmt.Println("We didn't return anything else?")
	return ""
}

func setProperty(property string, value string) bool {
	if debug {
		fmt.Println(time.Now().Format(time.StampMilli), "DEBUG: Attempting to set property", property, "to", value)
	}
	message("!1" + property + value)
	// We'll wait up to one second to confirm
	timeout := time.Now().Add(time.Second)
	for time.Now().Before(timeout) {
		if properties[property] == value {
			// Sucess - the value has been set and returned
			if debug {
				fmt.Println(time.Now().Format(time.StampMilli), "DEBUG: Successfully set value")
			}
			return true
			break
		}else{
			// Wait a millisecond and check again
			time.Sleep(time.Millisecond)
		}
	}
	if debug {
		fmt.Println(time.Now().Format(time.StampMilli), "DEBUG: Property", property, "should have been set to", value, "but is", properties[property])
	}
	return false
}


