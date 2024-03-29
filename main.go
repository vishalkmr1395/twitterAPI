package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"golang.org/x/net/context"
)

var (
	appContext context.Context
	projectID  string
	queryFlag  string
	canceling  bool
)

func main() {
  flag.StringVar(&projectID, "projectID", os.Getenv("GCLOUD_PROJECT"), "GCP Project ID")
	flag.StringVar(&queryFlag, "query", os.Getenv("T_QUERY"), "ilovelearning, byjus") //Tweets to search
	flag.Parse()
	if projectID == "" {
		log.Fatal("ProjectID argument required")
	}
	if queryFlag == "" {
		log.Fatal("Query argument required")
	}
	queryArgs := strings.Split(queryFlag, ",")
	ctx, cancel := context.WithCancel(context.Background())
	appContext = ctx
	go func() {
		// Wait for SIGINT and SIGTERM (HIT CTRL-C)
		ch := make(chan os.Signal)
		signal.Notify(ch, syscall.SIGINT, syscall.SIGTERM)
		log.Println(<-ch)
		canceling = true
		cancel()
		os.Exit(0)
	}()

	// messages channel with  buffer
	messages := make(chan MiniTweet)
	results := make(chan ProcessResult)

	// initialize publsher
	initPublisher()

	// start processing
	go process(results)

	// configure ingester
	ingester := newIngester()
	err := ingester.start(queryArgs, messages)
	if err != nil {
		log.Fatal(err)
	}
	defer ingester.stop()

	// counter stuff
	var mu sync.Mutex
	processedCount := 0
	acquiredCount := 0

	for {
		select {
		case <-appContext.Done():
			break
		case m := <-messages:
			publish(tweetTopic, m)
			mu.Lock()
			acquiredCount++
			mu.Unlock()
		case r := <-results:
			publish(resultTopic, r)
			mu.Lock()
			processedCount++
			fmt.Printf("\aAcquired:%d Processed:%d\n", acquiredCount, processedCount)
			mu.Unlock()
		}
	}
}
