package loadtest

import (
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"flag"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/cli"
	"github.com/ether/etherpad-go/lib/utils"
	"go.uber.org/zap"
)

func RunFromCLI(logger *zap.SugaredLogger, args []string) {
	host, authors, lurkers, duration, untilFail, err := parseRunArgs(args)
	if err != nil {
		return
	}
	StartLoadTest(logger, host, authors, lurkers, duration, untilFail)
}

func parseRunArgs(args []string) (string, int, int, int, bool, error) {
	fs := flag.NewFlagSet("loadtest", flag.ContinueOnError)
	host := fs.String("host", "http://127.0.0.1:9001", "The host to test")
	authors := fs.Int("authors", 0, "Number of authors")
	lurkers := fs.Int("lurkers", 0, "Number of lurkers")
	duration := fs.Int("duration", 0, "Duration of the test in seconds")
	untilFail := fs.Bool("loadUntilFail", false, "Load until the server fails")

	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		*host = args[0]
		args = args[1:]
	}

	err := fs.Parse(args)
	return *host, *authors, *lurkers, *duration, *untilFail, err
}

func RunMultiFromCLI(logger *zap.SugaredLogger, args []string) {
	host, maxPads, err := parseMultiRunArgs(args)
	if err != nil {
		return
	}
	StartMultiLoadTest(logger, host, maxPads)
}

func parseMultiRunArgs(args []string) (string, int, error) {
	fs := flag.NewFlagSet("multiload", flag.ContinueOnError)
	host := fs.String("host", "http://127.0.0.1:9001", "The host to test")
	maxPads := fs.Int("maxPads", 10, "Maximum number of pads")

	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		*host = args[0]
		args = args[1:]
	}

	err := fs.Parse(args)
	return *host, *maxPads, err
}

type Metrics struct {
	ClientsConnected  int64
	AuthorsConnected  int64
	LurkersConnected  int64
	AppendSent        int64
	ErrorCount        int64
	AcceptedCommit    int64
	ChangeFromServer  int64
	NumConnectedUsers int64 // From server
	StartTime         time.Time
}

var stats Metrics
var maxPS float64
var statsLock sync.Mutex

func randomPadName() string {
	const chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	const strLen = 10
	var b strings.Builder
	for i := 0; i < strLen; i++ {
		b.WriteByte(chars[rand.Intn(len(chars))])
	}
	return b.String()
}

func updateMetricsUI(host string) {
	if os.Getenv("SILENT_METRICS") == "true" {
		return
	}
	statsLock.Lock()
	defer statsLock.Unlock()

	testDuration := time.Since(stats.StartTime)

	// Clear screen and move cursor to top-left
	fmt.Print("\033[2J\033[0;0H")
	fmt.Printf("Load Test Metrics -- Target Pad %s\n\n", host)

	if atomic.LoadInt64(&stats.NumConnectedUsers) > 0 {
		fmt.Printf("Total Clients Connected: %d\n", atomic.LoadInt64(&stats.NumConnectedUsers))
	}
	fmt.Printf("Local Clients Connected: %d\n", atomic.LoadInt64(&stats.ClientsConnected))
	fmt.Printf("Authors Connected: %d\n", atomic.LoadInt64(&stats.AuthorsConnected))
	fmt.Printf("Lurkers Connected: %d\n", atomic.LoadInt64(&stats.LurkersConnected))
	fmt.Printf("Sent Append messages: %d\n", atomic.LoadInt64(&stats.AppendSent))
	fmt.Printf("Errors: %d\n", atomic.LoadInt64(&stats.ErrorCount))
	fmt.Printf("Commits accepted by server: %d\n", atomic.LoadInt64(&stats.AcceptedCommit))

	changesFromServer := atomic.LoadInt64(&stats.ChangeFromServer)
	fmt.Printf("Commits sent from Server to Client: %d\n", changesFromServer)

	durationSec := testDuration.Seconds()
	if durationSec > 0 {
		currentRate := float64(changesFromServer) / durationSec // This is mean rate actually in this simple impl
		fmt.Printf("Current rate per second of Commits sent from Server to Client: %.0f\n", currentRate)
		fmt.Printf("Mean(per second) of # of Commits sent from Server to Client: %.0f\n", currentRate)

		if currentRate > maxPS {
			maxPS = currentRate
		}
		fmt.Printf("Max(per second) of # of Messages (SocketIO has cap of 10k): %.0f\n", maxPS)
	}

	diff := atomic.LoadInt64(&stats.AppendSent) - atomic.LoadInt64(&stats.AcceptedCommit)
	if diff > 5 {
		fmt.Printf("Number of commits not yet replied as ACCEPT_COMMIT from server: %d\n", diff)
	}

	fmt.Printf("Seconds test has been running for: %d\n", int(durationSec))
}

