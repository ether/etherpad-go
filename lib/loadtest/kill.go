package loadtest

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ether/etherpad-go/lib/cli"
	"go.uber.org/zap"
)

// KillBenchConfig configures RunKillBench.
type KillBenchConfig struct {
	// Victim is the instance the authors write through; it is killed.
	Victim string
	// Survivors are the instances the authors reconnect to, like a load
	// balancer picking another pod.
	Survivors []string
	// VictimPID is the process killed with SIGKILL after KillAfter.
	VictimPID int
	KillAfter time.Duration
	Pad       string
	Authors   int
	// Duration is how long the authors write, from the start.
	Duration time.Duration
	Interval time.Duration
	Settle   time.Duration
}

// KillBenchResult reports what happened to the changes around the kill.
type KillBenchResult struct {
	Sent, Accepted int
	// AcceptedAfterKill counts changes acknowledged after the kill: they
	// went through the survivors.
	AcceptedAfterKill int
	// Unacknowledged are changes no instance acknowledged in time.
	Unacknowledged []string
	// Lost are acknowledged changes missing from the pad.
	Lost []string
	// Duplicated are changes found more than once.
	Duplicated []string
	// Failover is the longest time an author waited for an
	// acknowledgement across the kill.
	Failover time.Duration
	Killed   bool
}

// Failed reports whether a change was lost, duplicated or never stored.
func (r *KillBenchResult) Failed() bool {
	return !r.Killed || len(r.Lost) > 0 || len(r.Duplicated) > 0 || len(r.Unacknowledged) > 0 || r.AcceptedAfterKill == 0
}

func (r *KillBenchResult) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "victim killed:  %v\n", r.Killed)
	fmt.Fprintf(&b, "changes:        %d sent, %d acknowledged (%d after the kill)\n", r.Sent, r.Accepted, r.AcceptedAfterKill)
	fmt.Fprintf(&b, "failover:       longest wait for an acknowledgement %s\n", r.Failover.Round(time.Millisecond))
	fmt.Fprintf(&b, "lost: %d, duplicated: %d, never acknowledged: %d\n", len(r.Lost), len(r.Duplicated), len(r.Unacknowledged))
	return b.String()
}

