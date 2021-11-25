package main

import (
	"context"
	"log"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/howeyc/fsnotify"
)

type Runner struct {
	cmd    *exec.Cmd
	close  chan int
	closed chan int
}

func (r *Runner) Build() {
	absPath, _ := filepath.Abs(".")
	cmd := exec.Command("go", "build", "-o", "bin/api", "cmd/api/main.go")
	cmd.Dir = absPath
	out, err := cmd.Output()

	if err != nil {
		log.Println("Build failed: ", err.Error())
		log.Println("Output:", string(out))
		return
	}
}

func (r *Runner) Start() {
	r.Build()

	absPath, err := filepath.Abs(".")
	if err != nil {
		panic("Fail to retrieve abs path")
	}

	ctx, cancel := context.WithCancel(context.TODO())

	r.cmd = exec.CommandContext(ctx, "bin/api")
	r.cmd.Dir = absPath

	log.Println("Start server...")

	err = r.cmd.Start()

	go func() {
		r.cmd.Wait()
		r.closed <- 1
	}()

	<-r.close

	time.Sleep(time.Second)

	cancel()
}

func (r *Runner) Close() {
	r.close <- 1
	log.Println("Stopping server...")
	<-r.closed
	log.Println("Server stopped!")

}

func (r *Runner) Restart() {
	r.Close()
	go r.Start()
}

type RestartQueue struct {
	r         *Runner
	mutex     sync.RWMutex
	requested bool
}

func (q *RestartQueue) Enqueue(e *fsnotify.FileEvent) {
	q.requested = true
}

func (q *RestartQueue) Drain() {
	for {
		select {
		case _ = <-time.After(time.Second):
			if q.requested {
				q.requested = false
				q.r.Restart()
			}
		}
	}
}

func main() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	done := make(chan bool)

	r := Runner{
		close:  make(chan int),
		closed: make(chan int),
	}
	go r.Start()

	q := RestartQueue{
		r: &r,
	}

	go q.Drain()

	// Process events
	go func() {
		for {
			select {
			case ev := <-watcher.Event:
				q.Enqueue(ev)
			case err := <-watcher.Error:
				log.Println("error:", err)
			}
		}
	}()

	watcher.Watch("./pkg/*")
	watcher.Watch("./cmd/api")

	// Hang so program doesn't exit
	<-done

	/* ... do stuff ... */
	watcher.Close()
}
