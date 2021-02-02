package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	_ "github.com/gin-gonic/gin/binding"
	logging "github.com/ipfs/go-log/v2"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

var (
	log = logging.Logger("main")

	port   = flag.Int("port", 6161, "--port")
	from   = flag.String("from", "", "--from")
	amount = flag.String("amount", "1000", "--amount")

	lastSentMap map[string]time.Time
	locker      sync.RWMutex
)

func sendFil(c *gin.Context) {
	address, ok := c.GetQuery("address")
	if !ok {
		c.JSON(http.StatusBadRequest, nil)
		return
	}
	locker.Lock()
	defer locker.Unlock()

	lastSendAt, ok := lastSentMap[address]
	if !ok || lastSendAt.Add(time.Hour*24).Before(time.Now()) {
		err := cmdSend(*from, address, *amount)
		if err == nil {
			lastSentMap[address] = time.Now()
			c.JSON(http.StatusBadRequest, err.Error())
			return
		} else {
			c.JSON(http.StatusOK, "success")
			return
		}
	}
	c.JSON(http.StatusNotAcceptable, "Please come back tomorrow")
}

func cmdSend(from, to, amount string) error {
	dealCmd := exec.Command("lotus", "send", to, amount)
	if from != "" {
		dealCmd = exec.Command("lotus", "send", "--from", from, amount)
	}
	output, err := dealCmd.Output()
	if err != nil {
		log.Errorf("failed to send fil, %v, addr=%s\n", err, to)
		return err
	}
	log.Info("output", output)
	return nil
}

func main() {
	flag.Parse()

	engine := gin.Default()
	engine.GET("/api/v1/send", sendFil)

	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	httpServer := &http.Server{Addr: addr, Handler: engine}

	w := sync.WaitGroup{}
	w.Add(1)
	go func() {
		defer w.Done()
		httpServer.ListenAndServe()
	}()

	waitShutdown()
	httpServer.Shutdown(context.Background())

	w.Wait()
}

func waitShutdown() {
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan
}
