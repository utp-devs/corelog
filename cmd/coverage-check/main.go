package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func main() {
	threshold := flag.Float64("min", 40.0, "Minimum coverage percentage")
	profile := flag.String("profile", "coverage.out", "Path to coverage profile")
	flag.Parse()

	cmd := exec.Command("go", "tool", "cover", "-func", *profile)
	out, err := cmd.Output()
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to run go tool cover: %v\n", err)
		os.Exit(1)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	if len(lines) == 0 {
		fmt.Fprintln(os.Stderr, "no coverage output found")
		os.Exit(1)
	}

	lastLine := lines[len(lines)-1]
	fields := strings.Fields(lastLine)
	if len(fields) < 3 {
		fmt.Fprintf(os.Stderr, "unexpected total line format: %s\n", lastLine)
		os.Exit(1)
	}

	pctStr := strings.TrimSuffix(fields[len(fields)-1], "%")
	pct, err := strconv.ParseFloat(pctStr, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to parse coverage percentage: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Total coverage: %.1f%% (minimum required: %.1f%%)\n", pct, *threshold)
	if pct < *threshold {
		fmt.Fprintf(os.Stderr, "FAIL: coverage %.1f%% is below threshold %.1f%%\n", pct, *threshold)
		os.Exit(1)
	}
	fmt.Println("PASS: coverage threshold satisfied.")
}
