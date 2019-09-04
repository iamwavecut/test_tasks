package telnet

import (
	"context"
	"io"
	"log"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/reiver/go-oi"
	"github.com/reiver/go-telnet"
	"github.com/reiver/go-telnet/telsh"
	"oakv/engine"
)

func ListenAndServe(ctx context.Context, server *engine.Server) error {
	shellHandler := telsh.NewShellHandler()
	shellHandler.WelcomeMessage = "Hello, type \"help\"\r\n"
	telnetSvr := telnet.Server{
		Addr:    ":23",
		Handler: shellHandler,
	}
	handlers := map[string]func(in io.ReadCloser, out io.WriteCloser, errout io.WriteCloser, args ...string) error{

		"help": func(in io.ReadCloser, out io.WriteCloser, errout io.WriteCloser, args ...string) error {
			help := "Available commands:\r\n" +
				"add {key} {seconds_expiration} {value}\r\n" +
				"keys\r\n" +
				"set {key} {seconds_expiration} {value}\r\n" +
				"update {key} {value}\r\n" +
				"remove {key}\r\n\r\n"
			_, err := oi.LongWriteString(out, help)
			lilSleep()

			return err
		},

		"keys": func(in io.ReadCloser, out io.WriteCloser, errout io.WriteCloser, args ...string) error {
			var list string

			keys := server.Keys()
			sort.Strings(keys)
			list = strings.Join(keys, ", ")

			_, err := oi.LongWriteString(out, list+"\r\n")
			lilSleep()

			return err
		},

		"add": func(in io.ReadCloser, out io.WriteCloser, errout io.WriteCloser, args ...string) error {
			i, err := strconv.Atoi(args[1])
			if err != nil {
				return err
			}
			if err := server.Add(args[0], args[2], time.Second*time.Duration(i)); err != nil {
				return err
			}
			_, err = oi.LongWriteString(out, "ADDED\r\n")
			lilSleep()

			return err
		},

		"set": func(in io.ReadCloser, out io.WriteCloser, errout io.WriteCloser, args ...string) error {
			i, err := strconv.Atoi(args[1])
			if err != nil {
				return err
			}
			if err := server.Set(args[0], args[2], time.Second*time.Duration(i)); err != nil {
				return err
			}
			_, err = oi.LongWriteString(out, "SET\r\n")
			lilSleep()

			return err
		},

		"update": func(in io.ReadCloser, out io.WriteCloser, errout io.WriteCloser, args ...string) error {
			if err := server.Update(args[0], args[1]); err != nil {
				return err
			}
			_, err := oi.LongWriteString(out, "UPDATED\r\n")
			lilSleep()

			return err
		},

		"remove": func(in io.ReadCloser, out io.WriteCloser, errout io.WriteCloser, args ...string) error {
			if err := server.Remove(args[0]); err != nil {
				return err
			}
			_, err := oi.LongWriteString(out, "REMOVED\r\n")
			lilSleep()

			return err
		},
	}

	for name, handlerFunc := range handlers {
		if err := shellHandler.RegisterHandlerFunc(name, handlerFunc); err != nil {
			return err
		}
	}

	listener, err := net.Listen("tcp", telnetSvr.Addr)
	if err != nil {
		log.Println("tcp listen error", err)
	}
	go func() {
		log.Println("telnet listening")
		if err := telnetSvr.Serve(listener); err != nil {
			if !strings.Contains(err.Error(), "use of closed network connection") {
				log.Panicln("telnet serve error", err)
			}
		}
	}()

	select {
	case <-ctx.Done():
		log.Println("closing telnet")
		return listener.Close()
	}

	return nil
}

func lilSleep() {
	time.Sleep(50 * time.Millisecond)
}
