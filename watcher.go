/* -.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.-.

* File Name : watcher.go

* Purpose :

* Creation Date : 01-17-2017

* Last Modified : Tue 17 Jan 2017 10:57:41 PM UTC

* Created By : Kiyor

_._._._._._._._._._._._._._._._._._._._._.*/

package main

import (
	"github.com/fsnotify/fsnotify"
	"log"
	"time"
)

func watcher(file string, action func(string) error) {
	log.Println("start monitor", file)
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Println(err.Error())
		return
	}
	defer watcher.Close()
	done := make(chan bool)

	chAdd := make(chan string)

	go func() {
		for {
			select {
			case event := <-watcher.Events:
				log.Println("event", event)
				if event.Op == fsnotify.Create || event.Op == fsnotify.Rename {
					time.Sleep(2 * time.Second)
					go action(event.Name)
				}
				if event.Op == fsnotify.Remove {
					chAdd <- event.Name
				}
			case err := <-watcher.Errors:
				log.Println("error:", err.Error())
			}
		}
	}()

	go func() {
		for q := range chAdd {
			var err error
			err = watcher.Add(q)
			if err != nil {
				log.Println("error:", err.Error())
				for err != nil {
					err = watcher.Add(q)
					log.Println("retry add monitor", q)
					time.Sleep(5 * time.Second)
				}
			}
			log.Println("monitor", q)
			go action(q)
		}
	}()

	chAdd <- file

	<-done
}
