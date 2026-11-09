package loadtest

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/cli"
	"go.uber.org/zap"
)

// ClusterBenchConfig configures RunClusterBench.
type ClusterBenchConfig struct {
	// Hosts are the base URLs of the Etherpad instances, e.g.
	// http://pod-1:9001. Clients are spread over them round-robin.
	Hosts []string
	// Pad is the pad to edit; a random one when empty.
	Pad string
	// Authors is the number of clients that write.
	Authors int
	// Duration is how long the authors write.
	Duration time.Duration
	// Interval is the pause between two changes of one author.
	Interval time.Duration
	// PrefillBytes makes the pad that large before the measurement starts,
	// to see what pad size costs.
	PrefillBytes int
	// Settle is how long to wait after the last change before the final
	// comparison.
	Settle time.Duration
}

// ClusterBenchResult is what RunClusterBench measured.
type ClusterBenchResult struct {
	Commits int
	// CommitLatency is the time from sending a change to its ACCEPT_COMMIT.
	CommitLatency Latencies
	// SamePod and CrossPod are the times until an observer saw a change,
	// for observers on the author's instance and on other instances.
	SamePod  Latencies
	CrossPod Latencies
	// Timeouts are changes without ACCEPT_COMMIT within 10 seconds.
	Timeouts  int
	OutOfSync int
	// Lost and Duplicated are accepted changes missing from, or appearing
	// more than once in, an instance's export of the pad.
	Lost       []string
	Duplicated []string
	// Divergent lists instances whose export differs from the first one's.
	Divergent []string
}

// Failed reports whether the run found lost or inconsistent data.
func (r *ClusterBenchResult) Failed() bool {
	return len(r.Lost) > 0 || len(r.Duplicated) > 0 || len(r.Divergent) > 0 || r.OutOfSync > 0
}

// Latencies is a set of samples.
type Latencies []time.Duration

func (l Latencies) Percentile(p float64) time.Duration {
	if len(l) == 0 {
		return 0
	}
	s := slices.Clone(l)
	slices.Sort(s)
	i := int(float64(len(s)-1) * p)
	return s[i]
}

func (l Latencies) String() string {
	if len(l) == 0 {
		return "n=0"
	}
	return fmt.Sprintf("n=%d p50=%v p95=%v p99=%v max=%v", len(l),
		l.Percentile(0.50).Round(time.Microsecond*100), l.Percentile(0.95).Round(time.Microsecond*100),
		l.Percentile(0.99).Round(time.Microsecond*100), l.Percentile(1).Round(time.Microsecond*100))
}

func (r *ClusterBenchResult) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "commits:        %d (timeouts %d, out of sync %d)\n", r.Commits, r.Timeouts, r.OutOfSync)
	fmt.Fprintf(&b, "commit latency: %s\n", r.CommitLatency)
	fmt.Fprintf(&b, "seen same pod:  %s\n", r.SamePod)
	fmt.Fprintf(&b, "seen other pod: %s\n", r.CrossPod)
	fmt.Fprintf(&b, "lost: %d, duplicated: %d, divergent instances: %v\n", len(r.Lost), len(r.Duplicated), r.Divergent)
	return b.String()
}

type benchState struct {
	mu        sync.Mutex
	sentAt    map[string]time.Time
	sentHost  map[string]int
	accepted  []string
	result    ClusterBenchResult
	observers []*observer
}

type observer struct {
	host int
	seen map[string]bool
}

// markerPrefix starts every line the bench writes, so it can find them in
// the pad text.
const markerPrefix = "cb-"

func padURL(host, pad string) string {
	return strings.TrimSuffix(host, "/") + "/p/" + url.PathEscape(pad)
}

