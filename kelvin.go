// MIT License
//
// Copyright (c) 2017 Stefan Wichmann
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in all
// copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
// SOFTWARE.
package main

import log "github.com/Sirupsen/logrus"
import "os/signal"
import "syscall"
import "os"
import "sync"
import "flag"

var applicationVersion = "development"
var debug = flag.Bool("debug", false, "Enable debug logging")
var logfile = flag.String("log", "", "Redirect log output to specified file")
var forceUpdate = flag.Bool("forceUpdate", false, "Update to new major version")

func main() {
	flag.Parse()
	configureLogging()
	log.Printf("Kelvin %v starting up... 🚀", applicationVersion)
	go CheckForUpdate(applicationVersion, *forceUpdate)
	go validateSystemTime()
	go handleSIGHUP()

	// load configuration or create a new one
	configuration, err := InitializeConfiguration()
	if err != nil {
		log.Fatal(err)
	}

	// find bridge
	bridge, err := InitializeBridge(configuration)
	if err != nil {
		log.Fatal(err)
	}
	err = bridge.printDevices()
	if err != nil {
		log.Fatal(err)
	}

	// find location
	_, err = InitializeLocation(configuration)
	if err != nil {
		log.Fatal(err)
	}

	// Save configuration if we changed it.
	if configuration.Modified {
		log.Debugf("Configuration changed. Saving...")
		err = configuration.Write()
		if err != nil {
			log.Fatal(err)
		}
	}

	// start routine for every light
	hueLights, err := bridge.Lights()
	if err != nil {
		log.Fatal(err)
	}
	var wg sync.WaitGroup
	for _, hueLight := range hueLights {
		hueLight := hueLight
		wg.Add(1)
		go func() {
			hueLight.updateCyclic(configuration)
			wg.Done()
		}()
	}
	wg.Wait()
	log.Debugf("All routines ended...")
}

func handleSIGHUP() {
	sighup := make(chan os.Signal, 1)
	signal.Notify(sighup, syscall.SIGHUP)
	<-sighup // wait for signal
	log.Printf("Received signal SIGHUP. Restarting...")
	Restart()
}

func configureLogging() {
	formatter := new(log.TextFormatter)
	formatter.FullTimestamp = true
	formatter.TimestampFormat = "2006/02/01 15:04:05"
	log.SetFormatter(formatter)
	if *debug {
		log.SetLevel(log.DebugLevel)
	}
	if logfile != nil && *logfile != "" {
		file, err := os.OpenFile(*logfile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err == nil {
			log.SetOutput(file)
		} else {
			log.Info("Failed to log to file, using default stderr")
		}
	}
}

func validateSystemTime() {
	// validate local clock as it forms the basis for all time calculations.
	valid, err := IsLocalTimeValid()
	if err != nil {
		log.Fatal(err)
	}
	if !valid {
		log.Warningf("WARNING: Your local system time seems to be more than one minute off. Timings may be inaccurate.")
	} else {
		log.Debugf("Local system time validated.")
	}
}
