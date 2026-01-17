package loadtest

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"time"

	"go.uber.org/zap"
)

func StartMultiLoadTest(logger *zap.SugaredLogger, host string, maxPads int) {
	if maxPads <= 0 {
		maxPads = 10
	}

	fmt.Printf("Starting multi-pad load test: %d pads for 30 seconds each\n", maxPads)

	executable, err := os.Executable()
	if err != nil {
		logger.Errorf("Failed to get executable path: %v", err)
		os.Exit(1)
	}

	var wg sync.WaitGroup
	messageCount := 0 // Simplified as in JS

	for i := 0; i < maxPads; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			// Equivalent to: node app.js -a 3 -d 30
			// We use the same binary with 'loadtest' subcommand
			cmd := exec.Command(executable, "loadtest", host, "-a", "3", "-d", "30")
			cmd.Env = append(os.Environ(), "SILENT_METRICS=true")

			// In JS it uses fork, here we use exec.
			// We don't necessarily want all of them clearing the screen,
			// but the JS version would have them all writing to the same terminal too.
			// To keep it simple and similar to JS, we just run them.

			output, err := cmd.CombinedOutput()
			if err != nil {
				fmt.Printf("Child process %d exited with error: %v\n", id, err)
				fmt.Printf("Output: %s\n", string(output))
				fmt.Println("total pads made:", id) // Approximation
				fmt.Println("total messages", messageCount)
				os.Exit(1)
			}
		}(i)

		// Small delay between starts to not overwhelm everything at once
		time.Sleep(100 * time.Millisecond)
	}

	wg.Wait()
	fmt.Println("Multi-pad load test completed successfully")
}