// RunClusterBench writes to one pad through several Etherpad instances and
// measures latencies and whether every accepted change ends up exactly once
// in the pad on every instance.
func RunClusterBench(logger *zap.SugaredLogger, cfg ClusterBenchConfig) (*ClusterBenchResult, error) {
	if len(cfg.Hosts) == 0 {
		return nil, errors.New("no hosts")
	}
	if cfg.Pad == "" {
		cfg.Pad = "clusterbench-" + randomPadName()
	}
	if cfg.Authors <= 0 {
		cfg.Authors = 2
	}
	if cfg.Interval <= 0 {
		cfg.Interval = 200 * time.Millisecond
	}
	if cfg.Settle <= 0 {
		cfg.Settle = 3 * time.Second
	}

	st := &benchState{sentAt: map[string]time.Time{}, sentHost: map[string]int{}}

	if cfg.PrefillBytes > 0 {
		if err := prefill(logger, padURL(cfg.Hosts[0], cfg.Pad), cfg.PrefillBytes); err != nil {
			return nil, err
		}
	}

	// One observer per instance.
	var pads []*cli.Pad
	defer func() {
		for _, p := range pads {
			p.Close()
		}
	}()
	for h, host := range cfg.Hosts {
		obs := &observer{host: h, seen: map[string]bool{}}
		st.observers = append(st.observers, obs)
		p, err := connectAndWait(logger, padURL(host, cfg.Pad))
		if err != nil {
			return nil, err
		}
		pads = append(pads, p)
		p.OnNewContents(func(atext apool.AText) { st.observe(obs, atext.Text) })
		p.On("outOfSync", func(interface{}) {
			st.mu.Lock()
			st.result.OutOfSync++
			st.mu.Unlock()
		})
	}

	deadline := time.Now().Add(cfg.Duration)
	var wg sync.WaitGroup
	for a := 0; a < cfg.Authors; a++ {
		host := a % len(cfg.Hosts)
		p, err := connectAndWait(logger, padURL(cfg.Hosts[host], cfg.Pad))
		if err != nil {
			return nil, err
		}
		pads = append(pads, p)
		accepted := make(chan struct{}, 1)
		p.OnAcceptCommit(func(int) {
			select {
			case accepted <- struct{}{}:
			default:
			}
		})
		p.On("outOfSync", func(interface{}) {
			st.mu.Lock()
			st.result.OutOfSync++
			st.mu.Unlock()
		})

		wg.Add(1)
		go func(author int) {
			defer wg.Done()
			for seq := 0; time.Now().Before(deadline); seq++ {
				marker := fmt.Sprintf("%s%d-%d", markerPrefix, author, seq)
				st.mu.Lock()
				st.sentAt[marker] = time.Now()
				st.sentHost[marker] = host
				st.mu.Unlock()

				start := time.Now()
				p.Append(marker)
				select {
				case <-accepted:
					st.mu.Lock()
					st.result.Commits++
					st.result.CommitLatency = append(st.result.CommitLatency, time.Since(start))
					st.accepted = append(st.accepted, marker)
					st.mu.Unlock()
				case <-time.After(10 * time.Second):
					st.mu.Lock()
					st.result.Timeouts++
					st.mu.Unlock()
					return
				}
				time.Sleep(cfg.Interval)
			}
		}(a)
	}
	wg.Wait()
	time.Sleep(cfg.Settle)

	// Compare what every instance has stored.
	var first string
	for h, host := range cfg.Hosts {
		text, err := exportText(padURL(host, cfg.Pad))
		if err != nil {
			return nil, err
		}
		if h == 0 {
			first = text
			st.checkMarkers(text)
		} else if text != first {
			st.result.Divergent = append(st.result.Divergent, host)
		}
	}
	return &st.result, nil
}

func (st *benchState) observe(obs *observer, text string) {
	now := time.Now()
	st.mu.Lock()
	defer st.mu.Unlock()
	forEachMarker(text, func(line string) {
		if obs.seen[line] {
			return
		}
		obs.seen[line] = true
		sent, ok := st.sentAt[line]
		if !ok {
			return
		}
		if st.sentHost[line] == obs.host {
			st.result.SamePod = append(st.result.SamePod, now.Sub(sent))
		} else {
			st.result.CrossPod = append(st.result.CrossPod, now.Sub(sent))
		}
	})
}

// forEachMarker calls fn for every line of text that starts with
// markerPrefix. It searches for the prefix instead of splitting the whole
// text, which matters for prefilled megabyte pads.
func forEachMarker(text string, fn func(line string)) {
	for i := 0; i < len(text); {
		j := strings.Index(text[i:], markerPrefix)
		if j < 0 {
			return
		}
		start := i + j
		end := strings.IndexByte(text[start:], '\n')
		if end < 0 {
			end = len(text)
		} else {
			end += start
		}
		if start == 0 || text[start-1] == '\n' {
			fn(text[start:end])
		}
		i = end
	}
}

