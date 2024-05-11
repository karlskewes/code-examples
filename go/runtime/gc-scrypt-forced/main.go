package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"runtime/trace"
	"time"

	"golang.org/x/crypto/scrypt"
)

func main() {
	fs := flag.NewFlagSet("gc-scrypt", flag.ExitOnError)
	forceGC := fs.Bool("force", false, "force GC after scrypt.Key() (default false)")
	count := fs.Int("count", 1, "execute n times")

	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Fatalf("fs.Parse() error: %v\n", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/debug/pprof/", pprof.Index)
	mux.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	mux.HandleFunc("/debug/pprof/profile", pprof.Profile)
	mux.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	mux.HandleFunc("/debug/pprof/trace", pprof.Trace)
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tn := time.Now().Format(time.TimeOnly)
		output := fmt.Sprintf("%s executing scrypt.Key() count: %d\n", tn, *count)
		_, _ = w.Write([]byte(output))
		for range *count {
			go executeScrypt(*forceGC, "http at "+tn)
		}
	}))

	f, _ := os.Create("trace.out")
	if err := trace.Start(f); err != nil {
		log.Fatalf("trace.Start() error: %v", err)
	}

	defer trace.Stop()

	go executeScrypt(*forceGC, "startup")

	log.Print("listening on: http://localhost:8080/")

	srv := http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	go func() {
		if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Printf("srv.ListenAndServ() error: %v\n", err)
		}
	}()

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)

	<-ch
	log.Println("ctrl+c received, shutting down")

	if err := srv.Shutdown(context.TODO()); err != nil {
		log.Printf("srv.Shutdown() error: %v\n", err)
	}
}

func executeScrypt(forceGC bool, logPrefix string) {
	log.Printf("executeScrypt() requester: %s\n", logPrefix)

	salt := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		log.Fatal(err)
	}

	trace.WithRegion(context.Background(), "scrpyt.Key", func() {
		_, err := scrypt.Key([]byte("example_phrase"), salt, 1<<17, 8, 1, 32)
		if err != nil {
			log.Print(err)
		}
	})

	trace.WithRegion(context.Background(), "runtime.GC", func() {
		if forceGC {
			// HACK:
			// TODO:
			runtime.GC()
		}
	})
}
