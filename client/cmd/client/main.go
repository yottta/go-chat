package main

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/yottta/chat/client/domain"
	"github.com/yottta/chat/client/infra/data"
	"github.com/yottta/chat/client/infra/data/inmemory"
	"github.com/yottta/chat/client/infra/http/directory"
	"github.com/yottta/chat/client/infra/socket"
	"github.com/yottta/chat/client/infra/tui"
	"log"
	"os"
	"os/signal"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"syscall"
	"time"
)

var (
	currentUserName = MustEnv("USER_NAME")
	serverURL       = MustEnv("SERVER_URL")
)

func main() {
	// prepare the closing signals and contexts
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)
	ctx, cancelFunc := context.WithCancel(context.Background())
	defer cancelFunc()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer func() {
			close(exit)
			cancelFunc()
			wg.Done()
		}()
		select {
		case <-exit:
		case <-ctx.Done():
		}
	}()

	// create new socket service
	so, err := socket.NewSocket()
	if err != nil {
		log.Fatalf("failed to get local address: %s", err)
	}

	currentUserId := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%s_%d", so.LocalIP(), so.AllocatedPort())))
	// prepare the store to hold the messages exchanged
	store := inmemory.NewStore(
		ctx,
		domain.User{
			Id:      currentUserId,
			Name:    currentUserName,
			Address: so.LocalIP(),
			Port:    so.AllocatedPort(),
		},
	)
	so.RegisterStore(store)

	// prepare directory client and register
	dc := directory.NewClient(serverURL)

	wg.Add(1)
	go func() {
		defer func() {
			log.Println("closing directory sync")
			wg.Done()
		}()
		tick := time.NewTicker(5 * time.Second)
		for {
			select {
			case <-ctx.Done():
				tick.Stop()
				return
			case <-tick.C:
				ping(ctx, dc, store.CurrentUser())
				loadClients(ctx, dc, store)
			}
		}
	}()

	// start listening for new connections
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := so.Listen(ctx); err != nil {
			log.Fatal(err)
		}
	}()

	// init the UI and start it
	tui := tui.New(store)
	ping(ctx, dc, store.CurrentUser())
	loadClients(ctx, dc, store)
	if err := tui.Start(ctx); err != nil {
		log.Printf("error during starting tui app: %s", err)
	}

	cancelFunc()
	wg.Wait()
	<-time.After(1 * time.Second)

	// just print things out to be sure that there are no leaks
	debug.PrintStack()
	fmt.Println("num goroutines", runtime.NumGoroutine())
}

func MustEnv(key string) string {
	e := strings.TrimSpace(os.Getenv(key))
	if len(e) == 0 {
		panic(fmt.Errorf("missing %s env var", key))
	}
	return e
}

func ping(ctx context.Context, dc directory.Client, currentUser domain.User) {
	if err := dc.Ping(ctx, currentUser); err != nil {
		log.Printf("failed to ping directory %s: %s", serverURL, err)
	}
}

func loadClients(ctx context.Context, dc directory.Client, store data.Store) {
	users, err := dc.Users(ctx)
	if err != nil {
		log.Printf("failed to get clients from %s: %s", serverURL, err)
		return
	}
	if err := store.RefreshUsers(users); err != nil {
		log.Printf("failed to get refresh store users: %s", err)
		return
	}
}