func (st *benchState) checkMarkers(text string) {
	count := map[string]int{}
	forEachMarker(text, func(line string) { count[line]++ })
	for _, m := range st.accepted {
		switch {
		case count[m] == 0:
			st.result.Lost = append(st.result.Lost, m)
		case count[m] > 1:
			st.result.Duplicated = append(st.result.Duplicated, m)
		}
	}
	sort.Strings(st.result.Lost)
	sort.Strings(st.result.Duplicated)
}

func connectAndWait(logger *zap.SugaredLogger, u string) (*cli.Pad, error) {
	p := cli.Connect(u, logger)
	select {
	case <-p.Ready():
		return p, nil
	case <-time.After(15 * time.Second):
		p.Close()
		return nil, fmt.Errorf("connecting to %s timed out", u)
	}
}

// prefill writes size bytes of filler text to the pad.
func prefill(logger *zap.SugaredLogger, u string, size int) error {
	p, err := connectAndWait(logger, u)
	if err != nil {
		return err
	}
	defer p.Close()
	accepted := make(chan struct{}, 1)
	p.OnAcceptCommit(func(int) { accepted <- struct{}{} })
	line := strings.Repeat("lorem ipsum dolor sit amet ", 3) + "\n"
	const chunk = 64 * 1024
	for written := 0; written < size; written += chunk {
		n := min(chunk, size-written)
		p.Append(strings.Repeat(line, max(1, n/len(line))))
		select {
		case <-accepted:
		case <-time.After(30 * time.Second):
			return errors.New("prefill: no ACCEPT_COMMIT")
		}
	}
	return nil
}

func exportText(padURL string) (string, error) {
	resp, err := http.Get(padURL + "/export/txt")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("export %s: %s", padURL, resp.Status)
	}
	b, err := io.ReadAll(resp.Body)
	return string(b), err
}

// RunClusterBenchFromCLI runs "etherpad-go clusterbench".
func RunClusterBenchFromCLI(logger *zap.SugaredLogger, args []string) {
	cfg, err := parseClusterBenchArgs(args)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	res, err := RunClusterBench(logger, cfg)
	if err != nil {
		fmt.Fprintln(os.Stderr, "clusterbench:", err)
		os.Exit(1)
	}
	fmt.Print(res.String())
	if res.Failed() {
		if len(res.Lost) > 0 {
			fmt.Printf("lost changes (first 10): %v\n", res.Lost[:min(10, len(res.Lost))])
		}
		fmt.Println("FAIL: the instances lost or diverged on changes")
		os.Exit(1)
	}
	fmt.Println("PASS")
}

func parseClusterBenchArgs(args []string) (ClusterBenchConfig, error) {
	fs := flag.NewFlagSet("clusterbench", flag.ContinueOnError)
	hosts := fs.String("hosts", "http://127.0.0.1:9001", "comma-separated base URLs of the Etherpad instances")
	pad := fs.String("pad", "", "pad to edit (default: a new random pad)")
	authors := fs.Int("authors", 4, "number of writing clients, spread over the hosts")
	duration := fs.Duration("duration", 20*time.Second, "how long the authors write")
	interval := fs.Duration("interval", 200*time.Millisecond, "pause between two changes of one author")
	prefill := fs.Int("prefill", 0, "bytes of text to put into the pad before measuring")
	settle := fs.Duration("settle", 3*time.Second, "wait before comparing the instances")
	if err := fs.Parse(args); err != nil {
		return ClusterBenchConfig{}, err
	}
	var hs []string
	for _, h := range strings.Split(*hosts, ",") {
		if h = strings.TrimSpace(h); h != "" {
			hs = append(hs, h)
		}
	}
	return ClusterBenchConfig{
		Hosts: hs, Pad: *pad, Authors: *authors, Duration: *duration,
		Interval: *interval, PrefillBytes: *prefill, Settle: *settle,
	}, nil
}
