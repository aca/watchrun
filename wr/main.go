package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/charmbracelet/log"

	"github.com/rjeczalik/notify"
	"golang.org/x/sys/unix"
)

type ProcInfo struct {
	sync.Mutex
	cmd *exec.Cmd
}

func terminateproc(pid int) error {
	pgid, err := unix.Getpgid(pid)
	if err != nil {
		return err
	}

	if pgid == pid {
		pid = -1 * pid
	}

	target, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return target.Signal(os.Interrupt)
}

func main() {
	// slog.SetDefault(slog.New(log.NewWithOptions(os.Stderr, log.Options{
	// 	Level:           log.DebugLevel,
	// 	Prefix:          "watchrun",
	// 	ReportTimestamp: true,
	// 	TimeFormat:      "15:04:05.00",
	// })))

	log.SetTimeFormat("15:04:05.00")
	log.SetPrefix("watchrun")

	runchan := make(chan struct{})
	stopchan := make(chan struct{})
	// donechan := make(chan struct{})

	// log.SetFlags(log.LUTC | log.Lmicroseconds)
	// log.SetPrefix("> watchrun: ")
	//

	shcmd := os.Args[1]
	shargs := os.Args[2:]

	go func() {
		for {
			<-runchan
			cmd := exec.Command(shcmd, shargs...)
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr

			err := cmd.Start()
			if err != nil {
				log.Fatal(err)
			}

			var pid int
			if cmd.Process != nil {
				pid = cmd.Process.Pid
			}
			// slog.Info(strings.Join(os.Args[1:], " "))
			log.Infof("run: [%v] %v", pid, strings.Join(os.Args[1:], " "))

			go func(cmd *exec.Cmd) {
				<-stopchan
				err = terminateproc(pid)
				if err != nil {
					log.Errorf("failed to terminate process [%v]: %v", pid, err)
				}
				log.Infof("SIGTERM [%d]", pid)
			}(cmd)

			err = cmd.Wait()
			if err != nil {
				log.Errorf("process [%v] exit: %v", pid, err)
			}
		}
	}()

	go func() {
		runchan <- struct{}{}
	}()

	c := make(chan notify.EventInfo)

	// Set up a watchpoint listening for inotify-specific events within a
	// current working directory. Dispatch each InCloseWrite and InMovedTo
	// events separately to c.
	if err := notify.Watch("./...", c, notify.Write); err != nil {
		log.Fatal(err)
	}
	defer notify.Stop(c)

	wd, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	for {
		ei := <-c
		relpath, err := filepath.Rel(wd, ei.Path())
		if err != nil {
			log.Fatal(err)
		}

		if strings.HasSuffix(relpath, "~") {
			continue
		} else if strings.HasPrefix(relpath, ".git") {
			continue
		}

		log.Infof("%v: %v", ei.Event(), relpath)

		stopchan <- struct{}{}
		runchan <- struct{}{}
	}

}
