package main

import (
	"context"
	"flag"
	"golang.org/x/sync/errgroup"
	"log"
	"oakv/api/restful"
	"oakv/api/telnet"
	"oakv/engine"
	"os"
	"os/signal"
	"os/user"
	"path/filepath"
	"syscall"
)

func main() {
	usr, err := user.Current()
	if err != nil {
		log.Fatal(err)
	}
	persistToPtr := flag.String("persist", filepath.Join(usr.HomeDir, "oakv.gob.db"), "path to db file")
	flag.Parse()

	fp, err := os.OpenFile(*persistToPtr, os.O_RDWR|os.O_CREATE, 0666)
	if err != nil {
		log.Fatalln("file open", err)
	}
	defer fp.Close()

	ctx, cancel := context.WithCancel(context.Background())
	eg, ctx := errgroup.WithContext(ctx)
	oakv := engine.New(ctx, engine.Config{
		TTLCheckIntervalSecs: 10,
		Debug:                true,
		Storage:              fp,
		//StorageDeferredWriteIntervalSecs: 5,
	})

	eg.Go(func() error {
		return telnet.ListenAndServe(ctx, oakv)
	})
	eg.Go(func() error {
		return restful.ListenAndServe(ctx, oakv)
	})

	eg.Go(func() error {
		sigint := make(chan os.Signal, 1)

		signal.Notify(sigint, os.Interrupt)
		signal.Notify(sigint, syscall.SIGTERM)

		<-sigint
		log.Println("stopping engine")
		cancel()

		return nil
	})

	//time.Sleep(2 * time.Second)
	//oakv.Reset()
	//oakv.Add("key1", "value1", 100 * expires.Second)
	//oakv.Add("key2", "value2", 10 * expires.Hour)
	//oakv.Add("key3", "value3", 1 * expires.Week)
	//cancel()
	//var res *string
	//if err := oakv.Get("key1", res); err != nil {
	//	log.Println(err)
	//}
	//fmt.Println(res)
	//fmt.Println(oakv.Keys())

	if err := eg.Wait(); err != nil {
		log.Println(err)
		os.Exit(1)
	}
	return
}
