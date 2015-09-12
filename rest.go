package main

import (
	"os"
	"net/http"
	"fmt"
	"strconv"
	"github.com/gorilla/mux"
	"time"
)

func GetStatus(w http.ResponseWriter, r *http.Request) {
	if globalSocket != nil {
		w.WriteHeader(200)
	}else{
		w.WriteHeader(410)
	}
}

func PutDevice(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	if globalSocket != nil {
		disconnectFlag = true
		// Wait to ensure we've stopped using the socket for reading
		for disconnectFlag {
			time.Sleep(time.Millisecond)
		}

		// Destroy all the evidence
		globalSocket.Close()
		globalSocket = nil
	}
	defaultDevice = vars["ip"]
	devicePort, _ = strconv.Atoi(vars["port"])

	go connectDevice()

	w.WriteHeader(200)
}

func DeleteDevice(w http.ResponseWriter, r *http.Request) {
	if globalSocket != nil {
		disconnectFlag = true
		// Wait to ensure we've stopped using the socket for reading
		for disconnectFlag {
			time.Sleep(time.Millisecond)
		}

		// Destroy all the evidence
		globalSocket.Close()
		globalSocket = nil
	}

	w.WriteHeader(200)
}

func GetProperty(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	property := vars["property"]
	value := getProperty(property)
	if debug {
		fmt.Println(time.Now().Format(time.StampMilli), "DEBUG: Got value", value)
	}
	w.Write([]byte(value))
	if statsEnabled {
		getInt, err := strconv.Atoi(value)
		if err == nil {
			stats.Absolute("get." + property, int64(getInt))
		}
	}
}

func PostProperty(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	property := vars["property"]
	value := vars["value"]
	success := setProperty(property, value)
	if success {
		w.Write([]byte(strconv.FormatBool(success)))
		if statsEnabled {
			postInt, err := strconv.Atoi(value)
			if err == nil {
				stats.Absolute("post." + property, int64(postInt))
			}
		}
	}else{
		w.Write([]byte(strconv.FormatBool(success)))
	}
}

func HandleKill(w http.ResponseWriter, r *http.Request) { // Debug function
	w.Write([]byte("Goodbye"))
	fmt.Println("REST Request Shutdown")
	go os.Exit(0)
}
