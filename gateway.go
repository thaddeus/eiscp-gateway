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
	"github.com/quipo/statsd"
)

// Command line flags
var debug bool
var statsEnabled bool
var defaultDevice string
var devicePort int
var defaultPort int
var pollingRate int
var statsdAddress string
var statsdPrefix string

// Global socket
var globalSocket *net.TCPConn
var lastMessage int64
var hesDeadJim bool

// Disconnection flag to end read loop
var disconnectFlag bool

// Device properties
var properties = make(map[string]string)

// Send recv stats
var sendCount int
var recvCount int

var stats *statsd.StatsdBuffer

func main() {
	fmt.Println("Starting eISCP (ethernet Integra Serial Communication Protocol) Gateway")
	// Command line options
	flag.BoolVar(&debug, "debug", false, "enable verbose debugging")
	flag.BoolVar(&statsEnabled, "stats", false, "enable stats collecting")
	flag.StringVar(&defaultDevice, "device", "127.0.0.1", "IP address of device to connect to")
	flag.IntVar(&devicePort, "port", 60128, "port on device to commmunicate with")
	flag.IntVar(&defaultPort, "serve", 3000, "port to host REST API on")
	flag.StringVar(&statsdAddress, "statsd", "localhost:8125", "IP and Port of Statsd server")
	flag.StringVar(&statsdPrefix, "prefix", "eiscp", "A prefix prepended to all stats")

	// Now that we've defined our flags, parse them
	flag.Parse()

	if debug {
		fmt.Println("Displaying debug output.")
	}

	// init
  statsdclient := statsd.NewStatsdClient(statsdAddress, statsdPrefix)
  if statsEnabled {
  	if debug {
		fmt.Println("Attempting connection to statsd")
	}
    statsdclient.CreateSocket()
    interval := time.Second * 2 // aggregate stats and flush every 2 seconds
    stats = statsd.NewStatsdBuffer(interval, statsdclient)
    defer stats.Close()
	}

	fmt.Println("Searching for device on port", devicePort, "at", defaultDevice)

	// Do our device stuff here
	go func() {
		for true {
	    deviceLoop()
	  }
  }()

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

func openConnection() (bool, *net.TCPConn) {
	deviceSocket, err := net.DialTCP("tcp4", nil, &net.TCPAddr{
		IP:   net.ParseIP(defaultDevice),
		Port: devicePort,
	})
	if err != nil {
		fmt.Println("Error connecting to device", defaultDevice)
		if debug {
			fmt.Println(time.Now().Format(time.StampMilli), "DEBUG: Detailed error message:", err)
		}
		return false, nil
	}
	return true, deviceSocket
}

func stillAlive() {
	time.Sleep(time.Second * 5)
	if (time.Now().Unix() - lastMessage) > 6 {
		if debug {
			fmt.Println(time.Now().Format(time.StampMilli), "DEBUG: Last message was more than 6 seconds ago")
		}
		hesDeadJim = true
	}
}

func deviceLoop() {
	var deviceSocket *net.TCPConn
	success := false
	for !success {
		success, deviceSocket = openConnection()
		if !success {
			fmt.Println("Connection failed, waiting and trying again")
			time.Sleep(time.Second)
		}
	}

	// We seem to have succeeded. Continue.
	globalSocket = deviceSocket

	for !disconnectFlag {
		data := make([]byte, 1024)
		deviceSocket.SetReadDeadline(time.Now().Add(time.Second * 10))
		if debug {
			fmt.Println(time.Now().Format(time.StampMilli), "DEBUG: Waiting for data")
		}
		read, err := deviceSocket.Read(data)
		switch err := err.(type) {
		case net.Error:
			if err.Timeout() {
				if hesDeadJim {
					// We checked stay alive on our last loop, and it appears we're dead
					fmt.Println("Connection appears to be gone, closing.")
					disconnectFlag = true
					err := deviceSocket.Close()
					if err != nil {
						fmt.Println("Error closing socket")
					}
					hesDeadJim = false
				}else{
					// Check if our connection is timed out, or if the receiver is quiet
					if debug {
						fmt.Println(time.Now().Format(time.StampMilli), "DEBUG: KeepAlive Check")
					}
					go message("!1PWRQSTN")
					go stillAlive()
				}
			} else {
				fmt.Println("Encountered unexpected error during main loop:", err)
			}
		default:
			// No error? Update when we got our last message
			lastMessage = time.Now().Unix()
			if debug {
				fmt.Println(time.Now().Format(time.StampMilli), "DEBUG: Message received succesfully")
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
				if statsEnabled {
					postInt, err := strconv.ParseInt(packetData, 16, 0)
					if err == nil {
						stats.Gauge(".update." + packetType, postInt)
						if debug {
							fmt.Println(time.Now().Format(time.StampMilli), "DEBUG: Sending stat update", packetType, postInt)
						}
					}
				}
			}
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