// RunKillBench writes through the victim, kills it mid-typing and checks
// that the authors carry on through the survivors without losing or
// duplicating a change.
func RunKillBench(logger *zap.SugaredLogger, cfg KillBenchConfig) (*KillBenchResult, error) {
	if cfg.Victim == "" || len(cfg.Survivors) == 0 || cfg.VictimPID <= 0 {
		return nil, errors.New("need -victim, -survivors and -victim-pid")
	}
	if cfg.Pad == "" {
		cfg.Pad = "killbench-" + randomPadName()
	}
	cfg.Authors = max(cfg.Authors, 1)
	if cfg.Interval <= 0 {
		cfg.Interval = 100 * time.Millisecond
	}
	if cfg.Settle <= 0 {
		cfg.Settle = 3 * time.Second
	}

	res := &KillBenchResult{}
	var mu sync.Mutex
	var sent []string
	accepted := map[string]bool{}
	var killedAt time.Time

	var pads []*cli.Pad
	defer func() {
		for _, p := range pads {
			p.Close()
		}
	}()
	for i := 0; i < cfg.Authors; i++ {
		p, err := connectAndWait(logger, padURL(cfg.Victim, cfg.Pad))
		if err != nil {
			return nil, err
		}
		p.EnableReconnect(cfg.Survivors)
		pads = append(pads, p)
	}

	go func() {
		time.Sleep(cfg.KillAfter)
		proc, err := os.FindProcess(cfg.VictimPID)
		if err == nil {
			err = proc.Kill() // SIGKILL: no shutdown, no draining
		}
		mu.Lock()
		defer mu.Unlock()
		if err != nil {
			logger.Errorf("killing %d: %v", cfg.VictimPID, err)
			return
		}
		res.Killed = true
		killedAt = time.Now()
	}()

	deadline := time.Now().Add(cfg.Duration)
	var wg sync.WaitGroup
	for a, p := range pads {
		acks := make(chan struct{}, 16)
		p.OnAcceptCommit(func(int) { acks <- struct{}{} })
		wg.Add(1)
		go func(author int, p *cli.Pad) {
			defer wg.Done()
			for seq := 0; time.Now().Before(deadline); seq++ {
				marker := fmt.Sprintf("%s%d-%d", markerPrefix, author, seq)
				mu.Lock()
				sent = append(sent, marker)
				mu.Unlock()
				start := time.Now()
				p.Append(marker)
				select {
				case <-acks:
					wait := time.Since(start)
					mu.Lock()
					accepted[marker] = true
					res.Accepted++
					if !killedAt.IsZero() && time.Now().After(killedAt) {
						res.AcceptedAfterKill++
					}
					res.Failover = max(res.Failover, wait)
					mu.Unlock()
				case <-time.After(45 * time.Second):
					mu.Lock()
					res.Unacknowledged = append(res.Unacknowledged, marker)
					mu.Unlock()
					return
				}
				time.Sleep(cfg.Interval)
			}
		}(a, p)
	}
	wg.Wait()
	time.Sleep(cfg.Settle)

	text, err := exportText(padURL(cfg.Survivors[0], cfg.Pad))
	if err != nil {
		return nil, err
	}
	count := map[string]int{}
	forEachMarker(text, func(line string) { count[line]++ })
	res.Sent = len(sent)
	for _, m := range sent {
		switch {
		case count[m] > 1:
			res.Duplicated = append(res.Duplicated, m)
		case count[m] == 0 && accepted[m]:
			res.Lost = append(res.Lost, m)
		}
	}
	sort.Strings(res.Lost)
	sort.Strings(res.Duplicated)
	return res, nil
}

// RunKillBenchFromCLI runs "etherpad-go killbench".
func RunKillBenchFromCLI(logger *zap.SugaredLogger, args []string) {
	fs := flag.NewFlagSet("killbench", flag.ContinueOnError)
	victim := fs.String("victim", "", "base URL of the instance the authors write through and that is killed")
	survivors := fs.String("survivors", "", "comma-separated base URLs the authors reconnect to")
	pid := fs.Int("victim-pid", 0, "process id of the victim instance")
	killAfter := fs.Duration("kill-after", 5*time.Second, "when to kill the victim")
	pad := fs.String("pad", "", "pad to edit (default: a new random pad)")
	authors := fs.Int("authors", 4, "number of writing clients")
	duration := fs.Duration("duration", 15*time.Second, "how long the authors write")
	interval := fs.Duration("interval", 100*time.Millisecond, "pause between two changes of one author")
	if err := fs.Parse(args); err != nil {
		os.Exit(2)
	}
	var hs []string
	for _, h := range strings.Split(*survivors, ",") {
		if h = strings.TrimSpace(h); h != "" {
			hs = append(hs, h)
		}
	}
	res, err := RunKillBench(logger, KillBenchConfig{
		Victim: *victim, Survivors: hs, VictimPID: *pid, KillAfter: *killAfter,
		Pad: *pad, Authors: *authors, Duration: *duration, Interval: *interval,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "killbench:", err)
		os.Exit(1)
	}
	fmt.Print(res.String())
	if res.Failed() {
		if len(res.Lost) > 0 {
			fmt.Printf("lost (first 10): %v\n", res.Lost[:min(10, len(res.Lost))])
		}
		if len(res.Duplicated) > 0 {
			fmt.Printf("duplicated (first 10): %v\n", res.Duplicated[:min(10, len(res.Duplicated))])
		}
		fmt.Println("FAIL")
		os.Exit(1)
	}
	fmt.Println("PASS")
}