func newAuthor(host string, logger *zap.SugaredLogger) {
	pad := cli.Connect(host, logger)

	pad.OnDisconnect(func(err interface{}) {
		fmt.Printf("connection error connecting to pad: %v\n", err)
		os.Exit(1)
	})

	pad.OnConnected(func(p *cli.Pad) {
		atomic.AddInt64(&stats.ClientsConnected, 1)
		atomic.AddInt64(&stats.AuthorsConnected, 1)
		updateMetricsUI(host)

		ticker := time.NewTicker(400 * time.Millisecond)
		go func() {
			for range ticker.C {
				atomic.AddInt64(&stats.AppendSent, 1)
				updateMetricsUI(host)
				p.Append(utils.RandomString(10))
			}
		}()
	})

	pad.OnNumConnectedUsers(func(count int) {
		atomic.StoreInt64(&stats.NumConnectedUsers, int64(count))
		updateMetricsUI(host)
	})

	pad.OnAcceptCommit(func(rev int) {
		atomic.AddInt64(&stats.AcceptedCommit, 1)
		updateMetricsUI(host)
	})

	pad.On("outOfSync", func(data interface{}) {
		info, _ := data.(map[string]interface{})
		logger.Warnf("Client out of sync: %+v - reconnecting", info)
		atomic.AddInt64(&stats.ErrorCount, 1)
		atomic.AddInt64(&stats.ClientsConnected, -1)
		atomic.AddInt64(&stats.AuthorsConnected, -1)
		pad.Close()

		time.Sleep(500 * time.Millisecond)
		go newAuthor(host, logger)
	})

	pad.OnNewContents(func(atext apool.AText) {
		atomic.AddInt64(&stats.ChangeFromServer, 1)
	})
}

func newLurker(host string, logger *zap.SugaredLogger) {
	pad := cli.Connect(host, logger)

	pad.OnDisconnect(func(err interface{}) {
		fmt.Printf("connection error connecting to pad: %v\n", err)
		os.Exit(1)
	})

	pad.OnConnected(func(p *cli.Pad) {
		atomic.AddInt64(&stats.ClientsConnected, 1)
		atomic.AddInt64(&stats.LurkersConnected, 1)
		updateMetricsUI(host)
	})

	pad.OnNumConnectedUsers(func(count int) {
		atomic.StoreInt64(&stats.NumConnectedUsers, int64(count))
		updateMetricsUI(host)
	})

	pad.OnNewContents(func(atext apool.AText) {
		atomic.AddInt64(&stats.ChangeFromServer, 1)
	})
}

func StartLoadTest(logger *zap.SugaredLogger, host string, numAuthors, numLurkers int, duration int, loadUntilFail bool) {
	stats.StartTime = time.Now()

	if host == "" {
		host = "http://127.0.0.1:9001"
	}

	if !strings.Contains(host, "/p/") {
		host = fmt.Sprintf("%s/p/%s", strings.TrimSuffix(host, "/"), randomPadName())
	} else {
		// Ensure it's a valid URL
		_, err := url.Parse(host)
		if err != nil {
			fmt.Printf("Invalid host: %v\n", err)
			os.Exit(1)
		}
	}

	var endTime time.Time
	if duration > 0 {
		endTime = stats.StartTime.Add(time.Duration(duration) * time.Second)
	}

	if numAuthors > 0 || numLurkers > 0 {
		var users []string
		for i := 0; i < numLurkers; i++ {
			users = append(users, "l")
		}
		for i := 0; i < numAuthors; i++ {
			users = append(users, "a")
		}

		go func() {
			for _, t := range users {
				if t == "l" {
					newLurker(host, logger)
				} else {
					newAuthor(host, logger)
				}
				time.Sleep(200 * time.Millisecond / time.Duration(len(users)))
			}
		}()
	} else {
		if duration > 0 {
			fmt.Printf("Creating load for %d seconds\n", duration)
		} else {
			fmt.Println("Creating load until the pad server stops responding in a timely fashion")
		}

		go func() {
			// Loads at ratio of 3(lurkers):1(author), every 1 second it adds more.
			users := []string{"a", "l", "l", "l"}
			ticker := time.NewTicker(1 * time.Second)
			for range ticker.C {
				for _, t := range users {
					if t == "l" {
						newLurker(host, logger)
					} else {
						newAuthor(host, logger)
					}
					time.Sleep(200 * time.Millisecond / time.Duration(len(users)))
				}
			}
		}()
	}

	ticker := time.NewTicker(100 * time.Millisecond)
	for range ticker.C {
		if !endTime.IsZero() && time.Now().After(endTime) {
			fmt.Println("Test duration complete and Load Tests PASS")
			// Print final stats
			fmt.Printf("%+v\n", stats)
			if os.Getenv("GO_TEST_MODE") == "true" {
				return
			}
			os.Exit(0)
		}

		if loadUntilFail {
			diff := atomic.LoadInt64(&stats.AppendSent) - atomic.LoadInt64(&stats.AcceptedCommit)
			if diff > 100 {
				fmt.Printf("Load test failed: too many pending commits (%d)\n", diff)
				if os.Getenv("GO_TEST_MODE") == "true" {
					return
				}
				os.Exit(1)
			}
		}
	}
}
